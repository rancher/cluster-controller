package controller

import (
	"io/ioutil"
	"math/rand"
	"net/url"
	"path/filepath"
	"time"

	"os"
	"os/exec"

	"strconv"

	"encoding/json"
	"fmt"
	"strings"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"golang.org/x/net/context"
	"k8s.io/api/core/v1"
	"github.com/rancher/norman/types/convert"
)

const (
	base       = 32768
	end        = 61000
	tillerName = "tiller"
	helmName   = "helm"
)

func (l *Lifecycle) writeTempFolder(templateVersion *v3.TemplateVersion) (string, error) {
	files := templateVersion.Spec.Files
	externalID := templateVersion.Spec.ExternalID
	query, err := url.ParseQuery(externalID)
	if err != nil {
		return "", err
	}
	templateName := query.Get("template")
	for _, file := range files {
		fp := filepath.Join(l.CacheRoot, file.Name)
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			return "", err
		}
		if err := ioutil.WriteFile(fp, []byte(file.Contents), 0755); err != nil {
			return "", err
		}
	}
	return filepath.Join(l.CacheRoot, templateName), nil
}

// startTiller start tiller server and return the listening address of the grpc address
func startTiller(context context.Context, port, namespace string) error {
	// todo: we need to pass impersonation kubeconfig
	cmd := exec.Command(tillerName, "--listen", ":"+port)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", "TILLER_NAMESPACE", namespace))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Wait()
	select {
	case <-context.Done():
		return cmd.Process.Kill()
	}
}

func generateRandomPort() string {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	port := base + r1.Intn(end-base+1)
	return strconv.Itoa(port)
}

func installCharts(rootDir, port string, obj *v1.Namespace) error {
	setValues := []string{}
	if obj.Annotations["field.cattle.io/answers"] != "" {
		answers := map[string]interface{}{}
		if err := json.Unmarshal([]byte(obj.Annotations["field.cattle.io/answers"]), &answers); err != nil {
			return err
		}
		result := []string{}
		for k, v := range answers {
			result = append(result, fmt.Sprintf("%s=%s", k, convert.ToString(v)))
		}
		setValues = append([]string{"--set"}, strings.Join(result, ","))
	}
	commands := append([]string{"install", "--namespace", obj.Name, "--name", obj.Name}, setValues...)
	commands = append(commands, rootDir)

	cmd := exec.Command(helmName, commands...)
	cmd.Env = []string{fmt.Sprintf("%s=%s", "HELM_HOST", "127.0.0.1:"+port)}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func deleteCharts(rootDir, port string, obj *v1.Namespace) error {
	cmd := exec.Command(helmName, "delete", "--purge", obj.Name)
	cmd.Env = []string{fmt.Sprintf("%s=%s", "HELM_HOST", "127.0.0.1:"+port)}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}
