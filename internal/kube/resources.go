package kube

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Client reads Kubernetes objects. It holds an interface so tests can use a fake.
type Client struct {
	cs kubernetes.Interface
}

// New wraps a clientset (real or fake) in a Client.
func New(cs kubernetes.Interface) *Client {
	return &Client{cs: cs}
}

func (c *Client) GetDeployment(ctx context.Context, ns, name string) (*appsv1.Deployment, error) {
	d, err := c.cs.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get deployment %s %s: %w", ns, name, err)
	}
	return d, nil
}

func (c *Client) ListPodsForDeployment(ctx context.Context, d *appsv1.Deployment) ([]corev1.Pod, error) {
	sel, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("parse selector for deployment %s %s: %w", d.Namespace, d.Name, err)
	}
	list, err := c.cs.CoreV1().Pods(d.Namespace).List(ctx, metav1.ListOptions{LabelSelector: sel.String()})
	if err != nil {
		return nil, fmt.Errorf("list pods for deployment %s %s: %w", d.Namespace, d.Name, err)
	}
	return list.Items, nil
}

func (c *Client) ListServices(ctx context.Context, ns string) ([]corev1.Service, error) {
	return nil, nil // TODO: you write this
}

func (c *Client) ListPVCs(ctx context.Context, ns string) ([]corev1.PersistentVolumeClaim, error) {
	return nil, nil // TODO: you write this
}

func (c *Client) ListIngresses(ctx context.Context, ns string) ([]networkingv1.Ingress, error) {
	return nil, nil // TODO: you write this
}

func (c *Client) ListEvents(ctx context.Context, ns string) ([]corev1.Event, error) {
	return nil, nil // TODO: you write this
}
