# Postgres Replication Cluster Exporter

This repository houses `pgrc_exporter`, a small utility that will connect to all the nodes in a Postgres replication cluster and exports replication lag metrics of each node (Prometheus format).

Inspired by: https://github.com/Qarik-Group/pg-replication-tester - thanks :+1:

## Metrics

- **pgrc_build_info**: Program build info
- **pgrc_cluster_node_info**: Cluster node info
- **pgrc_reconnects_count_total**: Cluster node reconnects total count
- **pgrc_queries_count_total**: All queries total count
- **pgrc_last_query_seconds**: Cluster node last query seconds
- **pgrc_current_wal_lsn_bytes**: The current write-ahead log write location - `SELECT pg_current_wal_lsn()`
- **pgrc_last_wal_receive_lsn_bytes**: The last write-ahead log location that has been received and synced to disk by streaming replication - `SELECT pg_last_wal_receive_lsn()`
- **pgrc_last_wal_replay_lsn_bytes**: The last write-ahead log location that has been replayed during recovery - `SELECT pg_last_wal_replay_lsn()`
- **pgrc_receive_lag_bytes**: Cluster node receive lag bytes: `pg_current_wal_lsn() - pg_last_wal_receive_lsn()`
- **pgrc_replay_lag_bytes**: Cluster node replay lag bytes: `pg_last_wal_receive_lsn() - pg_last_wal_reply_lsn()`

## Options

```
-A, --address, Address to listens on the TCP network. Default: :9188
-P, --path, Path under which to expose metrics. Default: /metrics
-C, --cluster-name, Cluster name. Default: cluster-hash(nodes)
-n, --node, Address of a replication cluster node in the format host:port; can be specified multiple times.
-u, --user, User to connect as.
-s, --password, Password to connect with.
-i, --interval, Collecting metrics interval in seconds. Default: 15 
-V, --verbosity, Verbosity level (0 errors, 1 +warnings, 2 +infos, 3 +debugs). Default: 2 
-v, --version, Output version information, then exit.
-h, --help, Show this help, then exit.
```

## Build

```bash
GOOS=linux GOARCH=amd64 go build -o pgrc_exporter-linux-amd64
```

## TODO

https://www.postgresql.org/docs/17/view-pg-replication-slots.html
