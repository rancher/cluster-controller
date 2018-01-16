package cluster

import (
	"fmt"

	"context"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/sirupsen/logrus"
)

func (c *Cluster) TunnelHosts(ctx context.Context, local bool) error {
	if local {
		if err := c.EtcdHosts[0].TunnelUpLocal(ctx); err != nil {
			return fmt.Errorf("Failed to connect to docker for local host [%s]: %v", c.EtcdHosts[0].Address, err)
		}
		return nil
	}
	for i := range c.EtcdHosts {
		if err := c.EtcdHosts[i].TunnelUp(ctx, c.DockerDialerFactory); err != nil {
			return fmt.Errorf("Failed to set up SSH tunneling for Etcd host [%s]: %v", c.EtcdHosts[i].Address, err)
		}
	}
	for i := range c.ControlPlaneHosts {
		err := c.ControlPlaneHosts[i].TunnelUp(ctx, c.DockerDialerFactory)
		if err != nil {
			return fmt.Errorf("Failed to set up SSH tunneling for Control host [%s]: %v", c.ControlPlaneHosts[i].Address, err)
		}
	}
	for i := range c.WorkerHosts {
		if err := c.WorkerHosts[i].TunnelUp(ctx, c.DockerDialerFactory); err != nil {
			return fmt.Errorf("Failed to set up SSH tunneling for Worker host [%s]: %v", c.WorkerHosts[i].Address, err)
		}
	}
	return nil
}

func (c *Cluster) InvertIndexHosts() error {
	c.EtcdHosts = make([]*hosts.Host, 0)
	c.WorkerHosts = make([]*hosts.Host, 0)
	c.ControlPlaneHosts = make([]*hosts.Host, 0)
	for _, host := range c.Nodes {
		newHost := hosts.Host{
			RKEConfigNode: host,
		}

		newHost.EnforceDockerVersion = c.EnforceDockerVersion

		for _, role := range host.Role {
			logrus.Debugf("Host: " + host.Address + " has role: " + role)
			switch role {
			case services.ETCDRole:
				newHost.IsEtcd = true
				c.EtcdHosts = append(c.EtcdHosts, &newHost)
			case services.ControlRole:
				newHost.IsControl = true
				c.ControlPlaneHosts = append(c.ControlPlaneHosts, &newHost)
			case services.WorkerRole:
				newHost.IsWorker = true
				c.WorkerHosts = append(c.WorkerHosts, &newHost)
			default:
				return fmt.Errorf("Failed to recognize host [%s] role %s", host.Address, role)
			}
		}
	}
	return nil
}

func (c *Cluster) SetUpHosts(ctx context.Context) error {
	if c.Authentication.Strategy == X509AuthenticationProvider {
		log.Infof(ctx, "[certificates] Deploying kubernetes certificates to Cluster nodes")
		err := pki.DeployCertificatesOnMasters(ctx, c.ControlPlaneHosts, c.Certificates, c.SystemImages[CertDownloaderImage])
		if err != nil {
			return err
		}
		err = pki.DeployCertificatesOnWorkers(ctx, c.WorkerHosts, c.Certificates, c.SystemImages[CertDownloaderImage])
		if err != nil {
			return err
		}
		err = pki.DeployAdminConfig(ctx, c.Certificates[pki.KubeAdminCommonName].Config, c.LocalKubeConfigPath)
		if err != nil {
			return err
		}
		log.Infof(ctx, "[certificates] Successfully deployed kubernetes certificates to Cluster nodes")
	}
	return nil
}

func CheckEtcdHostsChanged(kubeCluster, currentCluster *Cluster) error {
	if currentCluster != nil {
		etcdChanged := hosts.IsHostListChanged(currentCluster.EtcdHosts, kubeCluster.EtcdHosts)
		if etcdChanged {
			return fmt.Errorf("Adding or removing Etcd nodes is not supported")
		}
	}
	return nil
}
