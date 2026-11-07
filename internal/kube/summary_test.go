package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestSummarizeState(t *testing.T) {
	tests := []struct {
		name string
		in   corev1.ContainerState
		want StateSummary
	}{
		{
			name: "waiting in CrashLoopBackOff",
			in:   corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff", Message: "back-off 5m0s"}},
			want: StateSummary{State: "waiting", Reason: "CrashLoopBackOff", Message: "back-off 5m0s"},
		},
		{
			name: "running",
			in:   corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			want: StateSummary{State: "running"},
		},
		{
			name: "terminated by the OOM killer",
			in:   corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137}},
			want: StateSummary{State: "terminated", Reason: "OOMKilled", ExitCode: 137},
		},
		{
			name: "no state reported yet",
			in:   corev1.ContainerState{},
			want: StateSummary{State: "unknown"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := summarizeState(tt.in); got != tt.want {
				t.Errorf("summarizeState() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSummarizePodLastTermination(t *testing.T) {
	tests := []struct {
		name     string
		status   corev1.ContainerStatus
		wantLast *StateSummary
	}{
		{
			// Scenario 4: the current state hides the cause; the last termination shows it.
			name: "OOMKilled shows in last termination",
			status: corev1.ContainerStatus{
				Name:                 "app",
				RestartCount:         5,
				State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137}},
			},
			wantLast: &StateSummary{State: "terminated", Reason: "OOMKilled", ExitCode: 137},
		},
		{
			// Scenario 2: the container never ran, so LastTerminationState is empty.
			// This must not panic, and LastTermination must stay nil.
			name: "never terminated leaves last termination nil",
			status: corev1.ContainerStatus{
				Name:  "app",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CreateContainerConfigError"}},
			},
			wantLast: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := corev1.Pod{Status: corev1.PodStatus{
				Phase:             corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{tt.status},
			}}
			got := SummarizePod(pod)
			if len(got.Containers) != 1 {
				t.Fatalf("got %d containers, want 1", len(got.Containers))
			}
			last := got.Containers[0].LastTermination
			switch {
			case tt.wantLast == nil && last != nil:
				t.Errorf("LastTermination = %+v, want nil", *last)
			case tt.wantLast != nil && (last == nil || *last != *tt.wantLast):
				t.Errorf("LastTermination = %v, want %+v", last, *tt.wantLast)
			}
			if got.Phase != "Running" {
				t.Errorf("Phase = %q, want Running", got.Phase)
			}
		})
	}
}
