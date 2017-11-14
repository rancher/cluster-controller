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
	Run(ctx context.Context) error
	Init(config string) error
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
