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

	out := buf.String()
	if !strings.Contains(out, `Watching 2 jobs in namespace "default"`) {
		t.Fatalf("unexpected start output:\n%s", out)
	}
	if !strings.Contains(out, "job=job-a status=Running age=8s") {
		t.Fatalf("expected job line, got:\n%s", out)
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
