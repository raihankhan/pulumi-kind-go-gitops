// Package cluster encapsulates the kind cluster lifecycle. The primary goal is
// to keep the rest of the codebase unaware of whether the cluster is real,
// local, ephemeral, or pinned. Higher layers (bootstrap, pulumi program) only
// see a stable Cluster abstraction.
package cluster

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Cluster is a value object describing a single local kind cluster.
type Cluster struct {
	Name      string
	Context   string
	KubeYAML  string
	CreatedAt time.Time
}

// Options controls how a cluster is created.
type Options struct {
	Name        string
	ConfigPath  string
	WaitReady   bool
	WaitTimeout time.Duration
}

// Provider is a thin abstraction over `kind` and `kubectl` so the rest of the
// codebase can be unit-tested without shelling out.
type Provider interface {
	Create(ctx context.Context, opts Options) (Cluster, error)
	Delete(ctx context.Context, name string) error
	Exists(ctx context.Context, name string) (bool, error)
	Kubeconfig(ctx context.Context, name string) (string, error)
}

type execProvider struct {
	binDir  string
	workDir string
}

// NewProvider returns a Provider backed by the system `kind` and `kubectl`
// binaries.
func NewProvider(workDir string) Provider {
	return execProvider{workDir: workDir}
}

func (p execProvider) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if _, err := exec.LookPath(name); err != nil {
		return nil, fmt.Errorf("%s not found on PATH: %w", name, err)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = p.workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, out)
	}
	return out, nil
}

// Create brings up a kind cluster using the supplied config file.
func (p execProvider) Create(ctx context.Context, opts Options) (Cluster, error) {
	if opts.Name == "" {
		opts.Name = "pulumi-gitops"
	}
	if opts.WaitTimeout == 0 {
		opts.WaitTimeout = 5 * time.Minute
	}
	if opts.ConfigPath == "" {
		return Cluster{}, errors.New("cluster: ConfigPath is required")
	}
	abs, err := filepath.Abs(opts.ConfigPath)
	if err != nil {
		return Cluster{}, fmt.Errorf("cluster: resolve config path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return Cluster{}, fmt.Errorf("cluster: config %s not accessible: %w", abs, err)
	}

	createCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if _, err := p.run(createCtx, "kind", "create", "cluster",
		"--name", opts.Name,
		"--config", abs,
		"--retain", // keep cluster on failure for inspection
	); err != nil {
		return Cluster{}, fmt.Errorf("kind create: %w", err)
	}

	kube, err := p.Kubeconfig(ctx, opts.Name)
	if err != nil {
		return Cluster{}, fmt.Errorf("retrieve kubeconfig: %w", err)
	}

	if opts.WaitReady {
		waitCtx, cancelWait := context.WithTimeout(ctx, opts.WaitTimeout)
		defer cancelWait()
		if err := p.waitReady(waitCtx, opts.Name); err != nil {
			return Cluster{}, fmt.Errorf("wait for nodes ready: %w", err)
		}
	}

	return Cluster{
		Name:      opts.Name,
		Context:   "kind-" + opts.Name,
		KubeYAML:  kube,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (p execProvider) waitReady(ctx context.Context, name string) error {
	_, err := p.run(ctx, "kubectl",
		"--context", "kind-"+name,
		"wait", "--for=condition=Ready",
		"nodes", "--all", "--timeout=5m",
	)
	return err
}

// Delete tears down the named kind cluster. It is a no-op if the cluster does
// not exist.
func (p execProvider) Delete(ctx context.Context, name string) error {
	if name == "" {
		name = "pulumi-gitops"
	}
	if exists, _ := p.Exists(ctx, name); !exists {
		return nil
	}
	_, err := p.run(ctx, "kind", "delete", "cluster", "--name", name)
	return err
}

// Exists reports whether a cluster with the given name is registered with kind.
func (p execProvider) Exists(ctx context.Context, name string) (bool, error) {
	out, err := p.run(ctx, "kind", "get", "clusters")
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.TrimSpace(line) == name {
			return true, nil
		}
	}
	return false, nil
}

// Kubeconfig returns the kubeconfig YAML for the named cluster.
func (p execProvider) Kubeconfig(ctx context.Context, name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(home, ".kube", "config")
	data, err := os.ReadFile(candidate) //nolint:gosec // config is a standard kubeconfig path
	if err != nil {
		return "", fmt.Errorf("read kubeconfig: %w", err)
	}
	if !strings.Contains(string(data), "name: kind-"+name) {
		return "", fmt.Errorf("kubeconfig does not contain kind-%s context", name)
	}
	return string(data), nil
}
