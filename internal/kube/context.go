package kube

import (
	"context"
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// DeploymentContext is everything kai knows about one Deployment: the
// Deployment itself, its pods (with events and logs), and the related
// Services, PVCs and Ingresses. It is what `kai get-context` prints, and what
// the analyzers (Day 2) and the agent (Day 3) will read.
type DeploymentContext struct {
	Deployment DeploymentSummary `json:"deployment"`
	Pods       []PodContext      `json:"pods"`
	Services   []ServiceSummary  `json:"services,omitempty"`
	PVCs       []PVCSummary      `json:"pvcs,omitempty"`
	Ingresses  []IngressSummary  `json:"ingresses,omitempty"`
	// Errors lists the parts that could not be fetched. A missing piece is
	// reported here instead of failing the whole command.
	Errors []string `json:"errors,omitempty"`
}

// DeploymentSummary is the Deployment's spec and status, reduced.
type DeploymentSummary struct {
	Name       string                 `json:"name"`
	Namespace  string                 `json:"namespace"`
	Selector   string                 `json:"selector"`
	Replicas   ReplicaCounts          `json:"replicas"`
	Conditions []ConditionSummary     `json:"conditions,omitempty"`
	Containers []ContainerSpecSummary `json:"containers"`
	Events     []EventSummary         `json:"events,omitempty"`
}

// ReplicaCounts compares what the Deployment wants with what it has.
type ReplicaCounts struct {
	Desired     int32 `json:"desired"`
	Updated     int32 `json:"updated"`
	Ready       int32 `json:"ready"`
	Available   int32 `json:"available"`
	Unavailable int32 `json:"unavailable"`
}

// ContainerSpecSummary is what the pod template asks for: image, resources,
// ports and probes. Comparing this with the status finds bugs like a probe on
// the wrong port or a memory limit that is too low.
type ContainerSpecSummary struct {
	Name           string            `json:"name"`
	Image          string            `json:"image"`
	Requests       map[string]string `json:"requests,omitempty"`
	Limits         map[string]string `json:"limits,omitempty"`
	Ports          []int32           `json:"ports,omitempty"`
	LivenessProbe  string            `json:"liveness_probe,omitempty"`
	ReadinessProbe string            `json:"readiness_probe,omitempty"`
	EnvFrom        []string          `json:"env_from,omitempty"` // ConfigMap/Secret keys the env vars read
}

// PodContext is a pod's summary plus its events and logs.
type PodContext struct {
	PodSummary
	Events []EventSummary  `json:"events,omitempty"`
	Logs   []ContainerLogs `json:"logs,omitempty"`
}

// ContainerLogs holds the tail of a container's current and previous logs.
// A log that can't be read is recorded as an error string, not a failure.
type ContainerLogs struct {
	Container     string `json:"container"`
	Current       string `json:"current,omitempty"`
	CurrentError  string `json:"current_error,omitempty"`
	Previous      string `json:"previous,omitempty"`
	PreviousError string `json:"previous_error,omitempty"`
}

// ServiceSummary is a Service in the namespace, and whether its selector
// matches the Deployment's pods.
type ServiceSummary struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Selector     map[string]string `json:"selector"`
	Ports        []string          `json:"ports,omitempty"`
	SelectsPods  bool              `json:"selects_deployment_pods"`
	MatchingPods []string          `json:"matching_pods,omitempty"`
}

// PVCSummary is a PersistentVolumeClaim used by the Deployment's pods.
type PVCSummary struct {
	Name         string         `json:"name"`
	Phase        string         `json:"phase"`
	StorageClass string         `json:"storage_class,omitempty"`
	Requested    string         `json:"requested,omitempty"`
	Events       []EventSummary `json:"events,omitempty"`
}

// IngressSummary is an Ingress in the namespace and the backends it routes to.
type IngressSummary struct {
	Name     string   `json:"name"`
	Backends []string `json:"backends"` // service:port
}

// GetContext collects the DeploymentContext for one Deployment. Only a failure
// to get the Deployment itself is an error; anything else that can't be read
// is recorded in DeploymentContext.Errors.
func (c *Client) GetContext(ctx context.Context, ns, name string, logLines int64) (*DeploymentContext, error) {
	d, err := c.GetDeployment(ctx, ns, name)
	if err != nil {
		return nil, err
	}

	out := &DeploymentContext{Deployment: summarizeDeployment(d)}
	addErr := func(err error) { out.Errors = append(out.Errors, err.Error()) }

	if evs, err := c.ListEventsFor(ctx, ns, "Deployment", name); err != nil {
		addErr(err)
	} else {
		out.Deployment.Events = evs
	}

	pods, err := c.ListPodsForDeployment(ctx, d)
	if err != nil {
		addErr(err)
	}
	for _, p := range pods {
		out.Pods = append(out.Pods, c.podContext(ctx, p, logLines, addErr))
	}

	if svcs, err := c.ListServices(ctx, ns); err != nil {
		addErr(err)
	} else {
		out.Services = summarizeServices(svcs, pods)
	}

	if pvcs, err := c.ListPVCs(ctx, ns); err != nil {
		addErr(err)
	} else {
		for _, pvc := range pvcs {
			if !usesClaim(d, pvc.Name) {
				continue
			}
			s := summarizePVC(pvc)
			if evs, err := c.ListEventsFor(ctx, ns, "PersistentVolumeClaim", pvc.Name); err != nil {
				addErr(err)
			} else {
				s.Events = evs
			}
			out.PVCs = append(out.PVCs, s)
		}
	}

	if ings, err := c.ListIngresses(ctx, ns); err != nil {
		addErr(err)
	} else {
		for _, ing := range ings {
			out.Ingresses = append(out.Ingresses, summarizeIngress(ing))
		}
	}

	return out, nil
}

// podContext summarizes one pod and adds its events and log tails.
func (c *Client) podContext(ctx context.Context, p corev1.Pod, logLines int64, addErr func(error)) PodContext {
	pc := PodContext{PodSummary: SummarizePod(p)}

	if evs, err := c.ListEventsFor(ctx, p.Namespace, "Pod", p.Name); err != nil {
		addErr(err)
	} else {
		pc.Events = evs
	}

	for _, cs := range pc.Containers {
		logs := ContainerLogs{Container: cs.Name}
		// A container that is waiting and has never run (image pull errors,
		// CreateContainerConfigError) has no log to read.
		neverRan := cs.State.State == "waiting" && cs.RestartCount == 0
		if !neverRan {
			if s, err := c.GetLogs(ctx, p.Namespace, p.Name, cs.Name, false, logLines); err != nil {
				logs.CurrentError = err.Error()
			} else {
				logs.Current = s
			}
		}
		// The previous run's log holds a crashed container's last words.
		// Reading it sometimes fails (the old container is already cleaned
		// up), so that is recorded, not treated as fatal.
		if cs.LastTermination != nil {
			if s, err := c.GetLogs(ctx, p.Namespace, p.Name, cs.Name, true, logLines); err != nil {
				logs.PreviousError = err.Error()
			} else {
				logs.Previous = s
			}
		}
		if logs != (ContainerLogs{Container: cs.Name}) {
			pc.Logs = append(pc.Logs, logs)
		}
	}
	return pc
}

func summarizeDeployment(d *appsv1.Deployment) DeploymentSummary {
	desired := int32(1)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}
	s := DeploymentSummary{
		Name:      d.Name,
		Namespace: d.Namespace,
		Replicas: ReplicaCounts{
			Desired:     desired,
			Updated:     d.Status.UpdatedReplicas,
			Ready:       d.Status.ReadyReplicas,
			Available:   d.Status.AvailableReplicas,
			Unavailable: d.Status.UnavailableReplicas,
		},
	}
	if d.Spec.Selector != nil {
		s.Selector = labels.Set(d.Spec.Selector.MatchLabels).String()
	}
	for _, c := range d.Status.Conditions {
		s.Conditions = append(s.Conditions, ConditionSummary{
			Type:    string(c.Type),
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		})
	}
	for _, c := range d.Spec.Template.Spec.Containers {
		s.Containers = append(s.Containers, summarizeContainerSpec(c))
	}
	return s
}

func summarizeContainerSpec(c corev1.Container) ContainerSpecSummary {
	s := ContainerSpecSummary{
		Name:           c.Name,
		Image:          c.Image,
		Requests:       resourceMap(c.Resources.Requests),
		Limits:         resourceMap(c.Resources.Limits),
		LivenessProbe:  describeProbe(c.LivenessProbe),
		ReadinessProbe: describeProbe(c.ReadinessProbe),
	}
	for _, p := range c.Ports {
		s.Ports = append(s.Ports, p.ContainerPort)
	}
	for _, e := range c.Env {
		if e.ValueFrom == nil {
			continue
		}
		if r := e.ValueFrom.ConfigMapKeyRef; r != nil {
			s.EnvFrom = append(s.EnvFrom, fmt.Sprintf("%s <- ConfigMap %s key %s", e.Name, r.Name, r.Key))
		}
		if r := e.ValueFrom.SecretKeyRef; r != nil {
			s.EnvFrom = append(s.EnvFrom, fmt.Sprintf("%s <- Secret %s key %s", e.Name, r.Name, r.Key))
		}
	}
	return s
}

func resourceMap(rl corev1.ResourceList) map[string]string {
	if len(rl) == 0 {
		return nil
	}
	m := make(map[string]string, len(rl))
	for name, q := range rl {
		m[string(name)] = q.String()
	}
	return m
}

// describeProbe turns a probe into one readable line, such as "httpGet :9090/".
func describeProbe(p *corev1.Probe) string {
	if p == nil {
		return ""
	}
	var what string
	switch {
	case p.HTTPGet != nil:
		what = fmt.Sprintf("httpGet :%s%s", p.HTTPGet.Port.String(), p.HTTPGet.Path)
	case p.TCPSocket != nil:
		what = fmt.Sprintf("tcpSocket :%s", p.TCPSocket.Port.String())
	case p.Exec != nil:
		what = "exec " + strings.Join(p.Exec.Command, " ")
	case p.GRPC != nil:
		what = fmt.Sprintf("grpc :%d", p.GRPC.Port)
	default:
		what = "unknown"
	}
	return fmt.Sprintf("%s delay=%ds period=%ds failureThreshold=%d",
		what, p.InitialDelaySeconds, p.PeriodSeconds, p.FailureThreshold)
}

// summarizeServices reports every Service in the namespace and which of the
// Deployment's pods its selector matches. A Service that matches none of
// them is the evidence for a selector mismatch (scenario 8).
func summarizeServices(svcs []corev1.Service, pods []corev1.Pod) []ServiceSummary {
	var out []ServiceSummary
	for _, svc := range svcs {
		s := ServiceSummary{
			Name:     svc.Name,
			Type:     string(svc.Spec.Type),
			Selector: svc.Spec.Selector,
		}
		for _, p := range svc.Spec.Ports {
			s.Ports = append(s.Ports, fmt.Sprintf("%d->%s", p.Port, p.TargetPort.String()))
		}
		// A Service with no selector selects nothing automatically.
		if len(svc.Spec.Selector) > 0 {
			sel := labels.SelectorFromSet(svc.Spec.Selector)
			for _, p := range pods {
				if sel.Matches(labels.Set(p.Labels)) {
					s.MatchingPods = append(s.MatchingPods, p.Name)
				}
			}
		}
		s.SelectsPods = len(s.MatchingPods) > 0
		out = append(out, s)
	}
	return out
}

// usesClaim reports whether the Deployment's pod template mounts the PVC.
func usesClaim(d *appsv1.Deployment, claim string) bool {
	for _, v := range d.Spec.Template.Spec.Volumes {
		if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == claim {
			return true
		}
	}
	return false
}

func summarizePVC(pvc corev1.PersistentVolumeClaim) PVCSummary {
	s := PVCSummary{Name: pvc.Name, Phase: string(pvc.Status.Phase)}
	if pvc.Spec.StorageClassName != nil {
		s.StorageClass = *pvc.Spec.StorageClassName
	}
	if q, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		s.Requested = q.String()
	}
	return s
}

func summarizeIngress(ing networkingv1.Ingress) IngressSummary {
	s := IngressSummary{Name: ing.Name}
	add := func(b *networkingv1.IngressBackend) {
		if b == nil || b.Service == nil {
			return
		}
		port := b.Service.Port.Name
		if port == "" {
			port = fmt.Sprint(b.Service.Port.Number)
		}
		s.Backends = append(s.Backends, b.Service.Name+":"+port)
	}
	add(ing.Spec.DefaultBackend)
	for _, r := range ing.Spec.Rules {
		if r.HTTP == nil {
			continue
		}
		for _, p := range r.HTTP.Paths {
			add(&p.Backend)
		}
	}
	sort.Strings(s.Backends)
	return s
}
