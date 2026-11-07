package kube

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Options says which cluster to connect to. Empty fields mean "use the default".
type Options struct {
	Kubeconfig string // "" means $KUBECONFIG, then ~/.kube/config
	Context    string // "" means the current context
}

// loader builds the client-go config loader from Options. NewClientset and
// ResolveTarget share it, so they always agree on which cluster is used.
func loader(opts Options) clientcmd.ClientConfig {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.Kubeconfig != "" {
		rules.ExplicitPath = opts.Kubeconfig
	}
	overrides := &clientcmd.ConfigOverrides{CurrentContext: opts.Context}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
}

// NewClientset builds a Kubernetes clientset from the kubeconfig, falling back to
// in-cluster config when running inside a pod.
func NewClientset(opts Options) (*kubernetes.Clientset, error) {
	clientCfg, err := loader(opts).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	clientSet, err := kubernetes.NewForConfig(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}
	return clientSet, nil
}

// ResolveTarget returns the context name kai will talk to and the default
// namespace for it (from the kubeconfig context, or the pod's own namespace
// when running in-cluster). kai prints the context before doing anything, so
// the user can see it isn't talking to the wrong cluster.
func ResolveTarget(opts Options) (contextName, namespace string, err error) {
	cc := loader(opts)
	namespace, _, err = cc.Namespace()
	if err != nil {
		return "", "", fmt.Errorf("resolve namespace: %w", err)
	}
	if opts.Context != "" {
		return opts.Context, namespace, nil
	}
	raw, err := cc.RawConfig()
	if err != nil {
		return "", "", fmt.Errorf("read kubeconfig: %w", err)
	}
	if raw.CurrentContext == "" {
		return "in-cluster", namespace, nil
	}
	return raw.CurrentContext, namespace, nil
}
