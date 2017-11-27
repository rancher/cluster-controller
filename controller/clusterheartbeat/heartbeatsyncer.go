package heartbeatsyncer

import (
	"time"

	"github.com/rancher/cluster-controller/controller"
	clusterv1 "github.com/rancher/types/apis/cluster.cattle.io/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	syncInterval                  = 20 * time.Second
	ClusterConditionReady         = "Ready"
	ClusterConditionStatusUnknown = "Unknown"
)

var clusterToLastUpdated map[string]time.Time

type HeartBeatSyncer struct {
	config *controller.Config
}

func init() {
	h := &HeartBeatSyncer{}
	controller.RegisterController(h.GetName(), h)
	clusterToLastUpdated = make(map[string]time.Time)
	go h.syncHeartBeat(syncInterval)
}

func (h *HeartBeatSyncer) Start(config *controller.Config) {
	h.config = config
	h.config.ClusterController.AddHandler(h.sync)
}

func (h *HeartBeatSyncer) sync(key string, cluster *clusterv1.Cluster) error {
	logrus.Infof("Syncing cluster [%s] ", key)
	if cluster == nil {
		// cluster has been deleted
		if _, exists := clusterToLastUpdated[key]; exists {
			delete(clusterToLastUpdated, key)
			logrus.Infof("Cluster [%s] already deleted", key)
		}
	} else {
		condition := getConditionIfReady(cluster)
		if condition != nil {
			lastUpdateTime, _ := time.Parse(time.RFC3339, condition.LastUpdateTime)
			clusterToLastUpdated[key] = lastUpdateTime
		}
	}
	logrus.Infof("Synced cluster [%s] successfully", key)
	return nil
}

func (h *HeartBeatSyncer) syncHeartBeat(syncInterval time.Duration) {
	for _ = range time.Tick(syncInterval) {
		logrus.Infof("Sync heartbeat")
		h.checkHeartBeat()
	}
}

func (h *HeartBeatSyncer) checkHeartBeat() {
	for clusterName, lastUpdatedTime := range clusterToLastUpdated {
		if lastUpdatedTime.Add(syncInterval).Before(time.Now().UTC()) {
			cluster, err := h.config.ClientSet.ClusterClientV1.Clusters("").Get(clusterName, metav1.GetOptions{})
			if err != nil {
				logrus.Infof("Error getting Cluster [%s] - %v", clusterName, err)
				continue
			}
			setConditionStatus(cluster, ClusterConditionReady, ClusterConditionStatusUnknown)
			logrus.Infof("Cluster [%s] condition status unknown", clusterName)
		}
	}
}

func (h *HeartBeatSyncer) GetName() string {
	return "clusterHeartBeatSyncer"
}

func getConditionByType(cluster *clusterv1.Cluster, conditionType clusterv1.ClusterConditionType) *clusterv1.ClusterCondition {
	for _, condition := range cluster.Status.Conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

// Condition is Ready if conditionType is Ready and conditionStatus is True/False but not unknown.
func getConditionIfReady(cluster *clusterv1.Cluster) *clusterv1.ClusterCondition {
	for _, condition := range cluster.Status.Conditions {
		if condition.Type == ClusterConditionReady && condition.Status != ClusterConditionStatusUnknown {
			return &condition
		}
	}
	return nil
}

func setConditionStatus(cluster *clusterv1.Cluster, conditionType clusterv1.ClusterConditionType, status corev1.ConditionStatus) {
	condition := getConditionByType(cluster, conditionType)
	if condition != nil {
		condition.Status = status
	}
}
