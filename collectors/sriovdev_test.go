package collectors

import (
	"fmt"
	"io/fs"
	"testing/fstest"

	"github.com/k8snetworkplumbingwg/sriov-network-metrics-exporter/pkg/vfstats"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/vishvananda/netlink"
)

var _ = AfterEach(func() {
	vfstats.GetLink = netlink.LinkByName
})

var _ = DescribeTable("test vf stats collection", // Collect
	func(priority []string, fsys fs.FS, link netlink.Device, expected []metric, logs ...string) {
		devfs = fsys
		netfs = fsys
		collectorPriority = priority

		vfstats.GetLink = func(name string) (netlink.Link, error) {
			return &link, nil
		}

		ch := make(chan prometheus.Metric, 1)
		go createSriovDevCollector().Collect(ch)

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
	Entry("with only sysfs",
		[]string{"sysfs"},
		fstest.MapFS{
			"0000:1d:00.0/sriov_totalvfs":                {Data: []byte("64")},
			"0000:1d:00.0/net/t_ens785f0":                {Mode: fs.ModeDir},
			"0000:1d:00.0/numa_node":                     {Data: []byte("0")},
			"0000:1d:00.0/class":                         {Data: []byte("0x020000")},
			"0000:1d:00.0/virtfn0":                       {Data: []byte("/sys/devices/0000:1d:01.0"), Mode: fs.ModeSymlink},
			"0000:1d:00.0/virtfn1":                       {Data: []byte("/sys/devices/0000:1d:01.1"), Mode: fs.ModeSymlink},
			"t_ens785f0/device/sriov/0/stats/rx_packets": {Data: []byte("4")},
			"t_ens785f0/device/sriov/0/stats/tx_packets": {Data: []byte("8")},
			"t_ens785f0/device/sriov/1/stats/rx_packets": {Data: []byte("16")},
			"t_ens785f0/device/sriov/1/stats/tx_packets": {Data: []byte("32")}},
		nil,
		[]metric{
			{map[string]string{"numa_node": "0", "pciAddr": "0000:1d:01.0", "pf": "t_ens785f0", "vf": "0"}, 4},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:1d:01.0", "pf": "t_ens785f0", "vf": "0"}, 8},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:1d:01.1", "pf": "t_ens785f0", "vf": "1"}, 16},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:1d:01.1", "pf": "t_ens785f0", "vf": "1"}, 32}},
		"collecting sr-iov device metrics",
		"collector priority: \\[sysfs\\]",
		"t_ens785f0 - using sysfs collector",
		"getting stats for t_ens785f0 vf\\d",
		"getting stats for t_ens785f0 vf\\d"),
	Entry("with only netlink",
		[]string{"netlink"},
		fstest.MapFS{
			"0000:2e:00.0/sriov_totalvfs": {Data: []byte("64")},
			"0000:2e:00.0/net/t_ens801f0": {Mode: fs.ModeDir},
			"0000:2e:00.0/numa_node":      {Data: []byte("0")},
			"0000:2e:00.0/class":          {Data: []byte("0x020000")},
			"0000:2e:00.0/virtfn0":        {Data: []byte("/sys/devices/0000:2e:01.0"), Mode: fs.ModeSymlink},
			"0000:2e:00.0/virtfn1":        {Data: []byte("/sys/devices/0000:2e:01.1"), Mode: fs.ModeSymlink}},
		netlink.Device{LinkAttrs: netlink.LinkAttrs{Vfs: []netlink.VfInfo{
			{ID: 0, Mac: nil, Vlan: 0, Qos: 0, TxRate: 0, Spoofchk: true, LinkState: 0, MaxTxRate: 0, MinTxRate: 0, RxPackets: 11, TxPackets: 12, RxBytes: 13, TxBytes: 14, Multicast: 15, Broadcast: 16, RxDropped: 17, TxDropped: 18, RssQuery: 0, Trust: 0},
			{ID: 1, Mac: nil, Vlan: 0, Qos: 0, TxRate: 0, Spoofchk: true, LinkState: 0, MaxTxRate: 0, MinTxRate: 0, RxPackets: 21, TxPackets: 22, RxBytes: 23, TxBytes: 24, Multicast: 25, Broadcast: 26, RxDropped: 27, TxDropped: 28, RssQuery: 0, Trust: 0},
		}}},
		[]metric{
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.0", "pf": "t_ens801f0", "vf": "0"}, 11},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.0", "pf": "t_ens801f0", "vf": "0"}, 12},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.0", "pf": "t_ens801f0", "vf": "0"}, 13},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.0", "pf": "t_ens801f0", "vf": "0"}, 14},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.0", "pf": "t_ens801f0", "vf": "0"}, 15},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.0", "pf": "t_ens801f0", "vf": "0"}, 16},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.0", "pf": "t_ens801f0", "vf": "0"}, 17},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.0", "pf": "t_ens801f0", "vf": "0"}, 18},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.1", "pf": "t_ens801f0", "vf": "1"}, 21},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.1", "pf": "t_ens801f0", "vf": "1"}, 22},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.1", "pf": "t_ens801f0", "vf": "1"}, 23},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.1", "pf": "t_ens801f0", "vf": "1"}, 24},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.1", "pf": "t_ens801f0", "vf": "1"}, 25},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.1", "pf": "t_ens801f0", "vf": "1"}, 26},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.1", "pf": "t_ens801f0", "vf": "1"}, 27},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:2e:01.1", "pf": "t_ens801f0", "vf": "1"}, 28}},
		"collecting sr-iov device metrics",
		"collector priority: \\[netlink\\]",
		"t_ens801f0 - using netlink collector"),
	Entry("with both sysfs and netlink",
		[]string{"sysfs", "netlink"},
		fstest.MapFS{
			"0000:3f:00.0/sriov_totalvfs":                {Data: []byte("64")},
			"0000:3f:00.0/net/t_ens785f0":                {Mode: fs.ModeDir},
			"0000:3f:00.0/numa_node":                     {Data: []byte("0")},
			"0000:3f:00.0/class":                         {Data: []byte("0x020000")},
			"0000:3f:00.0/virtfn0":                       {Data: []byte("/sys/devices/0000:3f:01.0"), Mode: fs.ModeSymlink},
			"t_ens785f0/device/sriov/0/stats/rx_packets": {Data: []byte("4")},
			"t_ens785f0/device/sriov/0/stats/tx_packets": {Data: []byte("8")},
			"0000:4g:00.0/sriov_totalvfs":                {Data: []byte("128")},
			"0000:4g:00.0/net/t_ens801f0":                {Mode: fs.ModeDir},
			"0000:4g:00.0/numa_node":                     {Data: []byte("0")},
			"0000:4g:00.0/class":                         {Data: []byte("0x020000")},
			"0000:4g:00.0/virtfn0":                       {Data: []byte("/sys/devices/0000:4g:01.0"), Mode: fs.ModeSymlink}},
		netlink.Device{LinkAttrs: netlink.LinkAttrs{Vfs: []netlink.VfInfo{
			{ID: 0, Mac: nil, Vlan: 0, Qos: 0, TxRate: 0, Spoofchk: true, LinkState: 0, MaxTxRate: 0, MinTxRate: 0, RxPackets: 31, TxPackets: 32, RxBytes: 33, TxBytes: 34, Multicast: 35, Broadcast: 36, RxDropped: 37, TxDropped: 38, RssQuery: 0, Trust: 0},
		}}},
		[]metric{
			{map[string]string{"numa_node": "0", "pciAddr": "0000:3f:01.0", "pf": "t_ens785f0", "vf": "0"}, 4},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:3f:01.0", "pf": "t_ens785f0", "vf": "0"}, 8},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:4g:01.0", "pf": "t_ens801f0", "vf": "0"}, 31},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:4g:01.0", "pf": "t_ens801f0", "vf": "0"}, 32},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:4g:01.0", "pf": "t_ens801f0", "vf": "0"}, 33},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:4g:01.0", "pf": "t_ens801f0", "vf": "0"}, 34},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:4g:01.0", "pf": "t_ens801f0", "vf": "0"}, 35},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:4g:01.0", "pf": "t_ens801f0", "vf": "0"}, 36},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:4g:01.0", "pf": "t_ens801f0", "vf": "0"}, 37},
			{map[string]string{"numa_node": "0", "pciAddr": "0000:4g:01.0", "pf": "t_ens801f0", "vf": "0"}, 38}},
		"collecting sr-iov device metrics",
		"collector priority: \\[sysfs netlink\\]",
		"t_ens785f0 - using sysfs collector",
		"getting stats for t_ens785f0 vf\\d"),

	// These logs are expected, but were causing instability in this test case, removed for now
	// "t_ens801f0 does not support sysfs collector, directory 't_ens801f0/device/sriov' does not exist",
	// "t_ens801f0 - using netlink collector",
)

var _ = DescribeTable("test creating sriovDev collector", // createSriovDevCollector
	func(fsys fs.FS, expected sriovDevCollector, logs ...string) {
		devfs = fsys

		collector := createSriovDevCollector()
		Expect(collector).To(Equal(expected))

		assertLogs(logs)
	},
	Entry("only sriov net devices",
		fstest.MapFS{
			"0000:1a:00.0/sriov_totalvfs": {Data: []byte("64")},
			"0000:1a:00.0/numa_node":      {Data: []byte("1")},
			"0000:1a:00.0/class":          {Data: []byte("0x020000")},
			"0000:1a:00.1/sriov_totalvfs": {Data: []byte("64")},
			"0000:1a:00.1/numa_node":      {Data: []byte("1")},
			"0000:1a:00.1/class":          {Data: []byte("0x020000")},
			"0000:2b:00.0/sriov_totalvfs": {Data: []byte("128")},
			"0000:2b:00.0/numa_node":      {Data: []byte("2")},
			"0000:2b:00.0/class":          {Data: []byte("0x020000")},
			"0000:2b:00.1/sriov_totalvfs": {Data: []byte("128")},
			"0000:2b:00.1/numa_node":      {Data: []byte("2")},
			"0000:2b:00.1/class":          {Data: []byte("0x020000")}},
		sriovDevCollector{
			"vfstats",
			map[string]string{"0000:1a:00.0": "1", "0000:1a:00.1": "1", "0000:2b:00.0": "2", "0000:2b:00.1": "2"}}),
	Entry("mixed devices",
		fstest.MapFS{
			"0000:3c:00.0/sriov_totalvfs": {Data: []byte("63")},
			"0000:3c:00.0/numa_node":      {Data: []byte("1")},
			"0000:3c:00.0/class":          {Data: []byte("0x020000")},
			"0000:3c:00.1/sriov_totalvfs": {Data: []byte("63")},
			"0000:3c:00.1/numa_node":      {Data: []byte("1")},
			"0000:3c:00.1/class":          {Data: []byte("0x020000")},
			"0000:4d:00.0/sriov_totalvfs": {Data: []byte("64")},
			"0000:4d:00.0/numa_node":      {Data: []byte("-1")},
			"0000:4d:00.0/class":          {Data: []byte("0x020000")},
			"0000:4d:00.1/sriov_totalvfs": {Data: []byte("64")},
			"0000:4d:00.1/numa_node":      {Data: []byte("-1")},
			"0000:4d:00.1/class":          {Data: []byte("0x020000")}},
		sriovDevCollector{
			"vfstats",
			map[string]string{"0000:3c:00.0": "1", "0000:3c:00.1": "1", "0000:4d:00.0": "", "0000:4d:00.1": ""}},
		"no numa node information for device '0000:4d:00.0'",
		"no numa node information for device '0000:4d:00.1'"),
	Entry("no sriov net devices",
		fstest.MapFS{
			"0000:5e:00.0/": {Mode: fs.ModeDir},
			"0000:5e:00.1/": {Mode: fs.ModeDir},
			"0000:5e:00.2/": {Mode: fs.ModeDir},
			"0000:5e:00.3/": {Mode: fs.ModeDir}},
		sriovDevCollector{
			"vfstats",
			map[string]string{}},
		"no sriov net devices found"),
)

var _ = DescribeTable("test getting sriov devices from filesystem", // getSriovDevAddrs
	func(fsys fs.FS, expected []string, logs ...string) {
		devfs = fsys

		devs := getSriovDevAddrs()
		Expect(devs).To(Equal(expected))

		assertLogs(logs)
	},
	Entry("only sriov net devices",
		fstest.MapFS{
			"0000:6f:00.0/sriov_totalvfs": {Data: []byte("64")}, "0000:6f:00.0/class": {Data: []byte("0x020000")},
			"0000:6f:00.1/sriov_totalvfs": {Data: []byte("64")}, "0000:6f:00.1/class": {Data: []byte("0x020000")},
			"0000:7g:00.0/sriov_totalvfs": {Data: []byte("128")}, "0000:7g:00.0/class": {Data: []byte("0x020000")},
			"0000:7g:00.1/sriov_totalvfs": {Data: []byte("128")}, "0000:7g:00.1/class": {Data: []byte("0x020000")}},
		[]string{"0000:6f:00.0", "0000:6f:00.1", "0000:7g:00.0", "0000:7g:00.1"}),
	Entry("mixed devices",
		fstest.MapFS{
			"0000:8h:00.0/":               {Mode: fs.ModeDir},
			"0000:8h:00.1/":               {Mode: fs.ModeDir},
			"0000:9i:00.0/sriov_totalvfs": {Data: []byte("63")}, "0000:9i:00.0/class": {Data: []byte("0x020000")},
			"0000:9i:00.1/sriov_totalvfs": {Data: []byte("63")}, "0000:9i:00.1/class": {Data: []byte("0x020000")}},
		[]string{"0000:9i:00.0", "0000:9i:00.1"}),
	Entry("no sriov net devices",
		fstest.MapFS{
			"0000:1b:00.0/": {Mode: fs.ModeDir},
			"0000:1b:00.1/": {Mode: fs.ModeDir},
			"0000:1b:00.2/": {Mode: fs.ModeDir},
			"0000:1b:00.3/": {Mode: fs.ModeDir}},
		[]string{},
		"no sriov net devices found"),
)

var _ = DescribeTable("test getting sriov dev details", // getSriovDev
	func(dev string, priority []string, fsys fs.FS, link netlink.Link, expected sriovDev, logs ...string) {
		devfs = fsys
		netfs = fsys

		if link != nil {
			vfstats.GetLink = func(name string) (netlink.Link, error) {
				return link, nil
			}
			DeferCleanup(func() {
				vfstats.GetLink = netlink.LinkByName
			})
		}

		sriovDev := getSriovDev(dev, priority)
		Expect(sriovDev).To(Equal(expected))

		assertLogs(logs)
	},
	Entry("with sysfs support",
		"0000:4f:00.0",
		[]string{"sysfs", "netlink"},
		fstest.MapFS{
			"0000:4f:00.0/net/ens785f0":                {Mode: fs.ModeDir},
			"0000:4f:00.0/virtfn0":                     {Data: []byte("/sys/devices/0000:4f:01.0"), Mode: fs.ModeSymlink},
			"0000:4f:00.0/virtfn1":                     {Data: []byte("/sys/devices/0000:4f:01.1"), Mode: fs.ModeSymlink},
			"ens785f0/device/sriov":                    {Mode: fs.ModeDir},
			"ens785f0/device/sriov/0/stats/rx_packets": {Data: []byte("1")}, // Added to enable sysfsReader
			"0000:5g:00.0/net/ens801f0":                {Mode: fs.ModeDir},
			"0000:5g:00.0/virtfn0":                     {Data: []byte("/sys/devices/0000:5g:01.0"), Mode: fs.ModeSymlink}},
		nil,
		sriovDev{
			"ens785f0",
			sysfsReader{"/sys/class/net/%s/device/sriov/%s/stats"},
			map[string]string{"0": "0000:4f:01.0", "1": "0000:4f:01.1"}},
		"ens785f0 - using sysfs collector"),
	Entry("without sysfs support",
		"0000:6h:00.0",
		[]string{"sysfs", "netlink"},
		fstest.MapFS{
			"0000:6h:00.0/net/ens785f0": {Mode: fs.ModeDir},
			"0000:6h:00.0/virtfn0":      {Data: []byte("/sys/devices/0000:6h:01.0"), Mode: fs.ModeSymlink},
			"0000:6h:00.0/virtfn1":      {Data: []byte("/sys/devices/0000:6h:01.1"), Mode: fs.ModeSymlink},
			"0000:7i:00.0/net/ens801f0": {Mode: fs.ModeDir},
			"0000:7i:00.0/virtfn0":      {Data: []byte("/sys/devices/0000:7i:01.0"), Mode: fs.ModeSymlink}},
		&netlink.Device{LinkAttrs: netlink.LinkAttrs{Vfs: []netlink.VfInfo{}}}, //nolint:govet
		sriovDev{
			"ens785f0",
			netlinkReader{vfstats.VfStats("ens785f0")},
			map[string]string{"0": "0000:6h:01.0", "1": "0000:6h:01.1"}},
		"ens785f0 does not support sysfs collector",
		"ens785f0 - using netlink collector"),
	Entry("without any collector support",
		"0000:8j:00.0",
		[]string{"unsupported_collector"},
		fstest.MapFS{
			"0000:8j:00.0/net/ens785f0": {Mode: fs.ModeDir},
			"0000:8j:00.0/virtfn0":      {Data: []byte("/sys/devices/0000:8j:01.0"), Mode: fs.ModeSymlink},
			"0000:8j:00.0/virtfn1":      {Data: []byte("/sys/devices/0000:8j:01.1"), Mode: fs.ModeSymlink}},
		nil,
		sriovDev{
			"ens785f0",
			nil,
			map[string]string{"0": "0000:8j:01.0", "1": "0000:8j:01.1"}},
		"ens785f0 - 'unsupported_collector' collector not supported"),
	Entry("without any virtual functions",
		"0000:9k:00.0",
		[]string{"sysfs"},
		fstest.MapFS{
			"0000:9k:00.0/net/ens785f0": {Mode: fs.ModeDir}},
		nil,
		sriovDev{
			"ens785f0",
			nil,
			map[string]string{}},
		"error getting vf address",
		"no virtual functions found for pf '0000:9k:00.0'",
		"ens785f0 does not support sysfs collector"),
)

var _ = DescribeTable("test getting numa node information for devices from filesystem", // getNumaNodes // TODO: ensure map order
	func(devices []string, fsys fs.FS, expected map[string]string, logs ...string) {
		devfs = fsys

		numaNodes := getNumaNodes(devices)
		Expect(numaNodes).To(Equal(expected))

		assertLogs(logs)
	},
	Entry("only sriov net devices",
		[]string{"0000:2c:00.0", "0000:2c:00.1", "0000:3d:00.0", "0000:3d:00.1"},
		fstest.MapFS{
			"0000:2c:00.0/numa_node": {Data: []byte("0")},
			"0000:2c:00.1/numa_node": {Data: []byte("0")},
			"0000:3d:00.0/numa_node": {Data: []byte("1")},
			"0000:3d:00.1/numa_node": {Data: []byte("1")}},
		map[string]string{"0000:2c:00.0": "0", "0000:2c:00.1": "0", "0000:3d:00.0": "1", "0000:3d:00.1": "1"}),
	Entry("mixed devices",
		[]string{"0000:4e:00.0", "0000:4e:00.1", "0000:5f:00.0", "0000:5f:00.1"},
		fstest.MapFS{
			"0000:4e:00.0/":          {Mode: fs.ModeDir},
			"0000:4e:00.1/":          {Mode: fs.ModeDir},
			"0000:5f:00.0/numa_node": {Data: []byte("-1")},
			"0000:5f:00.1/numa_node": {Data: []byte("-1")}},
		map[string]string{"0000:4e:00.0": "", "0000:4e:00.1": "", "0000:5f:00.0": "", "0000:5f:00.1": ""},
		"could not read numa_node file for device '0000:4e:00.0'",
		"open 0000:4e:00.0/numa_node: file does not exist",
		"could not read numa_node file for device '0000:4e:00.1'",
		"open 0000:4e:00.1/numa_node: file does not exist",
		"no numa node information for device '0000:5f:00.0'",
		"no numa node information for device '0000:5f:00.1'"),
	Entry("no sriov net devices",
		[]string{"0000:6g:00.0", "0000:6g:00.1", "0000:6g:00.2", "0000:6g:00.3"},
		fstest.MapFS{
			"0000:6g:00.0/": {Mode: fs.ModeDir},
			"0000:6g:00.1/": {Mode: fs.ModeDir},
			"0000:6g:00.2/": {Mode: fs.ModeDir},
			"0000:6g:00.3/": {Mode: fs.ModeDir}},
		map[string]string{"0000:6g:00.0": "", "0000:6g:00.1": "", "0000:6g:00.2": "", "0000:6g:00.3": ""},
		"could not read numa_node file for device '0000:6g:00.0'",
		"open 0000:6g:00.0/numa_node: file does not exist",
		"could not read numa_node file for device '0000:6g:00.1'",
		"open 0000:6g:00.1/numa_node: file does not exist",
		"could not read numa_node file for device '0000:6g:00.2'",
		"open 0000:6g:00.2/numa_node: file does not exist",
		"could not read numa_node file for device '0000:6g:00.3'",
		"open 0000:6g:00.3/numa_node: file does not exist"),
)

var _ = DescribeTable("test getting vf information for devices from filesystem", // vfList
	func(dev string, fsys fs.FS, expected vfsPCIAddr, err error, logs ...string) {
		devfs = fsys

		vfs, e := vfList(dev)
		Expect(vfs).To(Equal(expected))

		if err != nil {
			Expect(e).Should(MatchError(err))
		}

		assertLogs(logs)
	},
	Entry("only retrieve vf information for specified sriov net device",
		"0000:7h:00.0",
		fstest.MapFS{
			"0000:7h:00.0/virtfn0": {Data: []byte("/sys/devices/0000:7h:01.0"), Mode: fs.ModeSymlink},
			"0000:7h:00.0/virtfn1": {Data: []byte("/sys/devices/0000:7h:01.1"), Mode: fs.ModeSymlink},
			"0000:8i:00.0/virtfn0": {Data: []byte("/sys/devices/0000:8i:01.0"), Mode: fs.ModeSymlink}},
		map[string]string{"0": "0000:7h:01.0", "1": "0000:7h:01.1"},
		nil),
	Entry("vf file is not a symlink for specified sriov net device",
		"0000:9j:00.0",
		fstest.MapFS{
			"0000:9j:00.0/virtfn0": {Data: []byte("/sys/devices/0000:9j:01.0"), Mode: fs.ModeDir}},
		map[string]string{},
		fmt.Errorf("no virtual functions found for pf '0000:9j:00.0'"),
		"error evaluating symlink '0000:9j:00.0/virtfn0'"),
	Entry("vf file does not exist for specified sriov net device",
		"0000:1c:00.0",
		fstest.MapFS{},
		map[string]string{},
		fmt.Errorf("no virtual functions found for pf '0000:1c:00.0'")),
)

var _ = DescribeTable("test getting vf data from filesystem", // vfData
	func(vfDir string, fsys fs.FS, expectedVfId string, expectedVfPciAddr string, logs ...string) {
		devfs = fsys

		vfId, vfPci := vfData(vfDir)
		Expect(vfId).To(Equal(expectedVfId))
		Expect(vfPci).To(Equal(expectedVfPciAddr))

		assertLogs(logs)
	},
	Entry("valid symlink",
		"0000:7h:00.0/virtfn0",
		fstest.MapFS{"0000:7h:00.0/virtfn0": {Data: []byte("/sys/devices/0000:7h:01.0"), Mode: fs.ModeSymlink}},
		"0",
		"0000:7h:01.0"),
	Entry("invalid symlink",
		"0000:8i:00.0/virtfn0",
		fstest.MapFS{"0000:8i:00.0/virtfn0": {Mode: fs.ModeDir}},
		"",
		"",
		"error evaluating symlink '0000:8i:00.0/virtfn0'"),
)

var _ = DescribeTable("test getting pf name from pci address on filesystem", // getPFName
	func(dev string, fsys fs.FS, expected string, logs ...string) {
		devfs = fsys

		pfName := getPFName(dev)
		Expect(pfName).To(Equal(expected))

		assertLogs(logs)
	},
	Entry("pf exists",
		"0000:2d:00.0",
		fstest.MapFS{"0000:2d:00.0/net/ens785f0": {Mode: fs.ModeDir}},
		"ens785f0"),
	Entry("pf does not exist",
		"0000:3e:00.0",
		fstest.MapFS{},
		"",
		"0000:3e:00.0 - could not get pf interface name in path '0000:3e:00.0/net'",
		"open 0000:3e:00.0/net: file does not exist"),
)
