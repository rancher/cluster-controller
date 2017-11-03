package controller

import (
	"fmt"
	"time"

	client "github.com/rancher/cluster-controller/client"
	"github.com/rancher/cluster-controller/client/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	ResyncPeriod = 1 * time.Minute
)

type ControllerConfig struct {
	ClientSet           *client.V1
	ClusterInformer     cache.SharedIndexInformer
	ClusterNodeInformer cache.SharedIndexInformer
}

type Controller interface {
	GetName() string
	Run(stopc <-chan struct{}) error
	Init(config *ControllerConfig)
	Shutdown()
}

var (
	controllers map[string]Controller
)

func GetControllers() map[string]Controller {
	return controllers
}

func RegisterController(name string, controller Controller) error {
	if controllers == nil {
		controllers = make(map[string]Controller)
	}
	if _, exists := controllers[name]; exists {
		return fmt.Errorf("controller already registered")
	}
	controllers[name] = controller
	return nil
}

func NewControllerConfig(config string) (*ControllerConfig, error) {
	clientSet, err := client.NewClientSetV1(config)
	if err != nil {
		return nil, err
	}

	controllerCfg := &ControllerConfig{
		ClientSet: clientSet,
	}

	controllerCfg.ClusterInformer = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  clientSet.ClusterClientV1.Clusters().List,
			WatchFunc: clientSet.ClusterClientV1.Clusters().Watch,
		},
		&v1.Cluster{}, ResyncPeriod, cache.Indexers{},
	)

	controllerCfg.ClusterNodeInformer = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  clientSet.ClusterClientV1.ClusterNodes().List,
			WatchFunc: clientSet.ClusterClientV1.ClusterNodes().Watch,
		},
		&v1.ClusterNode{}, ResyncPeriod, cache.Indexers{},
	)

	return controllerCfg, nil
}

func (c *ControllerConfig) Run(stopc <-chan struct{}) error {
	c.ClusterInformer.Run(stopc)
	c.ClusterNodeInformer.Run(stopc)
	return nil
}
