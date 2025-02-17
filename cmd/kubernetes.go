// Refactor: to be smarter about discovery
// Copyright (c) 2025 Robert B. Gordon
// Licensed under the MIT License.

package cmd

import (
	"fmt"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// getKubeConfig returns a Kubernetes REST config. It uses in-cluster
// configuration if available, otherwise falls back to the user's
// local kubeconfig file.
func getKubeConfig() (*rest.Config, error) {
	// Attempt in-cluster configuration
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	if err != rest.ErrNotInCluster {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Fall back to local kubeconfig
	kubeconfig := kubeconfigPath()
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

// kubeconfigPath returns the default kubeconfig path or an empty string if the home directory is not found.
func kubeconfigPath() string {
	if home, err := homedir.Dir(); err == nil {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}

// newK8sClient creates a new Kubernetes clientset based on the current configuration.
func newK8sClient() (*kubernetes.Clientset, error) {
	config, err := getKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load Kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return clientset, nil
}
