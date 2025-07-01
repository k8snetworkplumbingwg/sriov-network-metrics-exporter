// Package vfstats contains methods to pull the SRIOV stats from various locations in linux
package vfstats

import (
	"log"

	"github.com/vishvananda/netlink"
)

// PerPF returns stats related to each virtual function for a given physical function
type PerPF struct {
	Pf  string
	Vfs map[int]netlink.VfInfo
}

// VfStats returns the stats for all of the SRIOV Virtual Functions attached to the given Physical Function
func VfStats(pf string) PerPF {
	output := PerPF{pf, make(map[int]netlink.VfInfo)}
	lnk, err := GetLink(pf)
	if err != nil {
		log.Printf("netlink: error retrieving link for pf '%s'\n%v", pf, err)
		return output
	}

	for _, vf := range lnk.Attrs().Vfs {
		output.Vfs[vf.ID] = vf
	}

	return output
}

// DoesPfSupportNetlink returns true if the Physical Function supports the netlink APIs
func DoesPfSupportNetlink(pf string) bool {
	_, err := GetLink(pf)
	return err == nil
}

var GetLink = netlink.LinkByName
