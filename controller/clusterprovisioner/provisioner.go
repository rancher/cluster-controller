package provisioner

import (
	"context"

	"reflect"

	"github.com/rancher/cluster-controller/client"
	"github.com/rancher/cluster-controller/controller"
	clusterv1 "github.com/rancher/types/apis/cluster.cattle.io/v1"
)

type Provisioner struct {
	clientSet         *client.V1
	clusterController clusterv1.ClusterController
}

func init() {
	p := &Provisioner{}
	controller.RegisterController(p.GetName(), p)
}

func (p *Provisioner) Init(config string) error {
	clientSet, err := client.NewClientSetV1(config)
	if err != nil {
		return err
	}
	p.clientSet = clientSet
	p.clusterController = p.clientSet.ClusterClientV1.Clusters("").Controller()
	p.clusterController.AddHandler(p.sync)
	return nil
}

func configChanged(old *clusterv1.Cluster, current *clusterv1.Cluster) bool {
	changed := false
	if current.Spec.AKSConfig != nil {
		changed = !reflect.DeepEqual(current.Spec.AKSConfig, old.Spec.AKSConfig)
	} else if current.Spec.GKEConfig != nil {
		changed = !reflect.DeepEqual(current.Spec.GKEConfig, old.Spec.GKEConfig)
	} else if current.Spec.AKSConfig != nil {
		changed = !reflect.DeepEqual(current.Spec.AKSConfig, old.Spec.AKSConfig)
	}

	return changed
}

func (p *Provisioner) sync(key string, cluster *clusterv1.Cluster) error {
	if cluster == nil {
		// no longer exists if nil is passed to this call
		return nil
	} else {
		// TODO check delete annotation, and call delete method if present
		// otherwise call create/update
		return p.createOrUpdateCluster(cluster)
	}
}

func (p *Provisioner) deleteCluster(key string) error {
	return nil
}

func (p *Provisioner) createOrUpdateCluster(cluster *clusterv1.Cluster) error {
	return nil
}

func (p *Provisioner) Run(ctx context.Context) error {
	return p.clusterController.Start(1, ctx)
}

func (p *Provisioner) GetName() string {
	return "clusterProvisioner"
}

func (p *Provisioner) Shutdown() {
}
