package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"strconv"
)

const (
	namespace           = "pgrc" // postgres
	programNameLabel    = "name"
	programVersionLabel = "version"
	clusterNameLabel    = "cluster_name"
	inRecoveryLabel     = "in_rec"
	successLabel        = "success"
	nodeLabel           = "node"
	masterNodeLabel     = "master_node"
	queryLabel          = "query"
)

type Measurer struct {
	clusterName            string
	buildInfo              *prometheus.GaugeVec
	nodeInfo               *prometheus.GaugeVec
	pingSeconds            *prometheus.GaugeVec
	reconnectsCountTotal   *prometheus.CounterVec
	queriesCountTotal      *prometheus.CounterVec
	lastQuerySeconds       *prometheus.GaugeVec
	currentWalLsnBytes     *prometheus.GaugeVec
	lastWalReceiveLsnBytes *prometheus.GaugeVec
	lastWalReplayLsnBytes  *prometheus.GaugeVec
	receiveLagBytes        *prometheus.GaugeVec
	replayLagBytes         *prometheus.GaugeVec
}

func NewMeasurer(clusterName string) *Measurer {
	return &Measurer{
		clusterName: clusterName,

		buildInfo: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "build_info",
			Help:      "Program build info",
		}, []string{programNameLabel, programVersionLabel, clusterNameLabel}),

		nodeInfo: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "cluster_node_info",
			Help:      "Cluster node info",
		}, []string{clusterNameLabel, nodeLabel, inRecoveryLabel}),

		reconnectsCountTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "reconnects_count_total",
			Help:      "Cluster node reconnects total count",
		}, []string{clusterNameLabel, nodeLabel}),

		queriesCountTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "queries_count_total",
			Help:      "All queries total count",
		}, []string{clusterNameLabel, nodeLabel, queryLabel, successLabel}),

		lastQuerySeconds: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_query_seconds",
			Help:      "Cluster node last query seconds",
		}, []string{clusterNameLabel, nodeLabel, queryLabel}),

		currentWalLsnBytes: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "current_wal_lsn_bytes",
			Help:      "The current write-ahead log write location: SELECT pg_current_wal_lsn()",
		}, []string{clusterNameLabel, nodeLabel, inRecoveryLabel}),

		lastWalReceiveLsnBytes: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_wal_receive_lsn_bytes",
			Help:      "The last write-ahead log location that has been received and synced to disk by streaming replication: SELECT pg_last_wal_receive_lsn()",
		}, []string{clusterNameLabel, nodeLabel, inRecoveryLabel}),

		lastWalReplayLsnBytes: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_wal_replay_lsn_bytes",
			Help:      "The last write-ahead log location that has been replayed during recovery: SELECT pg_last_wal_replay_lsn()",
		}, []string{clusterNameLabel, nodeLabel, inRecoveryLabel}),

		receiveLagBytes: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "receive_lag_bytes",
			Help:      "Cluster node receive lag bytes: pg_current_wal_lsn() - pg_last_wal_receive_lsn()",
		}, []string{clusterNameLabel, nodeLabel, inRecoveryLabel, masterNodeLabel}),

		replayLagBytes: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "replay_lag_bytes",
			Help:      "Cluster node replay lag bytes: pg_last_wal_receive_lsn() - pg_last_wal_reply_lsn()",
		}, []string{clusterNameLabel, nodeLabel, inRecoveryLabel, masterNodeLabel}),
	}
}

func (m *Measurer) updateRuntimeInfo(name, version, clusterName string) {
	m.buildInfo.With(prometheus.Labels{programNameLabel: name, programVersionLabel: version, clusterNameLabel: clusterName}).Set(0)
}

func (m *Measurer) updateQueryStats(node string, q string, milliseconds int64, success bool) {
	if success {
		sec := float64(milliseconds) / float64(1000)
		m.lastQuerySeconds.With(prometheus.Labels{clusterNameLabel: m.clusterName, nodeLabel: node, queryLabel: q}).Set(sec)
	}
	m.queriesCountTotal.With(prometheus.Labels{clusterNameLabel: m.clusterName, nodeLabel: node, queryLabel: q, successLabel: strconv.FormatBool(success)}).Inc()
}

func (m *Measurer) updateClusterState(masterState *NodeState, slaveStates *map[string]*NodeState) {
	m.nodeInfo.With(prometheus.Labels{clusterNameLabel: m.clusterName, nodeLabel: masterState.address, inRecoveryLabel: strconv.FormatBool(false)}).Set(0)
	m.currentWalLsnBytes.With(prometheus.Labels{clusterNameLabel: m.clusterName, nodeLabel: masterState.address, inRecoveryLabel: strconv.FormatBool(false)}).Set(float64(masterState.currentWalLsnBytes))
	for node, slaveState := range *slaveStates {
		m.nodeInfo.With(prometheus.Labels{clusterNameLabel: m.clusterName, nodeLabel: node, inRecoveryLabel: strconv.FormatBool(true)}).Set(0)
		m.lastWalReceiveLsnBytes.With(prometheus.Labels{clusterNameLabel: m.clusterName, nodeLabel: node, inRecoveryLabel: strconv.FormatBool(true)}).Set(float64(slaveState.lastWalReceiveLsnBytes))
		m.lastWalReplayLsnBytes.With(prometheus.Labels{clusterNameLabel: m.clusterName, nodeLabel: node, inRecoveryLabel: strconv.FormatBool(true)}).Set(float64(slaveState.lastWalReplayLsnBytes))
	}
}

func (m *Measurer) updateSlaveLag(masterState *NodeState, slaveState *NodeState, lag *SlaveLag) {
	m.receiveLagBytes.With(prometheus.Labels{clusterNameLabel: m.clusterName, nodeLabel: slaveState.address, masterNodeLabel: masterState.address, inRecoveryLabel: strconv.FormatBool(true)}).Set(float64(lag.receiveLag))
	m.replayLagBytes.With(prometheus.Labels{clusterNameLabel: m.clusterName, nodeLabel: slaveState.address, masterNodeLabel: masterState.address, inRecoveryLabel: strconv.FormatBool(true)}).Set(float64(lag.replayLag))
}

func (m *Measurer) incReconnects(host string) {
	m.reconnectsCountTotal.With(prometheus.Labels{clusterNameLabel: m.clusterName, nodeLabel: host}).Inc()
}
