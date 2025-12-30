package collectors

// pod_dev_link publishes which devices are connected to which pods in Kubernetes by querying the Kubelet api

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"regexp"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	v1 "k8s.io/kubelet/pkg/apis/podresources/v1"

	"github.com/k8snetworkplumbingwg/sriov-network-metrics-exporter/pkg/utils"
)

const (
	defaultPodResourcesMaxSize = 1024 * 1024 * 16 // 16 Mb
	kubeletConnTimeout         = 10 * time.Second
)

var (
	podDevLinkName   = "kubepoddevice"
	podResourcesPath = flag.String("path.kubeletsocket",
		"/var/lib/kubelet/pod-resources/kubelet.sock", "Path to kubelet resources socket")
	pciAddressPattern = regexp.MustCompile(`^[[:xdigit:]]{4}:[[:xdigit:]]{2}:[[:xdigit:]]{2}\.\d$`)
)

// podDevLinkCollector the basic type used to collect information on kubernetes device links
type podDevLinkCollector struct {
	name string
}

// init runs the registration for this collector on package import
func init() {
	register(podDevLinkName, disabled, createPodDevLinkCollector)
}

// This collector starts by making a call to the kubelet api which could create a delay.
// This information could be cached on a loop after the previous call to improve prometheus scraping performance.

// Collect scrapes the kubelet api and structures the returned value into a prometheus info metric.
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

					desc := prometheus.NewDesc(
						prometheus.BuildFQName(collectorNamespace, "", c.name),
						c.name,
						[]string{"pciAddr", "dev_type", "pod", "namespace", "container"}, nil,
					)

					ch <- prometheus.MustNewConstMetric(
						desc,
						prometheus.CounterValue,
						1,
						dev,
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

// Describe has no defined behavior for this collector
func (c podDevLinkCollector) Describe(ch chan<- *prometheus.Desc) {
}

func createPodDevLinkCollector() prometheus.Collector {
	return podDevLinkCollector{
		name: podDevLinkName,
	}
}

// PodResources uses the kubernetes kubelet api to get information about the devices
// and the pods they are attached to.
// We create and close a new connection here on each run. The performance impact of this
// seems marginal - but sharing a connection might save cpu time
func PodResources() []*v1.PodResources {
	var podResource []*v1.PodResources

	kubeletSocket := "unix:///" + *podResourcesPath
	client, conn, err := GetV1Client(kubeletSocket, kubeletConnTimeout, defaultPodResourcesMaxSize)
	if err != nil {
		log.Print(err)
		return podResource
	}

	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("failed to close connection: %v", err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), kubeletConnTimeout)
	defer cancel()
	resp, err := client.List(ctx, &v1.ListPodResourcesRequest{})
	if err != nil {
		log.Printf("getPodResources: failed to list pod resources, %v.Get(_) = _, %v", client, err)
		return podResource
	}

	podResource = resp.PodResources

	return podResource
}

// Checks to see if a device id matches a pci address. If not we're able to discard it.
func isPci(id string) bool {
	return pciAddressPattern.MatchString(id)
}

func resolveKubePodDeviceFilepaths() error {
	if err := utils.ResolveFlag("path.kubeletsocket", podResourcesPath); err != nil {
		return err
	}

	return nil
}

// GetV1Client returns a client for the PodResourcesLister grpc service
// Extracted from package k8s.io/kubernetes/pkg/kubelet/apis/podresources client.go v1.24.3
// This is what is recommended for consumers of this package
func GetV1Client(socket string, connectionTimeout time.Duration, maxMsgSize int) (v1.PodResourcesListerClient, *grpc.ClientConn, error) {
	parsedURL, err := url.Parse(socket)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, parsedURL.Path,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)))
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing socket %s: %v", socket, err)
	}
	return v1.NewPodResourcesListerClient(conn), conn, nil
}

func dialer(ctx context.Context, addr string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", addr)
}
