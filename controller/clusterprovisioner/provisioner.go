package provisioner

import (
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	clusterV1 "github.com/rancher/cluster-controller/client/v1"
	"github.com/rancher/cluster-controller/controller"
	"github.com/rancher/cluster-controller/controller/utils"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	p.syncQueue = utils.NewTaskQueue("clustersync", p.keyFunc, p.sync)

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
	logrus.Infof("Cluster updated [%s]", key)
	p.syncQueue.Enqueue(current)
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

func (p *Provisioner) sync(key string) {
	logrus.Infof("Syncing cluster [%s]", key)
	p.config.ClusterInformer.GetStore().GetByKey(key)
	c, exists, err := p.config.ClusterInformer.GetStore().GetByKey(key)
	if err != nil {
		logrus.Errorf("Failed to get cluster by name [%s] %v", key, err)
		p.syncQueue.Requeue(key, err)
		return
	}
	if !exists {
		err = p.deleteCluster(key)
		if err != nil {
			logrus.Errorf("Failed to delete cluster [%s] %v", key, err)
			p.syncQueue.Requeue(key, err)
		}
	} else {
		cluster := c.(*clusterV1.Cluster)
		err = p.createOrUpdateCluster(cluster)
		if err != nil {
			logrus.Errorf("Failed to create/update cluster [%s] %v", key, err)
			p.syncQueue.Requeue(key, err)
		}
	}

	logrus.Infof("Successfully synced cluster [%s]", key)
}

func (p *Provisioner) deleteCluster(key string) error {
	return nil
}

// createOrUpdateCluster calls drivers to provision the cluster
// TODO - add implementation. Currently it just creates a client/tests a cluster
func (p *Provisioner) createOrUpdateCluster(cluster *clusterV1.Cluster) error {
	clusterClient, err := utils.CreateClusterClient(cluster.Status.APIEndpoint, cluster.Status.ServiceAccountToken,
		cluster.Status.CACert)
	if err != nil {
		return fmt.Errorf("Failed to contact cluster %v", err)
	}
	nodes, err := clusterClient.Core().Nodes().List(v1.ListOptions{})
	if err != nil {
		return fmt.Errorf("Failed to list nodes %v", err)
	}
	for _, node := range nodes.Items {
		logrus.Infof("Node is %v", node.Name)
	}
	return nil
}

func (p *Provisioner) cleanup(key string) {
	logrus.Infof("Removing cluster %s", key)
	//TODO call cluster provisioner drivers
	logrus.Infof("Successfully removed cluster [%s]", key)
}

func (p *Provisioner) Run(stopc <-chan struct{}) error {

	go p.syncQueue.Run(time.Second, stopc)

	<-stopc
	return nil
}

func (p *Provisioner) GetName() string {
	return "clusterProvisioner"
}

func (p *Provisioner) Shutdown() {
	p.syncQueue.Shutdown()
}
