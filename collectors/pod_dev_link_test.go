package collectors

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TODO: create Collector and dialer unit tests

var _ = Describe("test creating podDevLink collector", func() { // createPodDevLinkCollector
	It("returns the correct collector", func() {
		collector := createPodDevLinkCollector()
		Expect(collector).To(Equal(podDevLinkCollector{name: podDevLinkName}))
	})
})

var _ = DescribeTable("test pci address regexp: "+pciAddressPattern.String(), // isPci
	func(pciAddr string, expected bool) {
		Expect(isPci(pciAddr)).To(Equal(expected))
	},
	Entry("valid, 0000:00:00.0", "0000:00:00.0", true),
	Entry("valid, ffff:00:00.0", "ffff:00:00.0", true),
	Entry("valid, 0000:ff:00.0", "0000:ff:00.0", true),
	Entry("valid, 0000:00:ff.0", "0000:00:ff.0", true),
	Entry("valid, 0000:00:00.0", "0000:00:00.0", true),
	Entry("invalid, 0000.00:00.0", "0000.00:00.0", false),
	Entry("invalid, 0000:00.00.0", "0000:00.00.0", false),
	Entry("invalid, 0000:00:00:0", "0000:00:00:0", false),
	Entry("invalid, gggg:00:00.0", "gggg:00:00.0", false),
	Entry("invalid, 0000:gg:00.0", "0000:gg:00.0", false),
	Entry("invalid, 0000:00:gg.0", "0000:00:gg.0", false),
	Entry("invalid, 0000:00:00.a", "0000:00:00.a", false),
	Entry("invalid, 00000:00:00.0", "00000:00:00.0", false),
	Entry("invalid, 0000:000:00.0", "0000:000:00.0", false),
	Entry("invalid, 0000:00:000.0", "0000:00:000.0", false),
	Entry("invalid, 0000:00:00.00", "0000:00:00.00", false),
)

// TODO: create integration tests for GetV1Client and PodResources, they require the kubelet API
