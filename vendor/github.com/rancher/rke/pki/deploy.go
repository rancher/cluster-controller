package pki

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/sirupsen/logrus"
)

func DeployCertificatesOnMasters(ctx context.Context, cpHosts []*hosts.Host, crtMap map[string]CertificatePKI, certDownloaderImage string) error {
	// list of certificates that should be deployed on the masters
	crtList := []string{
		CACertName,
		KubeAPICertName,
		KubeControllerName,
		KubeSchedulerName,
		KubeProxyName,
		KubeNodeName,
	}
	env := []string{}
	for _, crtName := range crtList {
		c := crtMap[crtName]
		env = append(env, c.ToEnv()...)
	}

	for i := range cpHosts {
		err := doRunDeployer(ctx, cpHosts[i], env, certDownloaderImage)
		if err != nil {
			return err
		}
	}
	return nil
}

func DeployCertificatesOnWorkers(ctx context.Context, workerHosts []*hosts.Host, crtMap map[string]CertificatePKI, certDownloaderImage string) error {
	// list of certificates that should be deployed on the workers
	crtList := []string{
		CACertName,
		KubeProxyName,
		KubeNodeName,
	}
	env := []string{}
	for _, crtName := range crtList {
		c := crtMap[crtName]
		env = append(env, c.ToEnv()...)
	}

	for i := range workerHosts {
		err := doRunDeployer(ctx, workerHosts[i], env, certDownloaderImage)
		if err != nil {
			return err
		}
	}
	return nil
}

func doRunDeployer(ctx context.Context, host *hosts.Host, containerEnv []string, certDownloaderImage string) error {
	if err := docker.UseLocalOrPull(ctx, host.DClient, host.Address, certDownloaderImage, CertificatesServiceName); err != nil {
		return err
	}
	imageCfg := &container.Config{
		Image: certDownloaderImage,
		Env:   containerEnv,
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			"/etc/kubernetes:/etc/kubernetes",
		},
		Privileged: true,
	}
	resp, err := host.DClient.ContainerCreate(ctx, imageCfg, hostCfg, nil, CrtDownloaderContainer)
	if err != nil {
		return fmt.Errorf("Failed to create Certificates deployer container on host [%s]: %v", host.Address, err)
	}

	if err := host.DClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("Failed to start Certificates deployer container on host [%s]: %v", host.Address, err)
	}
	logrus.Debugf("[certificates] Successfully started Certificate deployer container: %s", resp.ID)
	for {
		isDeployerRunning, err := docker.IsContainerRunning(ctx, host.DClient, host.Address, CrtDownloaderContainer, false)
		if err != nil {
			return err
		}
		if isDeployerRunning {
			time.Sleep(5 * time.Second)
			continue
		}
		if err := host.DClient.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{}); err != nil {
			return fmt.Errorf("Failed to delete Certificates deployer container on host [%s]: %v", host.Address, err)
		}
		return nil
	}
}

func DeployAdminConfig(ctx context.Context, kubeConfig, localConfigPath string) error {
	logrus.Debugf("Deploying admin Kubeconfig locally: %s", kubeConfig)
	err := ioutil.WriteFile(localConfigPath, []byte(kubeConfig), 0640)
	if err != nil {
		return fmt.Errorf("Failed to create local admin kubeconfig file: %v", err)
	}
	log.Infof(ctx, "Successfully Deployed local admin kubeconfig at [%s]", localConfigPath)
	return nil
}

func RemoveAdminConfig(ctx context.Context, localConfigPath string) {
	log.Infof(ctx, "Removing local admin Kubeconfig: %s", localConfigPath)
	if err := os.Remove(localConfigPath); err != nil {
		logrus.Warningf("Failed to remove local admin Kubeconfig file: %v", err)
		return
	}
	log.Infof(ctx, "Local admin Kubeconfig removed successfully")
}
