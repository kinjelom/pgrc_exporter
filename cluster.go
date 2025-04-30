package main

import (
	"fmt"
)

type Cluster struct {
	name       string
	nodes      map[string]*Node
	dataSource *DataSource
}

type SlaveLag struct {
	receiveLag uint64
	replayLag  uint64
}

func NewCluster(dataSource *DataSource, clusterName string, nodes []string) (*Cluster, error) {
	cluster := &Cluster{}
	cluster.name = clusterName
	cluster.dataSource = dataSource
	cluster.nodes = make(map[string]*Node)
	var err error
	for _, node := range nodes {
		cluster.nodes[node], err = NewNode(cluster.dataSource, node)
		if err != nil {
			return nil, err
		}
	}
	return cluster, nil
}

func (cluster *Cluster) queryForState() (*NodeState, *map[string]*NodeState, error) {
	var master = &NodeState{}
	var slaves = make(map[string]*NodeState)
	mi := 0
	si := 0
	for nodeAddr, nodeStruct := range cluster.nodes {
		nodeState := nodeStruct.queryForState()
		if nodeState.err == nil {
			if nodeState.isInRecovery {
				slaves[nodeAddr] = nodeState
				si++
			} else {
				if mi == 0 {
					master = nodeState
					mi++
				} else {
					return nil, nil, fmt.Errorf("too many masters, konwn %s, pretending: %s", master.address, nodeStruct.address)
				}
			}
		}
	}
	if mi == 0 || si == 0 {
		return master, &slaves, fmt.Errorf("this is not replication cluster, masters: %d, slaves: %d", mi, si)
	}
	return master, &slaves, nil
}

func (cluster *Cluster) calculateSlaveLag(master NodeState, slave NodeState) *SlaveLag {
	lag := &SlaveLag{receiveLag: 0, replayLag: 0}
	if master.currentWalLsnBytes > slave.lastWalReceiveLsnBytes {
		lag.receiveLag = master.currentWalLsnBytes - slave.lastWalReceiveLsnBytes
	} else {
		lag.receiveLag = 0
	}
	if slave.lastWalReceiveLsnBytes > slave.lastWalReplayLsnBytes {
		lag.replayLag = slave.lastWalReceiveLsnBytes - slave.lastWalReplayLsnBytes
	} else {
		lag.replayLag = 0
	}
	log.debug("calculate lag between master slave %s:\n"+
		"  master.currentWalLsn    = %d (%s)\n"+
		"  slave.lastWalReceiveLsn = %d (%s)\n"+
		"  slave.lastWalReplayLsn  = %d (%s)\n"+
		"  slave.receiveLag        = %d\n"+
		"  slave.replayLag         = %d",
		slave.address,
		master.currentWalLsnBytes, master.currentWalLsn,
		slave.lastWalReceiveLsnBytes, slave.lastWalReceiveLsn,
		slave.lastWalReplayLsnBytes, slave.lastWalReplayLsn,
		lag.receiveLag, lag.replayLag)
	return lag
}
