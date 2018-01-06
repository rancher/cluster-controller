package controller

import (
	"context"

	"os"
	"path/filepath"

	"encoding/json"

	"fmt"
	"time"

	"github.com/rancher/norman/types/slice"
	core "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	v3schema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	cacheRoot     = "helm-controller"
	releasesField = "releases"
)

func Register(management *config.ManagementContext) {
	namespaceClient := management.Core.Namespaces("")
	namespaceLifecycle := &Lifecycle{
		NameSpaceClient:       namespaceClient,
		K8sClient:             management.K8sClient,
		TemplateVersionClient: management.Management.TemplateVersions(""),
		CacheRoot:             filepath.Join(os.Getenv("HOME"), cacheRoot),
	}
	namespaceClient.AddLifecycle("helm-controller", namespaceLifecycle)
}

type Lifecycle struct {
	NameSpaceClient       core.NamespaceInterface
	TemplateVersionClient v3.TemplateVersionInterface
	K8sClient             kubernetes.Interface
	CacheRoot             string
}

func (l *Lifecycle) Create(obj *v1.Namespace) (*v1.Namespace, error) {
	templateVersionID := obj.Annotations["field.cattle.io/externalId"]
	if templateVersionID != "" {
		if err := l.Run(obj, "install", templateVersionID); err != nil {
			return obj, err
		}
		configMaps, err := l.K8sClient.CoreV1().ConfigMaps(obj.Name).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", "NAME", obj.Name),
		})
		if err != nil {
			return obj, err
		}
		releases := map[string]v3schema.ReleaseInfo{}
		for _, cm := range configMaps.Items {
			releaseInfo := v3schema.ReleaseInfo{}
			releaseInfo.Name = configMaps.Items[0].Name
			releaseInfo.Version = configMaps.Items[0].Labels["VERSION"]
			releaseInfo.CreateTimestamp = configMaps.Items[0].CreationTimestamp.Format(time.RFC3339)
			releaseInfo.ModifiedAt = configMaps.Items[0].Labels["MODIFIED_AT"]
			releaseInfo.TemplateVersionID = templateVersionID
			releases[cm.Name] = releaseInfo
		}
		data, err := json.Marshal(releases)
		if err != nil {
			return obj, err
		}
		obj.Annotations["releases"] = string(data)
	}
	return obj, nil
}

func (l *Lifecycle) Updated(obj *v1.Namespace) (*v1.Namespace, error) {
	templateVersionID := obj.Annotations["field.cattle.io/externalId"]
	if templateVersionID != "" {
		templateVersion, err := l.TemplateVersionClient.Get(templateVersionID, metav1.GetOptions{})
		if err != nil {
			return obj, err
		}
		if err := l.saveTemplates(obj, templateVersion); err != nil {
			return obj, err
		}
		configMaps, err := l.K8sClient.CoreV1().ConfigMaps(obj.Name).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", "NAME", obj.Name),
		})
		if err != nil {
			return obj, err
		}
		releasesInfo := map[string]v3schema.ReleaseInfo{}
		if obj.Annotations[releasesField] != "" {
			if err := json.Unmarshal([]byte(obj.Annotations[releasesField]), &releasesInfo); err != nil {
				return obj, err
			}
			alreadyExistedReleaseNames := []string{}
			for k := range releasesInfo {
				alreadyExistedReleaseNames = append(alreadyExistedReleaseNames, k)
			}
			for _, cm := range configMaps.Items {
				if !slice.ContainsString(alreadyExistedReleaseNames, cm.Name) {
					logrus.Infof("uploading release %s into namespace %s", cm.Name, obj.Name)
					releaseInfo := v3schema.ReleaseInfo{}
					releaseInfo.Name = cm.Name
					releaseInfo.Version = cm.Labels["VERSION"]
					releaseInfo.CreateTimestamp = cm.CreationTimestamp.Format(time.RFC3339)
					releaseInfo.ModifiedAt = cm.Labels["MODIFIED_AT"]
					releaseInfo.TemplateVersionID = templateVersionID
					releasesInfo[cm.Name] = releaseInfo
				}
			}
			data, err := json.Marshal(releasesInfo)
			if err != nil {
				return obj, err
			}
			obj.Annotations[releasesField] = string(data)
			return obj, nil
		}

	}
	return obj, nil
}

func (l *Lifecycle) Remove(obj *v1.Namespace) (*v1.Namespace, error) {
	return obj, nil
}

func (l *Lifecycle) Run(obj *v1.Namespace, action, templateVersionID string) error {
	templateVersion, err := l.TemplateVersionClient.Get(templateVersionID, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if err := l.saveTemplates(obj, templateVersion); err != nil {
		return err
	}
	dir, err := l.writeTempFolder(templateVersion)
	if err != nil {
		return err
	}
	cont, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr := generateRandomPort()
	go startTiller(cont, addr, obj.Name)
	switch action {
	case "install":
		if err := installCharts(dir, addr, obj); err != nil {
			return err
		}
	case "delete":
		if err := deleteCharts(dir, addr, obj); err != nil {
			return err
		}
	}
	return nil
}

func (l *Lifecycle) saveTemplates(obj *v1.Namespace, templateVersion *v3.TemplateVersion) error {
	templates := map[string]string{}
	for _, file := range templateVersion.Spec.Files {
		templates[file.Name] = file.Contents
	}
	data, err := json.Marshal(templates)
	if err != nil {
		return err
	}
	obj.Annotations["field.cattle.io/templates"] = string(data)
	return nil
}
