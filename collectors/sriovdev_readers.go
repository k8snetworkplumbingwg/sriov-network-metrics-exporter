// This file should contain different sriov stat implementations for different drivers and versions.

package collectors

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/k8snetworkplumbingwg/sriov-network-metrics-exporter/pkg/utils"
	"github.com/k8snetworkplumbingwg/sriov-network-metrics-exporter/pkg/vfstats"
)

const sriovVFStatsDir = "%s/device/sriov/%s/stats"

type sriovStats map[string]int64

// sriovStatReader is an interface which takes in the Physical Function name and vf id and returns the stats for the VF
type sriovStatReader interface {
	ReadStats(vfID string, pfName string) sriovStats
}

// netlinkReader is able to read stats from drivers that support the netlink interface
type netlinkReader struct {
	data vfstats.PerPF
}

// sysfsReader is able to read stats from Physical Functions running the i40e or ice driver
// Other drivers that store all VF stats in files under one folder could use this reader
type sysfsReader struct {
	statsFS string
}

// check if a reader can read stats for a given pf and vfID
func readerHasStats(reader sriovStatReader, pfName, vfID string) bool {
	stats := reader.ReadStats(pfName, vfID)
	return len(stats) > 0
}

// getStatsReader returns the correct stat reader for the given PF
// Currently only drivers that implement netlink or the sriov sysfs interface are supported
func getStatsReader(pf string, priority []string) (sriovStatReader, error) {
	// Try to find a collector that can actually read stats for at least VF 0
	vfTestID := "0"
	for _, collector := range priority {
		switch collector {
		case "sysfs":
			sriovPath := pf + "/device/sriov"
			if _, err := fs.Stat(netfs, sriovPath); !os.IsNotExist(err) {
				reader := sysfsReader{*sysClassNet + "/%s/device/sriov/%s/stats"}
				// Test if sysfsReader can read stats for VF 0
				if readerHasStats(reader, pf, vfTestID) {
					log.Printf("%s - using sysfs collector", pf)
					return reader, nil
				} else {
					log.Printf("%s - sysfs collector present but no stats found for vf%s", pf, vfTestID)
				}
			} else {
				log.Printf("%s does not support sysfs collector, directory '%s' does not exist", pf, sriovPath)
			}
		case "netlink":
			if vfstats.DoesPfSupportNetlink(pf) {
				reader := netlinkReader{vfstats.VfStats(pf)}
				// Test if netlinkReader can read stats for VF 0
				if readerHasStats(reader, pf, vfTestID) {
					log.Printf("%s - using netlink collector", pf)
					return reader, nil
				} else {
					log.Printf("%s - netlink collector present but no stats found for vf%s", pf, vfTestID)
				}
			} else {
				log.Printf("%s does not support netlink collector", pf)
			}
		default:
			log.Printf("%s - '%s' collector not supported", pf, collector)
		}
	}
	return nil, fmt.Errorf("no stats reader found for %s", pf)
}

// ReadStats takes in the name of a PF and the VF Id and returns a stats object.
func (r netlinkReader) ReadStats(pfName, vfID string) sriovStats {
	id, err := strconv.Atoi(vfID)
	if err != nil {
		log.Print("error reading passed virtual function id")
		return sriovStats{}
	}

	vf := r.data.Vfs[id]
	//nolint:gosec // G115: Values are network stats unlikely to overflow int64
	return map[string]int64{
		"tx_bytes":     int64(vf.TxBytes),
		"rx_bytes":     int64(vf.RxBytes),
		"tx_packets":   int64(vf.TxPackets),
		"rx_packets":   int64(vf.RxPackets),
		"tx_dropped":   int64(vf.TxDropped),
		"rx_dropped":   int64(vf.RxDropped),
		"rx_broadcast": int64(vf.Broadcast),
		"rx_multicast": int64(vf.Multicast),
	}
}

func (r sysfsReader) ReadStats(pfName, vfID string) sriovStats {
	stats := make(sriovStats, 0)

	statDir := fmt.Sprintf(sriovVFStatsDir, pfName, vfID)
	files, err := fs.ReadDir(netfs, statDir)
	if err != nil {
		log.Printf("error reading stats for %s vf%s\n%v", pfName, vfID, err)
		return stats
	}

	log.Printf("getting stats for %s vf%s", pfName, vfID)

	for _, f := range files {
		path := filepath.Join(statDir, f.Name())
		if utils.IsSymLink(netfs, path) {
			log.Printf("could not stat file '%s'", path)
			continue
		}

		statRaw, err := fs.ReadFile(netfs, path)
		if err != nil {
			log.Printf("error reading file, %v", err)
			continue
		}

		statString := strings.TrimSpace(string(statRaw))
		value, err := strconv.ParseInt(statString, 10, 64)
		if err != nil {
			log.Printf("%s - error parsing integer from value '%s'\n%v", f.Name(), statString, err)
			continue
		}

		stats[f.Name()] = value
	}

	return stats
}
