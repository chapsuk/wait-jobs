package k8s

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

type EventType string

const (
	EventAdded   EventType = "added"
	EventUpdated EventType = "updated"
	EventDeleted EventType = "deleted"
)

type JobStatus string

const (
	JobStatusPending  JobStatus = "Pending"
	JobStatusRunning  JobStatus = "Running"
	JobStatusComplete JobStatus = "Complete"
	JobStatusFailed   JobStatus = "Failed"
	JobStatusDeleted  JobStatus = "Deleted"
)

type JobEvent struct {
	Type      EventType
	Name      string
	Job       *batchv1.Job
	Status    JobStatus
	Timestamp time.Time
}

func WatchJobs(
	ctx context.Context,
	client kubernetes.Interface,
	namespace string,
	selector string,
	names []string,
) (<-chan JobEvent, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	events := make(chan JobEvent, 64)

	namesSet := map[string]struct{}{}
	for _, n := range names {
		if n != "" {
			namesSet[n] = struct{}{}
		}
	}

	send := func(evt JobEvent) {
		select {
		case events <- evt:
		case <-ctx.Done():
		}
	}

	isTracked := func(name string) bool {
		if len(namesSet) == 0 {
			return true
		}
		_, ok := namesSet[name]
		return ok
	}

	initial, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("initial list jobs: %w", err)
	}
	for _, item := range initial.Items {
		if !isTracked(item.Name) {
			continue
		}
		job := item.DeepCopy()
		send(JobEvent{
			Type:      EventAdded,
			Name:      job.Name,
			Job:       job,
			Status:    JobState(job),
			Timestamp: time.Now(),
		})
	}

	w, err := client.BatchV1().Jobs(namespace).Watch(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("watch jobs: %w", err)
	}

	go func() {
		defer close(events)
		defer w.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-w.ResultChan():
				if !ok {
					return
				}
				job, ok := evt.Object.(*batchv1.Job)
				if !ok || job == nil || !isTracked(job.Name) {
					continue
				}
				eventType := EventUpdated
				switch evt.Type {
				case watch.Added:
					eventType = EventAdded
				case watch.Deleted:
					eventType = EventDeleted
				}
				status := JobState(job)
				if evt.Type == watch.Deleted {
					status = JobStatusDeleted
				}
				send(JobEvent{
					Type:      eventType,
					Name:      job.Name,
					Job:       job.DeepCopy(),
					Status:    status,
					Timestamp: time.Now(),
				})
			}
		}
	}()

	return events, nil
}

func JobState(job *batchv1.Job) JobStatus {
	if job == nil {
		return JobStatusPending
	}
	for _, c := range job.Status.Conditions {
		if c.Status != "True" {
			continue
		}
		switch c.Type {
		case batchv1.JobComplete:
			return JobStatusComplete
		case batchv1.JobFailed:
			return JobStatusFailed
		}
	}
	if job.Status.Active > 0 {
		return JobStatusRunning
	}
	return JobStatusPending
}
