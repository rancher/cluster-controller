package main

import (
	// controllers
	_ "github.com/rancher/cluster-controller/controller/clusterheartbeat"
	_ "github.com/rancher/cluster-controller/controller/clusterprovisioner"
	_ "github.com/rancher/cluster-controller/controller/clusterstats"
)
