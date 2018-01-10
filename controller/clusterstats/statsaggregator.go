package clusterstats

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
)

type StatsAggregator struct {
	Machines v3.MachineLister
}

type MachineSyncer struct {
	Clusters v3.ClusterInterface
}

type ClusterNodeData struct {
	Capacity                        v1.ResourceList
	Allocatable                     v1.ResourceList
	Requested                       v1.ResourceList
	Limits                          v1.ResourceList
	ConditionNoDiskPressureStatus   v1.ConditionStatus
	ConditionNoMemoryPressureStatus v1.ConditionStatus
}

func Register(management *config.ManagementContext) {
	clustersClient := management.Management.Clusters("")
	machinesClient := management.Management.Machines("")
	m := &MachineSyncer{
		Clusters: clustersClient,
	}
	machinesClient.AddLifecycle(m.GetName(), m)

	s := &StatsAggregator{
		Machines: management.Management.Machines("").Controller().Lister(),
	}
	clustersClient.AddLifecycle(s.GetName(), s)
}

func (s *StatsAggregator) Create(cluster *v3.Cluster) (*v3.Cluster, error) {
	return s.update(cluster)
}

func (s *StatsAggregator) Updated(cluster *v3.Cluster) (*v3.Cluster, error) {
	return s.update(cluster)
}

func (s *StatsAggregator) Remove(cluster *v3.Cluster) (*v3.Cluster, error) {
	return s.update(cluster)
}

func (s *StatsAggregator) update(cluster *v3.Cluster) (*v3.Cluster, error) {
	err := s.aggregate(cluster, cluster.Name)
	if err != nil {
		return nil, err
	}
	return cluster, nil
}

func (s *StatsAggregator) aggregate(cluster *v3.Cluster, clusterName string) error {
	machines, err := s.Machines.List("", labels.Everything())
	if err != nil {
		return err
	}

	// capacity keys
	pods, mem, cpu := resource.Quantity{}, resource.Quantity{}, resource.Quantity{}
	// allocatable keys
	apods, amem, acpu := resource.Quantity{}, resource.Quantity{}, resource.Quantity{}
	// requested keys
	rpods, rmem, rcpu := resource.Quantity{}, resource.Quantity{}, resource.Quantity{}
	// limited keys
	lpods, lmem, lcpu := resource.Quantity{}, resource.Quantity{}, resource.Quantity{}

	condDisk := v1.ConditionTrue
	condMem := v1.ConditionTrue

	for _, machine := range machines {
		if clusterName != machine.Status.ClusterName {
			continue
		}

		capacity := machine.Status.NodeStatus.Capacity
		if capacity != nil {
			pods.Add(*capacity.Pods())
			mem.Add(*capacity.Memory())
			cpu.Add(*capacity.Cpu())
		}
		allocatable := machine.Status.NodeStatus.Allocatable
		if allocatable != nil {
			apods.Add(*allocatable.Pods())
			amem.Add(*allocatable.Memory())
			acpu.Add(*allocatable.Cpu())
		}
		requested := machine.Status.Requested
		if requested != nil {
			rpods.Add(*requested.Pods())
			rmem.Add(*requested.Memory())
			rcpu.Add(*requested.Cpu())
		}
		limits := machine.Status.Limits
		if limits != nil {
			lpods.Add(*limits.Pods())
			lmem.Add(*limits.Memory())
			lcpu.Add(*limits.Cpu())
		}

		if condDisk == v1.ConditionTrue && v3.ClusterConditionNoDiskPressure.IsTrue(machine) {
			condDisk = v1.ConditionFalse
		}
		if condMem == v1.ConditionTrue && v3.ClusterConditionNoMemoryPressure.IsTrue(machine) {
			condMem = v1.ConditionFalse
		}
	}

	cluster.Status.Capacity = v1.ResourceList{v1.ResourcePods: pods, v1.ResourceMemory: mem, v1.ResourceCPU: cpu}
	cluster.Status.Allocatable = v1.ResourceList{v1.ResourcePods: apods, v1.ResourceMemory: amem, v1.ResourceCPU: acpu}
	cluster.Status.Requested = v1.ResourceList{v1.ResourcePods: rpods, v1.ResourceMemory: rmem, v1.ResourceCPU: rcpu}
	cluster.Status.Limits = v1.ResourceList{v1.ResourcePods: lpods, v1.ResourceMemory: lmem, v1.ResourceCPU: lcpu}
	if condDisk == v1.ConditionTrue {
		v3.ClusterConditionNoDiskPressure.True(cluster)
	} else {
		v3.ClusterConditionNoDiskPressure.False(cluster)
	}
	if condMem == v1.ConditionTrue {
		v3.ClusterConditionNoMemoryPressure.True(cluster)
	} else {
		v3.ClusterConditionNoMemoryPressure.False(cluster)
	}

	return nil
}

func (s *StatsAggregator) GetName() string {
	return "cluster-stats-controller"
}

func (m *MachineSyncer) Create(machine *v3.Machine) (*v3.Machine, error) {
	return m.sync(machine)
}

func (m *MachineSyncer) Updated(machine *v3.Machine) (*v3.Machine, error) {
	return m.sync(machine)
}

func (m *MachineSyncer) Remove(machine *v3.Machine) (*v3.Machine, error) {
	return m.sync(machine)
}

func (m *MachineSyncer) sync(machine *v3.Machine) (*v3.Machine, error) {
	m.Clusters.Controller().Enqueue("", machine.ClusterName)
	return nil, nil
}

func (m *MachineSyncer) GetName() string {
	return "cluster-node-controller"
}
