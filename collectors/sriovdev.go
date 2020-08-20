//sriovDev has the methods for implementing an sriov stats reader and publishing its information to Prometheus
package collectors

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	noNumaInfo = "-1"
)

var (
	sysBusPci             = flag.String("path.sysbuspci", "/sys/bus/pci/devices", "Path to sys/bus/pci on host")
	sysClassNet           = flag.String("path.sysclassnet", "/sys/class/net/", "Path to sys/class/net on host")
	totalVfFile           = "sriov_totalvfs"
	pfNameFile            = "/net"
	netClassFile          = "/class"
	driverFile            = "/driver"
	netClass        int64 = 0x020000
	vfStatSubsystem       = "vf"
	sriovDev              = "vfstats"
	sriovPFs              = make([]string, 0)
)

//vfList contains a list of addresses for VFs with the name of the physical interface as value
type vfWithRoot map[string]string

//init runs the registration for this collector on package import
func init() {
	register(sriovDev, enabled, createSriovdevCollector)
}

//this is the generic collector for VF stats.
type sriovdevCollector struct {
	name       string
	pfWithNuma map[string]string
}

//SriovdevCollector is initialized with the physical functions on the host. This is not updated after initialization.
func createSriovdevCollector() prometheus.Collector {
	pfList, err := getSriovPFs()
	numaList := sriovNumaNodes(pfList)
	if err != nil {
		log.Fatal(err)
	}
	return sriovdevCollector{
		name:       sriovDev,
		pfWithNuma: numaList,
	}
}

//sriovNumaNodes returns the numa location for each of the PFs with SR-IOV capabilities
func sriovNumaNodes(pfList []string) map[string]string {
	numaList := make(map[string]string)
	for _, pf := range pfList {
		path := filepath.Join(*sysBusPci, pf, "numa_node")
		if isSymLink(path) {
			log.Printf("error: cannot read symlink %v", path)
			continue
		}
		numaRaw, err := ioutil.ReadFile(path)
		if err != nil {
			log.Printf("Could not read numa node for card at %v", path)
			numaList[pf] = ""
		}
		numaAsString := strings.TrimSpace(string(numaRaw))
		if err == nil && numaAsString != noNumaInfo {
			numaList[pf] = numaAsString
		} else {
			log.Printf("Could not read numa node for card at %v", path)
			numaList[pf] = ""
		}
	}
	return numaList
}

//Collect runs the appropriate collector for each SR-IOV vf on the system and publishes its statistics.
func (c sriovdevCollector) Collect(ch chan<- prometheus.Metric) {
	for pfAddr, numaNode := range c.pfWithNuma {
		pfName := getPFName(pfAddr)
		if pfName == "" {
			continue
		}
		// appropriate reader for VF is returned based on the PF.
		// TODO: This could be cached per PF.
		reader := statReaderForPF(pfName)
		if reader == nil {
			continue
		}
		vfs, err := vfList(pfAddr)
		if err != nil {
			continue
		}
		for id, address := range vfs {
			stats := reader.ReadStats(pfName, id)
			for name, v := range stats {
				desc := prometheus.NewDesc(
					prometheus.BuildFQName(collectorNamespace, vfStatSubsystem, name),
					fmt.Sprintf("Statistic %s.", name),
					[]string{"pf", "vf", "pciAddr", "numa_node"}, nil,
				)
				ch <- prometheus.MustNewConstMetric(
					desc,
					prometheus.CounterValue,
					v,
					pfName,
					id,
					address,
					numaNode,
				)
			}
		}
	}
}

//getSriovPFs returns the SRIOV capable Physical Network functions for the host.
func getSriovPFs() ([]string, error) {
	devs := getPCIDevs()
	if len(devs) == 0 {
		return sriovPFs, errors.New("pci devices could not be found")
	}
	for _, device := range devs {
		if isSriovNetPF(device.Name()) {
			sriovPFs = append(sriovPFs, device.Name())
		}
	}
	if len(sriovPFs) == 0 {
		return sriovPFs, errors.New("no sriov net devices found on host")
	}
	return sriovPFs, nil
}

//isSriovNetPF checks if is device SRIOV capable net device. It checks if the sriov_totalvfs file exists for the given PCI address
func isSriovNetPF(pciAddr string) bool {
	totalVfFilePath := filepath.Join(*sysBusPci, pciAddr, totalVfFile)
	devClassFilePath := filepath.Join(*sysBusPci, pciAddr, netClassFile)
	if !isNetDevice(devClassFilePath) {
		return false
	}
	if _, err := os.Stat(totalVfFilePath); err != nil {
		return false
	}
	return true
}

//isNetDevice checks if the device is a net device by checking its device class
func isNetDevice(filepath string) bool {
	if isSymLink(filepath) {
		log.Printf("error: cannot read symlink %v", filepath)
		return false
	}
	file, err := ioutil.ReadFile(filepath)
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

//getPCIDevs returns all of the PCI device files available on the host
func getPCIDevs() []os.FileInfo {
	links, err := ioutil.ReadDir(*sysBusPci)
	if err != nil {
		return make([]os.FileInfo, 0)
	}
	return links
}

//vfList returns the Virtual Functions associated with a specific SRIOV Physical Function
func vfList(pfAddress string) (vfWithRoot, error) {
	vfList := make(vfWithRoot, 0)
	pfDir := filepath.Join(*sysBusPci, pfAddress)
	_, err := os.Lstat(pfDir)
	if err != nil {
		err = fmt.Errorf("could not get PF directory information for device: %s, Err: %v", pfAddress, err)
		return vfList, err
	}
	vfDirs, err := filepath.Glob(filepath.Join(pfDir, "virtfn*"))
	if err != nil {
		err = fmt.Errorf("error reading VF directories %v", err)
		return vfList, err
	}
	//Read all VF directory and get add VF PCI addr to the vfList
	for _, dir := range vfDirs {
		dirInfo, err := os.Lstat(dir)
		if err == nil && (dirInfo.Mode()&os.ModeSymlink != 0) {
			linkName, err := filepath.EvalSymlinks(dir)
			if err == nil {
				vfLink := filepath.Base(linkName)
				vfID := dirInfo.Name()[6:]
				vfList[vfID] = vfLink
			}
		}
	}

	return vfList, nil
}

//getPFName resolves the system's name for a physical interface from the PCI address linked to it.
func getPFName(device string) string {
	pfdir, err := ioutil.ReadDir(filepath.Join(*sysBusPci, device, pfNameFile))
	if err != nil || len(pfdir) == 0 {
		log.Printf("Could not get name for pf: %v", err)
		return ""
	}
	return pfdir[0].Name()
}

//Describe isn't implemented for this collector
func (c sriovdevCollector) Describe(chan<- *prometheus.Desc) {
}
