package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/chapsuk/wait-jobs/internal/k8s"
)

type fakePrinter struct {
	logsCount int
}

func (p *fakePrinter) Start(_ int, _ string, _ time.Duration)               {}
func (p *fakePrinter) UpdateJob(_ string, _ k8s.JobStatus, _ time.Duration) {}
func (p *fakePrinter) PrintLogs(_ string, _ k8s.JobStatus, _ string) {
	p.logsCount++
}
func (p *fakePrinter) PrintSummary(_ int, _ int) {}

func TestRun_AllPass(t *testing.T) {
	origWatch := watchJobs
	origLogs := fetchLogs
	defer func() {
		watchJobs = origWatch
		fetchLogs = origLogs
	}()

	ch := make(chan k8s.JobEvent, 4)
	watchJobs = func(_ context.Context, _ kubernetes.Interface, _, _ string, _ []string) (<-chan k8s.JobEvent, error) {
		return ch, nil
	}
	fetchLogs = func(_ context.Context, _ kubernetes.Interface, _, _ string) (string, error) { return "ok", nil }

	go func() {
		ch <- k8s.JobEvent{Name: "job-a", Status: k8s.JobStatusComplete}
		ch <- k8s.JobEvent{Name: "job-b", Status: k8s.JobStatusComplete}
		close(ch)
	}()

	p := &fakePrinter{}
	res, err := Run(context.Background(), fake.NewSimpleClientset(), p, Options{
		Namespace: "default",
		JobNames:  []string{"job-a", "job-b"},
		Timeout:   2 * time.Second,
		LogMode:   LogModeAll,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Failed != 0 || res.Deleted != 0 || res.TimedOut {
		t.Fatalf("unexpected result: %+v", res)
	}
	if p.logsCount != 2 {
		t.Fatalf("expected logs for both jobs, got %d", p.logsCount)
	}
}

func TestRun_WithFailure(t *testing.T) {
	origWatch := watchJobs
	defer func() { watchJobs = origWatch }()

	ch := make(chan k8s.JobEvent, 4)
	watchJobs = func(_ context.Context, _ kubernetes.Interface, _, _ string, _ []string) (<-chan k8s.JobEvent, error) {
		return ch, nil
	}

	go func() {
		ch <- k8s.JobEvent{Name: "job-a", Status: k8s.JobStatusFailed}
		close(ch)
	}()

	res, err := Run(context.Background(), fake.NewSimpleClientset(), &fakePrinter{}, Options{
		Namespace: "default",
		JobNames:  []string{"job-a"},
		Timeout:   2 * time.Second,
		LogMode:   LogModeFailed,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Failed != 1 {
		t.Fatalf("expected one failed job, got %+v", res)
	}
}

func TestRun_Timeout(t *testing.T) {
	origWatch := watchJobs
	defer func() { watchJobs = origWatch }()

	ch := make(chan k8s.JobEvent)
	watchJobs = func(_ context.Context, _ kubernetes.Interface, _, _ string, _ []string) (<-chan k8s.JobEvent, error) {
		return ch, nil
	}

	_, err := Run(context.Background(), fake.NewSimpleClientset(), &fakePrinter{}, Options{
		Namespace: "default",
		JobNames:  []string{"job-a"},
		Timeout:   50 * time.Millisecond,
		LogMode:   LogModeNone,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}
