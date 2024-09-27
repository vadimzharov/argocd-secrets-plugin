package pluginkube

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ReadSecret reads a Kubernetes secret from the given namespace and name.
func ReadSecret(clientset *kubernetes.Clientset, namespace, secretName string) (map[string]string, error) {
	// Get the secret
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %v", err)
	}

	// Decode the secret data
	decodedSecret := make(map[string]string)
	for key, value := range secret.Data {
		//decodedSecret[key] = base64.StdEncoding.EncodeToString(value)
		deckey := strings.ReplaceAll(key, ".", "_")
		decodedSecret[deckey] = string(value)

	}

	return decodedSecret, nil

}

func KubeClientSet() *kubernetes.Clientset {
	// Build Kubernetes clientset

	// Check if LOCAL_KUBECONFIG variable is set - then use this file as a KUBECONFIG (local mode for testing purposes)
	// Otherwise work using in cluster config

	var clientset *kubernetes.Clientset
	//kubeconfig = "/Users/vadim/my-stuff/github/argocd-appsets-secret-plugin/kubeconfig"

	if kubeconfigEnv := os.Getenv("LOCAL_KUBECONFIG"); kubeconfigEnv != "" {
		// Load kubeconfig
		log.Println("LOCAL_KUBECONFIG environment variable is set, using local kubeconfig file instead of InClusterConfig")

		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigEnv)
		if err != nil {
			log.Fatalf("error loading kubeconfig: %v", err)
		}

		// Create Kubernetes clientset
		clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			log.Fatalf("error creating Kubernetes clientset: %v", err)
		}

	} else {
		config, err := rest.InClusterConfig()
		if err != nil {
			log.Fatalf("error creating in-cluster config: %v", err)
		}

		// Create Kubernetes clientset
		clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			log.Fatalf("error creating Kubernetes clientset: %v", err)
		}

	}

	return clientset
}
