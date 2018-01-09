package clusterprovisioner

import (
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	driver "github.com/rancher/kontainer-engine/stub"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type Provisioner struct {
	machines v3.MachineLister
}

func Register(management *config.ManagementContext) {
	p := &Provisioner{
		machines: management.Management.Machines("").Controller().Lister(),
	}
	management.Management.Clusters("").AddLifecycle(p.GetName(), p)
}

func configChanged(cluster *v3.Cluster) bool {
	changed := false
	if cluster.Spec.AzureKubernetesServiceConfig != nil {
		applied := cluster.Status.AppliedSpec.AzureKubernetesServiceConfig
		current := cluster.Spec.AzureKubernetesServiceConfig
		changed = applied != nil && !reflect.DeepEqual(applied, current)
	} else if cluster.Spec.GoogleKubernetesEngineConfig != nil {
		applied := cluster.Status.AppliedSpec.GoogleKubernetesEngineConfig
		current := cluster.Spec.GoogleKubernetesEngineConfig
		changed = applied != nil && !reflect.DeepEqual(applied, current)
	} else if cluster.Spec.RancherKubernetesEngineConfig != nil {
		applied := cluster.Status.AppliedSpec.RancherKubernetesEngineConfig
		current := cluster.Spec.RancherKubernetesEngineConfig
		changed = applied != nil && !reflect.DeepEqual(applied, current)
	}

	return changed
}

func (p *Provisioner) Remove(cluster *v3.Cluster) (*v3.Cluster, error) {
	logrus.Infof("Deleting cluster [%s]", cluster.Name)
	if needToProvision(cluster) {
		for i := 0; i < 4; i++ {
			err := driver.Remove(cluster.Name, cluster.Spec)
			if err == nil {
				break
			}
			if i == 3 {
				return cluster, fmt.Errorf("failed to remove the cluster [%s]: %v", cluster.Name, err)
			}
			time.Sleep(1 * time.Second)
		}
	}
	logrus.Infof("Deleted cluster [%s]", cluster.Name)

	return nil, nil
}

func (p *Provisioner) Updated(cluster *v3.Cluster) (*v3.Cluster, error) {
	if v3.ClusterConditionProvisioned.IsTrue(cluster) && configChanged(cluster) {
		return p.reconcileCluster(cluster, false)
	}
	return nil, nil
}

func (p *Provisioner) Create(cluster *v3.Cluster) (*v3.Cluster, error) {
	if v3.ClusterConditionProvisioned.IsTrue(cluster) {
		return nil, nil
	}
	return p.reconcileCluster(cluster, true)
}

func (p *Provisioner) reconcileCluster(cluster *v3.Cluster, create bool) (*v3.Cluster, error) {
	if needToProvision(cluster) {
		newObj, err := v3.ClusterConditionProvisioned.DoUntilTrue(cluster, func() (runtime.Object, error) {
			// Update status
			cluster, err := p.Clusters.Update(cluster)
			if err != nil {
				return nil, err
			}

			logrus.Infof("Provisioning cluster [%s]", cluster.Name)
			var apiEndpoint, serviceAccountToken, caCert string
			ready, spec, err := p.reconcileSpec(cluster.Spec, cluster.Name)
			if err != nil {
				return nil, fmt.Errorf("Failed to validate machine hosts for the cluster [%s]: %v", cluster.Name, err)
			}
			if !ready {
				return nil, fmt.Errorf("Machine hosts are not ready for the cluster [%s], resubmitting the create event", cluster.Name)
			}
			if create {
				logrus.Infof("Creating cluster [%s]", cluster.Name)
				apiEndpoint, serviceAccountToken, caCert, err = driver.Create(cluster.Name, spec)
			} else {
				logrus.Infof("Updating cluster [%s]", cluster.Name)
				apiEndpoint, serviceAccountToken, caCert, err = driver.Update(cluster.Name, spec)
			}
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to provision cluster [%s]", cluster.Name)
			}

			saved := false
			for i := 0; i < 20; i++ {
				cluster, err = p.Clusters.Get(cluster.Name, metav1.GetOptions{})
				if err != nil {
					return cluster, err
				}

				cluster.Status.AppliedSpec = cluster.Spec
				cluster.Status.APIEndpoint = apiEndpoint
				cluster.Status.ServiceAccountToken = serviceAccountToken
				cluster.Status.CACert = caCert

				if cluster, err = p.Clusters.Update(cluster); err == nil {
					saved = true
					break
				} else {
					logrus.Errorf("failed to update cluster [%s]: %v", cluster.Name, err)
					time.Sleep(2)
				}
			}

			if !saved {
				return cluster, fmt.Errorf("failed to update cluster")
			}

			logrus.Infof("Provisioned cluster [%s]", cluster.Name)
			return cluster, nil
		})
		return newObj.(*v3.Cluster), err
	}

	return nil, nil
}

func (p *Provisioner) GetName() string {
	return "cluster-provisioner-controller"
}

func needToProvision(cluster *v3.Cluster) bool {
	return cluster.Spec.RancherKubernetesEngineConfig != nil || cluster.Spec.AzureKubernetesServiceConfig != nil || cluster.Spec.GoogleKubernetesEngineConfig != nil
}

func (p *Provisioner) reconcileSpec(spec v3.ClusterSpec, clusterName string) (bool, v3.ClusterSpec, error) {
	if spec.RancherKubernetesEngineConfig == nil {
		return true, spec, nil
	}
	useMachines := false
	for _, node := range spec.RancherKubernetesEngineConfig.Nodes {
		if node.MachineName != "" {
			useMachines = true
			break
		}
	}
	if !useMachines {
		return true, spec, nil
	}

	nodesReady, updatedNodes, err := p.getConfigNodes(*spec.RancherKubernetesEngineConfig, clusterName)
	if err != nil || !nodesReady {
		return false, spec, err
	}
	spec.RancherKubernetesEngineConfig.Nodes = updatedNodes

	return true, spec, nil
}

func (p *Provisioner) getConfigNodes(config v3.RancherKubernetesEngineConfig, clusterName string) (bool, []v3.RKEConfigNode, error) {
	var err error
	config.Nodes, err = p.populateClusterNodes(config, clusterName)
	if err != nil {
		return false, config.Nodes, err
	}
	return true, config.Nodes, nil
}

func (p *Provisioner) populateClusterNodes(config v3.RancherKubernetesEngineConfig, clusterName string) ([]v3.RKEConfigNode, error) {
	populatedNodes := []v3.RKEConfigNode{}
	allMachines, err := p.machines.List("", labels.NewSelector())
	if err != nil {
		return populatedNodes, err
	}
	machineMap := make(map[string]v3.Machine)
	for _, machine := range allMachines {
		machineMap[machine.Name] = *machine
	}
	for _, node := range config.Nodes {
		if len(node.MachineName) != 0 {
			if _, ok := machineMap[node.MachineName]; !ok {
				return populatedNodes, fmt.Errorf("Machine [%s] does not exist for cluster [%s]", node.MachineName, clusterName)
			}
			machine := machineMap[node.MachineName]
			if len(node.User) == 0 {
				// Check if machine is ready
				readyMachine, err := p.waitForMachineToBeProvisioned(&machine)
				if err != nil {
					return populatedNodes, err
				}
				// Populate node
				node.Address = readyMachine.Status.NodeConfig.Address
				node.SSHKey = readyMachine.Status.NodeConfig.SSHKey
				node.User = readyMachine.Status.SSHUser
				node.HostnameOverride = readyMachine.Spec.RequestedHostname
			}
			node.MachineName = machine.Name
			populatedNodes = append(populatedNodes, node)
		}
	}

	return populatedNodes, nil
}

func (p *Provisioner) waitForMachineToBeProvisioned(machine *v3.Machine) (*v3.Machine, error) {
	for retryCount := 0; retryCount < 5; retryCount++ {
		allMachines, err := p.machines.List("", labels.NewSelector())
		if err != nil {
			return machine, err
		}
		for _, m := range allMachines {
			if machine.Name == m.Name {
				if checkMachineConditionProvisioned(m.Name, m.Status.Conditions) {
					return m, nil
				}
			}
		}
		time.Sleep(10 * time.Second)
	}
	return machine, fmt.Errorf("Timeout waiting for machine [%s] to be provisioned", machine.Name)
}

func checkMachineConditionProvisioned(machineName string, machineConditions []v3.MachineCondition) bool {
	for _, condition := range machineConditions {
		if condition.Type == v3.MachineConditionProvisioned {
			if condition.Status == v1.ConditionTrue {
				logrus.Debugf("Machine is ready: %v", machineName)
				return true
			}
		}
	}
	return false
}
