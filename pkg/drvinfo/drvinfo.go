package drvinfo

import (
	"log"
	"os"

	"github.com/hashicorp/go-version"
	"github.com/safchain/ethtool"
	"gopkg.in/yaml.v3"
)

var (
	newEthtool = newEthtoolHandler
)

type ethtoolInterface interface {
	Close()
	DriverInfo(name string) (ethtool.DrvInfo, error)
}

func newEthtoolHandler() (ethtoolInterface, error) {
	return ethtool.NewEthtool()
}

type DriverInfo struct {
	Name    string
	Version string
}

type DriversList struct {
	Drivers []DriverInfo
}

type SupportedDrivers struct {
	Drivers    DriversList
	DbFilePath string
}

var NewSupportedDrivers = func(fp string) SupportedDrivers {
	retv := SupportedDrivers{}
	supportedDrivers, err := readSupportedDrivers(fp)
	if err != nil {
		log.Printf("fetching supported drivers list from %s failed with error %v", fp, err)
		return retv
	}
	retv.Drivers = *supportedDrivers
	retv.DbFilePath = fp
	return retv
}

var GetDriverInfo = func(name string) (*DriverInfo, error) {
	ethHandle, err := newEthtool()
	if err != nil {
		return nil, err
	}
	defer ethHandle.Close()
	drvInfo, err := ethHandle.DriverInfo(name)
	if err != nil {
		return nil, err
	}
	return &DriverInfo{
		Name:    drvInfo.Driver,
		Version: drvInfo.Version,
	}, nil
}

func (dl *SupportedDrivers) IsDriverSupported(drv *DriverInfo) bool {
	for _, d := range dl.Drivers.Drivers {
		if d.Name != drv.Name {
			continue
		}
		supported, err := version.NewVersion(d.Version)
		if err != nil {
			continue
		}
		v, err := version.NewVersion(drv.Version)
		if err != nil {
			continue
		}
		if v.GreaterThanOrEqual(supported) {
			return true
		}
	}
	return false
}

func readSupportedDrivers(filePath string) (*DriversList, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	drivers := &DriversList{}
	err = yaml.Unmarshal(data, drivers)
	if err != nil {
		return drivers, err
	}
	return drivers, nil
}
