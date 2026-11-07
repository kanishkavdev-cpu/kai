package kube

import (
	"context"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

// EventSummary is one event, reduced to what matters for troubleshooting.
type EventSummary struct {
	Type     string    `json:"type"` // Normal or Warning
	Reason   string    `json:"reason"`
	Message  string    `json:"message"`
	Count    int32     `json:"count"`
	LastSeen time.Time `json:"last_seen"`
	Object   string    `json:"object"` // for example Pod/checkout-service-abc12
}

// ListEventsFor returns the events about one object (for example kind "Pod",
// name "checkout-service-abc12"), newest first.
func (c *Client) ListEventsFor(ctx context.Context, ns, kind, name string) ([]EventSummary, error) {
	sel := fields.Set{
		"involvedObject.kind": kind,
		"involvedObject.name": name,
	}.AsSelector().String()

	list, err := c.cs.CoreV1().Events(ns).List(ctx, metav1.ListOptions{FieldSelector: sel})
	if err != nil {
		return nil, fmt.Errorf("list events for %s %s/%s: %w", kind, ns, name, err)
	}

	var out []EventSummary
	for _, e := range list.Items {
		// The API server already filtered by the field selector, but the fake
		// clientset used in tests ignores field selectors, so check again.
		if e.InvolvedObject.Kind != kind || e.InvolvedObject.Name != name {
			continue
		}
		out = append(out, summarizeEvent(e))
	}
	SortEventsNewestFirst(out)
	return mergeDuplicateEvents(out), nil
}

// mergeDuplicateEvents folds events with the same type, reason, message and
// object into one, adding up their counts. The same failure is often
// recorded as many separate Event objects; repeating it wastes space (and,
// on Day 3, the model's tokens). evs must be sorted newest first, so the kept
// copy has the latest LastSeen.
func mergeDuplicateEvents(evs []EventSummary) []EventSummary {
	type key struct{ typ, reason, message, object string }
	index := make(map[key]int)
	var out []EventSummary
	for _, e := range evs {
		k := key{e.Type, e.Reason, e.Message, e.Object}
		if i, seen := index[k]; seen {
			out[i].Count += e.Count
			continue
		}
		index[k] = len(out)
		out = append(out, e)
	}
	return out
}

// SortEventsNewestFirst sorts events by LastSeen, most recent first.
func SortEventsNewestFirst(evs []EventSummary) {
	slices.SortStableFunc(evs, func(a, b EventSummary) int {
		return b.LastSeen.Compare(a.LastSeen)
	})
}

func summarizeEvent(e corev1.Event) EventSummary {
	count := e.Count
	if e.Series != nil && e.Series.Count > count {
		count = e.Series.Count
	}
	if count == 0 {
		count = 1
	}
	return EventSummary{
		Type:     e.Type,
		Reason:   e.Reason,
		Message:  e.Message,
		Count:    count,
		LastSeen: eventTime(e),
		Object:   e.InvolvedObject.Kind + "/" + e.InvolvedObject.Name,
	}
}

// eventTime picks the best "when did this last happen" timestamp. Events are
// written by different components, and each fills in different fields.
func eventTime(e corev1.Event) time.Time {
	switch {
	case e.Series != nil && !e.Series.LastObservedTime.IsZero():
		return e.Series.LastObservedTime.Time
	case !e.LastTimestamp.IsZero():
		return e.LastTimestamp.Time
	case !e.EventTime.IsZero():
		return e.EventTime.Time
	case !e.FirstTimestamp.IsZero():
		return e.FirstTimestamp.Time
	default:
		return e.CreationTimestamp.Time
	}
}
