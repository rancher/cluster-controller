package machinessyncer

import (
	"fmt"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	finalizerName = "machinesSyncer"
)

type Syncer struct {
	Machines       v3.MachineInterface
	clusters       v3.ClusterLister
	clustersClient v3.ClusterInterface
}

func Register(management *config.ManagementContext) {
	s := &Syncer{
		clusters:       management.Management.Clusters("").Controller().Lister(),
		clustersClient: management.Management.Clusters(""),
	}
	management.Management.Machines("").AddLifecycle("machinesSyncer", s)
}

func getClusterName(machine *v3.Machine) string {
	cluster := machine.Status.ClusterName
	if cluster == "" {
		cluster = machine.Spec.RequestedClusterName
	}
	return cluster
}

func (s *Syncer) Updated(machine *v3.Machine) (*v3.Machine, error) {
	return nil, nil
}

func (s *Syncer) Create(machine *v3.Machine) (*v3.Machine, error) {
	clusterName := getClusterName(machine)
	if clusterName == "" {
		return nil, nil
	}
	if machine.Spec.MachineTemplateName == "" {
		// regular, non machine provisioned host
		return nil, nil
	}

	if machine.Status.NodeConfig == nil {
		logrus.Debugf("Machine node [%s] for cluster [%s] is not provisioned yet", machine.Name, clusterName)
		return nil, nil
	}

	cluster, err := s.clusters.Get("", clusterName)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, fmt.Errorf("Cluster [%s] does not exist", clusterName)
	}

	if cluster.Spec.RancherKubernetesEngineConfig == nil {
		return nil, fmt.Errorf("Cluster [%s] can not accept nodes provisioned by machine", clusterName)
	}

	// add machine to RKE node if needed
	var updatedNodes []v3.RKEConfigNode
	needToAdd := true
	for _, node := range cluster.Spec.RancherKubernetesEngineConfig.Nodes {
		if node.MachineName == machine.Name {
			machine.Status.NodeConfig.MachineName = machine.Name
			machine.Status.NodeConfig.Role = machine.Spec.RequestedRoles
			machine.Status.NodeConfig.HostnameOverride = machine.Spec.RequestedHostname
			updatedNodes = append(updatedNodes, *machine.Status.NodeConfig)
			needToAdd = false
			continue
		}
		updatedNodes = append(updatedNodes, node)
	}
	if needToAdd {
		machine.Status.NodeConfig.MachineName = machine.Name
		machine.Status.NodeConfig.Role = machine.Spec.RequestedRoles
		machine.Status.NodeConfig.HostnameOverride = machine.Spec.RequestedHostname
		updatedNodes = append(updatedNodes, *machine.Status.NodeConfig)
	}
	cluster.Spec.RancherKubernetesEngineConfig.Nodes = updatedNodes
	_, err = s.clustersClient.Update(cluster)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *Syncer) Remove(machine *v3.Machine) (*v3.Machine, error) {
	cluster, err := s.clusters.Get("", getClusterName(machine))
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	var updatedNodes []v3.RKEConfigNode
	updated := false
	for _, node := range cluster.Spec.RancherKubernetesEngineConfig.Nodes {
		if node.MachineName == machine.Name {
			logrus.Infof("Removing machine [%s] from cluster [%s]", machine.Name, cluster.Name)
			updated = true
			continue
		}
		updatedNodes = append(updatedNodes, node)
	}
	if updated {
		cluster.Spec.RancherKubernetesEngineConfig.Nodes = updatedNodes
		_, err := s.clustersClient.Update(cluster)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}
