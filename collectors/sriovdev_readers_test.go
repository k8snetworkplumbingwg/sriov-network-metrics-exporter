package collectors

import (
	"io/fs"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"

	"github.com/k8snetworkplumbingwg/sriov-network-metrics-exporter/pkg/vfstats"
)

var _ = DescribeTable("test getting stats reader for pf", // getStatsReader
	func(pf string, priority []string, fsys fs.FS, link netlink.Link, expected sriovStatReader, logs ...string) {
		netfs = fsys

		if link != nil {
			vfstats.GetLink = func(name string) (netlink.Link, error) {
				return link, nil
			}
			DeferCleanup(func() {
				vfstats.GetLink = netlink.LinkByName
			})
		}

		statsReader, err := getStatsReader(pf, priority)

		if expected != nil {
			Expect(statsReader).To(Equal(expected))
			Expect(err).To(BeNil())
		} else {
			Expect(statsReader).To(BeNil())
			Expect(err).To(HaveOccurred())
		}

		assertLogs(logs)
	},
	Entry("with sysfs support",
		"ens785f0",
		[]string{"sysfs", "netlink"},
		fstest.MapFS{
			"ens785f0/device/sriov":                    {Mode: fs.ModeDir},
			"ens785f0/device/sriov/0/stats/rx_packets": {Data: []byte("1")}, // Added to enable sysfsReader
		},
		nil,
		sysfsReader{"/sys/class/net/%s/device/sriov/%s/stats"},
		"ens785f0 - using sysfs collector"),
	Entry("without sysfs support",
		"ens785f0",
		[]string{"sysfs", "netlink"},
		fstest.MapFS{},
		&netlink.Device{LinkAttrs: netlink.LinkAttrs{Vfs: []netlink.VfInfo{}}}, //nolint:govet
		netlinkReader{vfstats.VfStats("ens785f0")},
		"ens785f0 does not support sysfs collector",
		"ens785f0 - using netlink collector"),
	Entry("without any collector support",
		"ens785f0",
		[]string{"unsupported_collector"},
		fstest.MapFS{},
		nil,
		nil,
		"ens785f0 - 'unsupported_collector' collector not supported"),
	Entry("sysfs present but returns no stats, fallback to netlink",
		"ens785f0",
		[]string{"sysfs", "netlink"},
		fstest.MapFS{
			"ens785f0/device/sriov": {Mode: fs.ModeDir},
			// sysfs stats file exists but is empty (simulates no stats)
			"ens785f0/device/sriov/0/stats/rx_packets": {Data: []byte("")},
		},
		&netlink.Device{LinkAttrs: netlink.LinkAttrs{Vfs: []netlink.VfInfo{{ID: 0, TxPackets: 42}}}},
		netlinkReader{vfstats.PerPF{Pf: "ens785f0", Vfs: map[int]netlink.VfInfo{0: {ID: 0, TxPackets: 42}}}},
		"ens785f0 - sysfs collector present but no stats found for vf0",
		"ens785f0 - using netlink collector"),
)

var _ = DescribeTable("test getting reading stats through sriov sysfs interface", // sysfsReader.ReadStats
	func(pf string, vfId string, fsys fs.FS, expected sriovStats, logs ...string) {
		netfs = fsys

		statsReader := new(sysfsReader)
		stats := statsReader.ReadStats(pf, vfId)
		Expect(stats).To(Equal(expected))

		assertLogs(logs)
	},
	Entry("with stats files",
		"ens785f0",
		"0",
		fstest.MapFS{
			"ens785f0/device/sriov/0/stats/rx_packets": {Data: []byte("6")},
			"ens785f0/device/sriov/0/stats/rx_bytes":   {Data: []byte("24")},
			"ens785f0/device/sriov/0/stats/tx_packets": {Data: []byte("12")},
			"ens785f0/device/sriov/0/stats/tx_bytes":   {Data: []byte("48")}},
		map[string]int64{
			"rx_packets": 6,
			"rx_bytes":   24,
			"tx_packets": 12,
			"tx_bytes":   48},
		"getting stats for ens785f0 vf0"),
	Entry("without stats files",
		"ens785f0",
		"0",
		fstest.MapFS{},
		map[string]int64{},
		"error reading stats for ens785f0 vf0",
		"open ens785f0/device/sriov/0/stats: file does not exist"),
	Entry("with stat file as a symlink",
		"ens785f0",
		"0",
		fstest.MapFS{
			"ens785f0/device/sriov/0/stats/rx_packets": {Mode: fs.ModeSymlink}},
		map[string]int64{},
		"getting stats for ens785f0 vf0",
		"could not stat file 'ens785f0/device/sriov/0/stats/rx_packets'"),
	Entry("with stat file as a directory",
		"ens785f0",
		"0",
		fstest.MapFS{
			"ens785f0/device/sriov/0/stats/rx_packets": {Mode: fs.ModeDir}},
		map[string]int64{},
		"getting stats for ens785f0 vf0",
		"error reading file, read ens785f0/device/sriov/0/stats/rx_packets: invalid argument"),
	Entry("with invalid stat file",
		"ens785f0",
		"0",
		fstest.MapFS{
			"ens785f0/device/sriov/0/stats/rx_packets": {Data: []byte("NaN")}},
		map[string]int64{},
		"getting stats for ens785f0 vf0",
		"rx_packets - error parsing integer from value 'NaN'",
		"strconv.ParseInt: parsing \"NaN\": invalid syntax"),
)
