package nodestats

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/Sirupsen/logrus"
	clusterV1 "github.com/rancher/cluster-controller/client/v1"
	"github.com/rancher/cluster-controller/controller"
	"github.com/rancher/cluster-controller/controller/utils"
)

const (
	resyncPeriod = 10 * time.Minute
)

type ClusterSyncer struct {
	config        *controller.Config
	syncQueue     *utils.TaskQueue
	nodesMonitors map[string]NodesMonitor
	stopCh        <-chan struct{}
}

func init() {
	s := &ClusterSyncer{
		nodesMonitors: make(map[string]NodesMonitor),
	}
	controller.RegisterController(s.GetName(), s)
}

type NodesMonitor struct {
	clientSet    *kubernetes.Clientset
	nodeInformer cache.SharedIndexInformer
	syncQueue    *utils.TaskQueue
}

func (s *ClusterSyncer) Init(cfg *controller.Config) {
	s.config = cfg
	s.syncQueue = utils.NewTaskQueue("clustersync", s.keyFunc, s.syncCluster)
	s.config.ClusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    s.handleClusterCreate,
		DeleteFunc: s.handleClusterDelete,
		UpdateFunc: s.handleClusterUpdate,
	})
}

func (s *ClusterSyncer) GetName() string {
	return "nodeStats"
}
func (s *ClusterSyncer) Run(stopc <-chan struct{}) error {
	s.stopCh = stopc
	go s.syncQueue.Run()
	<-stopc
	return nil
}

func (s *ClusterSyncer) handleClusterCreate(obj interface{}) {
	key, err := s.keyFunc(obj)
	if err != nil {
		return
	}
	c := obj.(*clusterV1.Cluster)
	if apiEndpointReady(c) {
		logrus.Infof("Cluster [%s] created with API endpoint set", key)
		s.syncQueue.Enqueue(obj)
	}
}

func apiEndpointReady(c *clusterV1.Cluster) bool {
	return c.Status.APIEndpoint != "" && c.Status.ServiceAccountToken != "" && c.Status.CACert != ""
}

func endpointChanged(old *clusterV1.Cluster, current *clusterV1.Cluster) bool {
	if !apiEndpointReady(current) {
		return false
	}
	if current.Status.APIEndpoint != old.Status.APIEndpoint {
		logrus.Info("Cluster api endpoint changed")
		return true
	}
	if current.Status.ServiceAccountToken != old.Status.ServiceAccountToken {
		logrus.Info("Cluster service account token changed")
		return true
	}
	if current.Status.CACert != old.Status.CACert {
		logrus.Info("Cluster ca cert changed")
		return true
	}
	return false
}

func (s *ClusterSyncer) handleClusterUpdate(old, current interface{}) {
	key, err := s.keyFunc(current)
	if err != nil {
		return
	}

	oldC := old.(*clusterV1.Cluster)
	currentC := current.(*clusterV1.Cluster)
	if endpointChanged(oldC, currentC) {
		logrus.Infof("Cluster [%s] updated with new API endpoint ", key)
		s.syncQueue.Enqueue(current)
	}
}

func (s *ClusterSyncer) handleClusterDelete(obj interface{}) {
	key, err := s.keyFunc(obj)
	if err != nil {
		return
	}
	if nodeMonitor, ok := s.nodesMonitors[key]; ok {
		nodeMonitor.shutdown()
		delete(s.nodesMonitors, key)
	}
}

func (s *ClusterSyncer) syncCluster(key string) error {
	logrus.Infof("Creating cluster [%s] nodes sync monitor", key)
	if nodeMonitor, ok := s.nodesMonitors[key]; ok {
		// shutdown old node monitor
		nodeMonitor.shutdown()
	}

	// create a new node monitormonitor
	obj, exists, err := s.config.ClusterInformer.GetStore().GetByKey(key)
	if err != nil {
		return fmt.Errorf("Failed to get cluster by name [%s] %v", key, err)
	}
	if !exists {
		logrus.Infof("Cluster [%s] no longer exists, skipping syncing nodes", key)
		return nil
	}
	cluster := obj.(*clusterV1.Cluster)
	clientSet, err := utils.CreateClusterClient(cluster.Status.APIEndpoint, cluster.Status.ServiceAccountToken, cluster.Status.CACert)
	if err != nil {
		return fmt.Errorf("Failed to create a client for cluster [%s]: %v", key, err)
	}

	monitor := NodesMonitor{
		clientSet: clientSet,
	}
	watchList := cache.NewListWatchFromClient(clientSet.Core().RESTClient(), "nodes", v1.NamespaceAll, fields.Everything())
	monitor.nodeInformer = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  watchList.ListFunc,
			WatchFunc: watchList.WatchFunc,
		},
		&v1.Node{}, resyncPeriod, cache.Indexers{})

	monitor.nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    monitor.handleNodeCreate,
		DeleteFunc: monitor.handleNodeDelete,
		UpdateFunc: monitor.handleNodeUpdate,
	})

	monitor.syncQueue = utils.NewTaskQueue("nodesmonitor", monitor.keyFunc, monitor.syncNode)
	s.nodesMonitors[cluster.ObjectMeta.Name] = monitor
	go monitor.run(s.stopCh)
	logrus.Infof("Successfully created cluster [%s] nodes monitor", key)
	return nil
}

func (s *ClusterSyncer) keyFunc(obj interface{}) (string, error) {
	// Cluster object is not namespaced,
	// but DeletionHandlingMetaNamespaceKeyFunc already handles it
	return cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
}

func (s *ClusterSyncer) Shutdown() {
	logrus.Info("Shutting down node monitors")
	for _, nodeMonitor := range s.nodesMonitors {
		nodeMonitor.shutdown()
	}
	logrus.Info("Shutting down sync queue")
	s.syncQueue.Shutdown()
}

func (m *NodesMonitor) syncNode(key string) error {
	logrus.Infof("Syncing changes for node [%s]", key)
	// TODO - do real sync (create/udpate/remove)
	logrus.Infof("Successfully synced changes for the node [%s]", key)
	return nil
}

func (m *NodesMonitor) handleNodeCreate(obj interface{}) {
	key, err := m.keyFunc(obj)
	if err != nil {
		return
	}
	logrus.Infof("Node created [%s]", key)
	m.syncQueue.Enqueue(obj)
}

func (m *NodesMonitor) handleNodeDelete(obj interface{}) {
	key, err := m.keyFunc(obj)
	if err != nil {
		return
	}

	logrus.Infof("Node deleted [%s]", key)
	m.syncQueue.Enqueue(obj)
}

func (m *NodesMonitor) handleNodeUpdate(old, current interface{}) {
	key, err := m.keyFunc(current)
	if err != nil {
		return
	}
	logrus.Infof("Node updated [%s]", key)
	m.syncQueue.Enqueue(current)
}

func (m *NodesMonitor) run(stopc <-chan struct{}) error {
	go m.nodeInformer.Run(stopc)
	go m.syncQueue.Run()
	<-stopc
	return nil
}

func (m *NodesMonitor) shutdown() {
	m.syncQueue.Shutdown()
}

func (m *NodesMonitor) keyFunc(obj interface{}) (string, error) {
	// Node object is not namespaced,
	// but DeletionHandlingMetaNamespaceKeyFunc already handles it
	return cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
}
