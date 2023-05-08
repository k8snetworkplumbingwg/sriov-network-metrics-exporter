package drvinfo

import (
	"os"
	"path"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/safchain/ethtool"
	"gopkg.in/yaml.v3"
)

var (
	tempDir string
)

type ethtoolMock struct {
	drvInfo ethtool.DrvInfo
}

func (m *ethtoolMock) Close() {
	// do nothing
}

func (m *ethtoolMock) DriverInfo(name string) (ethtool.DrvInfo, error) {
	return m.drvInfo, nil
}

func TestDrvInfo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DrvInfo Test Suite")
}

var getDbFilePath = func() string {
	var err error
	tempDir, err = os.MkdirTemp("", "")
	Expect(err).ToNot(HaveOccurred())
	fp := path.Join(tempDir, "test.yaml")
	createDb(fp, *getTestDrv())
	return fp
}

var getTestDrv = func() *DriverInfo {
	return &DriverInfo{
		Name:    "ice",
		Version: "1.9.11",
	}
}

func createDb(filePath string, drv ...DriverInfo) {
	l := make([]DriverInfo, 0)
	l = append(l, drv...)
	drivers := &DriversList{
		Drivers: l,
	}
	data, err := yaml.Marshal(drivers)
	Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filePath, data, 0644)
	Expect(err).ToNot(HaveOccurred())
}

var _ = BeforeSuite(func() {
	newEthtool = func() (ethtoolInterface, error) {
		return &ethtoolMock{
			drvInfo: ethtool.DrvInfo{Driver: "dummy", Version: "1.0.0"},
		}, nil
	}
})

var _ = Describe("drvinfo", func() {
	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	DescribeTable("IsDriverSupported should", func(testDrv *DriverInfo, supported bool, dl SupportedDrivers) {
		Expect(dl.IsDriverSupported(testDrv)).To(Equal(supported))
	},
		Entry("return true if driver is supported", getTestDrv(), true, NewSupportedDrivers(getDbFilePath())),
		Entry("return false if driver is not supported", &DriverInfo{Name: "ice", Version: "1.8.1"}, false, NewSupportedDrivers(getDbFilePath())),
		Entry("return true if driver version is greater than supported", &DriverInfo{Name: "ice", Version: "1.10.1"}, true, NewSupportedDrivers(getDbFilePath())),
	)

	DescribeTable("readSupportedDrivers should", func(testDrv *DriverInfo) {
		filePath := getDbFilePath()
		db, err := readSupportedDrivers(filePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(db.Drivers).To(ContainElement(*testDrv))
	},
		Entry("return list of supported driver without error for valid config", getTestDrv()),
	)

	DescribeTable("GetDriverInfo should", func(testDrv DriverInfo) {
		info, err := GetDriverInfo("dummy")
		Expect(err).ToNot(HaveOccurred())
		Expect(info).ToNot(BeNil())
		Expect(info.Name).To(Equal(testDrv.Name))
		Expect(info.Version).To(Equal(testDrv.Version))
	},
		Entry("return correct DriverInfo without error", DriverInfo{Name: "dummy", Version: "1.0.0"}),
	)
})
