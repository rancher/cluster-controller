package controller

import (
	"github.com/rancher/cluster-controller/client/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type OperatorClient struct {
	kclient           kubernetes.Clientset
	clusterClient     v1.ClusterInterface
	clusterNodeClient v1.ClusterNodeInterface
	clusterInf        cache.SharedIndexInformer
	clusterNodeInf    cache.SharedIndexInformer
}
