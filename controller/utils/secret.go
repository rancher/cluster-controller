package utils

import (
	"fmt"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	SecretSuffix       = "secret-token"
	ServiceAccountName = "serviceAccountToken"
	CaCertName         = "caCert"
)

func GetSecret(k8sClient *kubernetes.Clientset, secretName string) (*v1.Secret, error) {
	return k8sClient.CoreV1().Secrets(metav1.NamespaceSystem).Get(secretName, metav1.GetOptions{})
}

func UpdateSecret(k8sClient *kubernetes.Clientset, data map[string][]byte, secretName string) error {
	secretData := make(map[string][]byte)
	for name, value := range data {
		secretData[name] = value
	}
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: metav1.NamespaceSystem,
		},
		Data: secretData,
	}
	if _, err := k8sClient.CoreV1().Secrets(metav1.NamespaceSystem).Create(secret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		// update secret if its already exist
		oldSecret, err := k8sClient.CoreV1().Secrets(metav1.NamespaceSystem).Get(secretName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		newData := oldSecret.Data
		for name, value := range data {
			newData[name] = value
		}
		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: metav1.NamespaceSystem,
			},
			Data: newData,
		}
		if _, err := k8sClient.CoreV1().Secrets(metav1.NamespaceSystem).Update(secret); err != nil {
			return err
		}
	}
	return nil
}

func GetSecretName(cluster *v3.Cluster) string {
	return fmt.Sprintf("%s-%s", cluster.Name, SecretSuffix)
}
