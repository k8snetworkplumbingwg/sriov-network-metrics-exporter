//Package Collectors defines the structure of the collector aggregator and contains the individual collectors used to gather metrics
//Each collector should be created in its own file with any required command line flags, its collection behavior and its registration method defined.

package collectors

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	collectorNamespace = "sriov"
	enabled            = true
	disabled           = false
	enabledCollectors  = make(map[string]*bool)
	allCollectors      = make(map[string]func() prometheus.Collector)
)

//sriovCollector registers the collectors used for specific data and exposes a Collect method to gather the data
type sriovCollector []prometheus.Collector

//Collect metrics from all enabled collectors in unordered sequence.
func (s sriovCollector) Collect(ch chan<- prometheus.Metric) {
	for _, collector := range s {
		collector.Collect(ch)
	}
}

//Describe each collector in unordered sequence
func (s sriovCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, collector := range s {
		collector.Describe(ch)
	}
}

//register defines a flag for a collector and adds it to the registry of enabled collectors if the flag is set to true - either through the default option or the flag passed on start
//Run by each individual collector in its init function.
func register(name string, isDefault bool, collector func() prometheus.Collector) {
	enabledCollectors[name] = &isDefault
	allCollectors[name] = collector
	flag.BoolVar(enabledCollectors[name], "collector."+name, isDefault, fmt.Sprintf("Enables the %v collector", name))
}

//Enabled adds collectors enabled by default or command line flag to an sriovCollector object
func Enabled() sriovCollector {
	collectors := make([]prometheus.Collector, 0)
	for k, v := range enabledCollectors {
		if v != nil && *v {
			log.Printf("The %v collector is enabled", k)
			collectors = append(collectors, allCollectors[k]())
		}
	}
	return collectors
}

func isSymLink(filename string) bool {
	info, err := os.Lstat(filename)
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSymlink == os.ModeSymlink {
		return true
	}
	return false
}
