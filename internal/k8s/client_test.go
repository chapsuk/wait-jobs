package k8s

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveKubeconfigPath_ExplicitWins(t *testing.T) {
	got := resolveKubeconfigPath("/tmp/custom-kubeconfig")
	if got != "/tmp/custom-kubeconfig" {
		t.Fatalf("expected explicit path, got %q", got)
	}
}

func TestResolveKubeconfigPath_FromEnv(t *testing.T) {
	t.Setenv("KUBECONFIG", "/tmp/a:/tmp/b")
	got := resolveKubeconfigPath("")
	if got != "/tmp/a" {
		t.Fatalf("expected first path from env, got %q", got)
	}
}

func TestResolveKubeconfigPath_DefaultHome(t *testing.T) {
	t.Setenv("KUBECONFIG", "")
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := resolveKubeconfigPath("")
	want := filepath.Join(home, ".kube", "config")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestReadServiceAccountNamespace_DefaultOnMissing(t *testing.T) {
	orig := serviceAccountNamespacePath
	if _, err := os.Stat(orig); err == nil {
		t.Skip("real service account namespace file exists on this runner")
	}

	if got := readServiceAccountNamespace(); got != "default" {
		t.Fatalf("expected default namespace, got %q", got)
	}
}
