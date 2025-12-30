package collectors

import (
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/k8snetworkplumbingwg/sriov-network-metrics-exporter/pkg/utils"
)

var buffer gbytes.Buffer

func TestCollectors(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "collectors test suite")
}

var _ = BeforeSuite(func() {
	utils.EvalSymlinks = evalSymlinks

	logFatal = func(msg string, args ...any) {
		log.Printf(msg, args...)
	}

	log.SetFlags(0)
})

var _ = BeforeEach(func() {
	buffer = *gbytes.NewBuffer()
	log.SetOutput(&buffer)
})

type metric struct {
	labels  map[string]string
	counter float64
}

type testCollector struct {
	name string
}

func createTestCollector() prometheus.Collector {
	return testCollector{
		name: "collector.test",
	}
}

func (c testCollector) Collect(ch chan<- prometheus.Metric) {}
func (c testCollector) Describe(chan<- *prometheus.Desc)    {}

var _ = DescribeTable("test registering collector", // register
	func(name string, enabled bool, collector func() prometheus.Collector) {
		register(name, enabled, collector)

		Expect(collectorState).To(HaveKey(name))
		Expect(collectorState[name]).To(Equal(&enabled))

		Expect(collectorFunctions).To(HaveKey(name))
		// Expect(allCollectors[name]).To(Equal(collector)) // TODO: verify expected collector is returned

	},
	Entry("the correct collector is enabled when default is true",
		"test_true",
		true,
		createTestCollector),
	Entry("the correct collector is not enabled when default is false",
		"test_false",
		false,
		createTestCollector),
)

// TODO: create Enabled unit test

func assertLogs(logs []string) {
	for _, log := range logs {
		Eventually(&buffer).WithTimeout(2 * time.Second).Should(gbytes.Say(log))
	}
}

// Replaces filepath.EvalSymlinks with an emulated evaluation to work with the in-memory fs.
var evalSymlinks = func(path string) (string, error) {
	path = filepath.Join(filepath.Base(filepath.Dir(path)), filepath.Base(path))
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	entries, err := fs.ReadDir(devfs, dir)
	if err != nil {
		return "", fmt.Errorf("error reading dir: %v", err)
	}

	for _, entry := range entries {
		if entry.Name() == base && entry.Type()&fs.ModeSymlink != 0 {
			// In Go 1.25, fstest.MapFS treats fs.ModeSymlink files as actual symlinks
			// and tries to follow them, so fs.ReadFile won't work.
			// Access the MapFS directly to get the symlink target data.
			if mapFS, ok := devfs.(fstest.MapFS); ok {
				if mapFile, exists := mapFS[path]; exists {
					return string(mapFile.Data), nil
				}
			}
			return "", fmt.Errorf("error reading symlink target")
		}
	}

	return "", fmt.Errorf("not a symlink or not found")
}
