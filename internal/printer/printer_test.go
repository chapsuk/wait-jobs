package printer

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/chapsuk/wait-jobs/internal/k8s"
)

func TestPrinter_UpdateJob_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, true)
	p.Start(2, "default", 5*time.Minute)
	p.UpdateJob("job-a", k8s.JobStatusRunning, 8*time.Second)
	p.UpdateJob("job-a", k8s.JobStatusRunning, 9*time.Second) // duplicate status should be suppressed
	p.UpdateJob("job-a", k8s.JobStatusComplete, 10*time.Second)

	out := buf.String()
	if !strings.Contains(out, `Watching 2 jobs in namespace "default"`) {
		t.Fatalf("unexpected start output:\n%s", out)
	}
	if !strings.Contains(out, "Progress updates:") {
		t.Fatalf("expected non-tty header, got:\n%s", out)
	}
	if !strings.Contains(out, "- job=job-a status=Running age=8s") {
		t.Fatalf("expected running transition line, got:\n%s", out)
	}
	if !strings.Contains(out, "- job=job-a status=Complete age=10s") {
		t.Fatalf("expected complete transition line, got:\n%s", out)
	}
	if strings.Count(out, "status=Running") > 1 {
		t.Fatalf("expected deduplicated running line, got:\n%s", out)
	}
}

func TestPrinter_PrintLogs(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, true)
	p.PrintLogs("job-a", k8s.JobStatusFailed, "line1\nline2")

	out := buf.String()
	if !strings.Contains(out, "--- Logs: job-a (Failed) ---") {
		t.Fatalf("expected logs header, got:\n%s", out)
	}
	if !strings.Contains(out, "line1") || !strings.Contains(out, "line2") {
		t.Fatalf("expected logs body, got:\n%s", out)
	}
}

// In TTY mode the live table redraws with \033[2J on every event, which used
// to erase any logs emitted between updates. PrintLogs must repaint the log
// block as part of every subsequent table render so the user keeps seeing it.
func TestPrinter_PrintLogs_TTY_StickyAcrossRedraws(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, false)
	p.isTTY = true // simulate terminal output for the buffer

	p.UpdateJob("schema-update", k8s.JobStatusFailed, 10*time.Second)
	p.PrintLogs("schema-update", k8s.JobStatusFailed, "ERROR: relation does not exist")

	// Right after PrintLogs, the log block must already have been painted.
	afterLogs := buf.String()
	if !strings.Contains(afterLogs, "ERROR: relation does not exist") {
		t.Fatalf("expected log block to be painted immediately in TTY mode, got:\n%s", afterLogs)
	}
	firstPaint := strings.Count(afterLogs, "--- Logs: schema-update (Failed) ---")
	if firstPaint != 1 {
		t.Fatalf("expected exactly one log header after PrintLogs, got %d:\n%s", firstPaint, afterLogs)
	}

	// A subsequent UpdateJob issues \033[2J — the log block must be re-emitted
	// as part of that frame so it survives the clear.
	p.UpdateJob("seed-data", k8s.JobStatusComplete, 17*time.Second)
	repainted := strings.Count(buf.String(), "--- Logs: schema-update (Failed) ---")
	if repainted < 2 {
		t.Fatalf("expected log block to be repainted after subsequent UpdateJob, count=%d:\n%s", repainted, buf.String())
	}

	p.PrintSummary(1, 0)

	out := buf.String()
	lastLogsIdx := strings.LastIndex(out, "--- Logs: schema-update")
	sumIdx := strings.Index(out, "Summary: failed=1")
	if lastLogsIdx == -1 || sumIdx == -1 || lastLogsIdx > sumIdx {
		t.Fatalf("expected logs to appear before summary, got:\n%s", out)
	}
}
