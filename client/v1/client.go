package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/rest"
)

const (
	Group = "monitoring.rancher.com"
)

var Version = "v1"

type ClustersManagerV1Interface interface {
	RESTClient() rest.Interface
	ClustersGetter
}

type ClustersManagerV1Client struct {
	restClient    rest.Interface
	dynamicClient *dynamic.Client
}

func (c *ClustersManagerV1Client) Clusters() ClusterInterface {
	return newClusters(c.restClient, c.dynamicClient)
}

func (c *ClustersManagerV1Client) RESTClient() rest.Interface {
	return c.restClient
}

func NewForConfig(apiGroup string, c *rest.Config) (*ClustersManagerV1Client, error) {
	config := *c
	SetConfigDefaults(apiGroup, &config)
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewClient(&config)
	if err != nil {
		return nil, err
	}

	return &ClustersManagerV1Client{client, dynamicClient}, nil
}

func SetConfigDefaults(apiGroup string, config *rest.Config) {
	config.GroupVersion = &schema.GroupVersion{
		Group:   apiGroup,
		Version: Version,
	}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: api.Codecs}
	return
}
