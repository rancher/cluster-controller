package controller

import (
	"fmt"
	"time"

	client "github.com/rancher/cluster-controller/client"
	"k8s.io/client-go/tools/cache"
)

const (
	ResyncPeriod = 1 * time.Minute
)

type Config struct {
	ClientSet           *client.V1
	ClusterInformer     cache.SharedIndexInformer
	ClusterNodeInformer cache.SharedIndexInformer
}

type Controller interface {
	GetName() string
	Run(stopc <-chan struct{}) error
	Init(config *Config)
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

func NewControllerConfig(config string) (*Config, error) {
	clientSet, err := client.NewClientSetV1(config)
	if err != nil {
		return nil, err
	}

	controllerCfg := &Config{
		ClientSet: clientSet,
	}

	controllerCfg.ClusterInformer = clientSet.ClusterClientV1.Clusters("").Controller().Informer()
	controllerCfg.ClusterNodeInformer = clientSet.ClusterClientV1.ClusterNodes("").Controller().Informer()

	return controllerCfg, nil
}

func (c *Config) Run(stopc <-chan struct{}) error {
	c.ClusterInformer.Run(stopc)
	c.ClusterNodeInformer.Run(stopc)
	return nil
}
