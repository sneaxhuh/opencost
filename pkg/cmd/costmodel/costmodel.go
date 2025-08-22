package costmodel

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"github.com/opencost/opencost/core/pkg/log"
	"github.com/opencost/opencost/core/pkg/util/apiutil"
	"github.com/opencost/opencost/core/pkg/version"
	"github.com/opencost/opencost/pkg/cloud/models"
	"github.com/opencost/opencost/pkg/cloud/provider"
	"github.com/opencost/opencost/pkg/cloudcost"
	"github.com/opencost/opencost/pkg/costmodel"
	"github.com/opencost/opencost/pkg/customcost"
	"github.com/opencost/opencost/pkg/env"
	"github.com/opencost/opencost/pkg/filemanager"
	"github.com/opencost/opencost/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"

	"github.com/opencost/opencost/core/pkg/errors"
)

func Execute(conf *Config) error {
	log.Infof("Starting cost-model version %s", version.FriendlyVersion())
	if conf == nil {
		conf = DefaultConfig()
	}
	conf.log()

	router := httprouter.New()
	var a *costmodel.Accesses
	var cp models.Provider
	if conf.KubernetesEnabled {
		a = costmodel.Initialize(router)
		err := StartExportWorker(context.Background(), a.Model)
		if err != nil {
			log.Errorf("couldn't start CSV export worker: %v", err)
		}

		// Register OpenCost Specific Endpoints
		router.GET("/allocation", a.ComputeAllocationHandler)
		router.GET("/allocation/summary", a.ComputeAllocationHandlerSummary)
		router.GET("/assets", a.ComputeAssetsHandler)
		if conf.CarbonEstimatesEnabled {
			router.GET("/assets/carbon", a.ComputeAssetsCarbonHandler)
		}

		// set cloud provider for cloud cost
		cp = a.CloudProvider
	}

	var cloudCostIntegration cloudcost.CloudCostIntegration
	if conf.CloudCostEnabled {
		var providerConfig models.ProviderConfig
		if cp != nil {
			providerConfig = provider.ExtractConfigFromProviders(cp)
		}
		// InitializeCloudCost doesn't return an integration, so we'll create a default one
		costmodel.InitializeCloudCost(router, providerConfig)
		// For now, we'll set a nil integration
		cloudCostIntegration = nil
	}

	var customCostPipelineService *customcost.PipelineService
	if conf.CloudCostEnabled {
		customCostPipelineService = costmodel.InitializeCustomCost(router)
	}

	// this endpoint is intentionally left out of the "if env.IsCustomCostEnabled()" conditional; in the handler, it is
	// valid for CustomCostPipelineService to be nil
	router.GET("/customCost/status", customCostPipelineService.GetCustomCostStatusHandler())

	// --- MCP Server Integration ---
	if a != nil && a.Model != nil && a.CloudProvider != nil {
		log.Info("Setting up MCP Server")
		mcpGin := gin.Default()
		mcpStore := NewInMemorySessionStore()
		mcpServer := NewMCPServer(mcpStore, a.Model, a.CloudProvider, cloudCostIntegration)
		mcpServer.RegisterMCPEndpoints(mcpGin)
		router.Handler(http.MethodPost, "/mcp/v1/query", mcpGin)
	} else {
		log.Info("Skipping MCP Server setup due to missing kubernetes-enabled dependency.")
	}
	// --- End MCP Server Integration ---

	apiutil.ApplyContainerDiagnosticEndpoints(router)

	rootMux := http.NewServeMux()
	rootMux.Handle("/", router)
	rootMux.Handle("/metrics", promhttp.Handler())
	telemetryHandler := metrics.ResponseMetricMiddleware(rootMux)
	handler := cors.AllowAll().Handler(telemetryHandler)

	return http.ListenAndServe(fmt.Sprint(":", conf.Port), errors.PanicHandlerMiddleware(handler))
}

func StartExportWorker(ctx context.Context, model costmodel.AllocationModel) error {
	exportPath := env.GetExportCSVFile()
	if exportPath == "" {
		log.Infof("%s is not set, CSV export is disabled", env.ExportCSVFile)
		return nil
	}
	fm, err := filemanager.NewFileManager(exportPath)
	if err != nil {
		return fmt.Errorf("could not create file manager: %v", err)
	}
	go func() {
		log.Info("Starting CSV exporter worker...")

		// perform first update immediately
		nextRunAt := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(nextRunAt.Sub(time.Now())):
				err := costmodel.UpdateCSV(ctx, fm, model, env.GetExportCSVLabelsAll(), env.GetExportCSVLabelsList())
				if err != nil {
					log.Errorf("Error updating CSV: %s", err)
				}
				now := time.Now().UTC()
				// next launch is at 00:10 UTC tomorrow
				// extra 10 minutes is to let prometheus to collect all the data for the previous day
				nextRunAt = time.Date(now.Year(), now.Month(), now.Day(), 0, 10, 0, 0, now.Location()).AddDate(0, 0, 1)
			}
		}
	}()
	return nil
}
