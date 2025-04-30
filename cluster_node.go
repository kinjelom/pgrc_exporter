package main

import (
	"fmt"
	"net"
	"strconv"
)

type Node struct {
	address string
	host    string
	port    int
	db      *DataSource
}

type NodeState struct {
	address                string
	err                    error
	isInRecovery           bool
	currentWalLsn          string
	currentWalLsnBytes     uint64
	lastWalReceiveLsn      string
	lastWalReceiveLsnBytes uint64
	lastWalReplayLsn       string
	lastWalReplayLsnBytes  uint64
}

// NewNode creates a new Node by parsing the address in "host:port" format
// and validating its components. Returns an error on bad input.
func NewNode(db *DataSource, address string) (*Node, error) {
	// split into host and port (supports IPv6 in [addr]:port notation)
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid node address format, expected host:port, got %q: %w", address, err)
	}
	// convert port string to integer
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("node port is not a valid integer: %q: %w", portStr, err)
	}
	// check port range
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("node port out of range (1–65535): %d", port)
	}
	// host must not be empty
	if host == "" {
		return nil, fmt.Errorf("node host cannot be empty")
	}
	return &Node{
		address: address,
		host:    host,
		port:    port,
		db:      db,
	}, nil
}

func (n *Node) queryIsInRecovery() (bool, error) {
	var isInRecoveryStr string
	var err error
	if isInRecoveryStr, err = n.db.QueryStrWithEffort(n.address, n.host, n.port, "SELECT pg_is_in_recovery()::TEXT"); err != nil {
		return false, fmt.Errorf("failed to query recovery mode: %v", err)
	}
	return isInRecoveryStr == "true", nil
}

func (n *Node) queryForState() *NodeState {
	var state = &NodeState{}
	state.address = n.address
	state.isInRecovery, state.err = n.queryIsInRecovery()
	if state.err == nil {
		// https://www.postgresql.org/docs/current/functions-admin.html
		if state.isInRecovery {
			// SLAVE
			if state.lastWalReceiveLsn, state.err = n.db.QueryStrWithEffort(n.address, n.host, n.port, "SELECT COALESCE(pg_last_wal_receive_lsn(),'0/0')"); state.err == nil {
				state.lastWalReceiveLsnBytes, state.err = parsePgLsn(state.lastWalReceiveLsn)
			} else {
				state.err = fmt.Errorf("failed to query last received wal location: %v", state.err)
			}
			if state.lastWalReplayLsn, state.err = n.db.QueryStrWithEffort(n.address, n.host, n.port, "SELECT COALESCE(pg_last_wal_replay_lsn(),'0/0')"); state.err == nil {
				state.lastWalReplayLsnBytes, state.err = parsePgLsn(state.lastWalReplayLsn)
			} else {
				state.err = fmt.Errorf("failed to query last replayed wal location: %v", state.err)
			}
		} else {
			// MASTER
			if state.currentWalLsn, state.err = n.db.QueryStrWithEffort(n.address, n.host, n.port, "SELECT COALESCE(pg_current_wal_lsn(),'0/0')"); state.err == nil {
				state.currentWalLsnBytes, state.err = parsePgLsn(state.currentWalLsn)
			} else {
				state.err = fmt.Errorf("failed to query current wal location: %v", state.err)
			}
		}
	}
	return state
}
