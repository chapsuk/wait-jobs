package runner

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/chapsuk/wait-jobs/internal/k8s"
)

type LogMode string

const (
	LogModeAll    LogMode = "all"
	LogModeFailed LogMode = "failed"
	LogModeNone   LogMode = "none"
)

type Options struct {
	Namespace string
	Selector  string
	JobNames  []string
	Timeout   time.Duration
	LogMode   LogMode
}

type Result struct {
	Failed   int
	Deleted  int
	TimedOut bool
}

type Printer interface {
	Start(total int, namespace string, timeout time.Duration)
	UpdateJob(name string, status k8s.JobStatus, age time.Duration)
	PrintLogs(jobName string, status k8s.JobStatus, logs string)
	PrintSummary(failed, deleted int)
}

type watchFn func(ctx context.Context, client kubernetes.Interface, namespace, selector string, names []string) (<-chan k8s.JobEvent, error)
type logsFn func(ctx context.Context, client kubernetes.Interface, namespace, jobName string) (string, error)

var watchJobs watchFn = k8s.WatchJobs
var fetchLogs logsFn = k8s.GetJobLogs

func Run(ctx context.Context, client kubernetes.Interface, p Printer, opts Options) (Result, error) {
	if len(opts.JobNames) == 0 && opts.Selector == "" {
		return Result{}, fmt.Errorf("either selector or job names must be provided")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Minute
	}
	if opts.LogMode == "" {
		opts.LogMode = LogModeNone
	}

	targets, err := resolveTargets(ctx, client, opts)
	if err != nil {
		return Result{}, err
	}
	if len(targets) == 0 {
		return Result{}, fmt.Errorf("no jobs to watch")
	}

	start := time.Now()
	state := map[string]k8s.JobStatus{}
	for _, name := range targets {
		state[name] = k8s.JobStatusPending
	}

	runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	p.Start(len(targets), opts.Namespace, opts.Timeout)
	for _, name := range targets {
		p.UpdateJob(name, k8s.JobStatusPending, 0)
	}

	events, err := watchJobs(runCtx, client, opts.Namespace, opts.Selector, targets)
	if err != nil {
		return Result{}, err
	}

	logged := map[string]bool{}
	for {
		if allDone(state) {
			break
		}
		select {
		case evt, ok := <-events:
			if !ok {
				if allDone(state) {
					break
				}
				return Result{}, fmt.Errorf("watch channel closed before completion")
			}
			if !slices.Contains(targets, evt.Name) {
				continue
			}
			state[evt.Name] = evt.Status
			p.UpdateJob(evt.Name, evt.Status, time.Since(start))

			if isTerminal(evt.Status) && !logged[evt.Name] && shouldPrintLogs(opts.LogMode, evt.Status) {
				logged[evt.Name] = true
				logs, logErr := fetchLogs(runCtx, client, opts.Namespace, evt.Name)
				if logErr != nil {
					logs = fmt.Sprintf("failed to fetch logs: %v", logErr)
				}
				p.PrintLogs(evt.Name, evt.Status, logs)
			}
		case <-runCtx.Done():
			if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
				res := summarize(state)
				res.TimedOut = true
				p.PrintSummary(res.Failed, res.Deleted)
				return res, context.DeadlineExceeded
			}
			return summarize(state), runCtx.Err()
		}
	}

	res := summarize(state)
	p.PrintSummary(res.Failed, res.Deleted)
	return res, nil
}

func resolveTargets(ctx context.Context, client kubernetes.Interface, opts Options) ([]string, error) {
	if len(opts.JobNames) > 0 {
		return uniqueNonEmpty(opts.JobNames), nil
	}
	list, err := client.BatchV1().Jobs(opts.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: opts.Selector,
	})
	if err != nil {
		return nil, fmt.Errorf("list jobs by selector: %w", err)
	}
	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		names = append(names, item.Name)
	}
	return uniqueNonEmpty(names), nil
}

func uniqueNonEmpty(values []string) []string {
	set := map[string]struct{}{}
	for _, v := range values {
		if v == "" {
			continue
		}
		set[v] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func allDone(state map[string]k8s.JobStatus) bool {
	for _, st := range state {
		if !isTerminal(st) {
			return false
		}
	}
	return true
}

func isTerminal(status k8s.JobStatus) bool {
	return status == k8s.JobStatusComplete || status == k8s.JobStatusFailed || status == k8s.JobStatusDeleted
}

func shouldPrintLogs(mode LogMode, status k8s.JobStatus) bool {
	switch mode {
	case LogModeAll:
		return status == k8s.JobStatusComplete || status == k8s.JobStatusFailed
	case LogModeFailed:
		return status == k8s.JobStatusFailed
	default:
		return false
	}
}

func summarize(state map[string]k8s.JobStatus) Result {
	var res Result
	for _, st := range state {
		if st == k8s.JobStatusFailed {
			res.Failed++
		}
		if st == k8s.JobStatusDeleted {
			res.Deleted++
		}
	}
	return res
}
