package k8s

import (
	"context"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestWatchJobs_TracksCompleteAndFailed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := fake.NewSimpleClientset()
	events, err := WatchJobs(ctx, client, "default", "", []string{"job-ok", "job-fail"})
	if err != nil {
		t.Fatalf("WatchJobs() error = %v", err)
	}

	jobOK := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "job-ok", Namespace: "default"}}
	jobFail := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "job-fail", Namespace: "default"}}
	if _, err := client.BatchV1().Jobs("default").Create(ctx, jobOK, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create job-ok: %v", err)
	}
	if _, err := client.BatchV1().Jobs("default").Create(ctx, jobFail, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create job-fail: %v", err)
	}

	jobOK.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: "True"}}
	if _, err := client.BatchV1().Jobs("default").UpdateStatus(ctx, jobOK, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update job-ok status: %v", err)
	}

	jobFail.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: "True"}}
	if _, err := client.BatchV1().Jobs("default").UpdateStatus(ctx, jobFail, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update job-fail status: %v", err)
	}

	waitForStatus(t, events, "job-ok", JobStatusComplete)
	waitForStatus(t, events, "job-fail", JobStatusFailed)
}

func TestWatchJobs_TracksDeleted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "job-delete", Namespace: "default"}}
	client := fake.NewSimpleClientset(job)

	events, err := WatchJobs(ctx, client, "default", "", []string{"job-delete"})
	if err != nil {
		t.Fatalf("WatchJobs() error = %v", err)
	}

	if err := client.BatchV1().Jobs("default").Delete(ctx, "job-delete", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("delete job: %v", err)
	}

	waitForStatus(t, events, "job-delete", JobStatusDeleted)
}

func waitForStatus(t *testing.T, ch <-chan JobEvent, name string, status JobStatus) {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		select {
		case evt := <-ch:
			if evt.Name == name && evt.Status == status {
				return
			}
		case <-timeout:
			t.Fatalf("timeout waiting for %s status %s", name, status)
		}
	}
}
