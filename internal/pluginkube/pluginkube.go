package pluginkube

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"unicode"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// sanitizeVariableName takes a string and converts it into a valid Go variable name
func sanitizeVariableName(input string) string {
	// Replace invalid characters (anything not alphanumeric or "_") with "-"
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	sanitized := re.ReplaceAllString(input, "-")

	// If the name starts with a digit, prepend an underscore to make it valid
	if len(sanitized) > 0 && unicode.IsDigit(rune(sanitized[0])) {
		sanitized = "_" + sanitized
	}

	// Replace any remaining underscores with dashes
	sanitized = strings.ReplaceAll(sanitized, "_", "-")

	return sanitized
}

// ReadSecret reads a Kubernetes secret from the given namespace and name.
func ReadSecret(clientset *kubernetes.Clientset, namespace, secretName string, convertToGoVars bool) (map[string]string, error) {
	// Get the secret
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %v", err)
	}

	// Decode the secret data
	decodedSecret := make(map[string]string)
	for key, value := range secret.Data {

		if convertToGoVars {
			sanitizedKey := sanitizeVariableName(key)
			decodedSecret[sanitizedKey] = string(value)
		} else {
			//decodedSecret[key] = base64.StdEncoding.EncodeToString(value)
			decodedSecret[key] = string(value)
		}

	}

	return decodedSecret, nil

}

func KubeClientSet() *kubernetes.Clientset {
	// Build Kubernetes clientset

	// Check if LOCAL_KUBECONFIG variable is set - then use this file as a KUBECONFIG (local mode for testing purposes)
	// Otherwise work using in cluster config

	var clientset *kubernetes.Clientset

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
