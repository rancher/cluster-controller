package controller

import (
	"github.com/rancher/cluster-controller/controller/clusterheartbeat"
	"github.com/rancher/cluster-controller/controller/clusterprovisioner"
	"github.com/rancher/cluster-controller/controller/clusterstats"
	"github.com/rancher/types/config"
)

func Register(cluster *config.ClusterContext) {
	clusterheartbeat.Register(cluster)
	clusterprovisioner.Register(cluster)
	clusterstats.Register(cluster)
}
