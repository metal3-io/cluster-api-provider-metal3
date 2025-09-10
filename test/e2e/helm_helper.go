package e2e

import (
	"context"
	"fmt"
	"log"

	"github.com/blang/semver/v4"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

type HelmOpts struct {
	Logger         *log.Logger
	Settings       *cli.EnvSettings
	ReleaseName    string
	ChartRef       string
	ChartLocation  string // Added to specify the chart location
	ReleaseVersion semver.Version
	Driver         string
}

func generateTemplateFromHelmChart(ctx context.Context, opts HelmOpts, manifestOverwriteValues map[string]interface{}, e2econfig *clusterctl.E2EConfig) (string, error) {
	actionConfig, err := initActionConfig(opts)
	if err != nil {
		return "", fmt.Errorf("failed to init action config: %w", err)
	}

	kubeversion, err := chartutil.ParseKubeVersion(e2econfig.MustGetVariable("KUBERNETES_VERSION"))
	if err != nil {
		return "", fmt.Errorf("failed to parse kube version: %w", err)
	}

	pullClient := action.NewPullWithOpts(
		action.WithConfig(actionConfig))
	pullClient.DestDir = "/tmp/"
	pullClient.Settings = opts.Settings
	pullClient.Version = opts.ReleaseVersion.String()

	_, err = pullClient.Run(opts.ChartRef)
	if err != nil {
		return "", fmt.Errorf("failed to pull chart: %w", err)
	}

	installClient := action.NewInstall(actionConfig)
	installClient.DryRun = true
	installClient.ClientOnly = true
	installClient.Replace = true
	installClient.ReleaseName = opts.ReleaseName
	installClient.Namespace = opts.Settings.Namespace()
	installClient.Version = opts.ReleaseVersion.String()
	installClient.KubeVersion = kubeversion

	chartPath, err := installClient.ChartPathOptions.LocateChart(opts.ChartLocation, opts.Settings)
	if err != nil {
		return "", err
	}

	providers := getter.All(opts.Settings)

	chart, err := loader.Load(chartPath)
	if err != nil {
		return "", err
	}

	// Check chart dependencies
	if chartDependencies := chart.Metadata.Dependencies; chartDependencies != nil {
		if err = action.CheckDependencies(chart, chartDependencies); err != nil {
			err = fmt.Errorf("failed to check chart dependencies: %w", err)
			if !installClient.DependencyUpdate {
				return "", err
			}

			manager := &downloader.Manager{
				Out:              opts.Logger.Writer(),
				ChartPath:        chartPath,
				Keyring:          installClient.ChartPathOptions.Keyring,
				SkipUpdate:       false,
				Getters:          providers,
				RepositoryConfig: opts.Settings.RepositoryConfig,
				RepositoryCache:  opts.Settings.RepositoryCache,
				Debug:            opts.Settings.Debug,
				RegistryClient:   installClient.GetRegistryClient(),
			}
			if err = manager.Update(); err != nil {
				return "", err
			}
			// Reload the chart with the updated Chart.lock file.
			if chart, err = loader.Load(chartPath); err != nil {
				return "", fmt.Errorf("failed to reload chart after repo update: %w", err)
			}
		}
	}

	release, err := installClient.RunWithContext(ctx, chart, manifestOverwriteValues)
	if err != nil {
		return "", fmt.Errorf("failed to run install: %w", err)
	}

	return release.Manifest, nil
}

func initActionConfig(opts HelmOpts) (*action.Configuration, error) {
	return initActionConfigList(opts, false)
}

func initActionConfigList(opts HelmOpts, allNamespaces bool) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)
	namespace := func() string {
		// For list action, you can pass an empty string instead of settings.Namespace() to list
		// all namespaces
		if allNamespaces {
			return ""
		}
		return opts.Settings.Namespace()
	}()

	if err := actionConfig.Init(
		opts.Settings.RESTClientGetter(),
		namespace,
		opts.Driver,
		opts.Logger.Printf); err != nil {
		return nil, err
	}

	return actionConfig, nil
}
