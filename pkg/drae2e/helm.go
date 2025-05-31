package drae2e

import (
	"fmt"
	"os"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
)

func NewHelmInstaller(logger logger, p HelmParameters) *Helm {
	return &Helm{HelmParameters: p, logger: logger}
}

type HelmParameters struct {
	Namespace       string
	CreateNamespace bool
	ChartURL        string
	ReleaseName     string
	ChartVersion    string
	Values          map[string]interface{}
}

type logger interface {
	Log(args ...any)
	Logf(format string, args ...any)
}

type Helm struct {
	HelmParameters
	logger logger

	// save it for uninstal
	config *action.Configuration
}

func (h *Helm) Install() error {
	os.Setenv("HELM_EXPERIMENTAL_OCI", "1")
	settings := cli.New()

	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptWriter(os.Stdout),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	)
	if err != nil {
		return fmt.Errorf("failed to create registry client: %w", err)
	}

	h.config = &action.Configuration{}
	h.config.RegistryClient = registryClient
	if err := h.config.Init(settings.RESTClientGetter(), h.Namespace, os.Getenv("HELM_DRIVER"), h.logger.Logf); err != nil {
		return fmt.Errorf("failed to initialize Helm action configuration: %w", err)
	}

	install := action.NewInstall(h.config)
	install.ChartPathOptions.Version = h.ChartVersion
	localChartPath, err := install.ChartPathOptions.LocateChart(h.ChartURL, settings)
	if err != nil {
		return fmt.Errorf("failed to locate chart: %w", err)
	}
	h.logger.Logf("Downloaded chart to: %s", localChartPath)

	chart, err := loader.Load(localChartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}
	h.logger.Logf("loaded chart: %s", h.ReleaseName)

	history := action.NewHistory(h.config)
	history.Max = 1
	if releases, err := history.Run(h.ReleaseName); err != nil || len(releases) <= 0 {
		client := action.NewInstall(h.config)
		client.Namespace = h.Namespace
		client.ReleaseName = h.ReleaseName
		client.CreateNamespace = h.CreateNamespace
		client.Wait = true
		client.Timeout = 5 * time.Minute
		if _, err := client.Run(chart, h.Values); err != nil {
			return fmt.Errorf("failed to install chart: %w", err)
		}
		return nil
	}

	client := action.NewUpgrade(h.config)
	client.Namespace = h.Namespace
	client.Wait = true
	client.Timeout = 5 * time.Minute
	if _, err := client.Run(h.ReleaseName, chart, h.Values); err != nil {
		return fmt.Errorf("failed to upgrade chart: %w", err)
	}
	return nil
}

func (h Helm) Remove() error {
	client := action.NewUninstall(h.config)
	client.KeepHistory = false
	if _, err := client.Run(h.ReleaseName); err != nil {
		return fmt.Errorf("failed to uninstall %s: %w", h.ReleaseName, err)
	}
	return nil
}
