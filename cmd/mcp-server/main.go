package main

import (
	"context"
	"fmt"
	"os"
	"time"

	opencost_mcp "github.com/opencost/opencost/pkg/mcp"
	"github.com/rs/zerolog/log"

	mcp_sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool argument structures
type AllocationArgs struct {
	Window    string `json:"window"`
	Aggregate string `json:"aggregate"`

	// Allocation query parameters
	Step                                  string `json:"step,omitempty"`                                      // Duration string (e.g., "1h", "30m")
	Resolution                            string `json:"resolution,omitempty"`                                // Duration string (e.g., "1h", "30m")
	Accumulate                            bool   `json:"accumulate,omitempty"`                                // Whether to accumulate over time
	ShareIdle                             bool   `json:"share_idle,omitempty"`                                // Whether to share idle costs
	IncludeIdle                           bool   `json:"include_idle,omitempty"`                              // Whether to include idle resources
	IdleByNode                            bool   `json:"idle_by_node,omitempty"`                              // Whether to calculate idle by node
	IncludeProportionalAssetResourceCosts bool   `json:"include_proportional_asset_resource_costs,omitempty"` // Whether to include proportional asset costs
	IncludeAggregatedMetadata             bool   `json:"include_aggregated_metadata,omitempty"`               // Whether to include aggregated metadata
	ShareLB                               bool   `json:"share_lb,omitempty"`                                  // Whether to share load balancer costs
}

type AssetArgs struct {
	Window string `json:"window"`
}

// generateSessionID creates a unique session ID based on timestamp and process ID
func generateSessionID() string {
	return fmt.Sprintf("session-%d-%d", time.Now().UnixNano(), os.Getpid())
}

func main() {
	log.Logger = log.Output(os.Stderr)

	log.Info().Msg("Initializing OpenCost server dependencies...")
	opencost_server, err := opencost_mcp.Initialize() // Initialize the OpenCost MCP server
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize OpenCost server dependencies")
	}
	log.Info().Msg("OpenCost server initialized successfully.")

	// Define handlers as closures to capture the opencost_server instance
	handleAllocationCosts := func(ctx context.Context, req *mcp_sdk.CallToolRequest, args AllocationArgs) (*mcp_sdk.CallToolResult, interface{}, error) {
		// Parse duration strings to time.Duration
		var step time.Duration
		var err error

		if args.Step != "" {
			step, err = time.ParseDuration(args.Step)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid step duration '%s': %w", args.Step, err)
			}
		}

		queryRequest := &opencost_mcp.OpenCostQueryRequest{
			QueryType: opencost_mcp.AllocationQueryType,
			Window:    args.Window,
			AllocationParams: &opencost_mcp.AllocationQuery{
				Step:                                  step,
				Accumulate:                            args.Accumulate,
				ShareIdle:                             args.ShareIdle,
				Aggregate:                             args.Aggregate,
				IncludeIdle:                           args.IncludeIdle,
				IdleByNode:                            args.IdleByNode,
				IncludeProportionalAssetResourceCosts: args.IncludeProportionalAssetResourceCosts,
				IncludeAggregatedMetadata:             args.IncludeAggregatedMetadata,
				ShareLB:                               args.ShareLB,
			},
		}

		mcpReq := &opencost_mcp.MCPRequest{
			SessionID: generateSessionID(),
			Query:     queryRequest,
		}

		mcpResp, err := opencost_server.ProcessMCPRequest(mcpReq)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process allocation request: %w", err)
		}

		return nil, mcpResp, nil
	}

	handleAssetCosts := func(ctx context.Context, req *mcp_sdk.CallToolRequest, args AssetArgs) (*mcp_sdk.CallToolResult, interface{}, error) {
		queryRequest := &opencost_mcp.OpenCostQueryRequest{
			QueryType: opencost_mcp.AssetQueryType,
			Window:    args.Window,

			AssetParams: &opencost_mcp.AssetQuery{},
		}

		mcpReq := &opencost_mcp.MCPRequest{
			SessionID: generateSessionID(),
			Query:     queryRequest,
		}

		mcpResp, err := opencost_server.ProcessMCPRequest(mcpReq)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process asset request: %w", err)
		}

		return nil, mcpResp, nil
	}

	// Initialize the SDK server
	sdkServer := mcp_sdk.NewServer(&mcp_sdk.Implementation{
		Name:    "opencost-mcp-server",
		Version: "v1.0.0",
	}, nil)

	// Register tools with the new typed handlers
	mcp_sdk.AddTool(sdkServer, &mcp_sdk.Tool{
		Name:        "get_allocation_costs",
		Description: "Retrieves allocation cost data.",
	}, handleAllocationCosts)

	mcp_sdk.AddTool(sdkServer, &mcp_sdk.Tool{
		Name:        "get_asset_costs",
		Description: "Retrieves asset cost data.",
	}, handleAssetCosts)

	log.Info().Msg("MCP SDK server initialized. About to run...")

	// Run the SDK server over stdin/stdout
	if err := sdkServer.Run(context.Background(), &mcp_sdk.StdioTransport{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to run MCP SDK server")
	}
	log.Info().Msg("MCP SDK server finished running.")
}
