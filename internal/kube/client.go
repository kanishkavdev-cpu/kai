package kube

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Options says which cluster to connect to. Empty fields mean "use the default"
type Options struct {
	Kubeconfig string // "" will mean $KUBECONFIG, then ~/.kube/config
	Context    string // "" mean the current context
}

// NewClientset builds a Kubernetes clientset from the kubeconfig, falling back to
// in-cluster config when running inside a pod.
func NewClientset(opts Options) (*kubernetes.Clientset, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.Kubeconfig != "" {
		rules.ExplicitPath = opts.Kubeconfig
	}
	overrides := &clientcmd.ConfigOverrides{CurrentContext: opts.Context}

	clientCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	clientSet, err := kubernetes.NewForConfig(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}
	return clientSet, nil
}
