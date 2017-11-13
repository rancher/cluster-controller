package provisioner

import (
	"fmt"

	"reflect"

	"github.com/rancher/cluster-controller/controller"
	"github.com/rancher/cluster-controller/controller/utils"
	clusterV1 "github.com/rancher/types/io.cattle.cluster/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
)

type Provisioner struct {
	config    *controller.Config
	syncQueue *utils.TaskQueue
}

func init() {
	p := &Provisioner{}
	controller.RegisterController(p.GetName(), p)
}

func (p *Provisioner) Init(cfg *controller.Config) {
	p.config = cfg
	p.syncQueue = utils.NewTaskQueue("clusterprovisionersync", p.keyFunc, p.sync)

	p.config.ClusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    p.handleClusterCreate,
		DeleteFunc: p.handleClusterDelete,
		UpdateFunc: p.handleClusterUpdate,
	})
}

func (p *Provisioner) handleClusterCreate(obj interface{}) {
	key, err := p.keyFunc(obj)
	if err != nil {
		return
	}
	logrus.Infof("Cluster created [%s]", key)
	p.syncQueue.Enqueue(obj)
}

func (p *Provisioner) handleClusterUpdate(old, current interface{}) {
	key, err := p.keyFunc(current)
	if err != nil {
		return
	}
	if configChanged(old, current) {
		logrus.Infof("Cluster [%s] updated with the new config", key)
		p.syncQueue.Enqueue(current)
	}
}

func configChanged(old, current interface{}) bool {
	oldC := old.(*clusterV1.Cluster)
	newC := current.(*clusterV1.Cluster)
	changed := false
	if newC.Spec.AKSConfig != nil {
		changed = !reflect.DeepEqual(newC.Spec.AKSConfig, oldC.Spec.AKSConfig)
	} else if newC.Spec.GKEConfig != nil {
		changed = !reflect.DeepEqual(newC.Spec.GKEConfig, oldC.Spec.GKEConfig)
	} else if newC.Spec.AKSConfig != nil {
		changed = !reflect.DeepEqual(newC.Spec.AKSConfig, oldC.Spec.AKSConfig)
	}

	return changed
}

func (p *Provisioner) handleClusterDelete(obj interface{}) {
	key, err := p.keyFunc(obj)
	if err != nil {
		return
	}
	logrus.Infof("Cluster deleted: %s", key)
	p.syncQueue.Enqueue(obj)
}

func (p *Provisioner) keyFunc(obj interface{}) (string, error) {
	// Cluster object is not namespaced,
	// but DeletionHandlingMetaNamespaceKeyFunc already handles it
	return cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
}

func (p *Provisioner) sync(key string) error {
	logrus.Infof("Syncing provisioning for cluster [%s]", key)
	c, exists, err := p.config.ClusterInformer.GetStore().GetByKey(key)
	if err != nil {
		return fmt.Errorf("Failed to get cluster by name [%s] %v", key, err)
	}
	if !exists {
		//TODO = handle removal using finalizer
		err = p.deleteCluster(key)
		if err != nil {
			return fmt.Errorf("Failed to delete cluster [%s] %v", key, err)
		}
	} else {
		cluster := c.(*clusterV1.Cluster)
		err = p.createOrUpdateCluster(cluster)
		if err != nil {
			return fmt.Errorf("Failed to create/update cluster [%s] %v", key, err)
		}
	}

	logrus.Infof("Successfully synced provisioning cluster [%s]", key)
	return nil
}

func (p *Provisioner) deleteCluster(key string) error {
	return nil
}

func (p *Provisioner) createOrUpdateCluster(cluster *clusterV1.Cluster) error {
	//TODO call drivers to provision the cluster
	return nil
}

func (p *Provisioner) Run(stopc <-chan struct{}) error {
	go p.syncQueue.Run()

	<-stopc
	return nil
}

func (p *Provisioner) GetName() string {
	return "clusterProvisioner"
}

func (p *Provisioner) Shutdown() {
	p.syncQueue.Shutdown()
}
