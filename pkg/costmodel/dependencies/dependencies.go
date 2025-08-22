package dependencies

import (
	"context"
	"fmt"
	"time"

	"github.com/opencost/opencost/pkg/clustercache"
	"github.com/opencost/opencost/core/pkg/clusters"
	"github.com/opencost/opencost/pkg/env"
	"github.com/opencost/opencost/core/pkg/kubeconfig"
	
	"github.com/opencost/opencost/core/pkg/source"
	"github.com/opencost/opencost/core/pkg/util/retry"
	mcp "github.com/opencost/opencost/pkg/cmd/costmodel"
	"github.com/opencost/opencost/pkg/cloud/provider"
	"github.com/opencost/opencost/pkg/config"
	"github.com/opencost/opencost/pkg/costmodel"
	"github.com/opencost/opencost/pkg/metrics"
	"github.com/opencost/opencost/pkg/util/watcher"
	"github.com/opencost/opencost/modules/prometheus-source/pkg/prom"
)

// NewServer creates a new MCPServer with all its dependencies initialized.
func NewServer() (*mcp.MCPServer, error) {
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

	store := mcp.NewInMemorySessionStore()

	// We are passing a nil for the cloud cost integration for now.
	mcpServer := mcp.NewMCPServer(store, cm, cloudProvider, nil)

	return mcpServer, nil
}
