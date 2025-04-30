package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/madflojo/tasks"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/voxelbrain/goptions"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	ProgramFullName              = "Postgres Replication Cluster Exporter"
	ProgramName                  = "pgrc_exporter"
	WrongParamsExitCode          = 1
	HttpServerFailureExitCode    = 2
	TaskSchedulerFailureExitCode = 3
)

var ProgramVersion = "dev"

func clusterHash(nodes []string) string {
	sort.Strings(nodes)
	hash := md5.Sum([]byte(strings.Join(nodes, ",")))
	return "cluster-" + hex.EncodeToString(hash[:3])
}

func main() {
	options := struct {
		Address     string   `goptions:"-A, --address, description='Address to listens on the TCP network'"`
		Path        string   `goptions:"-P, --path, description='Path under which to expose metrics'"`
		ClusterName string   `goptions:"-C, --cluster-name, description='Cluster name'"`
		Nodes       []string `goptions:"-n, --node, description='Address of a replication cluster node in the format host:port; can be specified multiple times.'"`
		User        string   `goptions:"-u, --user, description='User to connect as'"`
		Password    string   `goptions:"-s, --password, description='Password to connect with'"`
		Interval    int64    `goptions:"-i, --interval, description='Collecting metrics interval in seconds'"`
		Verbosity   int      `goptions:"-V, --verbosity, description='Verbosity level (0 errors, 1 +warnings, 2 +infos, 3 +debugs)'"`
		Version     bool     `goptions:"-v, --version, description='Output version information, then exit'"`
		Help        bool     `goptions:"-h, --help, description='Show this help, then exit'"`
	}{
		Address:   ":9188",
		Path:      "/metrics",
		Interval:  15,
		Verbosity: 2,
	}
	goptions.ParseAndFail(&options)
	if options.Help {
		fmt.Printf("%s - %s v%s\n", ProgramFullName, ProgramName, ProgramVersion)
		goptions.PrintHelp()
		os.Exit(0)
	}
	if options.Version {
		fmt.Printf("%s\n", ProgramVersion)
		os.Exit(0)
	}
	interval := options.Interval
	log.Verbosity = options.Verbosity

	if options.User == "" || options.Password == "" {
		log.error("User and password are mandatory, exit.")
		os.Exit(WrongParamsExitCode)
	}

	if len(options.Nodes) < 2 {
		log.error("Nodes count is less than 2, exit.")
		os.Exit(WrongParamsExitCode)
	}
	clusterName := options.ClusterName
	if len(clusterName) < 1 {
		clusterName = clusterHash(options.Nodes)
	}

	scheduler := tasks.New()
	defer scheduler.Stop()

	// manual injections framework ;)
	var measurer = NewMeasurer(clusterName)
	var dataSource = NewDataSource(measurer, options.User, options.Password)
	cluster, err := NewCluster(dataSource, clusterName, options.Nodes)
	if err != nil {
		log.error("FAILED to parse nodes: %v", err)
		os.Exit(WrongParamsExitCode)
	}
	// Add a task
	_, schedulerErr := scheduler.Add(&tasks.Task{
		Interval: time.Duration(interval) * time.Second,
		TaskFunc: func() error {
			measurer.updateRuntimeInfo(ProgramFullName, ProgramVersion, clusterName)
			masterState, slaveStates, collectErr := cluster.queryForState()
			if collectErr == nil {
				log.debug("master %s current wal LSN %d (%s)", masterState.address, masterState.currentWalLsnBytes, masterState.currentWalLsn)
				measurer.updateClusterState(masterState, slaveStates)
				for _, slaveState := range *slaveStates {
					slaveLag := cluster.calculateSlaveLag(*masterState, *slaveState)
					measurer.updateSlaveLag(masterState, slaveState, slaveLag)
					log.debug("slave %s receive lag %d, replay lag %d", slaveState.address, slaveLag.receiveLag, slaveLag.replayLag)
				}
			} else {
				log.error("collecting cluster data error: %v", collectErr)
			}
			return nil
		},
	})
	if schedulerErr != nil {
		log.error("FAILED to schedule task: %v", schedulerErr)
		os.Exit(TaskSchedulerFailureExitCode)
	}

	log.info("Started %s%s, scraping cluster %s every %d seconds. PID: %d", options.Address, options.Path, clusterName, interval, os.Getpid())
	http.Handle(options.Path, promhttp.Handler())
	httpServerErr := http.ListenAndServe(options.Address, nil)
	if httpServerErr != nil {
		log.error("FAILED to start http server: %v", httpServerErr)
		os.Exit(HttpServerFailureExitCode)
	}
}
