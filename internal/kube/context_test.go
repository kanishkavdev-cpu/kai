package kube

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const ns = "shop"

func deployment(name string, podLabels map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: podLabels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: podLabels},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "busybox:1.36"}}},
			},
		},
	}
}

func pod(name string, podLabels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: podLabels},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "app", Ready: true,
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}},
		},
	}
}

func event(name, kind, objName, reason string, at time.Time) *corev1.Event {
	return &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: name, Namespace: ns},
		InvolvedObject: corev1.ObjectReference{Kind: kind, Name: objName, Namespace: ns},
		Reason:         reason,
		Type:           "Warning",
		LastTimestamp:  metav1.NewTime(at),
	}
}

func TestListPodsForDeploymentUsesSelector(t *testing.T) {
	app := map[string]string{"app": "checkout-service"}
	cs := fake.NewClientset(
		deployment("checkout-service", app),
		pod("checkout-1", app),
		pod("checkout-2", app), // a stuck rollout can leave two pods
		pod("payments-1", map[string]string{"app": "payments"}),
	)
	c := New(cs)

	d, err := c.GetDeployment(context.Background(), ns, "checkout-service")
	if err != nil {
		t.Fatal(err)
	}
	pods, err := c.ListPodsForDeployment(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if len(pods) != 2 {
		t.Fatalf("got %d pods, want 2 (payments-1 must be excluded)", len(pods))
	}
}

func TestListEventsForFiltersAndSortsNewestFirst(t *testing.T) {
	now := time.Now()
	cs := fake.NewClientset(
		event("e1", "Pod", "checkout-1", "Pulling", now.Add(-3*time.Minute)),
		event("e2", "Pod", "checkout-1", "BackOff", now.Add(-1*time.Minute)),
		event("e3", "Pod", "checkout-1", "Failed", now.Add(-2*time.Minute)),
		event("e4", "Pod", "other-pod", "Killing", now),
	)
	evs, err := New(cs).ListEventsFor(context.Background(), ns, "Pod", "checkout-1")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range evs {
		got = append(got, e.Reason)
	}
	want := []string{"BackOff", "Failed", "Pulling"}
	if len(got) != len(want) {
		t.Fatalf("got reasons %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got reasons %v, want %v (newest first, other-pod excluded)", got, want)
		}
	}
}

func TestListEventsForMergesDuplicates(t *testing.T) {
	now := time.Now()
	cs := fake.NewClientset(
		event("e1", "PersistentVolumeClaim", "checkout-data", "ProvisioningFailed", now.Add(-2*time.Minute)),
		event("e2", "PersistentVolumeClaim", "checkout-data", "ProvisioningFailed", now.Add(-1*time.Minute)),
		event("e3", "PersistentVolumeClaim", "checkout-data", "ProvisioningFailed", now),
	)
	evs, err := New(cs).ListEventsFor(context.Background(), ns, "PersistentVolumeClaim", "checkout-data")
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("got %d events, want 1 merged event", len(evs))
	}
	if evs[0].Count != 3 {
		t.Errorf("Count = %d, want 3", evs[0].Count)
	}
	// The fake clientset keeps full precision; a real API server rounds to seconds.
	if !evs[0].LastSeen.Equal(now) {
		t.Errorf("LastSeen = %v, want the newest (%v)", evs[0].LastSeen, now)
	}
}

func TestGetContextServiceSelectorMismatch(t *testing.T) {
	app := map[string]string{"app": "checkout-service"}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "checkout-service", Namespace: ns},
		Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "checkout"}}, // scenario 8's bug
	}
	cs := fake.NewClientset(deployment("checkout-service", app), pod("checkout-1", app), svc)

	dc, err := New(cs).GetContext(context.Background(), ns, "checkout-service", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(dc.Services) != 1 {
		t.Fatalf("got %d services, want 1", len(dc.Services))
	}
	if dc.Services[0].SelectsPods {
		t.Errorf("SelectsPods = true, want false: selector app=checkout doesn't match app=checkout-service")
	}
	if len(dc.Pods) != 1 {
		t.Errorf("got %d pods, want 1", len(dc.Pods))
	}
}

func TestGetContextMissingDeployment(t *testing.T) {
	_, err := New(fake.NewClientset()).GetContext(context.Background(), ns, "nope", 10)
	if err == nil {
		t.Fatal("want an error for a missing deployment, got nil")
	}
}
