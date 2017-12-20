package agent

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	clusterController "github.com/rancher/cluster-agent/controller"
	"github.com/rancher/cluster-controller/controller/utils"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	SecretSuffix = "secret-token"
)

type Manager struct {
	ManagementConfig rest.Config
	LocalConfig      *rest.Config
	controllers      sync.Map
	K8sClientSet     *kubernetes.Clientset
}

type record struct {
	cluster *config.ClusterContext
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewManager(management *config.ManagementContext) *Manager {
	K8sClientSet, err := kubernetes.NewForConfig(&management.RESTConfig)
	if err != nil {
		logrus.Errorf("Error generating k8s client %v", err)
	}
	return &Manager{
		ManagementConfig: management.RESTConfig,
		LocalConfig:      management.LocalConfig,
		K8sClientSet:     K8sClientSet,
	}
}

func (m *Manager) Stop(ctx context.Context, cluster *v3.Cluster) {
	obj, ok := m.controllers.Load(cluster.UID)
	if !ok {
		return
	}
	logrus.Info("Stopping cluster agent for", obj.(*record).cluster.ClusterName)
	obj.(*record).cancel()
	m.controllers.Delete(cluster.UID)
}

func (m *Manager) Start(ctx context.Context, cluster *v3.Cluster) error {
	obj, ok := m.controllers.Load(cluster.UID)
	if ok {
		return nil
	}

	controller, err := m.toRecord(ctx, cluster)
	if controller == nil || err != nil {
		return err
	}

	obj, loaded := m.controllers.LoadOrStore(cluster.UID, controller)
	if !loaded {
		m.doStart(obj.(*record))
	}

	return nil
}

func (m *Manager) doStart(rec *record) {
	logrus.Info("Starting cluster agent for", rec.cluster.ClusterName)
	clusterController.Register(rec.ctx, rec.cluster)
	rec.cluster.Start(rec.ctx)
}

func (m *Manager) toRESTConfig(cluster *v3.Cluster) (*rest.Config, error) {
	if cluster == nil {
		return nil, nil
	}

	if cluster.Spec.Internal {
		return m.LocalConfig, nil
	}

	if cluster.Status.APIEndpoint == "" || cluster.Status.ServiceAccountSecretName == "" {
		return nil, nil
	}

	u, err := url.Parse(cluster.Status.APIEndpoint)
	if err != nil {
		return nil, err
	}

	secretData, err := m.getSecret(cluster)
	if err != nil {
		return nil, fmt.Errorf("Failed to get secret for cluster [%s]: %v", cluster.Name, err)
	}

	return &rest.Config{
		Host:        u.Host,
		Prefix:      u.Path,
		BearerToken: string(secretData[utils.ServiceAccountName][:]),
		TLSClientConfig: rest.TLSClientConfig{
			CAData: secretData[utils.CaCertName],
		},
	}, nil
}

func (m *Manager) toRecord(ctx context.Context, cluster *v3.Cluster) (*record, error) {
	kubeConfig, err := m.toRESTConfig(cluster)
	if kubeConfig == nil || err != nil {
		return nil, err
	}

	clusterContext, err := config.NewClusterContext(m.ManagementConfig, *kubeConfig, cluster.Name)
	if err != nil {
		return nil, err
	}

	s := &record{
		cluster: clusterContext,
	}
	s.ctx, s.cancel = context.WithCancel(ctx)

	return s, nil
}

func (m *Manager) getSecret(cluster *v3.Cluster) (map[string][]byte, error) {
	secretName := utils.GetSecretName(cluster)
	secret, err := k8s.GetSecret(m.K8sClientSet, secretName)
	if err != nil {
		return nil, err
	}
	if err := ensureFieldsExist(secret.Data, utils.ServiceAccountName, utils.CaCertName); err != nil {
		return nil, err
	}
	return secret.Data, nil
}

func ensureFieldsExist(data map[string][]byte, name1 string, name2 string) error {
	_, exists := data[name1]
	if !exists {
		return fmt.Errorf("%s not present in secret data", name1)
	}
	_, exists = data[name2]
	if !exists {
		return fmt.Errorf("%s not present in secret data", name2)
	}
	return nil
}
