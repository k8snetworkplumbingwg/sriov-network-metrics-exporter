package collectors

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

var _ = DescribeTable("test pod cpu link collection", // Collect
	func(fsys fs.FS, expected []metric, logs ...string) {
		cpuinfofs = fsys
		kubecgroupfs = fsys
		cpucheckpointfs = fsys

		ch := make(chan prometheus.Metric, 1)
		go createKubepodCPUCollector().Collect(ch)

		for i := 0; i < len(expected); i++ {
			m := dto.Metric{}
			err := (<-ch).Write(&m)
			Expect(err).ToNot(HaveOccurred())

			labels := make(map[string]string, 4)
			for _, label := range m.Label {
				labels[*label.Name] = *label.Value
			}

			metric := metric{labels: labels, counter: *m.Counter.Value}

			Expect(metric).To(BeElementOf(expected))
		}

		assertLogs(logs)
	},
	Entry("test numa node and cpuset collection",
		fstest.MapFS{
			"node0/cpu0":        {Mode: fs.ModeDir},
			"node0/cpu2":        {Mode: fs.ModeDir},
			"node1/cpu1":        {Mode: fs.ModeDir},
			"node1/cpu3":        {Mode: fs.ModeDir},
			"cpuset.cpus":       {Data: []byte("4-7")},
			"cpu_manager_state": {Data: []byte("{\"policyName\":\"none\",\"defaultCpuSet\":\"4-7\",\"checksum\":1353318690}")},
			"kubepods-pod6b5b533a_6307_48d1_911f_07bf5d4e1c82.slice/0123456789abcdefaaaa/cpuset.cpus": {Data: []byte("0-3")}},
		[]metric{
			{map[string]string{"cpu": "cpu0", "numa_node": "0"}, 1},
			{map[string]string{"cpu": "cpu2", "numa_node": "0"}, 1},
			{map[string]string{"cpu": "cpu1", "numa_node": "1"}, 1},
			{map[string]string{"cpu": "cpu3", "numa_node": "1"}, 1},
			{map[string]string{
				"cpu_id": "0", "numa_node": "0",
				"uid": "6b5b533a_6307_48d1_911f_07bf5d4e1c82", "container_id": "0123456789abcdefaaaa",
			}, 1},
			{map[string]string{
				"cpu_id": "2", "numa_node": "0",
				"uid": "6b5b533a_6307_48d1_911f_07bf5d4e1c82", "container_id": "0123456789abcdefaaaa",
			}, 1},
			{map[string]string{
				"cpu_id": "1", "numa_node": "1",
				"uid": "6b5b533a_6307_48d1_911f_07bf5d4e1c82", "container_id": "0123456789abcdefaaaa",
			}, 1},
			{map[string]string{
				"cpu_id": "3", "numa_node": "1",
				"uid": "6b5b533a_6307_48d1_911f_07bf5d4e1c82", "container_id": "0123456789abcdefaaaa",
			}, 1}}),
	Entry("test unavailable kube cgroup directory",
		fstest.MapFS{
			"node0/cpu0":        {Mode: fs.ModeDir},
			"node0/cpu2":        {Mode: fs.ModeDir},
			"node1/cpu1":        {Mode: fs.ModeDir},
			"node1/cpu3":        {Mode: fs.ModeDir},
			"cpuset.cpus":       {Data: []byte("4-7")},
			"cpu_manager_state": {Data: []byte("{\"policyName\":\"none\",\"defaultCpuSet\":\"4-7\",\"checksum\":1353318690}")},
			"kubepods-pod6b5b533a_6307_48d1_911f_07bf5d4e1c83.slice": {Mode: fs.ModeExclusive}},
		[]metric{
			{map[string]string{"cpu": "cpu0", "numa_node": "0"}, 1},
			{map[string]string{"cpu": "cpu2", "numa_node": "0"}, 1},
			{map[string]string{"cpu": "cpu1", "numa_node": "1"}, 1},
			{map[string]string{"cpu": "cpu3", "numa_node": "1"}, 1}},
		"pod cpu links not available: could not read cpu files directory: "+
			"readdir kubepods-pod6b5b533a_6307_48d1_911f_07bf5d4e1c83.slice: not implemented"),
)

var _ = DescribeTable("test reading default cpu set", // readDefaultSet
	func(data []byte, expected string, logs ...string) {
		Expect(readDefaultSet(data)).To(Equal(expected))

		assertLogs(logs)
	},
	Entry("read empty",
		[]byte("{\"policyName\":\"none\",\"defaultCpuSet\":\"\",\"checksum\":1353318690}"),
		""),
	Entry("read successful",
		[]byte("{\"policyName\":\"none\",\"defaultCpuSet\":\"1,2,3,4\",\"checksum\":1353318690}"),
		"1,2,3,4"),
	Entry("read failed with malformed data",
		[]byte("\"policyName\":\"none\",\"checksum\":1353318690"),
		"",
		"cpu checkpoint file could not be unmarshalled, error: invalid character ':' after top-level value"),
)

var _ = DescribeTable("test creating kubepodCPU collector", // createKubepodCPUCollector
	func(fsys fs.FS, expectedCollector kubepodCPUCollector, logs ...string) {
		cpuinfofs = fsys

		collector := createKubepodCPUCollector()
		Expect(collector).To(Equal(expectedCollector))

		assertLogs(logs)
	},
	Entry("successful creation",
		fstest.MapFS{
			"node0/cpu0": {Mode: fs.ModeDir},
			"node0/cpu2": {Mode: fs.ModeDir},
			"node1/cpu1": {Mode: fs.ModeDir},
			"node1/cpu3": {Mode: fs.ModeDir}},
		kubepodCPUCollector{cpuInfo: map[string]string{"0": "0", "2": "0", "1": "1", "3": "1"}, name: kubepodcpu}),
	Entry("directory doesn't exist",
		fstest.MapFS{".": {Mode: fs.ModeExclusive}}, // to emulate the directory doesn't exist
		kubepodCPUCollector{cpuInfo: map[string]string{}, name: kubepodcpu},
		"Fatal Error: cpu info for node can not be collected, failed to read directory '/sys/devices/system/node/'\nreaddir .: not implemented"),
)

var _ = DescribeTable("test getting kubernetes cpu list", // getKubeDefaults
	func(fsys fs.FS, expectedKubeCPUString, expectedDefaultSet string) {
		kubecgroupfs = fsys
		cpucheckpointfs = fsys

		kubeCPUString, kubeDefaultSet := getKubeDefaults()
		Expect(kubeCPUString).To(Equal(expectedKubeCPUString))
		Expect(kubeDefaultSet).To(Equal(expectedDefaultSet))
	},
	Entry("read empty",
		fstest.MapFS{
			"cpuset.cpus":       {Data: []byte("")},
			"cpu_manager_state": {Data: []byte("")}},
		"",
		""),
	Entry("read successful",
		fstest.MapFS{
			"cpuset.cpus":       {Data: []byte("0-87")},
			"cpu_manager_state": {Data: []byte("{\"policyName\":\"static\",\"defaultCpuSet\":\"0-63\",\"checksum\":1058907510}")}},
		"0-87",
		"0-63"),
	Entry("read successful with malformed data",
		fstest.MapFS{
			"cpuset.cpus":       {Data: []byte(" 0-87  ")},
			"cpu_manager_state": {Data: []byte("{\"policyName\":\"static\",\"defaultCpuSet\":\"0-63\",\"checksum\":1058 907 51 0}")}},
		"0-87",
		""),
	Entry("read failed, file doesn't exist",
		fstest.MapFS{},
		"",
		""),
)

var _ = DescribeTable("test getting guaranteed pod cpus", // guaranteedPodCPUs
	func(fsys fs.FS, expected []podCPULink, expectedErr error, logs ...string) {
		kubecgroupfs = fsys
		cpucheckpointfs = fsys

		data, err := getGuaranteedPodCPUs()
		Expect(data).To(Equal(expected))

		if expectedErr != nil {
			Expect(err).To(MatchError(expectedErr))
		}

		assertLogs(logs)
	},
	Entry("container cpuset available",
		fstest.MapFS{
			"cpuset.cpus":       {Data: []byte("0-3")},
			"cpu_manager_state": {Data: []byte("{\"policyName\":\"none\",\"defaultCpuSet\":\"4-7\",\"checksum\":1353318690}")},
			"kubepods-pod6b5b533a_6307_48d1_911f_07bf5d4e1c82.slice/0123456789abcdefaaaa/cpuset.cpus": {Data: []byte("8-11")}},
		[]podCPULink{
			{"6b5b533a_6307_48d1_911f_07bf5d4e1c82", "0123456789abcdefaaaa", "8"},
			{"6b5b533a_6307_48d1_911f_07bf5d4e1c82", "0123456789abcdefaaaa", "9"},
			{"6b5b533a_6307_48d1_911f_07bf5d4e1c82", "0123456789abcdefaaaa", "10"},
			{"6b5b533a_6307_48d1_911f_07bf5d4e1c82", "0123456789abcdefaaaa", "11"}},
		nil),
	Entry("cgroup directory doesn't exist",
		fstest.MapFS{".": {Mode: fs.ModeExclusive}},
		[]podCPULink{},
		fmt.Errorf("could not open path kubePod cgroups: readdir .: not implemented"),
		"cannot get information on Kubernetes CPU usage, could not open cgroup cpuset files, error: open cpuset.cpus: file does not exist",
		"unable to read cpu checkpoint file '/var/lib/kubelet/cpu_manager_state', error: open cpu_manager_state: file does not exist",
		"cpu checkpoint file could not be unmarshalled, error: unexpected end of JSON input"),
	Entry("unable to read pod cgroup directory",
		fstest.MapFS{
			"cpuset.cpus":       {Data: []byte("0-3")},
			"cpu_manager_state": {Data: []byte("{\"policyName\":\"none\",\"defaultCpuSet\":\"4-7\",\"checksum\":1353318690}")},
			"kubepods-pod6b5b533a_6307_48d1_911f_07bf5d4e1c82.slice": {Mode: fs.ModeExclusive}},
		[]podCPULink{},
		fmt.Errorf("could not read cpu files directory: readdir kubepods-pod6b5b533a_6307_48d1_911f_07bf5d4e1c82.slice: not implemented")),
	Entry("unable to read container cpuset file",
		fstest.MapFS{
			"cpuset.cpus":       {Data: []byte("0-3")},
			"cpu_manager_state": {Data: []byte("{\"policyName\":\"none\",\"defaultCpuSet\":\"4-7\",\"checksum\":1353318690}")},
			"kubepods-pod6b5b533a_6307_48d1_911f_07bf5d4e1c82.slice/0123456789abcdefaaaa/cpuset.cpus": {Mode: fs.ModeDir}},
		[]podCPULink{},
		fmt.Errorf("could not open cgroup cpuset files, error: "+
			"read kubepods-pod6b5b533a_6307_48d1_911f_07bf5d4e1c82.slice/0123456789abcdefaaaa/cpuset.cpus: invalid argument")),
	Entry("container cpuset range covered by defaults",
		fstest.MapFS{
			"cpuset.cpus":       {Data: []byte("0-3")},
			"cpu_manager_state": {Data: []byte("{\"policyName\":\"none\",\"defaultCpuSet\":\"4-7\",\"checksum\":1353318690}")},
			"kubepods-pod6b5b533a_6307_48d1_911f_07bf5d4e1c82.slice/0123456789abcdefaaaa/cpuset.cpus": {Data: []byte("0-3")}},
		[]podCPULink{},
		nil),
)

var _ = DescribeTable("test parsing cpu file", // parseCPUFile
	func(path string, fsys fs.FS, expectedString string, expectedErr error) {
		kubecgroupfs = fsys

		data, err := readCPUSet(path)
		Expect(data).To(Equal(expectedString))

		if expectedErr != nil {
			Expect(err).To(Equal(expectedErr))
		}
	},
	Entry("read empty",
		"cpuset.cpus",
		fstest.MapFS{
			"cpuset.cpus": {Data: []byte("")}},
		"",
		nil),
	Entry("read successful",
		"cpuset.cpus",
		fstest.MapFS{
			"cpuset.cpus": {Data: []byte("0-87")}},
		"0-87",
		nil),
	Entry("read successful with malformed data",
		"cpuset.cpus",
		fstest.MapFS{
			"cpuset.cpus": {Data: []byte(" 0-87  ")}},
		"0-87",
		nil),
	Entry("read failed, file doesn't exist",
		"cpuset.cpus",
		fstest.MapFS{},
		"",
		fmt.Errorf("could not open cgroup cpuset files, error: open cpuset.cpus: file does not exist")),
)

var _ = DescribeTable("test parsing cpu range", // parseCPURange
	func(cpuString string, expected []string, expectedErr error) {
		data, err := parseCPURange(cpuString)
		Expect(data).To(Equal(expected))

		if expectedErr != nil {
			Expect(err).To(MatchError(expectedErr))
		}
	},
	Entry("valid range '0-3,7-9'",
		"0-3,7-9",
		[]string{"0", "1", "2", "3", "7", "8", "9"},
		nil),
	Entry("valid range '0-3'",
		"0-3",
		[]string{"0", "1", "2", "3"},
		nil),
	Entry("valid range '7'",
		"7",
		[]string{"7"},
		nil),
	Entry("invalid range '-1'",
		"-1",
		[]string{},
		strconv.ErrSyntax),
	Entry("invalid range '0-'",
		"0-",
		[]string{},
		strconv.ErrSyntax),
)

var _ = DescribeTable("test getting cpu info", // getCPUInfo
	func(fsys fs.FS, expectedData map[string]string, expectedErr error) {
		cpuinfofs = fsys

		data, err := getCPUInfo()

		for k, v := range expectedData {
			Expect(data).To(HaveKey(k))
			Expect(data[k]).To(Equal(v))
		}

		Expect(data).To(Equal(expectedData))

		if expectedErr != nil {
			Expect(err).To(MatchError(expectedErr))
		}
	},
	Entry("valid info",
		fstest.MapFS{
			"node0/cpu0": {Mode: fs.ModeDir},
			"node0/cpu2": {Mode: fs.ModeDir},
			"node1/cpu1": {Mode: fs.ModeDir},
			"node1/cpu3": {Mode: fs.ModeDir}},
		map[string]string{"0": "0", "2": "0", "1": "1", "3": "1"},
		nil),
	Entry("directory doesn't exist",
		fstest.MapFS{".": {Mode: fs.ModeExclusive}}, // to emulate the directory doesn't exist
		map[string]string{},
		errors.New("failed to read directory '/sys/devices/system/node/'\nreaddir .: not implemented")),
)

// TODO: create integration tests for GetV1Client and PodResources, they require the kubelet API
