package provisioner

import (
	"time"

	"github.com/Sirupsen/logrus"
	operator "github.com/rancher/cluster-controller/operator"
	"github.com/rancher/cluster-controller/operator/utils"
	"k8s.io/client-go/tools/cache"
)

type Operator struct {
	config       *operator.OperatorConfig
	syncQueue    *utils.TaskQueue
	cleanupQueue *utils.TaskQueue
}

func init() {
	o := &Operator{}

	operator.RegisterOperator(o.GetName(), o)
}

func (o *Operator) Init(cfg *operator.OperatorConfig) {
	o.config = cfg
	o.syncQueue = utils.NewTaskQueue("clustersync", o.keyFunc, o.sync)
	o.cleanupQueue = utils.NewTaskQueue("clustercleanup", o.keyFunc, o.cleanup)

	o.config.ClusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    o.handleClusterCreate,
		DeleteFunc: o.handleClusterDelete,
		UpdateFunc: o.handleClusterUpdate,
	})
}

func (o *Operator) handleClusterCreate(obj interface{}) {
	key, err := o.keyFunc(obj)
	if err != nil {
		return
	}
	logrus.Infof("Cluster created: %s", key)
	o.syncQueue.Enqueue(obj)
}

func (o *Operator) handleClusterUpdate(old, current interface{}) {
	key, err := o.keyFunc(current)
	if err != nil {
		return
	}
	logrus.Infof("Cluster updated: %s", key)
	o.syncQueue.Enqueue(current)
}

func (o *Operator) handleClusterDelete(obj interface{}) {
	key, err := o.keyFunc(obj)
	if err != nil {
		return
	}
	logrus.Infof("Cluster deleted: %s", key)
	o.cleanupQueue.Enqueue(obj)
}

func (o *Operator) keyFunc(obj interface{}) (string, error) {
	// Cluster object is not namespaced,
	// but DeletionHandlingMetaNamespaceKeyFunc already handles it
	return cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
}

func (o *Operator) sync(key string) {
	logrus.Infof("Syncing cluster %s", key)
	//TODO call cluster provisioner drivers
	logrus.Infof("Successfully synced cluster %s", key)
}

func (o *Operator) cleanup(key string) {
	logrus.Infof("Removing cluster %s", key)
	//TODO call cluster provisioner drivers
	logrus.Infof("Successfully removed cluster %s", key)
}

func (o *Operator) Run(stopc <-chan struct{}) error {

	go o.syncQueue.Run(time.Second, stopc)
	go o.cleanupQueue.Run(time.Second, stopc)

	<-stopc
	logrus.Infof("shutting down [%s] operator", o.GetName())
	return nil
}

func (o *Operator) GetName() string {
	return "clusterProvisioner"
}
