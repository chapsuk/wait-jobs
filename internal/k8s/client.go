package k8s

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const serviceAccountNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// ClientInit contains details about chosen Kubernetes auth/config source.
type ClientInit struct {
	Clientset      *kubernetes.Clientset
	Namespace      string
	Source         string
	KubeconfigPath string
}

// NewClientset keeps compatibility with the planned signature.
func NewClientset(kubeconfig, context string) (*kubernetes.Clientset, string, error) {
	init, err := InitClientset(kubeconfig, context)
	if err != nil {
		return nil, "", err
	}
	return init.Clientset, init.Namespace, nil
}

// InitClientset resolves cluster config in this order:
// 1) explicit kubeconfig flag,
// 2) in-cluster service account,
// 3) KUBECONFIG / ~/.kube/config fallback.
func InitClientset(kubeconfig, context string) (*ClientInit, error) {
	if strings.TrimSpace(kubeconfig) != "" {
		cs, ns, path, err := fromKubeconfig(kubeconfig, context)
		if err != nil {
			return nil, err
		}
		return &ClientInit{
			Clientset:      cs,
			Namespace:      ns,
			Source:         "kubeconfig",
			KubeconfigPath: path,
		}, nil
	}

	inCfg, inErr := rest.InClusterConfig()
	if inErr == nil {
		cs, err := kubernetes.NewForConfig(inCfg)
		if err != nil {
			return nil, fmt.Errorf("create in-cluster clientset: %w", err)
		}
		return &ClientInit{
			Clientset: cs,
			Namespace: readServiceAccountNamespace(),
			Source:    "in-cluster",
		}, nil
	}

	cs, ns, path, err := fromKubeconfig("", context)
	if err != nil {
		return nil, fmt.Errorf("in-cluster config unavailable (%v), kubeconfig fallback failed: %w", inErr, err)
	}
	return &ClientInit{
		Clientset:      cs,
		Namespace:      ns,
		Source:         "kubeconfig",
		KubeconfigPath: path,
	}, nil
}

func fromKubeconfig(explicitPath, context string) (*kubernetes.Clientset, string, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if explicitPath != "" {
		loadingRules.ExplicitPath = explicitPath
	}

	overrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		overrides.CurrentContext = context
	}

	clientCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	cfg, err := clientCfg.ClientConfig()
	if err != nil {
		return nil, "", "", fmt.Errorf("load kubeconfig: %w", err)
	}

	ns, _, err := clientCfg.Namespace()
	if err != nil {
		ns = "default"
	}

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, "", "", fmt.Errorf("create kubeconfig clientset: %w", err)
	}

	return cs, ns, resolveKubeconfigPath(explicitPath), nil
}

func resolveKubeconfigPath(explicitPath string) string {
	if explicitPath != "" {
		return explicitPath
	}
	env := os.Getenv("KUBECONFIG")
	if env != "" {
		parts := filepath.SplitList(env)
		if len(parts) > 0 && parts[0] != "" {
			return parts[0]
		}
	}
	home := homedir.HomeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}

func readServiceAccountNamespace() string {
	data, err := os.ReadFile(serviceAccountNamespacePath)
	if err != nil {
		return "default"
	}
	ns := strings.TrimSpace(string(data))
	if ns == "" {
		return "default"
	}
	return ns
}

// IsInClusterError helps tests/assertions around environment resolution.
func IsInClusterError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, rest.ErrNotInCluster)
}
