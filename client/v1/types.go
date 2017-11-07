package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
)

type ClusterConditionType string

const (
	// ClusterConditionReady Cluster ready to serve API (healthy when true, unehalthy when false)
	ClusterConditionReady = "Ready"
	// ClusterConditionProvisioned Cluster is provisioned
	ClusterConditionProvisioned = "Provisioned"
	// ClusterConditionUpdating Cluster is being updating (upgrading, scaling up)
	ClusterConditionUpdating = "Updating"
	// More conditions can be added if unredlying controllers request it
)

type Cluster struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec ClusterSpec `json:"spec"`
	// Most recent observed status of the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Status *ClusterStatus `json:"status"`
}

type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	// List of Clusters
	Items []*Cluster `json:"items"`
}

type ClusterSpec struct {
	GKEConfig *GKEConfig
	AKSConfig *AKSConfig
	RKEConfig *RKEConfig
}

type ClusterStatus struct {
	//Conditions represent the latest available observations of an object's current state:
	//More info: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
	Conditions []ClusterCondition `json:"conditions,omitempty"`
	//Component statuses will represent cluster's components (etcd/controller/scheduler) health
	// https://kubernetes.io/docs/api-reference/v1.8/#componentstatus-v1-core
	ComponentStatuses   v1.ComponentStatusList
	APIEndpoint         string
	ServiceAccountToken string
	CACert              string
}

type ClusterCondition struct {
	// Type of cluster condition.
	Type ClusterConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
}

type GKEConfig struct {
	// ProjectID is the ID of your project to use when creating a cluster
	ProjectID string
	// The zone to launch the cluster
	Zone string
	// The IP address range of the container pods
	ClusterIpv4Cidr string
	// An optional description of this cluster
	Description string
	// The number of nodes to create in this cluster
	InitialNodeCount int64
	// Size of the disk attached to each node
	DiskSizeGb int64
	// The name of a Google Compute Engine
	MachineType string
	// the initial kubernetes version
	InitialClusterVersion string
	// The map of Kubernetes labels (key/value pairs) to be applied
	// to each node.
	Labels map[string]string
	// The path to the credential file(key.json)
	CredentialPath string
	// Enable alpha feature
	EnableAlphaFeature bool
	// NodePool id
	NodePoolID string

	// Update Config
	UpdateConfig gkeUpdateConfig
}

type gkeUpdateConfig struct {
	// the number of node
	NodeCount int64
	// Master kubernetes version
	MasterVersion string
	// Node kubernetes version
	NodeVersion string
}

type AKSConfig struct {
	//TBD
}

type RKEConfig struct {
	//TBD
}

type ClusterNode struct {
	v1.Node
}

type ClusterNodeList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	// List of Clusters
	Items []*Cluster `json:"items"`
}
