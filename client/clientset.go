package client

import (
	clusterv1 "github.com/rancher/types/io.cattle.cluster/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type V1 struct {
	CoreClientV1    *kubernetes.Clientset
	ClusterClientV1 clusterv1.Interface
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

	clusterClient, err := clusterv1.NewForConfig(*cfg)
	if err != nil {
		return nil, err
	}

	clientSet := &V1{coreClient, clusterClient}
	return clientSet, nil
}
