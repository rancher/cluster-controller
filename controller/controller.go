package controller

import (
	"context"
	"fmt"
	"time"

	client "github.com/rancher/cluster-controller/client"
	clusterv1 "github.com/rancher/types/apis/cluster.cattle.io/v1"
)

const (
	ResyncPeriod = 1 * time.Minute
)

type Config struct {
	ClientSet             *client.V1
	ClusterController     clusterv1.ClusterController
	ClusterNodeController clusterv1.ClusterNodeController
}

type Controller interface {
	GetName() string
	Start(config *Config)
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

	controllerCfg.ClusterController = clientSet.ClusterClientV1.Clusters("").Controller()
	controllerCfg.ClusterNodeController = clientSet.ClusterClientV1.ClusterNodes("").Controller()

	return controllerCfg, nil
}

func (c *Config) Run(ctx context.Context) error {
	err := c.ClusterController.Start(1, ctx)
	if err != nil {
		return err
	}

	err = c.ClusterNodeController.Start(1, ctx)
	if err != nil {
		return err
	}
	return nil
}
