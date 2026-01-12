package collectors

// kubepodCPUCollector is a Kubernetes focused collector that exposes information about CPUs
// linked to specific Kubernetes pods through the CPU Manager component in Kubelet

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/k8snetworkplumbingwg/sriov-network-metrics-exporter/pkg/utils"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	kubepodcpu        = "kubepodcpu"
	kubePodCgroupPath = flag.String("path.kubecgroup",
		"/sys/fs/cgroup/cpuset/kubepods.slice/", "Path for kubernetes cgroups")
	sysDevSysNodePath = flag.String("path.nodecpuinfo", "/sys/devices/system/node/", "Path for location of system cpu information")
	cpuCheckPointFile = flag.String("path.cpucheckpoint",
		"/var/lib/kubelet/cpu_manager_state", "Path for cpu manager checkpoint file")

	kubecgroupfs    fs.FS
	cpuinfofs       fs.FS
	cpucheckpointfs fs.FS
)

// kubepodCPUCollector holds a static representation of node cpu topology and uses it to update information about kubernetes pod cpu usage.
type kubepodCPUCollector struct {
	cpuInfo map[string]string
	name    string
}

// podCPULink contains the information about the pod and container a single cpu is attached to
type podCPULink struct {
	podID       string
	containerID string
	cpu         string
}

// cpuManagerCheckpoint is the structure needed to extract the default cpuSet information from the kubelet checkpoint file
type cpuManagerCheckpoint struct {
	DefaultCPUSet string "json:\"defaultCpuSet\""
}

// init runs the registration for this collector on package import
func init() {
	register(kubepodcpu, disabled, createKubepodCPUCollector)
}

// Collect publishes the cpu information and all kubernetes pod cpu information to the prometheus channel
// On each run it reads the guaranteed pod cpus and exposes the pod, container, and NUMA IDs to the collector
func (c kubepodCPUCollector) Collect(ch chan<- prometheus.Metric) {
	// This exposes the basic cpu alignment to prometheus.
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

	links, err := getGuaranteedPodCPUs()
	if err != nil {
		log.Printf("pod cpu links not available: %v", err)
		return
	}

	for _, link := range links {
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

// Describe is not defined for this collector
func (c kubepodCPUCollector) Describe(ch chan<- *prometheus.Desc) {
}

// createKubepodCPUCollector creates a static picture of the cpu topology of the system and returns a collector
// It also creates a static list of cpus in the kubernetes parent cgroup.
func createKubepodCPUCollector() prometheus.Collector {
	cpuInfo, err := getCPUInfo()
	if err != nil {
		// Exporter will fail here if file can not be read.
		logFatal("Fatal Error: cpu info for node can not be collected, %v", err.Error())
	}

	return kubepodCPUCollector{
		cpuInfo: cpuInfo,
		name:    kubepodcpu,
	}
}

// getCPUInfo looks in the sys directory for information on CPU IDs and NUMA topology. This method runs once on initialization of the pod.
func getCPUInfo() (map[string]string, error) {
	cpuInfo := make(map[string]string, 0)
	files, err := fs.ReadDir(cpuinfofs, ".")
	if err != nil {
		return cpuInfo, fmt.Errorf("failed to read directory '%s'\n%v", *sysDevSysNodePath, err)
	}

	fileRE := regexp.MustCompile(`node\d+`)
	cpuFileRE := regexp.MustCompile(`cpu\d+`)
	for _, f := range files {
		if f.IsDir() {
			if fileRE.MatchString(f.Name()) {
				numaNode := f.Name()[4:]
				cpuFiles, err := fs.ReadDir(cpuinfofs, f.Name())
				if err != nil {
					return cpuInfo, fmt.Errorf("failed to read directory '%s'\n%v", filepath.Join(*sysDevSysNodePath, numaNode), err)
				}

				for _, cpu := range cpuFiles {
					if cpuFileRE.MatchString(cpu.Name()) {
						cpuID := cpu.Name()[3:]
						cpuInfo[cpuID] = numaNode
					}
				}
			}
		}
	}
	return cpuInfo, nil
}

// getGuaranteedPodCPUs  creates a podCPULink for each CPU that is guaranteed
// This information is exposed under the cpuset in the cgroup file system with Kubernetes1.18/Docker/
// This accounting will create an entry for each guaranteed pod, even if that pod isn't managed by CPU manager
// i.e. it will still create an entry if the pod is looking for millis of CPU
// Todo: validate regex matching and evaluate performance of this approach
// Todo: validate assumptions about directory structure against other runtimes and kubelet config.
// Plausibly problematic with CgroupsPerQos and other possible future cgroup changes
func getGuaranteedPodCPUs() ([]podCPULink, error) {
	links := make([]podCPULink, 0)

	kubeCPUString, kubeDefaultSet := getKubeDefaults()

	podDirectoryFilenames, err := getPodDirectories()
	if err != nil {
		return links, err
	}

	for _, directory := range podDirectoryFilenames {
		containerIDs, err := getContainerIDs(directory)
		if err != nil {
			return links, err
		}

		for _, container := range containerIDs {
			cpuSet, err := readCPUSet(filepath.Join(directory, container, "cpuset.cpus"))
			if err != nil {
				return links, err
			}
			if cpuSet == kubeCPUString || cpuSet == kubeDefaultSet {
				continue
			}

			cpuRange, err := parseCPURange(cpuSet)
			if err != nil {
				return links, err
			}

			for _, link := range cpuRange {
				links = append(links, podCPULink{directory[12 : len(directory)-6], container, link})
			}
		}
	}
	return links, nil
}

func getPodDirectories() ([]string, error) {
	podDirectoryFilenames := make([]string, 0)

	files, err := fs.ReadDir(kubecgroupfs, ".") // all files in the directory
	if err != nil {
		return podDirectoryFilenames, fmt.Errorf("could not open path kubePod cgroups: %v", err)
	}

	podDirectoryRegex := regexp.MustCompile("pod[[:xdigit:]]{8}[_-][[:xdigit:]]{4}[_-][[:xdigit:]]{4}[_-][[:xdigit:]]{4}[_-][[:xdigit:]]{12}")
	for _, podDirectory := range files {
		podDirectoryFilename := podDirectory.Name()
		if match := podDirectoryRegex.MatchString(podDirectoryFilename); match {
			podDirectoryFilenames = append(podDirectoryFilenames, podDirectoryFilename)
		}
	}
	return podDirectoryFilenames, nil
}

func getContainerIDs(podDirectoryFilename string) ([]string, error) {
	containerDirectoryFilenames := make([]string, 0)

	files, err := fs.ReadDir(kubecgroupfs, podDirectoryFilename)
	if err != nil {
		return containerDirectoryFilenames, fmt.Errorf("could not read cpu files directory: %v", err)
	}

	containerIDRegex := regexp.MustCompile("[[:xdigit:]]{20,}") // change regexback
	for _, containerDirectory := range files {
		containerID := containerDirectory.Name()
		if match := containerIDRegex.MatchString(containerID); match {
			containerDirectoryFilenames = append(containerDirectoryFilenames, containerID)
		}
	}

	return containerDirectoryFilenames, nil
}

// readDefaultSet extracts the information about the "default" set of cpus available to kubernetes
func readDefaultSet(data []byte) string {
	checkpointFile := cpuManagerCheckpoint{}

	if err := json.Unmarshal(data, &checkpointFile); err != nil {
		log.Printf("cpu checkpoint file could not be unmarshalled, error: %v", err)
		return ""
	}

	return checkpointFile.DefaultCPUSet
}

// readCPUSet can read cpuFiles in the Kernel cpuset format
func readCPUSet(cpuSetFilepath string) (string, error) {
	cpuSetBytes, err := fs.ReadFile(kubecgroupfs, cpuSetFilepath)
	if err != nil {
		return "", fmt.Errorf("could not open cgroup cpuset files, error: %v", err)
	}
	return strings.TrimSpace(string(cpuSetBytes)), err
}

// parseCPURanges can read cpuFiles in the Kernel cpuset format
func parseCPURange(cpuString string) ([]string, error) {
	cpuList := make([]string, 0)
	cpuRanges := strings.Split(cpuString, ",")
	for _, r := range cpuRanges {
		endpoints := strings.Split(r, "-")
		if len(endpoints) == 1 {
			cpuList = append(cpuList, endpoints[0])
		} else if len(endpoints) == 2 { //nolint:mnd
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

func getKubeDefaults() (string, string) {
	kubeCPUString, err := readCPUSet("cpuset.cpus")
	if err != nil {
		// Exporter killed here as CPU collector can not work without this information.
		logFatal("Fatal Error: cannot get information on Kubernetes CPU usage, %v", err.Error())
	}

	cpuRawBytes, err := fs.ReadFile(cpucheckpointfs, filepath.Base(*cpuCheckPointFile))
	if err != nil {
		log.Printf("unable to read cpu checkpoint file '%s', error: %v", *cpuCheckPointFile, err)
	}

	return kubeCPUString, readDefaultSet(cpuRawBytes)
}

func resolveKubePodCPUFilepaths() error {
	if err := utils.ResolveFlag("path.kubecgroup", kubePodCgroupPath); err != nil {
		return err
	}

	if err := utils.ResolveFlag("path.nodecpuinfo", sysDevSysNodePath); err != nil {
		return err
	}

	if err := utils.ResolveFlag("path.cpucheckpoint", cpuCheckPointFile); err != nil {
		return err
	}

	kubecgroupfs = os.DirFS(*kubePodCgroupPath)
	cpuinfofs = os.DirFS(*sysDevSysNodePath)
	cpucheckpointfs = os.DirFS(filepath.Dir(*cpuCheckPointFile))

	return nil
}
