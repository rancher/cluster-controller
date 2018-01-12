package cluster

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/rancher/rke/authz"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
)

type Cluster struct {
	v3.RancherKubernetesEngineConfig `yaml:",inline"`
	ConfigPath                       string
	LocalKubeConfigPath              string
	EtcdHosts                        []*hosts.Host
	WorkerHosts                      []*hosts.Host
	ControlPlaneHosts                []*hosts.Host
	KubeClient                       *kubernetes.Clientset
	KubernetesServiceIP              net.IP
	Certificates                     map[string]pki.CertificatePKI
	ClusterDomain                    string
	ClusterCIDR                      string
	ClusterDNSServer                 string
	DockerDialerFactory              hosts.DialerFactory
	LocalConnDialerFactory           hosts.DialerFactory
}

const (
	X509AuthenticationProvider = "x509"
	StateConfigMapName         = "cluster-state"
	UpdateStateTimeout         = 30
	GetStateTimeout            = 30
	KubernetesClientTimeOut    = 30
	AplineImage                = "alpine"
	NginxProxyImage            = "nginx_proxy"
	CertDownloaderImage        = "cert_downloader"
	KubeDNSImage               = "kubedns_image"
	DNSMasqImage               = "dnsmasq_image"
	KubeDNSSidecarImage        = "kubedns_sidecar_image"
	KubeDNSAutoScalerImage     = "kubedns_autoscaler_image"
	ServiceSidekickImage       = "service_sidekick_image"
	NoneAuthorizationMode      = "none"
)

func (c *Cluster) DeployControlPlane(ctx context.Context) error {
	// Deploy Etcd Plane
	if err := services.RunEtcdPlane(ctx, c.EtcdHosts, c.Services.Etcd, c.LocalConnDialerFactory); err != nil {
		return fmt.Errorf("[etcd] Failed to bring up Etcd Plane: %v", err)
	}
	// Deploy Control plane
	if err := services.RunControlPlane(ctx, c.ControlPlaneHosts,
		c.EtcdHosts,
		c.Services,
		c.SystemImages[ServiceSidekickImage],
		c.Authorization.Mode,
		c.LocalConnDialerFactory); err != nil {
		return fmt.Errorf("[controlPlane] Failed to bring up Control Plane: %v", err)
	}
	// Apply Authz configuration after deploying controlplane
	if err := c.ApplyAuthzResources(ctx); err != nil {
		return fmt.Errorf("[auths] Failed to apply RBAC resources: %v", err)
	}
	return nil
}

func (c *Cluster) DeployWorkerPlane(ctx context.Context) error {
	// Deploy Worker Plane
	if err := services.RunWorkerPlane(ctx, c.ControlPlaneHosts,
		c.WorkerHosts,
		c.Services,
		c.SystemImages[NginxProxyImage],
		c.SystemImages[ServiceSidekickImage],
		c.LocalConnDialerFactory); err != nil {
		return fmt.Errorf("[workerPlane] Failed to bring up Worker Plane: %v", err)
	}
	return nil
}

func ParseConfig(clusterFile string) (*v3.RancherKubernetesEngineConfig, error) {
	logrus.Debugf("Parsing cluster file [%v]", clusterFile)
	var rkeConfig v3.RancherKubernetesEngineConfig
	if err := yaml.Unmarshal([]byte(clusterFile), &rkeConfig); err != nil {
		return nil, err
	}
	return &rkeConfig, nil
}

func ParseCluster(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, clusterFilePath string, dockerDialerFactory, localConnDialerFactory hosts.DialerFactory) (*Cluster, error) {
	var err error
	c := &Cluster{
		RancherKubernetesEngineConfig: *rkeConfig,
		ConfigPath:                    clusterFilePath,
		DockerDialerFactory:           dockerDialerFactory,
		LocalConnDialerFactory:        localConnDialerFactory,
	}
	// Setting cluster Defaults
	c.setClusterDefaults(ctx)

	if err := c.InvertIndexHosts(); err != nil {
		return nil, fmt.Errorf("Failed to classify hosts from config file: %v", err)
	}

	if err := c.ValidateCluster(); err != nil {
		return nil, fmt.Errorf("Failed to validate cluster: %v", err)
	}

	c.KubernetesServiceIP, err = services.GetKubernetesServiceIP(c.Services.KubeAPI.ServiceClusterIPRange)
	if err != nil {
		return nil, fmt.Errorf("Failed to get Kubernetes Service IP: %v", err)
	}
	c.ClusterDomain = c.Services.Kubelet.ClusterDomain
	c.ClusterCIDR = c.Services.KubeController.ClusterCIDR
	c.ClusterDNSServer = c.Services.Kubelet.ClusterDNSServer
	if len(c.ConfigPath) == 0 {
		c.ConfigPath = DefaultClusterConfig
	}
	c.LocalKubeConfigPath = GetLocalKubeConfig(c.ConfigPath)
	return c, nil
}

func (c *Cluster) setClusterDefaults(ctx context.Context) {
	if len(c.SSHKeyPath) == 0 {
		c.SSHKeyPath = DefaultClusterSSHKeyPath
	}
	for i, host := range c.Nodes {
		if len(host.InternalAddress) == 0 {
			c.Nodes[i].InternalAddress = c.Nodes[i].Address
		}
		if len(host.HostnameOverride) == 0 {
			// This is a temporary modification
			c.Nodes[i].HostnameOverride = c.Nodes[i].Address
		}
		if len(host.SSHKeyPath) == 0 {
			c.Nodes[i].SSHKeyPath = c.SSHKeyPath
		}
	}
	if len(c.Authorization.Mode) == 0 {
		c.Authorization.Mode = DefaultAuthorizationMode
	}
	if c.Services.KubeAPI.PodSecurityPolicy && c.Authorization.Mode != services.RBACAuthorizationMode {
		log.Warnf(ctx, "PodSecurityPolicy can't be enabled with RBAC support disabled")
		c.Services.KubeAPI.PodSecurityPolicy = false
	}
	c.setClusterServicesDefaults()
	c.setClusterNetworkDefaults()
	c.setClusterImageDefaults()
}

func (c *Cluster) setClusterServicesDefaults() {
	serviceConfigDefaultsMap := map[*string]string{
		&c.Services.KubeAPI.ServiceClusterIPRange:        DefaultServiceClusterIPRange,
		&c.Services.KubeController.ServiceClusterIPRange: DefaultServiceClusterIPRange,
		&c.Services.KubeController.ClusterCIDR:           DefaultClusterCIDR,
		&c.Services.Kubelet.ClusterDNSServer:             DefaultClusterDNSService,
		&c.Services.Kubelet.ClusterDomain:                DefaultClusterDomain,
		&c.Services.Kubelet.InfraContainerImage:          DefaultInfraContainerImage,
		&c.Authentication.Strategy:                       DefaultAuthStrategy,
		&c.Services.KubeAPI.Image:                        DefaultK8sImage,
		&c.Services.Scheduler.Image:                      DefaultK8sImage,
		&c.Services.KubeController.Image:                 DefaultK8sImage,
		&c.Services.Kubelet.Image:                        DefaultK8sImage,
		&c.Services.Kubeproxy.Image:                      DefaultK8sImage,
		&c.Services.Etcd.Image:                           DefaultEtcdImage,
	}
	for k, v := range serviceConfigDefaultsMap {
		setDefaultIfEmpty(k, v)
	}
}

func (c *Cluster) setClusterImageDefaults() {
	if c.SystemImages == nil {
		// don't break if the user didn't define rke_images
		c.SystemImages = make(map[string]string)
	}
	systemImagesDefaultsMap := map[string]string{
		AplineImage:            DefaultAplineImage,
		NginxProxyImage:        DefaultNginxProxyImage,
		CertDownloaderImage:    DefaultCertDownloaderImage,
		KubeDNSImage:           DefaultKubeDNSImage,
		DNSMasqImage:           DefaultDNSMasqImage,
		KubeDNSSidecarImage:    DefaultKubeDNSSidecarImage,
		KubeDNSAutoScalerImage: DefaultKubeDNSAutoScalerImage,
		ServiceSidekickImage:   DefaultServiceSidekickImage,
	}
	for k, v := range systemImagesDefaultsMap {
		setDefaultIfEmptyMapValue(c.SystemImages, k, v)
	}
}

func GetLocalKubeConfig(configPath string) string {
	baseDir := filepath.Dir(configPath)
	fileName := filepath.Base(configPath)
	baseDir += "/"
	return fmt.Sprintf("%s%s%s", baseDir, pki.KubeAdminConfigPrefix, fileName)
}

func rebuildLocalAdminConfig(ctx context.Context, kubeCluster *Cluster) error {
	log.Infof(ctx, "[reconcile] Rebuilding and updating local kube config")
	var workingConfig, newConfig string
	currentKubeConfig := kubeCluster.Certificates[pki.KubeAdminCommonName]
	caCrt := kubeCluster.Certificates[pki.CACertName].Certificate
	for _, cpHost := range kubeCluster.ControlPlaneHosts {
		if (currentKubeConfig == pki.CertificatePKI{}) {
			kubeCluster.Certificates = make(map[string]pki.CertificatePKI)
			newConfig = getLocalAdminConfigWithNewAddress(kubeCluster.LocalKubeConfigPath, cpHost.Address)
		} else {
			kubeURL := fmt.Sprintf("https://%s:6443", cpHost.Address)
			caData := string(cert.EncodeCertPEM(caCrt))
			crtData := string(cert.EncodeCertPEM(currentKubeConfig.Certificate))
			keyData := string(cert.EncodePrivateKeyPEM(currentKubeConfig.Key))
			newConfig = pki.GetKubeConfigX509WithData(kubeURL, pki.KubeAdminCommonName, caData, crtData, keyData)
		}
		if err := pki.DeployAdminConfig(ctx, newConfig, kubeCluster.LocalKubeConfigPath); err != nil {
			return fmt.Errorf("Failed to redeploy local admin config with new host")
		}
		workingConfig = newConfig
		if _, err := GetK8sVersion(kubeCluster.LocalKubeConfigPath); err == nil {
			log.Infof(ctx, "[reconcile] host [%s] is active master on the cluster", cpHost.Address)
			break
		}
	}
	currentKubeConfig.Config = workingConfig
	kubeCluster.Certificates[pki.KubeAdminCommonName] = currentKubeConfig
	return nil
}

func isLocalConfigWorking(ctx context.Context, localKubeConfigPath string) bool {
	if _, err := GetK8sVersion(localKubeConfigPath); err != nil {
		log.Infof(ctx, "[reconcile] Local config is not vaild, rebuilding admin config")
		return false
	}
	return true
}

func getLocalConfigAddress(localConfigPath string) (string, error) {
	config, err := clientcmd.BuildConfigFromFlags("", localConfigPath)
	if err != nil {
		return "", err
	}
	splittedAdress := strings.Split(config.Host, ":")
	address := splittedAdress[1]
	return address[2:], nil
}

func getLocalAdminConfigWithNewAddress(localConfigPath, cpAddress string) string {
	config, _ := clientcmd.BuildConfigFromFlags("", localConfigPath)
	config.Host = fmt.Sprintf("https://%s:6443", cpAddress)
	return pki.GetKubeConfigX509WithData(
		"https://"+cpAddress+":6443",
		pki.KubeAdminCommonName,
		string(config.CAData),
		string(config.CertData),
		string(config.KeyData))
}

func (c *Cluster) ApplyAuthzResources(ctx context.Context) error {
	if err := authz.ApplyJobDeployerServiceAccount(ctx, c.LocalKubeConfigPath); err != nil {
		return fmt.Errorf("Failed to apply the ServiceAccount needed for job execution: %v", err)
	}
	if c.Authorization.Mode == NoneAuthorizationMode {
		return nil
	}
	if c.Authorization.Mode == services.RBACAuthorizationMode {
		if err := authz.ApplySystemNodeClusterRoleBinding(ctx, c.LocalKubeConfigPath); err != nil {
			return fmt.Errorf("Failed to apply the ClusterRoleBinding needed for node authorization: %v", err)
		}
	}
	if c.Authorization.Mode == services.RBACAuthorizationMode && c.Services.KubeAPI.PodSecurityPolicy {
		if err := authz.ApplyDefaultPodSecurityPolicy(ctx, c.LocalKubeConfigPath); err != nil {
			return fmt.Errorf("Failed to apply default PodSecurityPolicy: %v", err)
		}
		if err := authz.ApplyDefaultPodSecurityPolicyRole(ctx, c.LocalKubeConfigPath); err != nil {
			return fmt.Errorf("Failed to apply default PodSecurityPolicy ClusterRole and ClusterRoleBinding: %v", err)
		}
	}
	return nil
}
