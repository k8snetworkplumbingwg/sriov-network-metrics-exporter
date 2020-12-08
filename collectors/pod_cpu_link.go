package collectors
//kubepodCPUCollector is a Kubernetes focused collector that exposes information about CPUs linked to specific Kubernetes pods through the CPU Manager component in Kubelet

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	kubepodcpu        = "kubepodcpu"
	kubePodCgroupPath = flag.String("path.kubecgroup", "/sys/fs/cgroup/cpuset/kubepods/", "Path for location of kubernetes cgroups on the host system")
	sysDevSysNodePath = flag.String("path.nodecpuinfo", "/sys/devices/system/node/", "Path for location of system cpu information")
	cpuCheckPointFile = flag.String("path.cpucheckpoint", "/var/lib/kubelet/cpu_manager_state", "Path for location of cpu manager checkpoint file")
)

//init runs the registration for this collector on package import
func init() {
	register(kubepodcpu, disabled, createKubepodCPUCollector)
}

//cpuManagerCheckpoint is the structure needed to extract the default cpuSet information from the kubelet checkpoint file
type cpuManagerCheckpoint struct {
	DefaultCPUSet string "json:\"defaultCpuSet\""
}

//readDefaultSet extracts the information about the "default" set of cpus available to kubernetes
func readDefaultSet() string {
	if isSymLink(*cpuCheckPointFile) {
		log.Printf("error: cannot read symlink %v", *cpuCheckPointFile)
		return ""
	}
	cpuRaw, err := ioutil.ReadFile(*cpuCheckPointFile)
	if err != nil {
		log.Printf("cpu checkpoint file can not be read: %v", err)
		return ""
	}
	checkpointFile := cpuManagerCheckpoint{}
	err = json.Unmarshal(cpuRaw, &checkpointFile)
	if err != nil {
		log.Printf("cpu checkpoint file can not be read: %v", err)
		return ""
	}
	return checkpointFile.DefaultCPUSet
}

//kubepodCPUCollector holds a static representation of node cpu topology and uses it to update information about kubernetes pod cpu usage.
type kubepodCPUCollector struct {
	cpuInfo  map[string]string
	kubeCPUs []string
	name     string
}

//podCPULink contains the information about the pod and container a single cpu is attached to
type podCPULink struct {
	podID       string
	containerID string
	cpu         string
}

//createKubepodCPUCollector creates a static picture of the cpu topology of the system and returns a collector
//It also creates a static list of cpus in the kubernetes parent cgroup.
func createKubepodCPUCollector() prometheus.Collector {
	cpuInfo, err := getCPUInfo()
	if err != nil {
		//Exporter will fail here if file can not be read.
		log.Fatal("Fatal Error:"+"CPU Info for node can not be collected", err)
	}
	return kubepodCPUCollector{
		cpuInfo: cpuInfo,
		name:    kubepodcpu,
	}
}

//getKubernetesCPUList returns the information about the CPUs being used by Kubernetes overall
func getKubernetesCPUList() (string, error) {
	kubeCPUString, err := parseCPUFile(filepath.Join(*kubePodCgroupPath, "cpuset.cpus"))
	if err != nil {
		return "", err
	}
	return kubeCPUString, nil
}

//guaranteedPodCPUs creates a podCPULink for each CPU that is guaranteed
//This information is exposed under the cpuset in the cgroup file system with Kubernetes1.18/Docker/
//This accounting will create an entry for each guaranteed pod, even if that pod isn't managed by CPU manager
//i.e. it will still create an entry if the pod is looking for millis of CPU
//Todo: validate regex matching and evaluate performance of this approach
//Todo: validate assumptions about directory structure against other runtimes and kubelet config. Plausibly problematic with CgroupsPerQos and other possible future cgroup changes
func (c kubepodCPUCollector) guaranteedPodCPUs() ([]podCPULink, error) {
	//This generate method should be updated to create the non-exclusive cores mask used by guaranteed pods with fractional core usage as well as maximum Kubernetes usage.
	kubeCPUString, err := getKubernetesCPUList()
	if err != nil {
		//Exporter killed here as CPU collector can not work without this information.
		log.Fatal("Fatal error: Cannot get information on Kubernetes CPU usage", err)
	}
	defaultSet := readDefaultSet()
	if err != nil {
		//Exporter killed here as CPU collector can not work without this information.
		log.Fatal("Fatal error: Cannot get information on Kubernetes CPU usage", err)
	}
	links := make([]podCPULink, 0)
	files, err := ioutil.ReadDir(*kubePodCgroupPath)
	if err != nil {
		return links, fmt.Errorf("could not open path kubePod cgroups")
	}
	for _, f := range files {
		if f.IsDir() {
			//Searching for files matching uuid pattern of 8-4-4-4-12
			if matches, err := regexp.MatchString("pod[0-9a-f]{8}-[0-9a-f]*", f.Name()); matches {
				if err != nil {
					return links, fmt.Errorf("could not open cpu files: %v", err)
				}
				if len(f.Name()) < 4 {
					continue
				}
				podUID := f.Name()[3:]
				containerFiles, err := ioutil.ReadDir(filepath.Join(*kubePodCgroupPath, f.Name()))
				if err != nil {
					return links, fmt.Errorf("could not open cpu files: %v", err)
				}
				for _, cpusetFile := range containerFiles {
					if matches, err := regexp.MatchString("[a-z0-9]{20,}", cpusetFile.Name()); matches {
						if err != nil {
							return links, fmt.Errorf("could not open cpu files: %v: %v", cpusetFile.Name(), err)
						}
						cpuSetDesc, err := parseCPUFile(filepath.Join(*kubePodCgroupPath, f.Name(), cpusetFile.Name(), "cpuset.cpus"))
						if err != nil {
							return links, err
						}
						cpuList, err := parseCPURange(cpuSetDesc)
						if err != nil {
							return links, err
						}
						if cpuSetDesc == kubeCPUString || cpuSetDesc == defaultSet {
							continue
						}
						for _, c := range cpuList {
							links = append(links, podCPULink{podID: podUID, containerID: cpusetFile.Name(), cpu: c})
						}
					}
				}
			}
		}
	}
	return links, nil
}

//parseCPUFile can read cpuFiles in the Kernel cpuset format
func parseCPUFile(path string) (string, error) {
	if isSymLink(path) {
		return "", fmt.Errorf("cpuset file is a symlink. Can not read")
	}
	cpuRaw, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("could not open cgroup cpuset files: %v", err)
	}
	cpuString := strings.TrimSpace(string(cpuRaw))
	return cpuString, err
}

//parseCPURanges can read cpuFiles in the Kernel cpuset format
func parseCPURange(cpuString string) ([]string, error) {
	cpuList := make([]string, 0)
	cpuRanges := strings.Split(cpuString, ",")
	for _, r := range cpuRanges {
		endpoints := strings.Split(r, "-")
		if len(endpoints) == 1 {
			cpuList = append(cpuList, endpoints[0])
		} else if len(endpoints) == 2 {
			start, err := strconv.Atoi(endpoints[0])
			if err != nil {
				return cpuList, err
			}
			end, err := strconv.Atoi(endpoints[1])
			if err != nil {
				return cpuList, err
			}
			for e := start; e <= end; e++ {
				cpuList = append(cpuList, strconv.Itoa(e))
			}
		}
	}
	return cpuList, nil
}

//Collect publishes the cpu information and all kubernetes pod cpu information to the prometheus channel
//On each run it reads the guaranteed pod cpus and exposes the pod, container, and NUMA IDs to the collector
func (c kubepodCPUCollector) Collect(ch chan<- prometheus.Metric) {
	//This exposes the basic cpu alignment to prometheus.
	for cpu, numa := range c.cpuInfo {
		cpuID := "cpu" + cpu
		desc := prometheus.NewDesc(
			prometheus.BuildFQName(collectorNamespace, "", "cpu_info"),
			c.name,
			[]string{"cpu", "numa_node"}, nil,
		)
		ch <- prometheus.MustNewConstMetric(
			desc,
			prometheus.CounterValue,
			1,
			cpuID,
			numa,
		)
	}
	podCPULinks, err := c.guaranteedPodCPUs()
	if err != nil {
		log.Printf("pod cpu links not available: %v", err)
		return
	}
	for _, link := range podCPULinks {
		desc := prometheus.NewDesc(
			prometheus.BuildFQName(collectorNamespace, "", c.name),
			"pod_cpu",
			[]string{"cpu_id", "numa_node", "uid", "container_id"}, nil,
		)
		ch <- prometheus.MustNewConstMetric(
			desc,
			prometheus.CounterValue,
			1,
			link.cpu,
			c.cpuInfo[link.cpu],
			link.podID,
			link.containerID,
		)
	}
}

//getCPUInfo looks in the sys directory for information on CPU IDs and NUMA topology. This method runs once on initialization of the pod.
func getCPUInfo() (map[string]string, error) {
	cpuInfo := make(map[string]string, 0)
	files, err := ioutil.ReadDir(*sysDevSysNodePath)
	if err != nil {
		return cpuInfo, fmt.Errorf("could not open path to cpu info")
	}
	for _, f := range files {
		if f.IsDir() {
			if matches, err := regexp.MatchString("node[0-9]+", f.Name()); matches {
				if err != nil {
					log.Printf("CPU file not found %v", err)
				}
				if len(f.Name()) < 5 {
					continue
				}
				numaNode := f.Name()[4:]
				cpuFiles, err := ioutil.ReadDir(filepath.Join(*sysDevSysNodePath, f.Name()))
				if err != nil {
					return cpuInfo, fmt.Errorf("could not open cpu files: %v", err)
				}
				for _, cpu := range cpuFiles {
					if matches, _ := regexp.MatchString("cpu[0-9]+", cpu.Name()); matches {
						if len(cpu.Name()) < 4 {
							continue
						}
						cpuID := cpu.Name()[3:]
						cpuInfo[cpuID] = numaNode
					}
				}
			}
		}
	}
	return cpuInfo, nil
}

//Describe is not defined for this collector
func (c kubepodCPUCollector) Describe(ch chan<- *prometheus.Desc) {

}
