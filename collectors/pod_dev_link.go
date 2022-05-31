package collectors

//pod_dev_link publishes which devices are connected to which pods in Kubernetes by querying the Kubelet api

import (
	"context"
	"flag"
	"regexp"
	"sriov-network-metrics-exporter/pkg/utils"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/kubernetes/pkg/kubelet/apis/podresources"
	podresourcesapi "k8s.io/kubernetes/pkg/kubelet/apis/podresources/v1alpha1"

	"log"
	"time"
)

var (
	defaultPodResourcesMaxSize = 1024 * 1024 * 16 // 16 Mb
	podDevLinkName             = "kubepoddevice"
	podResourcesPath           = flag.String("path.kubeletSocket", "/var/lib/kubelet/pod-resources/kubelet.sock", "Path to kubelet resources socket")
	pciAddressPattern          = regexp.MustCompile(`^[[:xdigit:]]{4}:([[:xdigit:]]{2}):([[:xdigit:]]{2})\.([[:xdigit:]])$`)
)

//init runs the registration for this collector on package import
func init() {
	register(podDevLinkName, disabled, createPodDevLinkCollector)
}

//podDevLinkCollector the basic type used to collect information on kubernetes device links
type podDevLinkCollector struct {
	name string
}

func createPodDevLinkCollector() prometheus.Collector {
	return podDevLinkCollector{
		name: podDevLinkName,
	}
}

//This collector starts by making a call to the kubelet api which could create a delay.
// This information could be cached on a loop after the previous call to improve prometheus scraping performance.

//Collect scrapes the kubelet api and structures the returned value into a prometheus info metric.
func (c podDevLinkCollector) Collect(ch chan<- prometheus.Metric) {
	resources := PodResources()
	for _, podRes := range resources {
		podName := podRes.GetName()
		podNamespace := podRes.GetNamespace()
		for _, contRes := range podRes.Containers {
			contName := contRes.GetName()
			for _, devices := range contRes.GetDevices() {
				devType := devices.ResourceName
				for _, dev := range devices.DeviceIds {
					if !isPci(dev) {
						continue
					}
					devID := dev
					desc := prometheus.NewDesc(
						prometheus.BuildFQName(collectorNamespace, "", c.name),
						c.name,
						[]string{"pciAddr", "dev_type", "pod", "namespace", "container"}, nil,
					)
					ch <- prometheus.MustNewConstMetric(
						desc,
						prometheus.CounterValue,
						1,
						devID,
						devType,
						podName,
						podNamespace,
						contName,
					)
				}
			}
		}
	}
}

//PodResources uses the kubernetes kubelet api to get information about the devices and the pods they are attached to.
//We create and close a new connection here on each run. The performance impact of this seems marginal - but sharing a connection might save cpu time
func PodResources() []*podresourcesapi.PodResources {
	var podResource []*podresourcesapi.PodResources
	kubeletSocket := strings.Join([]string{"unix:///", *podResourcesPath}, "")
	client, conn, err := podresources.GetClient(kubeletSocket, 10*time.Second, defaultPodResourcesMaxSize)
	if err != nil {
		log.Print(err)
		return podResource
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := client.List(ctx, &podresourcesapi.ListPodResourcesRequest{})
	if err != nil {
		log.Printf("getPodResources: failed to list pod resources, %v.Get(_) = _, %v", client, err)
		return podResource
	}
	podResource = resp.PodResources

	return podResource
}

//Describe has no defined behaviour for this collector
func (c podDevLinkCollector) Describe(ch chan<- *prometheus.Desc) {
}

//checks to see if a device id matches a pci address. If not we're able to discard it.
func isPci(id string) bool {
	return pciAddressPattern.MatchString(id)
}

func VerifyKubePodDeviceFilepaths() {
	utils.ResolvePath(podResourcesPath)
}
