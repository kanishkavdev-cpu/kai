package kube

import (
	corev1 "k8s.io/api/core/v1"
)

// PodSummary holds the parts of a pod that matter for troubleshooting, like
// the useful sections of `kubectl describe pod`.
type PodSummary struct {
	Name       string             `json:"name"`
	Phase      string             `json:"phase"`
	Labels     map[string]string  `json:"labels,omitempty"`
	Node       string             `json:"node,omitempty"`
	Conditions []ConditionSummary `json:"conditions,omitempty"`
	Containers []ContainerSummary `json:"containers,omitempty"`
}

// ConditionSummary is one pod or deployment condition, such as Ready or Progressing.
type ConditionSummary struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// ContainerSummary is one container's status.
type ContainerSummary struct {
	Name         string       `json:"name"`
	Image        string       `json:"image"`
	Ready        bool         `json:"ready"`
	RestartCount int32        `json:"restart_count"`
	State        StateSummary `json:"state"`
	// LastTermination is nil when the container has never terminated
	// (for example CreateContainerConfigError, where it never started).
	LastTermination *StateSummary `json:"last_termination,omitempty"`
}

// StateSummary flattens corev1.ContainerState, which has three pointers of
// which only one is set, into a single value.
type StateSummary struct {
	State    string `json:"state"` // "waiting", "running", "terminated" or "unknown"
	Reason   string `json:"reason,omitempty"`
	Message  string `json:"message,omitempty"`
	ExitCode int32  `json:"exit_code,omitempty"`
}

// SummarizePod converts a pod into a PodSummary. It does not call the API,
// so it can be tested with a hand-built corev1.Pod.
func SummarizePod(p corev1.Pod) PodSummary {
	s := PodSummary{
		Name:   p.Name,
		Phase:  string(p.Status.Phase),
		Labels: p.Labels,
		Node:   p.Spec.NodeName,
	}

	for _, c := range p.Status.Conditions {
		s.Conditions = append(s.Conditions, ConditionSummary{
			Type:    string(c.Type),
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		})
	}

	for _, cs := range p.Status.ContainerStatuses {
		cont := ContainerSummary{
			Name:         cs.Name,
			Image:        cs.Image,
			Ready:        cs.Ready,
			RestartCount: cs.RestartCount,
			State:        summarizeState(cs.State),
		}
		if cs.LastTerminationState.Terminated != nil {
			last := summarizeState(cs.LastTerminationState)
			cont.LastTermination = &last
		}
		s.Containers = append(s.Containers, cont)
	}
	return s
}

// summarizeState turns the one non-nil pointer in a ContainerState into a
// StateSummary. Every pointer is checked before it is read.
func summarizeState(st corev1.ContainerState) StateSummary {
	switch {
	case st.Waiting != nil:
		return StateSummary{State: "waiting", Reason: st.Waiting.Reason, Message: st.Waiting.Message}
	case st.Running != nil:
		return StateSummary{State: "running"}
	case st.Terminated != nil:
		return StateSummary{
			State:    "terminated",
			Reason:   st.Terminated.Reason,
			Message:  st.Terminated.Message,
			ExitCode: st.Terminated.ExitCode,
		}
	default:
		// The kubelet hasn't reported a state yet.
		return StateSummary{State: "unknown"}
	}
}
