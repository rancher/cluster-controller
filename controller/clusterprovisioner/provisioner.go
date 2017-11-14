package provisioner

import (
	"github.com/sirupsen/logrus"

	"reflect"

	"fmt"

	"github.com/rancher/cluster-controller/controller"
	driver "github.com/rancher/kontainer-engine/stub"
	clusterv1 "github.com/rancher/types/apis/cluster.cattle.io/v1"
)

type Provisioner struct {
	config *controller.Config
}

func init() {
	p := &Provisioner{}
	controller.RegisterController(p.GetName(), p)
}

func (p *Provisioner) Start(config *controller.Config) {
	p.config = config
	p.config.ClusterController.AddHandler(p.sync)
}

func configChanged(cluster *clusterv1.Cluster) bool {
	changed := false
	if cluster.Status == nil {
		return true
	}
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

func (p *Provisioner) sync(key string, cluster *clusterv1.Cluster) error {
	if cluster == nil {
		// no longer exists if nil is passed to this call
		// do nothing
		return nil
	}
	// TODO check delete annotation, and call delete method if present
	// otherwise call create/update
	if configChanged(cluster) {
		return p.createOrUpdateCluster(cluster)
	}
	return nil
}

func (p *Provisioner) deleteCluster(key string) error {
	logrus.Infof("Deleting cluster [%s]", key)
	logrus.Infof("Deleted cluster [%s]", key)
	return nil
}

func (p *Provisioner) createOrUpdateCluster(cluster *clusterv1.Cluster) error {
	logrus.Infof("Updating cluster [%s]", cluster.Name)
	_, _, _, err := driver.Create(cluster.Name, cluster.Spec)
	if err != nil {
		return fmt.Errorf("Failed to provision the cluster [%s]: %v", cluster.Name, err)
	}
	logrus.Infof("Updated cluster [%s]", cluster.Name)
	return nil
}

func (p *Provisioner) GetName() string {
	return "clusterProvisioner"
}
