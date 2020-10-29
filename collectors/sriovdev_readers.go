//This file should contain different sriov stat implementations for different drivers and versions.

package collectors

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sriov-network-metrics-exporter/pkg/vfStats"
	"strconv"
	"strings"
)

type sriovStats map[string]int64

//sriovStatReader is an interface which takes in the Physical Function name and vf id and returns the stats for the VF
type sriovStatReader interface {
	ReadStats(vfID string, pfName string) sriovStats
}

//netlinkReader is able to read stats from drivers that support the netlink interface
type netlinkReader struct {
	data vfStats.PerPF
}

//i40eReader is able to read stats from Physical Functions running the i40e driver.
type i40eReader struct {
	statsFS string
}

//statReaderForPF returns the correct stat reader for the given PF
//currently only i40e is implemented, but other drivers can be implemented and picked up here.
func statReaderForPF(pf string) sriovStatReader {
	if *netlinkEnabled {
		return netlinkReader{
			vfStats.VfStats(pf)}
	}
	pfDriverPath := filepath.Join(*sysClassNet, pf, "device", driverFile)
	//driver type is found by getting the destination of the symbolic link on the driver path from /sys/bus/pci
	driverInfo, err := os.Readlink(pfDriverPath)
	if err != nil {
		log.Printf("failed to get driver info: %v", err)
		return nil
	}
	pfDriver := filepath.Base(driverInfo)
	switch pfDriver {
	case "i40e":
		return i40eReader{filepath.Join(*sysClassNet, "/%s/device/sriov/%s/stats/")}
	default:
		log.Printf("No stats reader available for Physical Function %v", pf)
		return nil
	}
}

//ReadStats takes in the name of a PF and the VF Id and returns a stats object.
func (r i40eReader) ReadStats(pfName string, vfID string) sriovStats {
	stats := make(sriovStats, 0)
	statroot := fmt.Sprintf(r.statsFS, pfName, vfID)
	files, err := ioutil.ReadDir(statroot)
	if err != nil {
		return stats
	}
	log.Printf("getting stats for pf %v %v", pfName, vfID)
	for _, f := range files {
		path := filepath.Join(statroot, f.Name())
		if isSymLink(path) {
			log.Printf("error: cannot read symlink %v", path)
			continue
		}
		statRaw, err := ioutil.ReadFile(path)
		if err != nil {
			continue
		}
		statString := strings.TrimSpace(string(statRaw))
		value, err := strconv.ParseInt(statString, 10, 64)
		if err != nil {
			log.Printf("Error reading file %v: %v", f.Name(), err)
			continue
		}
		stats[f.Name()] = value
	}
	return stats
}

func (r netlinkReader) ReadStats(pfName string, vfID string) sriovStats {
	id, err := strconv.Atoi(vfID)
	if err != nil {
		log.Print("Error reading passed Virtual Function ID")
		return sriovStats{}
	}
	return func() sriovStats {
		vf := r.data.Vfs[id]
		return map[string]int64{
			"tx_bytes":   int64(vf.TxBytes),
			"rx_bytes":   int64(vf.RxBytes),
			"tx_packets": int64(vf.TxPackets),
			"rx_packets": int64(vf.RxPackets),
			"tx_dropped": int64(vf.TxDropped),
			"rx_dropped": int64(vf.RxDropped),
			"rx_broadcast": int64(vf.Broadcast),
			"rx_multicast": int64(vf.Multicast),
		}
	}()
}
