package controller

import (
	"github.com/rancher/cluster-controller/controller/clusterheartbeat"
	"github.com/rancher/cluster-controller/controller/clusterprovisioner"
	"github.com/rancher/cluster-controller/controller/clusterstats"
	"github.com/rancher/types/config"
)

func Register(management *config.ManagementContext) {
	clusterheartbeat.Register(management)
	clusterprovisioner.Register(management)
	clusterstats.Register(management)
}
