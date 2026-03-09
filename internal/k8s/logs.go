package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type logStreamOpener func(ctx context.Context, client kubernetes.Interface, namespace, pod, container string) (io.ReadCloser, error)

var openLogStream logStreamOpener = defaultOpenLogStream

func GetJobLogs(ctx context.Context, client kubernetes.Interface, namespace, jobName string) (string, error) {
	if jobName == "" {
		return "", fmt.Errorf("job name is required")
	}

	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return "", fmt.Errorf("list pods for job %q: %w", jobName, err)
	}
	if len(pods.Items) == 0 {
		return "", nil
	}

	sort.Slice(pods.Items, func(i, j int) bool { return pods.Items[i].Name < pods.Items[j].Name })
	var out bytes.Buffer
	for _, pod := range pods.Items {
		logs, err := collectPodLogs(ctx, client, namespace, pod)
		if err != nil {
			fmt.Fprintf(&out, "--- Logs: %s (error) ---\n%v\n", pod.Name, err)
			continue
		}
		if logs == "" {
			continue
		}
		out.WriteString(logs)
	}

	return out.String(), nil
}

func collectPodLogs(ctx context.Context, client kubernetes.Interface, namespace string, pod corev1.Pod) (string, error) {
	containers := pod.Spec.InitContainers
	containers = append(containers, pod.Spec.Containers...)
	if len(containers) == 0 {
		return "", nil
	}

	var out bytes.Buffer
	for _, c := range containers {
		stream, err := openLogStream(ctx, client, namespace, pod.Name, c.Name)
		if err != nil {
			return "", fmt.Errorf("open log stream pod=%s container=%s: %w", pod.Name, c.Name, err)
		}

		body, readErr := io.ReadAll(stream)
		_ = stream.Close()
		if readErr != nil {
			return "", fmt.Errorf("read log stream pod=%s container=%s: %w", pod.Name, c.Name, readErr)
		}
		if len(body) == 0 {
			continue
		}

		fmt.Fprintf(&out, "--- Logs: %s/%s ---\n%s\n", pod.Name, c.Name, string(body))
	}
	return out.String(), nil
}

func defaultOpenLogStream(ctx context.Context, client kubernetes.Interface, namespace, pod, container string) (io.ReadCloser, error) {
	req := client.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{
		Container: container,
	})
	return req.Stream(ctx)
}
