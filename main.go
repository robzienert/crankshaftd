// +build linux darwin

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/robzienert/crankshaftd/crankshaft"
)

var (
	// VERSION of crankshaftd
	VERSION    = "0.0.3-alpha"
	configFile = flag.String("config", "config.toml", "Configuration file")
	config     crankshaft.Config
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: crankshaft [opts]")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if _, err := toml.DecodeFile("config.toml", &config); err != nil {
		fmt.Println(err)
		return
	}

	if len(config.Host) == 0 {
		fmt.Fprintln(os.Stderr, "Error: Must specify a turbine host")
		usage()
	}

	if len(config.Clusters) == 0 {
		fmt.Fprintln(os.Stderr, "Error: Must specify at least one cluster")
		usage()
	}

	fmt.Println(strings.Repeat("#", 80))
	fmt.Println("Crankshaft ", VERSION)
	fmt.Println(strings.Repeat("#", 80) + "\n")

	crankshaft.MonitorClusters(config)
}
