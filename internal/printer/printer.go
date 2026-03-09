package printer

import (
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/chapsuk/wait-jobs/internal/k8s"
	"github.com/fatih/color"
	"golang.org/x/term"
)

type JobView struct {
	Name   string
	Status k8s.JobStatus
	Age    time.Duration
}

type Printer struct {
	out    io.Writer
	isTTY  bool
	noANSI bool

	mu sync.Mutex

	jobs            map[string]JobView
	lastSnapshot    time.Time
	snapshotEvery   time.Duration
	nonTTYPrinted   map[string]k8s.JobStatus
	nonTTYHasHeader bool
}

func New(out io.Writer, noANSI bool) *Printer {
	if out == nil {
		out = os.Stdout
	}
	return &Printer{
		out:           out,
		isTTY:         isTTYWriter(out),
		noANSI:        noANSI,
		jobs:          make(map[string]JobView),
		snapshotEvery: 30 * time.Second,
		nonTTYPrinted: make(map[string]k8s.JobStatus),
	}
}

func (p *Printer) Start(total int, namespace string, timeout time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.out, "Watching %d jobs in namespace %q (timeout: %s)\n", total, namespace, timeout)
}

func (p *Printer) UpdateJob(name string, status k8s.JobStatus, age time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	prev, hadPrev := p.jobs[name]
	p.jobs[name] = JobView{Name: name, Status: status, Age: age}

	if !p.isTTY || p.noANSI {
		p.renderNonTTYLocked(name, status, age, hadPrev, prev.Status)
		return
	}
	p.renderTTYLocked()
}

func (p *Printer) PrintLogs(jobName string, status k8s.JobStatus, logs string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if logs == "" {
		return
	}
	fmt.Fprintf(p.out, "\n--- Logs: %s (%s) ---\n%s\n", jobName, status, logs)
}

func (p *Printer) PrintSummary(failed, deleted int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintln(p.out)
	fmt.Fprintf(p.out, "Summary: failed=%d deleted=%d total=%d\n", failed, deleted, len(p.jobs))
}

func (p *Printer) renderNonTTYLocked(name string, status k8s.JobStatus, age time.Duration, hadPrev bool, prevStatus k8s.JobStatus) {
	now := time.Now()
	if !p.nonTTYHasHeader {
		fmt.Fprintln(p.out, "Progress updates:")
		p.nonTTYHasHeader = true
	}

	// Print line only on meaningful transition to avoid CI log flood.
	if !hadPrev || prevStatus != status {
		fmt.Fprintf(p.out, "- job=%s status=%s age=%s\n", name, status, age.Truncate(time.Second))
		p.nonTTYPrinted[name] = status
	}

	if p.lastSnapshot.IsZero() || now.Sub(p.lastSnapshot) >= p.snapshotEvery {
		p.lastSnapshot = now
		p.printSnapshotLocked()
	}
}

func (p *Printer) printSnapshotLocked() {
	var pending, running, complete, failed, deleted int
	for _, job := range p.jobs {
		switch job.Status {
		case k8s.JobStatusPending:
			pending++
		case k8s.JobStatusRunning:
			running++
		case k8s.JobStatusComplete:
			complete++
		case k8s.JobStatusFailed:
			failed++
		case k8s.JobStatusDeleted:
			deleted++
		}
	}
	fmt.Fprintf(
		p.out,
		"  snapshot: pending=%d running=%d complete=%d failed=%d deleted=%d total=%d\n",
		pending, running, complete, failed, deleted, len(p.jobs),
	)
}

func (p *Printer) renderTTYLocked() {
	if !p.isTTY || p.noANSI {
		return
	}

	fmt.Fprint(p.out, "\033[H\033[2J")
	fmt.Fprintln(p.out, "  JOB                 STATUS      AGE")
	for _, job := range sortJobs(p.jobs) {
		fmt.Fprintf(p.out, "  %-18s  %-10s  %s\n", job.Name, p.colorStatus(job.Status), job.Age.Truncate(time.Second))
	}
}

func (p *Printer) colorStatus(status k8s.JobStatus) string {
	if p.noANSI {
		return string(status)
	}
	switch status {
	case k8s.JobStatusComplete:
		return color.New(color.FgGreen).Sprint(status)
	case k8s.JobStatusFailed:
		return color.New(color.FgRed).Sprint(status)
	case k8s.JobStatusRunning:
		return color.New(color.FgYellow).Sprint(status)
	case k8s.JobStatusDeleted:
		return color.New(color.FgHiBlack).Sprint(status)
	default:
		return string(status)
	}
}

func sortJobs(m map[string]JobView) []JobView {
	res := make([]JobView, 0, len(m))
	for _, v := range m {
		res = append(res, v)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Name < res[j].Name })
	return res
}

func isTTYWriter(out io.Writer) bool {
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
