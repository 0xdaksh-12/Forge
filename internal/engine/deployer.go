package engine

import (
	"context"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IDeployer defines the interface for applying manifests.
type IDeployer interface {
	Apply(ctx context.Context, manifestPath, namespace, imageTag string) error
}

// Deployer applies Kubernetes manifests with image tag substitution.
type Deployer struct {
	dynClient dynamic.Interface
}


// NewDeployer creates a Deployer. It prefers in-cluster config, then falls
// back to the provided kubeconfig path (or KUBECONFIG env).
func NewDeployer(kubeconfigPath string) (*Deployer, error) {
	var cfg *rest.Config
	var err error

	// Try in-cluster first.
	cfg, err = rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig file.
		loadRules := clientcmd.NewDefaultClientConfigLoadingRules()
		if kubeconfigPath != "" {
			loadRules.ExplicitPath = kubeconfigPath
		}
		cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadRules, &clientcmd.ConfigOverrides{},
		).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("kubeconfig: %w", err)
		}
	}

	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}
	return &Deployer{dynClient: dynClient}, nil
}

// Apply reads a manifest YAML, substitutes imageTag, and applies it to the cluster.
func (d *Deployer) Apply(ctx context.Context, manifestPath, namespace, imageTag string) error {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	// Substitute image tag placeholder.
	content := strings.ReplaceAll(string(raw), "${{ git.sha }}", imageTag)
	content = strings.ReplaceAll(content, "${IMAGE_TAG}", imageTag)

	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(content), 4096)
	var obj unstructured.Unstructured
	if err := decoder.Decode(&obj); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}

	gvk := obj.GroupVersionKind()
	resource, err := gvrForKind(gvk)
	if err != nil {
		return err
	}

	ns := namespace
	if ns == "" {
		ns = "default"
	}

	ri := d.dynClient.Resource(resource).Namespace(ns)

	// Try update first; create if not found.
	existing, err := ri.Get(ctx, obj.GetName(), metav1.GetOptions{})
	if err != nil {
		_, err = ri.Create(ctx, &obj, metav1.CreateOptions{})
		return err
	}

	obj.SetResourceVersion(existing.GetResourceVersion())
	_, err = ri.Update(ctx, &obj, metav1.UpdateOptions{})
	return err
}

// gvrForKind maps common Kubernetes kinds to their GVR.
func gvrForKind(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	kind := strings.ToLower(gvk.Kind)
	switch kind {
	case "deployment":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, nil
	case "service":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}, nil
	case "configmap":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}, nil
	case "ingress":
		return schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, nil
	case "secret":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, nil
	default:
		return schema.GroupVersionResource{}, fmt.Errorf("unsupported kind %q — add it to gvrForKind()", gvk.Kind)
	}
}

// MockDeployer is used when no real Kubernetes config is available.
type MockDeployer struct{}

func (m *MockDeployer) Apply(ctx context.Context, manifestPath, namespace, imageTag string) error {
	return fmt.Errorf("kubernetes deployer is in MOCK mode (no config found) — would apply %s to %s with tag %s", manifestPath, namespace, imageTag)
}
