package main

import (
	"context"
	"encoding/base64"
	"flag"
	"io/ioutil"
	"log"
	"net/http"

	"gopkg.in/yaml.v3"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

var (
	kubeconfigPath = flag.String("kubeconfig", "kube", "Path to kubeconfig yaml file")
)

type KubeConfig struct {
	Contexts []struct {
		Name    string `yaml:"name"`
		Context struct {
			Cluster string `yaml:"cluster"`
			User    string `yaml:"user"`
		} `yaml:"context"`
	} `yaml:"contexts"`
	Clusters []struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server                   string `yaml:"server"`
			CertificateAuthorityData string `yaml:"certificate-authority-data"`
		} `yaml:"cluster"`
	} `yaml:"clusters"`
	Users []struct {
		Name string `yaml:"name"`
		User struct {
			ClientCertificateData string `yaml:"client-certificate-data"`
			ClientKeyData         string `yaml:"client-key-data"`
		} `yaml:"user"`
	} `yaml:"users"`
}

func loadKubeConfig(path string) (*api.Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var kcfg api.Config
	if err := yaml.Unmarshal(data, &kcfg); err == nil {
		return &kcfg, nil
	}
	// fallback: parse as our struct and convert to api.Config
	var ycfg KubeConfig
	if err := yaml.Unmarshal(data, &ycfg); err != nil {
		return nil, err
	}
	acfg := &api.Config{
		Clusters: map[string]*api.Cluster{},
		Contexts: map[string]*api.Context{},
		AuthInfos: map[string]*api.AuthInfo{},
		CurrentContext: "",
	}
	for _, c := range ycfg.Clusters {
		caData, _ := base64.StdEncoding.DecodeString(c.Cluster.CertificateAuthorityData)
		acfg.Clusters[c.Name] = &api.Cluster{
			Server: c.Cluster.Server,
			CertificateAuthorityData: caData,
		}
	}
	for _, u := range ycfg.Users {
		certData, _ := base64.StdEncoding.DecodeString(u.User.ClientCertificateData)
		keyData, _ := base64.StdEncoding.DecodeString(u.User.ClientKeyData)
		acfg.AuthInfos[u.Name] = &api.AuthInfo{
			ClientCertificateData: certData,
			ClientKeyData:         keyData,
		}
	}
	for _, ctx := range ycfg.Contexts {
		acfg.Contexts[ctx.Name] = &api.Context{
			Cluster: ctx.Context.Cluster,
			AuthInfo: ctx.Context.User,
		}
	}
	return acfg, nil
}

type NodeInfoCollector struct {
	kubeConfig *api.Config
	descs      map[string]*prometheus.Desc
}

func NewNodeInfoCollector(cfg *api.Config) *NodeInfoCollector {
	descs := map[string]*prometheus.Desc{
		"info": prometheus.NewDesc(
			"k8s_node_info",
			"Kubernetes node info",
			[]string{"context", "node", "osImage", "operatingSystem", "kubeletVersion", "kernelVersion", "containerRuntimeVersion", "architecture"},
			nil,
		),
		"condition": prometheus.NewDesc(
			"k8s_node_condition",
			"Kubernetes node condition",
			[]string{"context", "node", "type", "status"},
			nil,
		),
		"capacity": prometheus.NewDesc(
			"k8s_node_capacity",
			"Kubernetes node capacity",
			[]string{"context", "node", "resource"},
			nil,
		),
		"allocatable": prometheus.NewDesc(
			"k8s_node_allocatable",
			"Kubernetes node allocatable",
			[]string{"context", "node", "resource"},
			nil,
		),
	}
	return &NodeInfoCollector{kubeConfig: cfg, descs: descs}
}

func (c *NodeInfoCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descs {
		ch <- d
	}
}

func (c *NodeInfoCollector) Collect(ch chan<- prometheus.Metric) {
	for ctxName := range c.kubeConfig.Contexts {
		clientConfig := clientcmd.NewNonInteractiveClientConfig(*c.kubeConfig, ctxName, nil, nil)
		restConfig, err := clientConfig.ClientConfig()
		if err != nil {
			log.Printf("failed to get rest config for context %s: %v", ctxName, err)
			continue
		}
		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			log.Printf("failed to create clientset for context %s: %v", ctxName, err)
			continue
		}
		nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			log.Printf("failed to list nodes for context %s: %v", ctxName, err)
			continue
		}
		for _, node := range nodes.Items {
			ch <- prometheus.MustNewConstMetric(
				c.descs["info"],
				prometheus.GaugeValue,
				1,
				ctxName,
				node.Name,
				node.Status.NodeInfo.OSImage,
				node.Status.NodeInfo.OperatingSystem,
				node.Status.NodeInfo.KubeletVersion,
				node.Status.NodeInfo.KernelVersion,
				node.Status.NodeInfo.ContainerRuntimeVersion,
				node.Status.NodeInfo.Architecture,
			)
			for _, cond := range node.Status.Conditions {
				ch <- prometheus.MustNewConstMetric(
					c.descs["condition"],
					prometheus.GaugeValue,
					boolToFloat(cond.Status == "True"),
					ctxName,
					node.Name,
					string(cond.Type),
					string(cond.Status),
				)
			}
			for res, val := range node.Status.Capacity {
				f, _ := val.AsInt64()
				ch <- prometheus.MustNewConstMetric(
					c.descs["capacity"],
					prometheus.GaugeValue,
					float64(f),
					ctxName,
					node.Name,
					string(res),
				)
			}
			for res, val := range node.Status.Allocatable {
				f, _ := val.AsInt64()
				ch <- prometheus.MustNewConstMetric(
					c.descs["allocatable"],
					prometheus.GaugeValue,
					float64(f),
					ctxName,
					node.Name,
					string(res),
				)
			}
		}
	}
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func main() {
	flag.Parse()
	cfg, err := loadKubeConfig(*kubeconfigPath)
	if err != nil {
		log.Fatalf("failed to load kubeconfig: %v", err)
	}
	collector := NewNodeInfoCollector(cfg)
	prometheus.MustRegister(collector)
	http.Handle("/metrics", promhttp.Handler())
	log.Println("Starting exporter on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("failed to start http server: %v", err)
	}
}
