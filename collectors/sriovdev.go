package collectors

// sriovDev has the methods for implementing an sriov stats reader and publishing its information to Prometheus

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/k8snetworkplumbingwg/sriov-network-metrics-exporter/pkg/utils"
)

const (
	noNumaInfo = "-1"
)

var (
	collectorPriority    utils.StringListFlag
	defaultPriority            = utils.StringListFlag{"sysfs", "netlink"}
	sysBusPci                  = flag.String("path.sysbuspci", "/sys/bus/pci/devices", "Path to sys/bus/pci/devices/ on host")
	sysClassNet                = flag.String("path.sysclassnet", "/sys/class/net", "Path to sys/class/net/ on host")
	pfNameFile                 = "net"
	netClassFile               = "class"
	netClass             int64 = 0x020000
	vfStatsSubsystem           = "vf"
	vfStatsCollectorName       = "vfstats"

	devfs fs.FS
	netfs fs.FS
)

// vfsPCIAddr is a map of VF IDs to VF PCI addresses i.e. {"0": "0000:3b:02.0", "1": "0000:3b:02.1"}
type vfsPCIAddr map[string]string

// init runs the registration for this collector on package import
func init() {
	flag.Var(&collectorPriority, "collector.vfstatspriority", "Priority of collectors")
	register(vfStatsCollectorName, enabled, createSriovDevCollector)
}

// This is the generic collector for VF stats.
type sriovDevCollector struct {
	name            string
	pfsWithNumaInfo map[string]string
}

type sriovDev struct {
	name   string
	reader sriovStatReader
	vfs    vfsPCIAddr
}

// Collect runs the appropriate collector for each SR-IOV vf on the system and publishes its statistics.
func (c sriovDevCollector) Collect(ch chan<- prometheus.Metric) {
	log.Printf("collecting sr-iov device metrics")

	priority := collectorPriority
	if len(priority) == 0 {
		log.Printf("collector.priority not specified in flags, using default priority")
		priority = defaultPriority
	}

	log.Printf("collector priority: %s", priority)
	for pfAddr, numaNode := range c.pfsWithNumaInfo {
		pf := getSriovDev(pfAddr, priority)

		if pf.reader == nil {
			continue
		}

		for id, address := range pf.vfs {
			stats := pf.reader.ReadStats(pf.name, id)
			for name, v := range stats {
				desc := prometheus.NewDesc(
					prometheus.BuildFQName(collectorNamespace, vfStatsSubsystem, name),
					fmt.Sprintf("Statistic %s.", name),
					[]string{"pf", "vf", "pciAddr", "numa_node"}, nil,
				)

				ch <- prometheus.MustNewConstMetric(
					desc,
					prometheus.CounterValue,
					float64(v),
					pf.name,
					id,
					address,
					numaNode,
				)
			}
		}
	}
}

// Describe isn't implemented for this collector
func (c sriovDevCollector) Describe(ch chan<- *prometheus.Desc) {
}

// sriovDevCollector is initialized with the physical functions on the host. This is not updated after initialization.
func createSriovDevCollector() prometheus.Collector {
	devs := getSriovDevAddrs()
	numaNodes := getNumaNodes(devs)

	return sriovDevCollector{
		name:            vfStatsCollectorName,
		pfsWithNumaInfo: numaNodes,
	}
}

// getSriovDevAddrs returns the PCI addresses of the SRIOV capable Physical Functions on the host.
func getSriovDevAddrs() []string {
	sriovDevs := make([]string, 0)

	devs, err := fs.Glob(devfs, "*/sriov_totalvfs")
	if err != nil {
		log.Printf("Invalid pattern\n%v", err) // unreachable code
	}

	if len(devs) == 0 {
		log.Printf("no sriov net devices found")
	}

	for _, dev := range devs {
		devAddr := filepath.Dir(dev)
		if isNetDevice(filepath.Join(devAddr, netClassFile)) {
			sriovDevs = append(sriovDevs, devAddr)
		}
	}

	return sriovDevs
}

// getSriovDev returns a sriovDev record containing the physical function interface name, stats reader and initialized virtual functions.
func getSriovDev(pfAddr string, priority []string) sriovDev {
	name := getPFName(pfAddr)
	vfs, err := vfList(pfAddr)
	if err != nil {
		log.Printf("error getting vf address\n%v", err)
	}

	reader, err := getStatsReader(name, priority)
	if err != nil {
		log.Printf("error getting stats reader for %s: %v", name, err)
	}

	return sriovDev{
		name,
		reader,
		vfs,
	}
}

// getNumaNodes returns the numa location for each of the PFs with SR-IOV capabilities
func getNumaNodes(devs []string) map[string]string {
	pfNumaInfo := make(map[string]string)

	for _, dev := range devs {
		numaFilepath := filepath.Join(dev, "numa_node")
		numaRaw, err := fs.ReadFile(devfs, numaFilepath)
		if err != nil {
			log.Printf("could not read numa_node file for device '%s'\n%v", dev, err)
			pfNumaInfo[dev] = ""
			continue
		}

		numaNode := strings.TrimSpace(string(numaRaw))
		if numaNode == noNumaInfo {
			log.Printf("no numa node information for device '%s'", dev)
			pfNumaInfo[dev] = ""
			continue
		}

		pfNumaInfo[dev] = numaNode
	}

	return pfNumaInfo
}

// vfList returns the virtual functions associated with the specified SRIOV physical function
func vfList(dev string) (vfsPCIAddr, error) {
	vfList := make(vfsPCIAddr, 0)

	vfs, err := fs.Glob(devfs, filepath.Join(dev, "virtfn*"))
	if err != nil {
		log.Printf("Invalid pattern\n%v", err) // unreachable code
	}

	// Read all VF directories and add VF PCI addr to the vfList
	for _, vf := range vfs {
		if id, link := vfData(vf); id != "" && link != "" {
			vfList[id] = link
		}
	}

	if len(vfList) == 0 {
		return vfList, fmt.Errorf("no virtual functions found for pf '%s'", dev)
	}

	return vfList, nil
}

// vfData gets vf id and pci address from the path specified
func vfData(vfDir string) (string, string) {
	if link, err := utils.EvalSymlinks(filepath.Join(*sysBusPci, vfDir)); err == nil {
		return filepath.Base(vfDir)[6:], filepath.Base(link)
	} else {
		log.Printf("error evaluating symlink '%s'\n%v", vfDir, err)
		return "", ""
	}
}

// getPFName resolves the system's name for a physical interface from the PCI address linked to it.
func getPFName(device string) string {
	pfDevPath := filepath.Join(device, pfNameFile)
	pfdir, err := fs.ReadDir(devfs, pfDevPath)
	if err != nil || len(pfdir) == 0 {
		log.Printf("%s - could not get pf interface name in path '%s'\n%v", device, pfDevPath, err)
		return ""
	}

	return pfdir[0].Name()
}

// isNetDevice checks if the device is a net device by checking its device class
func isNetDevice(filepath string) bool {
	file, err := fs.ReadFile(devfs, filepath)
	if err != nil {
		return false
	}

	classHex := strings.TrimSpace(string(file))
	deviceClass, err := strconv.ParseInt(classHex, 0, 64)
	if err != nil {
		log.Printf("could not parse class file: %v", err)
		return false
	}

	return deviceClass == netClass
}

func resolveSriovDevFilepaths() error {
	if err := utils.ResolveFlag("path.sysbuspci", sysBusPci); err != nil {
		return err
	}

	if err := utils.ResolveFlag("path.sysclassnet", sysClassNet); err != nil {
		return err
	}

	devfs = os.DirFS(*sysBusPci)
	netfs = os.DirFS(*sysClassNet)

	return nil
}
