package vfstats

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
)

func TestNetlink(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "netlink test suite")
}

var _ = DescribeTable("test vf stats collection", // VfStats
	func(devName string, link netlink.Device, err error, expectedPerPF PerPF, logs ...string) {
		GetLink = func(name string) (netlink.Link, error) {
			return &link, err
		}

		Expect(VfStats(devName)).To(Equal(expectedPerPF))
	},
	Entry("Without error",
		"ens801f0",
		netlink.Device{LinkAttrs: netlink.LinkAttrs{Vfs: []netlink.VfInfo{
			{
				ID: 0, Mac: nil, Vlan: 0, Qos: 0, TxRate: 0, Spoofchk: true, LinkState: 0,
				MaxTxRate: 0, MinTxRate: 0, RxPackets: 11, TxPackets: 12, RxBytes: 13,
				TxBytes: 14, Multicast: 15, Broadcast: 16, RxDropped: 17, TxDropped: 18,
			},
			{
				ID: 1, Mac: nil, Vlan: 0, Qos: 0, TxRate: 0, Spoofchk: true, LinkState: 0,
				MaxTxRate: 0, MinTxRate: 0, RxPackets: 21, TxPackets: 22, RxBytes: 23,
				TxBytes: 24, Multicast: 25, Broadcast: 26, RxDropped: 27, TxDropped: 28,
			},
		}}},
		nil,
		PerPF{"ens801f0", map[int]netlink.VfInfo{
			0: {
				ID: 0, Mac: nil, Vlan: 0, Qos: 0, TxRate: 0, Spoofchk: true, LinkState: 0,
				MaxTxRate: 0, MinTxRate: 0, RxPackets: 11, TxPackets: 12, RxBytes: 13,
				TxBytes: 14, Multicast: 15, Broadcast: 16, RxDropped: 17, TxDropped: 18,
			},
			1: {
				ID: 1, Mac: nil, Vlan: 0, Qos: 0, TxRate: 0, Spoofchk: true, LinkState: 0,
				MaxTxRate: 0, MinTxRate: 0, RxPackets: 21, TxPackets: 22, RxBytes: 23,
				TxBytes: 24, Multicast: 25, Broadcast: 26, RxDropped: 27, TxDropped: 28,
			},
		}},
	),
	Entry("With error",
		"ens801f0",
		nil,
		fmt.Errorf("link not found"),
		PerPF{"ens801f0", map[int]netlink.VfInfo{}},
	),
)
