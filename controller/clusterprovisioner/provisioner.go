package provisioner

import (
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/cluster-controller/controller"
	"github.com/rancher/cluster-controller/controller/utils"
	"k8s.io/client-go/tools/cache"
)

type Provisioner struct {
	config       *controller.ControllerConfig
	syncQueue    *utils.TaskQueue
	cleanupQueue *utils.TaskQueue
}

func init() {
	p := &Provisioner{}

	controller.RegisterController(p.GetName(), p)
}

func (p *Provisioner) Init(cfg *controller.ControllerConfig) {
	p.config = cfg
	p.syncQueue = utils.NewTaskQueue("clustersync", p.keyFunc, p.sync)
	p.cleanupQueue = utils.NewTaskQueue("clustercleanup", p.keyFunc, p.cleanup)

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
	p.cleanupQueue.Enqueue(obj)
}

func (p *Provisioner) keyFunc(obj interface{}) (string, error) {
	// Cluster object is not namespaced,
	// but DeletionHandlingMetaNamespaceKeyFunc already handles it
	return cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
}

func (p *Provisioner) sync(key string) {
	logrus.Infof("Syncing cluster [%s]", key)
	//TODO call cluster provisioner drivers
	logrus.Infof("Successfully synced cluster [%s]", key)
}

func (p *Provisioner) cleanup(key string) {
	logrus.Infof("Removing cluster %s", key)
	//TODO call cluster provisioner drivers
	logrus.Infof("Successfully removed cluster [%s]", key)
}

func (p *Provisioner) Run(stopc <-chan struct{}) error {

	go p.syncQueue.Run(time.Second, stopc)
	go p.cleanupQueue.Run(time.Second, stopc)

	<-stopc
	return nil
}

func (p *Provisioner) GetName() string {
	return "clusterProvisioner"
}

func (p *Provisioner) Shutdown() {
	p.syncQueue.Shutdown()
	p.cleanupQueue.Shutdown()
}
