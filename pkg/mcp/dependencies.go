package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/opencost/opencost/core/pkg/clusters"
	"github.com/opencost/opencost/core/pkg/kubeconfig"
	"github.com/opencost/opencost/pkg/clustercache"
	"github.com/opencost/opencost/pkg/env"

	"github.com/opencost/opencost/core/pkg/log"
	"github.com/opencost/opencost/core/pkg/source"
	"github.com/opencost/opencost/core/pkg/util/retry"
	"github.com/opencost/opencost/modules/prometheus-source/pkg/prom"
	"github.com/opencost/opencost/pkg/cloud/provider"
	"github.com/opencost/opencost/pkg/cloudcost"
	"github.com/opencost/opencost/pkg/config"
	"github.com/opencost/opencost/pkg/costmodel"
	"github.com/opencost/opencost/pkg/metrics"
	"github.com/opencost/opencost/pkg/util/watcher"
)

// Initialize creates a new MCPServer with all its dependencies initialized.
func Initialize() (*MCPServer, error) {
	// Kubernetes API setup
	kubeClientset, err := kubeconfig.LoadKubeClient("")
	if err != nil {
		return nil, fmt.Errorf("failed to build Kubernetes client: %w", err)
	}

	// Create Kubernetes Cluster Cache + Watchers
	k8sCache := clustercache.NewKubernetesClusterCache(kubeClientset)
	k8sCache.Run()

	// Create ConfigFileManager for synchronization of shared configuration
	confManager := config.NewConfigFileManager(nil)

	cloudProviderKey := env.GetCloudProviderAPIKey()
	cloudProvider, err := provider.NewProvider(k8sCache, cloudProviderKey, confManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create cloud provider: %w", err)
	}

	// ClusterInfo Provider to provide the cluster map with local and remote cluster data
	var clusterInfoProvider clusters.ClusterInfoProvider
	if env.IsClusterInfoFileEnabled() {
		clusterInfoFile := confManager.ConfigFileAt(env.GetClusterInfoFilePath())
		clusterInfoProvider = costmodel.NewConfiguredClusterInfoProvider(clusterInfoFile)
	} else {
		clusterInfoProvider = costmodel.NewLocalClusterInfoProvider(kubeClientset, cloudProvider)
	}

	const maxRetries = 10
	const retryInterval = 10 * time.Second

	var fatalErr error

	ctx, cancel := context.WithCancel(context.Background())
	fn := func() (source.OpenCostDataSource, error) {
		ds, e := prom.NewDefaultPrometheusDataSource(clusterInfoProvider)
		if e != nil {
			if source.IsRetryable(e) {
				return nil, e
			}
			fatalErr = e
			cancel()
		}

		return ds, e
	}

	dataSource, _ := retry.Retry(
		ctx,
		fn,
		maxRetries,
		retryInterval,
	)

	if fatalErr != nil {
		return nil, fmt.Errorf("failed to create Prometheus data source: %w", fatalErr)
	}

	// Append the pricing config watcher
	installNamespace := env.GetOpencostNamespace()

	configWatchers := watcher.NewConfigMapWatchers(kubeClientset, installNamespace)
	configWatchers.AddWatcher(provider.ConfigWatcherFor(cloudProvider))
	configWatchers.AddWatcher(metrics.GetMetricsConfigWatcher())
	configWatchers.Watch()

	clusterMap := dataSource.ClusterMap()

	cm := costmodel.NewCostModel(dataSource, cloudProvider, k8sCache, clusterMap, dataSource.BatchDuration())

	// Initialize cloud cost integration from configuration
	var cloudCostIntegration cloudcost.CloudCostIntegration
	if env.IsCloudCostEnabled() {
		cloudCostIntegration, err = initializeCloudCostIntegration(confManager)
		if err != nil {
			log.Warnf("Failed to initialize cloud cost integration: %v", err)
			// Continue without cloud cost integration rather than failing
		}
	}

	mcpServer := NewMCPServer(cm, cloudProvider, cloudCostIntegration)

	return mcpServer, nil
}

// initializeCloudCostIntegration initializes cloud cost integration from cloud-integration.json
func initializeCloudCostIntegration(confManager *config.ConfigFileManager) (cloudcost.CloudCostIntegration, error) {
	configPath := env.GetCloudCostConfigPath()
	if configPath == "" {
		return nil, fmt.Errorf("cloud cost config path not set")
	}

	// Read configuration file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cloud cost config file %s: %w", configPath, err)
	}

	// Parse the configuration as a generic map first
	var configMap map[string]interface{}
	if err := json.Unmarshal(configData, &configMap); err != nil {
		return nil, fmt.Errorf("failed to parse cloud cost config JSON: %w", err)
	}

	// Extract provider type
	provider, ok := configMap["provider"].(string)
	if !ok {
		return nil, fmt.Errorf("cloud cost config missing 'provider' field")
	}

	log.Infof("Initializing cloud cost integration for provider: %s", provider)

	// Create appropriate integration based on provider
	switch provider {
	case "aws":
		return initializeAWSIntegration(configData)
	case "gcp":
		return initializeGCPIntegration(configData)
	case "azure":
		return initializeAzureIntegration(configData)
	case "oracle":
		return initializeOracleIntegration(configData)
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %s", provider)
	}
}

// initializeAWSIntegration creates AWS cloud cost integration
func initializeAWSIntegration(configData []byte) (cloudcost.CloudCostIntegration, error) {
	// For now, return a placeholder - you would implement actual AWS config parsing
	log.Infof("AWS cloud cost integration would be initialized here")
	return nil, fmt.Errorf("AWS integration not yet implemented")
}

// initializeGCPIntegration creates GCP cloud cost integration
func initializeGCPIntegration(configData []byte) (cloudcost.CloudCostIntegration, error) {
	// For now, return a placeholder - you would implement actual GCP config parsing
	log.Infof("GCP cloud cost integration would be initialized here")
	return nil, fmt.Errorf("GCP integration not yet implemented")
}

// initializeAzureIntegration creates Azure cloud cost integration
func initializeAzureIntegration(configData []byte) (cloudcost.CloudCostIntegration, error) {
	// For now, return a placeholder - you would implement actual Azure config parsing
	log.Infof("Azure cloud cost integration would be initialized here")
	return nil, fmt.Errorf("Azure integration not yet implemented")
}

// initializeOracleIntegration creates Oracle cloud cost integration
func initializeOracleIntegration(configData []byte) (cloudcost.CloudCostIntegration, error) {
	// For now, return a placeholder - you would implement actual Oracle config parsing
	log.Infof("Oracle cloud cost integration would be initialized here")
	return nil, fmt.Errorf("Oracle integration not yet implemented")
}
