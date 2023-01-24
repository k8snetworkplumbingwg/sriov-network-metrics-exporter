package collectors

import (
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/prometheus/client_golang/prometheus"

	"sriov-network-metrics-exporter/pkg/utils"
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
		Eventually(&buffer).WithTimeout(time.Duration(2 * time.Second)).Should(gbytes.Say(log))
	}
}

// Replaces filepath.EvalSymlinks with an emulated evaluation to work with the in-memory fs.
var evalSymlinks = func(path string) (string, error) {
	path = filepath.Join(filepath.Base(filepath.Dir(path)), filepath.Base(path))

	if stat, err := fs.Stat(devfs, path); err == nil && stat.Mode() == fs.ModeSymlink {
		if target, err := fs.ReadFile(devfs, path); err == nil {
			return string(target), nil
		} else {
			return "", fmt.Errorf("error")
		}
	} else {
		return "", fmt.Errorf("error")
	}
}
