//Package vfstats contains methods to pull the SRIOV stats from various locations in linux
package vfstats

import (
	"github.com/vishvananda/netlink"
	"log"
)
//PerPF returns stats related to each virtual function for a given physical function
type PerPF struct {
	pf  string
	Vfs map[int]netlink.VfInfo
}

//VfStats returns the stats for all of the SRIOV Virtual Functions attached to the given Physical Function
func VfStats(pf string) PerPF {
	log.Printf("PerPF called for %v", pf)
	output := PerPF{pf, make(map[int]netlink.VfInfo)}
	lnk, err := netlink.LinkByName(pf)
	if err != nil{
		return output
	}
	for _, vf := range lnk.Attrs().Vfs {
		output.Vfs[vf.ID] = vf
	}
	return output
}
