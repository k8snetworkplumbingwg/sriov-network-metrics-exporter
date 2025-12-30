// Package collectors defines the structure of the collector aggregator and contains the individual collectors
// used to gather metrics.
// Each collector should be created in its own file with any required command line flags, its collection
// behavior and its registration method defined.

package collectors

import (
	"flag"
	"fmt"
	"log"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	collectorNamespace = "sriov"
	enabled            = true
	disabled           = false
	collectorState     = make(map[string]*bool)
	collectorFunctions = make(map[string]func() prometheus.Collector)
)

// SriovCollector registers the collectors used for specific data and exposes a Collect method to gather the data
type SriovCollector []prometheus.Collector

// Register defines a flag for a collector and adds it to the registry of enabled collectors
// if the flag is set to true - either through the default option or the flag passed on start.
// Run by each individual collector in its init function.
func register(name string, enabled bool, collector func() prometheus.Collector) {
	collectorState[name] = &enabled
	collectorFunctions[name] = collector
	flag.BoolVar(collectorState[name], "collector."+name, enabled, fmt.Sprintf("Enables the %v collector", name))
}

// Collect metrics from all enabled collectors in unordered sequence.
func (s SriovCollector) Collect(ch chan<- prometheus.Metric) {
	for _, collector := range s {
		collector.Collect(ch)
	}
}

// Describe each collector in unordered sequence
func (s SriovCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, collector := range s {
		collector.Describe(ch)
	}
}

// Enabled adds collectors enabled by default or command line flag to an SriovCollector object
func Enabled() SriovCollector {
	collectors := make([]prometheus.Collector, 0)
	for collector, enabled := range collectorState {
		if enabled != nil && *enabled {
			log.Printf("The %v collector is enabled", collector)
			collectors = append(collectors, collectorFunctions[collector]())
		}
	}
	return collectors
}

func ResolveFilepaths() error {
	resolveFuncs := []func() error{
		resolveSriovDevFilepaths,
		resolveKubePodCPUFilepaths,
		resolveKubePodDeviceFilepaths,
	}

	for _, resolveFunc := range resolveFuncs {
		if err := resolveFunc(); err != nil {
			return err
		}
	}

	return nil
}

var logFatal = func(msg string, args ...any) {
	log.Fatalf(msg, args...)
}
