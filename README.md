# SR-IOV Network Metrics Exporter
Exporter that reads metrics for [SR-IOV Virtual Functions](https://www.intel.com/content/dam/doc/white-paper/pci-sig-single-root-io-virtualization-support-in-virtualization-technology-for-connectivity-paper.pdf) and exposes them in the Prometheus format.

The SR-IOV Network Metrics Exporter is designed with the Kubernetes SR-IOV stack in mind, including the [SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-cni) and the [SR-IOV Network Device Plugin](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin). Only devices allocated with the SR-IOV Network Device Plugin will be able to be matched to specific Virtual Function metrics.

**This software is a pre-production alpha version and should not be deployed to production servers.**

## Hardware support
The default netlink implementation for Virtual Function telemetry relies on driver support and a kernel version of 4.4 or higher. This version requires i40e driver of 2.11+ for Intel® 700 series NICs. Updated i40e drivers can be fould at the [Intel Download Center](https://downloadcenter.intel.com/download/24411/Intel-Network-Adapter-Driver-for-PCIe-40-Gigabit-Ethernet-Network-Connections-under-Linux-?v=t)

For kernels older than 4.4 a driver specific collector is enabled which is compatible with Intel® 700 series NICs using and i40e driver of 2.11 or above. To check your current driver version run: ``modinfo i40e | grep ^version``
To upgrade visit the [official driver download site](https://downloadcenter.intel.com/download/24411/Intel-Network-Adapter-Driver-for-PCIe-40-Gigabit-Ethernet-Network-Connections-Under-Linux-).
To use this version the flag collector.netlink must be set to "false".

## Metrics
This exporter will make the following metrics available:

- **sriov_vf_rx_bytes:** Received bytes per virtual function
- **sriov_vf_tx_bytes:** Transmitted bytes per virtual function
- **sriov_vf_rx_packets:** Received packets per virtual function
- **sriov_vf_tx_packets:** Transmitted packets per virtual function
- **sriov_vf_rx_dropped:** Dropped packets on receipt per virtual function
- **sriov_vf_tx_dropped:** Dropped packets on transmit per virtual function
- **sriov_vf_tx_errors:** Transmit errors per virtual function
- **kubepoddevice:** Virtual functions linked to active pods
- **kubepodcpu:** CPUs linked to pods (Guaranteed Pods managed by CPU Manager Static policy only)

## Usage
Once the SR-IOV Network Metrics Exporter is up and running metrics can be queried in the usual way from Prometheus.
The following PromQL query returns virtual function metrics with the name and namespace of the Pod it is attached to:
```
(sriov_vf_tx_errors * on (pciAddr)  group_left(pod,namespace)  sriov_kubepoddevice)
```
To get more detailed information about the pod the above can be joined with information from [Kube State Metrics](https://github.com/kubernetes/kube-state-metrics).

For example, to get the VF along with the application name from the standard Kubernetes pod label:
```
(sriov_vf_tx_errors * on (pciAddr)  group_left(pod,namespace)  sriov_kubepoddevice) * on (pod,namespace) group_left (label_app_kubernetes_io_name) kube_pod_labels
```

Once available through Prometheus VF metrics can be used by metrics applications like Grafana, or the Horizontal Pod Autoscaler.

## Installation
### Kubernetes installation
Typical deployment is as a daemonset in a cluster. A daemonset requires the image to be available on each node in the cluster or at a registry accessible from each node.
The following assumes a local Docker registry available at localhost:5000, and assumes Docker is being used to build and manage containers in the cluster.

In order to build the container and load it to a local registry run:

```
docker build . -t localhost:5000/sriov-metrics-exporter && docker push localhost:5000/sriov-metrics-exporter
```

The above assumes a registry available across the cluster at localhost:5000, for example on using the [Docker Registry Proxy](https://github.com/kubernetes-sigs/kubespray/blob/master/roles/kubernetes-apps/registry/README.md). If your registry is at a different address the image name will need to be changed to reflect that in the [Kubernetes daemonset](/deployment/daemonset.yaml)

Once the image is available from each node in the cluster run:

```
kubectl apply -f deployment/daemonset.yaml
```
This will create the daemonset and set it running. To ensure it's running as expected run:
```
kubectl -n monitoring exec -it $(kubectl get pods -nmonitoring -o=jsonpath={.items[0].metadata.name} -lapp.kubernetes.io/name=sriov-metrics-exporter) -- wget -O- localhost:9808/metrics
```
The output of this command - which pulls data from the endpoint of the first instance of SR-IOV Network Metrics Exporter - should look something like:
```
sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.0",pf="ens785f2",vf="0"} 0
sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.1",pf="ens785f2",vf="1"} 0
sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.2",pf="ens785f2",vf="2"} 0
sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.3",pf="ens785f2",vf="3"} 0
sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.4",pf="ens785f2",vf="4"} 0
sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.5",pf="ens785f2",vf="5"} 0
sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.6",pf="ens785f2",vf="6"} 0
sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.7",pf="ens785f2",vf="7"} 0
```
The above may show other metrics if there are no applicable SR-IOV Virtual Functions available on the system. Any metrics at all shows the pod is up and running and exposing metrics.

#### Configuring Prometheus for in-Cluster installation
In order to expose these metrics to Prometheus we need to configure the database to scrape our new endpoint. With the service contained in the daemonset file this can be done by adding:

```
      - job_name: 'sriov-metrics'
        kubernetes_sd_configs:
        - role: endpoints
        relabel_configs:
        - source_labels: [__meta_kubernetes_endpoint_node_name]
          target_label: instance
        - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_target]
          action: keep
          regex: true
        static_configs:
        - targets: ['sriov-metrics-exporter.monitoring.svc.cluster.local']
        scheme: http
```
The above should be added to the Prometheus configuration as a new target. For more about configuring Prometheus see the [official guide.](https://prometheus.io/docs/prometheus/latest/configuration/configuration/) Once Prometheus is started with this included in its config sriov-metrics should appear on the "Targets page". Metrics should be available by querying the Prometheus API or in the web interface.

In this mode it will serve stats on an endpoint inside the cluster. Prometheus will detect the label on the service endpoint throught the above configuration.

### Standalone installation to an endpoint on the host. 

To run as standalone the SR-IOV Metrics exporter will have to be run on each host in the cluster.
Go 1.14+ is required to build the exporter. 
Run:
```make build```
The binary should then be started on each relevant host in the cluster. Once running hitting the endpoint with:

```curl localhost:9808/metrics```

Will produce a list of metrics looking something like:
 ```
 sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.0",pf="ens785f2",vf="0"} 0
 sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.1",pf="ens785f2",vf="1"} 0
 sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.2",pf="ens785f2",vf="2"} 0
 sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.3",pf="ens785f2",vf="3"} 0
 sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.4",pf="ens785f2",vf="4"} 0
 sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.5",pf="ens785f2",vf="5"} 0
 sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.6",pf="ens785f2",vf="6"} 0
 sriov_vf_tx_packets{numa_node="0",pciAddr="0000:02:0a.7",pf="ens785f2",vf="7"} 0
 ```
Note: The exact metrics will depend on the set up of each system, but the format will be similar.

#### Configuring Prometheus for standalone installation
With the default settings the SR-IOV Network Metrics Exporter will expose metrics at port 9808. The below configuration will tell Prometheus to scrape this port and each and every host in the cluster.

```
      - job_name: 'sriov-metrics-standalone'
        scheme: http
        kubernetes_sd_configs:
        - role: node
        relabel_configs:
        - source_labels: [__address__]
          regex: ^(.*):\d+$
          target_label: __address__
          replacement: $1:9808
        - target_label: __scheme__
          replacement: http
```

The above should be added to the Prometheus configuration as a new target. For more about configuring Prometheus see the [official guide.](https://prometheus.io/docs/prometheus/latest/configuration/configuration/) Once Prometheus is started with this included in its config sriov-metrics-standalone should appear on the "Targets page". Metrics should be available by querying the Prometheus API or the web interface.

### Configuration
A number of configuration flags can be passed to the SR-IOV Network Metrics Exporter in order to change enabled collectors, the paths it reads from and some properties of its web endpoint.

| Flag | Type | Description | Default Value |
|----|:----|:----|:----|
| collector.kubepodcpu | boolean | Enables the kubepodcpu collector | false |
| collector.kubepoddevice | boolean | Enables the kubepoddevice collector | false |
| collector.vfstats | boolean |Enables the vfstats collector |  true |
| collector.netlink | boolean |Enables using netlink for vfstats collection |  true |
| path.cpucheckpoint | string | Path for location of cpu manager checkpoint file | /var/lib/kubelet/cpu_manager_state |
| path.kubecgroup |string | Path for location of kubernetes cgroups on the host system | /sys/fs/cgroup/cpuset/kubepods/|
| path.kubeletSocket | string | Path to kubelet resources socket | /var/lib/kubelet/pod-resources/kubelet.sock |
| path.nodecpuinfo | string | Path for location of system cpu information | /sys/devices/system/node/ |
| path.sysbuspci | string | Path to sys/bus/pci on host | /sys/bus/pci/devices |
| path.sysclassnet | string | Path to sys/class/net on host | /sys/class/net/ |
| web.listen-address | string | Address to listen on for web interface and telemetry. | :9808 |
| web.rate-burst | int | Maximum per second burst rate for requests. | 10 |
| web.rate-limit | int | Limit for requests per second. | 1 |

## Communication and contribution

Report a bug by [filing a new issue](https://github.com/k8snetworkplumbingwg/sriov-network-metrics-exporter/issues).

Contribute by [opening a pull request](https://github.com/k8snetworkplumbingwg/sriov-network-metrics-exporter/pulls).

Learn [about pull requests](https://help.github.com/articles/using-pull-requests/).
