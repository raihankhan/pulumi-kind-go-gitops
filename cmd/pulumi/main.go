// Package main is the Pulumi program entry point for the local kind platform.
//
// What this program deploys (in order):
//
//  1. Pulumi Kubernetes Operator (Helm).
//  2. ingress-nginx (Helm) — required by the nginx sample workload.
//  3. Argo CD core (Helm) + AppProject + App-of-Apps root Application.
//
// Why this order:
//
//   - The Pulumi Operator does not need to be running before Argo CD; both
//     ship together in the same Pulumi up.
//   - Argo CD needs the AppProject applied before the root Application so
//     that the project exists when Argo CD validates the spec.
//   - The root Application depends on the Argo CD namespace existing.
//
// Usage:
//
//	pulumi up --yes
//	pulumi destroy --yes
//
// Required CLI tooling: kind, kubectl, helm, pulumi.
package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	helmv4 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v4"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	yamlv2 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/yaml"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Pinned chart versions. Bump via PRs; never let Helm drift away from Git.
const (
	ingressNginxChart   = "ingress-nginx"
	ingressNginxVersion = "4.11.3"

	argocdChart   = "argo-cd"
	argocdVersion = "7.6.12"

	pulumiOperatorRepo      = "https://pulumi.github.io/pulumi-kubernetes-operator"
	pulumiOperatorChart     = "pulumi-kubernetes-operator"
	pulumiOperatorVersion   = "2.5.0"
	pulumiOperatorNamespace = "pulumi-kubernetes-operator"
)

// Inputs resolved once from pulumi.Config so deep functions stay terse.
type deployCtx struct {
	ClusterName  string
	GitHubRepo   string
	GitHubBranch string
	IngressClass string
	ArgoCDDomain string
}

func main() {
	pulumi.Run(func(pCtx *pulumi.Context) error {
		return deploy(pCtx)
	})
}

func deploy(pCtx *pulumi.Context) error {
	c, err := loadConfig(pCtx)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	provider, err := kubernetes.NewProvider(pCtx, "kind", &kubernetes.ProviderArgs{
		Context: pulumi.String(c.ClusterName),
	})
	if err != nil {
		return fmt.Errorf("kubernetes provider: %w", err)
	}

	// 1. Pulumi Kubernetes Operator
	operatorChart, err := installPulumiOperator(pCtx, provider)
	if err != nil {
		return err
	}

	// 2. ingress-nginx
	if _, err := installIngressNginx(pCtx, provider, c); err != nil {
		return err
	}

	// 3. Argo CD
	argoNS, argoChart, err := installArgoCD(pCtx, provider, c)
	if err != nil {
		return err
	}

	// 4. AppProject + App-of-Apps root Application (depends on Argo CD + NS)
	if err := installAppProject(pCtx, provider, argoNS, argoChart); err != nil {
		return err
	}
	if err := installRootApplication(pCtx, provider, c, argoNS, argoChart); err != nil {
		return err
	}

	// 5. Stack outputs
	pCtx.Export("clusterName", pulumi.String(c.ClusterName))
	pCtx.Export("githubRepo", pulumi.String(c.GitHubRepo))
	pCtx.Export("ingressClass", pulumi.String(c.IngressClass))
	pCtx.Export("argocdNamespace", argoNS.Metadata.Name())
	pCtx.Export("pulumiOperatorNamespace", pulumi.String(pulumiOperatorNamespace))
	_ = operatorChart // reserved for future Stack CR wiring
	return nil
}

func loadConfig(pCtx *pulumi.Context) (deployCtx, error) {
	cfg := config.New(pCtx, "")
	clusterName := cfg.Get("clusterName")
	if clusterName == "" {
		clusterName = "kind-pulumi-gitops"
	}
	repo := cfg.Get("githubRepo")
	if repo == "" {
		repo = "https://github.com/raihankhan/pulumi-kind-go-gitops-apps.git"
	}
	branch := cfg.Get("githubBranch")
	if branch == "" {
		branch = "master"
	}
	class := cfg.Get("ingressClass")
	if class == "" {
		class = "nginx"
	}
	domain := cfg.Get("argocdDomain")
	if domain == "" {
		domain = "argocd.local"
	}
	return deployCtx{
		ClusterName:  clusterName,
		GitHubRepo:   repo,
		GitHubBranch: branch,
		IngressClass: class,
		ArgoCDDomain: domain,
	}, nil
}

func installPulumiOperator(pCtx *pulumi.Context, provider *kubernetes.Provider) (*helmv4.Chart, error) {
	rel, err := helmv4.NewChart(pCtx, "pulumi-operator", &helmv4.ChartArgs{
		Chart:     pulumi.String(pulumiOperatorChart),
		Namespace: pulumi.String(pulumiOperatorNamespace),
		Values: pulumi.Map{
			"operator": pulumi.Map{
				"installCRDs":    pulumi.Bool(true),
				"leaderElection": pulumi.Bool(true),
			},
			"image": pulumi.Map{
				"repository": pulumi.String("pulumi/pulumi-kubernetes-operator"),
				"tag":        pulumi.String("v" + pulumiOperatorVersion),
				"pullPolicy": pulumi.String("IfNotPresent"),
			},
			"resources": pulumi.Map{
				"requests": pulumi.Map{
					"cpu":    pulumi.String("50m"),
					"memory": pulumi.String("64Mi"),
				},
				"limits": pulumi.Map{
					"cpu":    pulumi.String("500m"),
					"memory": pulumi.String("256Mi"),
				},
			},
			"existingSecret": pulumi.String("pulumi-kubernetes-operator"),
		},
		RepositoryOpts: helmv4.RepositoryOptsArgs{
			Repo: pulumi.String(pulumiOperatorRepo),
		},
		SkipAwait: pulumi.Bool(false),
	}, pulumi.Provider(provider))
	if err != nil {
		return nil, fmt.Errorf("install pulumi operator: %w", err)
	}
	return rel, nil
}

func installIngressNginx(pCtx *pulumi.Context, provider *kubernetes.Provider, c deployCtx) (*helmv4.Chart, error) {
	rel, err := helmv4.NewChart(pCtx, "ingress-nginx", &helmv4.ChartArgs{
		Chart:     pulumi.String(ingressNginxChart),
		Namespace: pulumi.String("ingress-nginx"),
		Values: pulumi.Map{
			"controller": pulumi.Map{
				"ingressClassResource": pulumi.Map{
					"name":            pulumi.String(c.IngressClass),
					"enabled":         pulumi.Bool(true),
					"default":         pulumi.Bool(true),
					"controllerValue": pulumi.String("k8s.io/" + c.IngressClass),
				},
				"ingressClass": pulumi.String(c.IngressClass),
				"replicaCount": pulumi.Int(1),
				"service": pulumi.Map{
					"type": pulumi.String("NodePort"),
					"nodePorts": pulumi.Map{
						"http":  pulumi.Int(30080),
						"https": pulumi.Int(30443),
					},
				},
				"resources": pulumi.Map{
					"requests": pulumi.Map{
						"cpu":    pulumi.String("50m"),
						"memory": pulumi.String("64Mi"),
					},
					"limits": pulumi.Map{
						"cpu":    pulumi.String("500m"),
						"memory": pulumi.String("256Mi"),
					},
				},
				"metrics": pulumi.Map{
					"enabled": pulumi.Bool(true),
				},
			},
		},
		RepositoryOpts: helmv4.RepositoryOptsArgs{
			Repo: pulumi.String("https://kubernetes.github.io/ingress-nginx"),
		},
		SkipAwait: pulumi.Bool(false),
	}, pulumi.Provider(provider))
	if err != nil {
		return nil, fmt.Errorf("install ingress-nginx: %w", err)
	}
	return rel, nil
}

func installArgoCD(pCtx *pulumi.Context, provider *kubernetes.Provider, c deployCtx) (*corev1.Namespace, *helmv4.Chart, error) {
	ns, err := corev1.NewNamespace(pCtx, "argocd", &corev1.NamespaceArgs{
		Metadata: metav1.ObjectMetaArgs{
			Name: pulumi.String("argocd"),
			Labels: pulumi.StringMap{
				"app.kubernetes.io/part-of": pulumi.String("platform"),
			},
		},
	}, pulumi.Provider(provider))
	if err != nil {
		return nil, nil, fmt.Errorf("create argocd namespace: %w", err)
	}
	chart, err := helmv4.NewChart(pCtx, "argocd", &helmv4.ChartArgs{
		Chart:     pulumi.String(argocdChart),
		Namespace: ns.Metadata.Name(),
		Values: pulumi.Map{
			"global": pulumi.Map{
				"domain": pulumi.String(c.ArgoCDDomain),
			},
			"server": pulumi.Map{
				"service": pulumi.Map{
					"type":          pulumi.String("NodePort"),
					"nodePortHttps": pulumi.Int(30443),
					"nodePortHttp":  pulumi.Int(30080),
				},
				"extraArgs": pulumi.Array{
					pulumi.String("--insecure"),
				},
			},
			"controller": pulumi.Map{
				"metrics": pulumi.Map{
					"enabled": pulumi.Bool(true),
				},
			},
			"repoServer": pulumi.Map{
				"metrics": pulumi.Map{
					"enabled": pulumi.Bool(true),
				},
			},
		},
		RepositoryOpts: helmv4.RepositoryOptsArgs{
			Repo: pulumi.String("https://argoproj.github.io/argo-helm"),
		},
		SkipAwait: pulumi.Bool(false),
	}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{ns}))
	if err != nil {
		return nil, nil, fmt.Errorf("install argocd: %w", err)
	}
	return ns, chart, nil
}

func installAppProject(pCtx *pulumi.Context, provider *kubernetes.Provider, ns *corev1.Namespace, argo *helmv4.Chart) error {
	project, err := os.ReadFile("manifests/argocd/10-appproject.yaml")
	if err != nil {
		return fmt.Errorf("read appproject: %w", err)
	}
	if _, err := yamlv2.NewConfigGroup(pCtx, "platform-appproject", &yamlv2.ConfigGroupArgs{
		YAML: []string{string(project)},
	}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{ns, argo})); err != nil {
		return fmt.Errorf("apply appproject: %w", err)
	}
	return nil
}

func installRootApplication(pCtx *pulumi.Context, provider *kubernetes.Provider, c deployCtx, ns *corev1.Namespace, argo *helmv4.Chart) error {
	if _, err := yamlv2.NewConfigGroup(pCtx, "platform-root-application", &yamlv2.ConfigGroupArgs{
		YAML: []string{renderRootApplication(c)},
	}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{ns, argo})); err != nil {
		return fmt.Errorf("apply root application: %w", err)
	}
	return nil
}

func renderRootApplication(c deployCtx) string {
	return fmt.Sprintf(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: platform-root
  namespace: argocd
  labels:
    app.kubernetes.io/part-of: platform
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: platform
  source:
    repoURL: %s
    targetRevision: %s
    path: apps
    directory:
      recurse: true
  destination:
    server: https://kubernetes.default.svc
    namespace: argocd
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
      allowEmpty: false
    syncOptions:
      - CreateNamespace=true
      - PruneLast=true
      - ApplyOutOfSyncOnly=true
    retry:
      limit: 5
      backoff:
        duration: 5s
        factor: 2
        maxDuration: 3m
  revisionHistoryLimit: 10
`, c.GitHubRepo, c.GitHubBranch)
}
