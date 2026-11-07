package kube

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// Log size caps. One huge log must not flood the output (or, on Day 3, the
// model's context window), so both lines and bytes are limited.
const (
	DefaultLogLines = 50
	maxLogBytes     = 16 * 1024
)

// GetLogs returns the last tailLines lines of one container's log. With
// previous set, it reads the log of the container's last run, which is where
// a crashed container's error message is.
func (c *Client) GetLogs(ctx context.Context, ns, pod, container string, previous bool, tailLines int64) (string, error) {
	if tailLines <= 0 {
		tailLines = DefaultLogLines
	}
	limitBytes := int64(maxLogBytes)
	opts := &corev1.PodLogOptions{
		Container:  container,
		Previous:   previous,
		TailLines:  &tailLines,
		LimitBytes: &limitBytes,
	}

	raw, err := c.cs.CoreV1().Pods(ns).GetLogs(pod, opts).DoRaw(ctx)
	if err != nil {
		which := "current"
		if previous {
			which = "previous"
		}
		return "", fmt.Errorf("get %s logs for %s/%s container %s: %w", which, ns, pod, container, err)
	}
	// When the requested container has already been removed, the kubelet
	// answers with success and puts this message where the log should be.
	// Turn it into an error so it can't be mistaken for the app's own output.
	if bytes.HasPrefix(raw, []byte("unable to retrieve container logs")) {
		return "", fmt.Errorf("%s/%s container %s: %w (%s)", ns, pod, container, ErrLogsUnavailable, raw)
	}
	return string(raw), nil
}

// ErrLogsUnavailable means the container existed but its log is gone, for
// example because the kubelet already removed an older crashed container.
// Callers can check for it with errors.Is.
var ErrLogsUnavailable = errors.New("logs no longer available")
