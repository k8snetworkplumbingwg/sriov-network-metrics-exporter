package collectors

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

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

var _ = Describe("GetV1Client", func() {
	var (
		tmpDir     string
		socketPath string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "grpc-test")
		Expect(err).NotTo(HaveOccurred())
		socketPath = filepath.Join(tmpDir, "test.sock")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	It("connects successfully to a valid unix socket", func() {
		lis, err := net.Listen("unix", socketPath)
		Expect(err).NotTo(HaveOccurred())

		server := grpc.NewServer()
		go func() { _ = server.Serve(lis) }()
		defer server.Stop()

		client, conn, err := GetV1Client("unix:///"+socketPath, 5*time.Second, defaultPodResourcesMaxSize)
		Expect(err).NotTo(HaveOccurred())
		Expect(client).NotTo(BeNil())
		Expect(conn).NotTo(BeNil())

		err = conn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error when connection times out on non-existent socket", func() {
		nonExistentSocket := filepath.Join(tmpDir, "nonexistent.sock")

		_, _, err := GetV1Client("unix:///"+nonExistentSocket, 500*time.Millisecond, defaultPodResourcesMaxSize)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("timed out waiting for connection"))
	})

	It("returns a functional client that can issue RPCs", func() {
		lis, err := net.Listen("unix", socketPath)
		Expect(err).NotTo(HaveOccurred())

		server := grpc.NewServer()
		go func() { _ = server.Serve(lis) }()
		defer server.Stop()

		client, conn, err := GetV1Client("unix:///"+socketPath, 5*time.Second, defaultPodResourcesMaxSize)
		Expect(err).NotTo(HaveOccurred())
		Expect(client).NotTo(BeNil())
		defer func() { _ = conn.Close() }()

		// The server does not implement PodResourcesLister, so a List call
		// should return an "Unimplemented" gRPC error, confirming the client
		// is connected and able to communicate with the server.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, listErr := client.List(ctx, nil)
		Expect(listErr).To(HaveOccurred())
		Expect(listErr.Error()).To(ContainSubstring("Unimplemented"))
	})
})
