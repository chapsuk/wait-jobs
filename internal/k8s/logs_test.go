package k8s

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetJobLogs_CollectsAllContainers(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "job-a-pod-1",
				Namespace: "default",
				Labels: map[string]string{
					"job-name": "job-a",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "main"}},
			},
		},
	)

	orig := openLogStream
	defer func() { openLogStream = orig }()
	openLogStream = func(_ context.Context, _ kubernetes.Interface, _, pod, container string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(pod + ":" + container + " log body")), nil
	}

	got, err := GetJobLogs(context.Background(), client, "default", "job-a")
	if err != nil {
		t.Fatalf("GetJobLogs() error = %v", err)
	}

	if !strings.Contains(got, "--- Logs: job-a-pod-1/main ---") {
		t.Fatalf("expected pod/container header in logs, got:\n%s", got)
	}
	if !strings.Contains(got, "job-a-pod-1:main log body") {
		t.Fatalf("expected log body in output, got:\n%s", got)
	}
}

func TestGetJobLogs_HandlesStreamError(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "job-b-pod-1",
				Namespace: "default",
				Labels: map[string]string{
					"job-name": "job-b",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "main"}},
			},
		},
	)

	orig := openLogStream
	defer func() { openLogStream = orig }()
	openLogStream = func(_ context.Context, _ kubernetes.Interface, _, _, _ string) (io.ReadCloser, error) {
		return nil, errors.New("boom")
	}

	got, err := GetJobLogs(context.Background(), client, "default", "job-b")
	if err != nil {
		t.Fatalf("GetJobLogs() unexpected hard error = %v", err)
	}
	if !strings.Contains(got, "(error)") {
		t.Fatalf("expected error marker in output, got:\n%s", got)
	}
}
