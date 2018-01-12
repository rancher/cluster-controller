package cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/rke/addons"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
)

const (
	KubeDNSAddonResourceName = "rke-kubedns-addon"
	UserAddonResourceName    = "rke-user-addon"
)

func (c *Cluster) DeployK8sAddOns(ctx context.Context) error {
	err := c.deployKubeDNS(ctx)
	return err
}

func (c *Cluster) DeployUserAddOns(ctx context.Context) error {
	log.Infof(ctx, "[addons] Setting up user addons..")
	if c.Addons == "" {
		log.Infof(ctx, "[addons] No user addons configured..")
		return nil
	}

	if err := c.doAddonDeploy(ctx, c.Addons, UserAddonResourceName); err != nil {
		return err
	}
	log.Infof(ctx, "[addons] User addon deployed successfully..")
	return nil

}

func (c *Cluster) deployKubeDNS(ctx context.Context) error {
	log.Infof(ctx, "[addons] Setting up KubeDNS")
	kubeDNSConfig := map[string]string{
		addons.KubeDNSServer:          c.ClusterDNSServer,
		addons.KubeDNSClusterDomain:   c.ClusterDomain,
		addons.KubeDNSImage:           c.SystemImages[KubeDNSImage],
		addons.DNSMasqImage:           c.SystemImages[DNSMasqImage],
		addons.KubeDNSSidecarImage:    c.SystemImages[KubeDNSSidecarImage],
		addons.KubeDNSAutoScalerImage: c.SystemImages[KubeDNSAutoScalerImage],
	}
	kubeDNSYaml, err := addons.GetKubeDNSManifest(kubeDNSConfig)
	if err != nil {
		return err
	}
	if err := c.doAddonDeploy(ctx, kubeDNSYaml, KubeDNSAddonResourceName); err != nil {
		return err
	}
	log.Infof(ctx, "[addons] KubeDNS deployed successfully..")
	return nil

}

func (c *Cluster) doAddonDeploy(ctx context.Context, addonYaml, resourceName string) error {

	err := c.StoreAddonConfigMap(ctx, addonYaml, resourceName)
	if err != nil {
		return fmt.Errorf("Failed to save addon ConfigMap: %v", err)
	}

	log.Infof(ctx, "[addons] Executing deploy job..")

	addonJob, err := addons.GetAddonsExcuteJob(resourceName, c.ControlPlaneHosts[0].HostnameOverride, c.Services.KubeAPI.Image)
	if err != nil {
		return fmt.Errorf("Failed to deploy addon execute job: %v", err)
	}
	err = c.ApplySystemAddonExcuteJob(addonJob)
	if err != nil {
		return fmt.Errorf("Failed to deploy addon execute job: %v", err)
	}
	return nil
}

func (c *Cluster) StoreAddonConfigMap(ctx context.Context, addonYaml string, addonName string) error {
	log.Infof(ctx, "[addons] Saving addon ConfigMap to Kubernetes")
	kubeClient, err := k8s.NewClient(c.LocalKubeConfigPath)
	if err != nil {
		return err
	}
	timeout := make(chan bool, 1)
	go func() {
		for {
			err := k8s.UpdateConfigMap(kubeClient, []byte(addonYaml), addonName)
			if err != nil {
				time.Sleep(time.Second * 5)
				fmt.Println(err)
				continue
			}
			log.Infof(ctx, "[addons] Successfully Saved addon to Kubernetes ConfigMap: %s", addonName)
			timeout <- true
			break
		}
	}()
	select {
	case <-timeout:
		return nil
	case <-time.After(time.Second * UpdateStateTimeout):
		return fmt.Errorf("[addons] Timeout waiting for kubernetes to be ready")
	}
}

func (c *Cluster) ApplySystemAddonExcuteJob(addonJob string) error {

	if err := k8s.ApplyK8sSystemJob(addonJob, c.LocalKubeConfigPath); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}
