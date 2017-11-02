package client

import (
	"github.com/rancher/cluster-controller/client/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type V1 struct {
	CoreClientV1    *kubernetes.Clientset
	ClusterClientV1 *v1.ClustersManagerV1Client
}

func NewClientSetV1(config string) (*V1, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", config)
	if err != nil {
		return nil, err
	}
	coreClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	clusterClient, err := v1.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	clientSet := &V1{coreClient, clusterClient}
	return clientSet, nil
}
