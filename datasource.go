package main

import (
	"database/sql"
	"fmt"
	"time"
)

type DataSource struct {
	measurer   *Measurer
	driverName string
	dbname     string
	user       string
	password   string
	sslMode    string
	connection map[string]*sql.DB
}

func NewDataSource(measurer *Measurer, user, password string) *DataSource {
	return &DataSource{
		measurer:   measurer,
		driverName: "postgres",
		dbname:     "postgres",
		user:       user,
		password:   password,
		sslMode:    "disable",
		connection: make(map[string]*sql.DB),
	}
}

func (db *DataSource) connect(node, host string, port int, force bool) (*sql.DB, error) {
	var err error
	if db.connection[node] == nil || force {
		cs := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s", host, port, db.dbname, db.user, db.password, db.sslMode)
		db.connection[node], err = sql.Open(db.driverName, cs)
		if err != nil {
			db.connection[node] = nil
			log.warn("Can't connect to %s, error: %v", host, err)
			return nil, err
		}
	}
	return db.connection[node], nil
}

func (db *DataSource) reconnect(node, host string, port int) (*sql.DB, error) {
	db.measurer.incReconnects(node)
	var err error
	_, err = db.connect(node, host, port, true)
	if err != nil {
		db.connection[node] = nil
		log.warn("Can't connect %s, error: %v", host, err)
		return nil, err
	} else {
		err = db.connection[node].Ping()
		if err != nil {
			db.connection[node] = nil
			log.warn("Can't ping %s, error: %v", host, err)
			return nil, err
		}
	}
	return db.connection[node], nil
}

func (db *DataSource) Ping(node string) (int64, error) {
	start := time.Now()
	if db.connection[node] == nil {
		return -1, fmt.Errorf("node %s is disconnected", node)
	}
	err := db.connection[node].Ping()
	return time.Since(start).Milliseconds(), err
}

func (db *DataSource) QueryStrWithEffort(node, host string, port int, q string) (string, error) {
	log.debug("query: `%s`", q)
	var err error
	_, err = db.connect(node, host, port, false)
	if err != nil {
		log.warn("Can't connect %s, error: %v", host, err)
		return "", err
	}
	var v string
	v, err = db.queryStr(node, q)
	if err != nil {
		_, err = db.reconnect(node, host, port)
		if err != nil {
			log.warn("Can't connect %s, error: %v", host, err)
			return "", err
		} else {
			v, err = db.queryStr(node, q)
		}
	}
	log.debug("query result: `%s`", v)
	return v, err
}

func (db *DataSource) queryStr(node, q string) (string, error) {
	start := time.Now()
	row := db.connection[node].QueryRow(q)
	var v string
	if err := row.Scan(&v); err != nil {
		db.measurer.updateQueryStats(node, q, time.Since(start).Milliseconds(), false)
		return "", err
	}
	db.measurer.updateQueryStats(node, q, time.Since(start).Milliseconds(), true)
	return v, nil
}
