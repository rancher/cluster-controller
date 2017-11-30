package clusterprovisioner

import (
	"fmt"
	"reflect"
	"time"

	driver "github.com/rancher/kontainer-engine/stub"
	clusterv1 "github.com/rancher/types/apis/cluster.cattle.io/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RemoveAction = "Remove"
	UpdateAction = "Update"
	CreateAction = "Create"
	NoopAction   = "Noop"
)

type Provisioner struct {
	ClusterNodes clusterv1.ClusterNodeInterface
	Clusters     clusterv1.ClusterInterface
}

func Register(cluster *config.ClusterContext) {
	p := &Provisioner{
		ClusterNodes: cluster.Cluster.ClusterNodes(""),
		Clusters:     cluster.Cluster.Clusters(""),
	}
	cluster.Cluster.Clusters("").Controller().AddHandler(p.sync)
}

func configChanged(cluster *clusterv1.Cluster) bool {
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

func getAction(cluster *clusterv1.Cluster) string {
	if cluster == nil {
		return NoopAction
	}
	if cluster.ObjectMeta.DeletionTimestamp != nil {
		return RemoveAction
	}

	if !isClusterProvisioned(cluster) {
		return CreateAction
	}

	if configChanged(cluster) {
		return UpdateAction
	}
	return NoopAction
}

func (p *Provisioner) sync(key string, cluster *clusterv1.Cluster) error {
	action := getAction(cluster)
	switch action {
	case CreateAction:
		return p.createCluster(cluster)
	case UpdateAction:
		return p.updateCluster(cluster)
	case RemoveAction:
		return p.removeCluster(cluster)
	default:
		return nil
	}
}

func (p *Provisioner) removeCluster(cluster *clusterv1.Cluster) error {
	set, first := p.finalizerSet(cluster)
	if set && first {
		logrus.Infof("Deleting cluster [%s]", cluster.Name)
		// 1. Call the driver to remove the cluster
		if needToProvision(cluster) && isClusterProvisioned(cluster) {
			for i := 0; i < 4; i++ {
				err := driver.Remove(cluster.Name, cluster.Spec)
				if err == nil {
					break
				}
				if i == 3 {
					return fmt.Errorf("Failed to remove the cluster [%s]: %v", cluster.Name, err)
				}
				time.Sleep(1 * time.Second)
			}
		}

		// 2. Remove cluster nodes
		nodes, err := p.getClusterNodes(cluster)
		if err != nil {
			return fmt.Errorf("Failed to fetch cluster nodes for cluster [%s]: %v", cluster.Name, err)
		}
		for _, node := range nodes {
			if err = p.ClusterNodes.Delete(node.Name, &metav1.DeleteOptions{}); err != nil {
				return fmt.Errorf("Failed to remove cluster node [%s] for cluster [%s]: %v", node.Name, cluster.Name, err)
			}
		}
		// 3. Remove k8s object
		cluster.ObjectMeta.Finalizers = []string{}
		_, err = p.Clusters.Update(cluster)
		if err != nil {
			return fmt.Errorf("Failed to reset finalizers for cluster [%s]: %v", cluster.Name, err)
		}
		logrus.Infof("Deleted cluster [%s]", cluster.Name)
	}

	return nil
}

func (p *Provisioner) getClusterNodes(cluster *clusterv1.Cluster) ([]clusterv1.ClusterNode, error) {
	var nodes []clusterv1.ClusterNode
	all, err := p.ClusterNodes.List(metav1.ListOptions{})
	if err != nil {
		return nodes, fmt.Errorf("Failed to fetch cluster nodes for cluster [%s]: %v", cluster.Name, err)
	}
	for _, node := range all.Items {
		if node.ClusterName == cluster.Name {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func (p *Provisioner) updateCluster(cluster *clusterv1.Cluster) error {
	err := p.preUpdateClusterStatus(cluster.Name)
	if err != nil {
		return fmt.Errorf("Failed to update status for cluster [%s]: %v", cluster.Name, err)
	}
	logrus.Infof("Updating cluster [%s]", cluster.Name)
	var apiEndpoint, serviceAccountToken, caCert string
	if needToProvision(cluster) {
		apiEndpoint, serviceAccountToken, caCert, err = driver.Update(cluster.Name, cluster.Spec)
		if err != nil {
			_ = p.postUpdateClusterStatusError(cluster, err)
			return fmt.Errorf("Failed to update the cluster [%s]: %v", cluster.Name, err)
		}
	}

	err = p.postUpdateClusterStatusSuccess(cluster, apiEndpoint, serviceAccountToken, caCert)
	if err != nil {
		return fmt.Errorf("Failed to update status for cluster [%s]: %v", cluster.Name, err)
	}
	logrus.Infof("Updated cluster [%s]", cluster.Name)
	return nil
}

func (p *Provisioner) createCluster(cluster *clusterv1.Cluster) error {
	err := p.preUpdateClusterStatus(cluster.Name)
	if err != nil {
		return fmt.Errorf("Failed to update status for cluster [%s]: %v", cluster.Name, err)
	}
	logrus.Infof("Provisioning cluster [%s]", cluster.Name)

	var apiEndpoint, serviceAccountToken, caCert string
	if needToProvision(cluster) {
		apiEndpoint, serviceAccountToken, caCert, err = driver.Create(cluster.Name, cluster.Spec)
		if err != nil {
			_ = p.postUpdateClusterStatusError(cluster, err)
			return fmt.Errorf("Failed to provision the cluster [%s]: %v", cluster.Name, err)
		}
	}

	err = p.postUpdateClusterStatusSuccess(cluster, apiEndpoint, serviceAccountToken, caCert)
	if err != nil {
		return fmt.Errorf("Failed to update status for cluster [%s]: %v", cluster.Name, err)
	}
	logrus.Infof("Provisioned cluster [%s]", cluster.Name)
	return nil
}

func (p *Provisioner) GetName() string {
	return "clusterProvisioner"
}

func (p *Provisioner) postUpdateClusterStatusError(cluster *clusterv1.Cluster, userError error) error {
	toUpdate, err := p.Clusters.Get(cluster.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	condition := newClusterCondition(clusterv1.ClusterConditionUpdating, "True", fmt.Sprintf("Failed to update cluster %s", userError.Error()))
	setClusterCondition(&toUpdate.Status, condition)
	_, err = p.Clusters.Update(toUpdate)
	return err
}

func (p *Provisioner) postUpdateClusterStatusSuccess(cluster *clusterv1.Cluster, apiEndpiont string, serviceAccountToken string, caCert string) error {
	toUpdate, err := p.Clusters.Get(cluster.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	toUpdate.Status.AppliedSpec = cluster.Spec
	toUpdate.Status.APIEndpoint = apiEndpiont
	toUpdate.Status.ServiceAccountToken = serviceAccountToken
	toUpdate.Status.CACert = caCert
	if !isClusterProvisioned(cluster) {
		condition := newClusterCondition(clusterv1.ClusterConditionProvisioned, "True", "Cluster providioned successfully")
		setClusterCondition(&toUpdate.Status, condition)
	}

	condition := newClusterCondition(clusterv1.ClusterConditionUpdating, "False", "Cluster updated successfully")
	setClusterCondition(&toUpdate.Status, condition)
	_, err = p.Clusters.Update(toUpdate)
	return err
}

func newClusterCondition(condType clusterv1.ClusterConditionType, status v1.ConditionStatus, reason string) clusterv1.ClusterCondition {
	now := time.Now().Format(time.RFC3339)
	return clusterv1.ClusterCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     now,
		LastTransitionTime: now,
		Reason:             reason,
	}
}

func (p *Provisioner) preUpdateClusterStatus(clusterName string) error {
	toUpdate, err := p.Clusters.Get(clusterName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if toUpdate.Status.Conditions == nil {
		// init conditions
		conditions := []clusterv1.ClusterCondition{}
		conditions = append(conditions, newClusterCondition(clusterv1.ClusterConditionNoMemoryPressure, "Unknown", ""))
		conditions = append(conditions, newClusterCondition(clusterv1.ClusterConditionNoDiskPressure, "Unknown", ""))
		conditions = append(conditions, newClusterCondition(clusterv1.ClusterConditionReady, "Unknown", ""))
		conditions = append(conditions, newClusterCondition(clusterv1.ClusterConditionUpdating, "True", ""))
		conditions = append(conditions, newClusterCondition(clusterv1.ClusterConditionProvisioned, "False", ""))
		toUpdate.Status.Conditions = conditions
		toUpdate.Status.ComponentStatuses = []clusterv1.ClusterComponentStatus{}
	} else {
		condition := newClusterCondition(clusterv1.ClusterConditionUpdating, "True", "")
		setClusterCondition(&toUpdate.Status, condition)
	}

	set, _ := p.finalizerSet(toUpdate)

	if !set {
		toUpdate.ObjectMeta.Finalizers = append(toUpdate.ObjectMeta.Finalizers, p.GetName())
	}
	_, err = p.Clusters.Update(toUpdate)
	return err
}

func (p *Provisioner) finalizerSet(cluster *clusterv1.Cluster) (bool, bool) {
	i := 0
	for _, value := range cluster.ObjectMeta.Finalizers {
		if value == p.GetName() {
			return true, i == 0
		}
		i++
	}
	return false, false
}

func setClusterCondition(status *clusterv1.ClusterStatus, c clusterv1.ClusterCondition) {
	pos, cp := getClusterCondition(status, c.Type)
	if cp != nil && cp.Status == c.Status {
		return
	}

	if cp != nil {
		status.Conditions[pos] = c
	} else {
		status.Conditions = append(status.Conditions, c)
	}
}

func getClusterCondition(status *clusterv1.ClusterStatus, t clusterv1.ClusterConditionType) (int, *clusterv1.ClusterCondition) {
	for i, c := range status.Conditions {
		if t == c.Type {
			return i, &c
		}
	}
	return -1, nil
}

func isClusterProvisioned(cluster *clusterv1.Cluster) bool {
	_, isProvisioned := getClusterCondition(&cluster.Status, clusterv1.ClusterConditionProvisioned)
	if isProvisioned == nil {
		return false
	}
	return isProvisioned.Status == "True"
}

func needToProvision(cluster *clusterv1.Cluster) bool {
	return cluster.Spec.RancherKubernetesEngineConfig != nil || cluster.Spec.AzureKubernetesServiceConfig != nil || cluster.Spec.GoogleKubernetesEngineConfig != nil
}
