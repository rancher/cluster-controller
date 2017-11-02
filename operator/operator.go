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

type OperatorConfig struct {
	ClientSet           *client.V1
	ClusterInformer     cache.SharedIndexInformer
	ClusterNodeInformer cache.SharedIndexInformer
}

type Operator interface {
	GetName() string
	Run(stopc <-chan struct{}) error
	Init(config *OperatorConfig)
	Shutdown()
}

var (
	operators map[string]Operator
)

func GetOperators() map[string]Operator {
	return operators
}

func RegisterOperator(name string, operator Operator) error {
	if operators == nil {
		operators = make(map[string]Operator)
	}
	if _, exists := operators[name]; exists {
		return fmt.Errorf("operator already registered")
	}
	operators[name] = operator
	return nil
}

func NewOperatorConfig(config string) (*OperatorConfig, error) {
	clientSet, err := client.NewClientSetV1(config)
	if err != nil {
		return nil, err
	}

	operatorCfg := &OperatorConfig{
		ClientSet: clientSet,
	}

	operatorCfg.ClusterInformer = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  clientSet.ClusterClientV1.Clusters().List,
			WatchFunc: clientSet.ClusterClientV1.Clusters().Watch,
		},
		&v1.Cluster{}, ResyncPeriod, cache.Indexers{},
	)

	operatorCfg.ClusterNodeInformer = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  clientSet.ClusterClientV1.ClusterNodes().List,
			WatchFunc: clientSet.ClusterClientV1.ClusterNodes().Watch,
		},
		&v1.ClusterNode{}, ResyncPeriod, cache.Indexers{},
	)

	return operatorCfg, nil
}

func (c *OperatorConfig) Run(stopc <-chan struct{}) error {
	c.ClusterInformer.Run(stopc)
	c.ClusterNodeInformer.Run(stopc)
	return nil
}
