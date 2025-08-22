package costmodel

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/opencost/opencost/core/pkg/opencost"
	models "github.com/opencost/opencost/pkg/cloud/models"
	"github.com/opencost/opencost/pkg/cloudcost"
	"github.com/opencost/opencost/pkg/costmodel"
)

// QueryType defines the type of query to be executed.
type QueryType string

const (
	AllocationQueryType QueryType = "allocation"
	AssetQueryType      QueryType = "asset"
	CloudCostQueryType  QueryType = "cloudcost"
)

// ExplanationLevel defines the level of detail for the response.
type ExplanationLevel string

const (
	SimpleExplanation        ExplanationLevel = "simple"
	DetailedExplanation      ExplanationLevel = "detailed"
	ComprehensiveExplanation ExplanationLevel = "comprehensive"
)

// AssetCategory defines the category of an asset.
type AssetCategory string

const (
	ComputeAssetCategory AssetCategory = "compute"
	StorageAssetCategory AssetCategory = "storage"
	NetworkAssetCategory AssetCategory = "network"
)

// MCPRequest represents a single turn in a conversation with the OpenCost MCP server.
type MCPRequest struct {
	SessionID string                `json:"sessionId"`
	Context   *ConversationContext  `json:"context"`
	Query     *OpenCostQueryRequest `json:"query"`
}

// MCPResponse is the response from the OpenCost MCP server for a single turn.
type MCPResponse struct {
	Data      interface{}          `json:"data"`
	QueryInfo QueryMetadata        `json:"queryInfo"`
	Summary   *DataSummary         `json:"summary,omitempty"`
	Insights  []*ActionableInsight `json:"insights,omitempty"`
}

// QueryMetadata contains metadata about the query execution.
type QueryMetadata struct {
	QueryID        string        `json:"queryId"`
	Timestamp      time.Time     `json:"timestamp"`
	ProcessingTime time.Duration `json:"processingTime"`
}

// DataSummary provides a summary of the data.
type DataSummary struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// ActionableInsight combines an observation with concrete, executable next steps.
type ActionableInsight struct {
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	Severity         string            `json:"severity"`
	SuggestedActions []SuggestedAction `json:"suggestedActions"`
}

// SuggestedAction provides a human-readable description of an action and the
// exact, machine-executable query to perform that action.
type SuggestedAction struct {
	Text       string                `json:"text"`
	Query      *OpenCostQueryRequest `json:"query"`
	ActionType string                `json:"actionType"`
}

// ConversationContext manages the state of an AI agent conversation.
type ConversationContext struct {
	StartTime     time.Time              `json:"startTime"`
	LastActivity  time.Time              `json:"lastActivity"`
	ActiveFilters map[string]interface{} `json:"activeFilters,omitempty"`
	LastWindow    string                 `json:"lastWindow,omitempty"`
	QueryHistory  []*QueryContext        `json:"queryHistory,omitempty"`
	UserIntent    string                 `json:"userIntent,omitempty"`
	Version       int                    `json:"version"`
}

// MCPContextRequest is a request to fetch the current session context.
type MCPContextRequest struct {
	SessionID string `json:"sessionId"`
}

// MCPContextResponse is the response containing a snapshot of the context.
type MCPContextResponse struct {
	SessionID string               `json:"sessionId"`
	Context   *ConversationContext `json:"context"`
}

// QueryContext stores the context of a single query in the history.
type QueryContext struct {
	Query     string    `json:"query"`
	Timestamp time.Time `json:"timestamp"`
}

// OpenCostQueryRequest provides a unified interface for all OpenCost query types.
type OpenCostQueryRequest struct {
	QueryType QueryType `json:"queryType" validate:"required,oneof=allocation asset cloudcost"`

	Window string         `json:"window" validate:"required"`
	Filter *UnifiedFilter `json:"filter,omitempty"`

	AllocationParams *AllocationQuery `json:"allocationParams,omitempty"`
	AssetParams      *AssetQuery      `json:"assetParams,omitempty"`
	CloudCostParams  *CloudCostQuery  `json:"cloudCostParams,omitempty"`

	ExplanationLevel ExplanationLevel      `json:"explanationLevel,omitempty"`
	ComparisonWith   *OpenCostQueryRequest `json:"comparisonWith,omitempty"`
}

// UnifiedFilter provides consistent filtering across all OpenCost data types.
type UnifiedFilter struct {
	Clusters   []string          `json:"clusters,omitempty"`
	Namespaces []string          `json:"namespaces,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`

	AssetTypes []string `json:"assetTypes,omitempty"`
	Providers  []string `json:"providers,omitempty"`
	Regions    []string `json:"regions,omitempty"`

	Accounts   []string `json:"accounts,omitempty"`
	Services   []string `json:"services,omitempty"`
	Categories []string `json:"categories,omitempty"`

	NaturalLanguage string `json:"naturalLanguage,omitempty"`
	ExcludePattern  string `json:"excludePattern,omitempty"`
	Field           string `json:"field"`
	Operator        string `json:"operator"`
	Value           string `json:"value"`
}

// AllocationQuery contains the parameters for an allocation query.
type AllocationQuery struct {
	Step                                  time.Duration `json:"step,omitempty"`
	Resolution                            time.Duration `json:"resolution,omitempty"`
	Accumulate                            bool          `json:"accumulate,omitempty"`
	ShareIdle                             bool          `json:"shareIdle,omitempty"`
	Aggregate                             string        `json:"aggregate,omitempty"`
	AccumulateBy                          []string      `json:"accumulateBy,omitempty"`
	IncludeIdle                           bool          `json:"includeIdle,omitempty"`
	IdleByNode                            bool          `json:"idleByNode,omitempty"`
	IncludeProportionalAssetResourceCosts bool          `json:"includeProportionalAssetResourceCosts,omitempty"`
	IncludeAggregatedMetadata             bool          `json:"includeAggregatedMetadata,omitempty"`
	Sharelb                               bool          `json:"sharelb,omitempty"`
}

// AssetQuery contains the parameters for an asset query.
type AssetQuery struct {
}

// CloudCostQuery contains the parameters for a cloud cost query.
type CloudCostQuery struct {
	Aggregate  string `json:"aggregate,omitempty"`  // Comma-separated list of aggregation properties
	Accumulate string `json:"accumulate,omitempty"` // e.g., "week"
}

// AllocationResponse represents the allocation data returned to the AI agent.
type AllocationResponse struct {
	// The allocation data, as a map of allocation sets.
	Allocations map[string]*AllocationSet `json:"allocations"`
}

// AllocationSet represents a set of allocation data.
type AllocationSet struct {
	// The name of the allocation set.
	Name        string            `json:"name"`
	Properties  map[string]string `json:"properties"`
	Allocations []*Allocation     `json:"allocations"`
}

// TotalCost calculates the total cost of all allocations in the set.
func (as *AllocationSet) TotalCost() float64 {
	var total float64
	for _, alloc := range as.Allocations {
		total += alloc.TotalCost
	}
	return total
}

// Allocation represents a single allocation data point.

type Allocation struct {
	Name string `json:"name"` // Allocation key (namespace, cluster, etc.)

	CPUCost      float64 `json:"cpuCost"`      // Cost of CPU usage
	GPUCost      float64 `json:"gpuCost"`      // Cost of GPU usage
	RAMCost      float64 `json:"ramCost"`      // Cost of memory usage
	PVCost       float64 `json:"pvCost"`       // Cost of persistent volumes
	NetworkCost  float64 `json:"networkCost"`  // Cost of network usage
	SharedCost   float64 `json:"sharedCost"`   // Shared/unallocated costs assigned here
	ExternalCost float64 `json:"externalCost"` // External costs (cloud services, etc.)
	TotalCost    float64 `json:"totalCost"`    // Sum of all costs above

	CPUCoreHours float64 `json:"cpuCoreHours"` // Usage metrics: CPU core-hours
	RAMByteHours float64 `json:"ramByteHours"` // Usage metrics: RAM byte-hours
	GPUHours     float64 `json:"gpuHours"`     // Usage metrics: GPU-hours
	PVByteHours  float64 `json:"pvByteHours"`  // Usage metrics: PV byte-hours

	Start time.Time `json:"start"` // Start timestamp for this allocation
	End   time.Time `json:"end"`   // End timestamp for this allocation
}

// AssetResponse represents the asset data returned to the AI agent.
type AssetResponse struct {
	// The asset data, as a map of asset sets.
	Assets map[string]*AssetSet `json:"assets"`
}

// AssetSet represents a set of asset data.
type AssetSet struct {
	// The name of the asset set.
	Name string `json:"name"`

	// The properties of the asset set.
	Properties map[string]string `json:"properties"`

	// The asset data for the set.
	Assets []*Asset `json:"assets"`
}

// Asset represents a single asset data point.
type Asset struct {
	Type       string            `json:"type"`
	Properties AssetProperties   `json:"properties"`
	Labels     map[string]string `json:"labels,omitempty"`

	Start time.Time `json:"start"`
	End   time.Time `json:"end"`

	Minutes       float64  `json:"minutes"`
	ByteHours     float64  `json:"byteHours"`
	Bytes         float64  `json:"bytes"`
	ByteHoursUsed *float64 `json:"byteHoursUsed,omitempty"` // nullable in JSON
	ByteUsageMax  *float64 `json:"byteUsageMax,omitempty"`  // nullable in JSON

	Breakdown AssetBreakdown `json:"breakdown"`

	Adjustment     float64 `json:"adjustment"`
	TotalCost      float64 `json:"totalCost"`
	StorageClass   string  `json:"storageClass"`
	VolumeName     string  `json:"volumeName"`
	ClaimName      string  `json:"claimName"`
	Local          float64 `json:"local"`
	ClaimNamespace string  `json:"claimNamespace"`
}
type AssetProperties struct {
	Category   string `json:"category"`
	Provider   string `json:"provider"`
	Service    string `json:"service"`
	Cluster    string `json:"cluster"`
	Name       string `json:"name"`
	ProviderID string `json:"providerID"`
}

type AssetBreakdown struct {
	Idle   float64 `json:"idle"`
	Other  float64 `json:"other"`
	System float64 `json:"system"`
	User   float64 `json:"user"`
}

// CloudCostResponse represents the cloud cost data returned to the AI agent.
type CloudCostResponse struct {
	// The cloud cost data, as a map of cloud cost sets.
	CloudCosts map[string]*CloudCostSet `json:"cloudCosts"`
}

// CloudCostSet represents a set of cloud cost data.
type CloudCostSet struct {
	// The name of the cloud cost set.
	Name string `json:"name"`

	// The properties of the cloud cost set.
	Properties map[string]string `json:"properties"`

	// The cloud cost data for the set.
	CloudCosts []*CloudCost `json:"cloudCosts"`
}

// CloudCostProperties defines the properties of a cloud cost item.
type CloudCostProperties struct {
	Provider string `json:"provider"`
	Account  string `json:"account"`
	Service  string `json:"service"`
	Category string `json:"category"`
	Region   string `json:"region"`
}

// CloudCost represents a single cloud cost data point.
type CloudCost struct {
	Start             time.Time           `json:"start"`
	End               time.Time           `json:"end"`
	Properties        CloudCostProperties `json:"properties"`
	NetCost           float64             `json:"netCost"`
	AmortizedNetCost  float64             `json:"amortizedNetCost"`
	InvoicedCost      float64             `json:"invoicedCost"`
	KubernetesPercent float64             `json:"kubernetesPercent,omitempty"`
}

// SessionStore defines an interface for managing conversation contexts.
type SessionStore interface {
	GetContext(sessionID string) *ConversationContext
	SaveContext(sessionID string, context *ConversationContext)
}

// InMemorySessionStore is a thread-safe, in-memory implementation of SessionStore.
type InMemorySessionStore struct {
	sync.RWMutex
	sessions map[string]*ConversationContext
}

// NewInMemorySessionStore creates a new in-memory session store.
func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions: make(map[string]*ConversationContext),
	}
}

// GetContext retrieves a conversation context for a given session ID.
// If no context exists, it creates and returns a new one.
func (s *InMemorySessionStore) GetContext(sessionID string) *ConversationContext {
	s.RLock()
	context, exists := s.sessions[sessionID]
	s.RUnlock()

	if !exists {
		// Create a new context if one doesn't exist for the session ID
		context = &ConversationContext{
			StartTime:    time.Now(),
			LastActivity: time.Now(),
			Version:      1,
		}
		s.Lock()
		s.sessions[sessionID] = context
		s.Unlock()
	}

	return context
}

// SaveContext saves a conversation context for a given session ID.
func (s *InMemorySessionStore) SaveContext(sessionID string, context *ConversationContext) {
	s.Lock()
	defer s.Unlock()

	context.LastActivity = time.Now()
	context.Version++
	s.sessions[sessionID] = context
}

// MCPServer holds the dependencies for the MCP API server.
type MCPServer struct {
	store       SessionStore
	costModel   *costmodel.CostModel
	provider    models.Provider
	integration cloudcost.CloudCostIntegration
}

// NewMCPServer creates a new MCP Server.
func NewMCPServer(store SessionStore, costModel *costmodel.CostModel, provider models.Provider, integration cloudcost.CloudCostIntegration) *MCPServer {
	return &MCPServer{
		store:       store,
		costModel:   costModel,
		provider:    provider,
		integration: integration,
	}
}

// ProcessMCPRequest processes an MCP request and returns an MCP response.
// generateAllocationInsights analyzes allocation data to generate actionable insights.
// summarizeAllocationData creates a human-readable summary of the allocation data.
func (s *MCPServer) summarizeAllocationData(request *OpenCostQueryRequest, resp *AllocationResponse) *DataSummary {
	// Basic summary implementation
	var totalCost float64
	for _, allocationSet := range resp.Allocations {
		totalCost += allocationSet.TotalCost()
	}

	return &DataSummary{
		Title:   fmt.Sprintf("Total Cost Over '%s'", request.Window),
		Content: fmt.Sprintf("The total cost for the selected window is $%.2f.", totalCost),
	}
}

func (s *MCPServer) generateAllocationInsights(request *OpenCostQueryRequest, resp *AllocationResponse) []*ActionableInsight {
	var insights []*ActionableInsight

	// Insight 1: Find the namespace with the highest total cost when aggregating by namespace.
	if request.AllocationParams != nil && request.AllocationParams.Aggregate == "namespace" {
		var maxCost float64
		var highestCostNamespace string

		for name, allocationSet := range resp.Allocations {
			if allocationSet.TotalCost() > maxCost {
				maxCost = allocationSet.TotalCost()
				highestCostNamespace = name
			}
		}

		if highestCostNamespace != "" {
			insight := &ActionableInsight{
				Title:       "Highest Cost Namespace",
				Description: fmt.Sprintf("The namespace '%s' incurred the highest cost of $%.2f over the queried window.", highestCostNamespace, maxCost),
				Severity:    "High",
				SuggestedActions: []SuggestedAction{
					{
						Text: fmt.Sprintf("Drill down into pod costs for the '%s' namespace.", highestCostNamespace),
						Query: &OpenCostQueryRequest{
							QueryType: AllocationQueryType,
							Window:    request.Window,
							AllocationParams: &AllocationQuery{
								Aggregate: "pod",
							},
							Filter: &UnifiedFilter{
								Field:    "namespace",
								Operator: "=",
								Value:    highestCostNamespace,
							},
						},
						ActionType: "query",
					},
				},
			}
			insights = append(insights, insight)
		}
	}

	return insights
}

func (s *MCPServer) ProcessMCPRequest(request *MCPRequest) (*MCPResponse, error) {
	// 1. Validate Request
	if err := validate.Struct(request); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// 2. Context Handling
	// If the request includes a context, use it. Otherwise, fetch from the store.
	context := request.Context
	if context == nil {
		context = s.store.GetContext(request.SessionID)
	}

	// 3. Query Dispatching
	var data interface{}
	var err error
	var insights []*ActionableInsight
	var summary *DataSummary

	queryStart := time.Now()

	switch request.Query.QueryType {
	case AllocationQueryType:
		allocationResponse, err := s.QueryAllocations(request.Query)
		if err != nil {
			return nil, err // Propagate error
		}
		data = allocationResponse

		// Generate insights and summary for the allocation data
		insights = s.generateAllocationInsights(request.Query, allocationResponse)
		summary = s.summarizeAllocationData(request.Query, allocationResponse)
	case AssetQueryType:
		data, err = s.QueryAssets(request.Query)
	case CloudCostQueryType:
		data, err = s.QueryCloudCosts(request.Query)
	default:
		return nil, fmt.Errorf("unsupported query type: %s", request.Query.QueryType)
	}

	if err != nil {
		// Handle error appropriately, maybe return a JSON-RPC error response
		return nil, err
	}

	processingTime := time.Since(queryStart)

	// 5. Construct Final Response
	mcpResponse := &MCPResponse{
		Data: data,
		QueryInfo: QueryMetadata{
			QueryID:        "some-random-id", // TODO: Generate a real query ID
			Timestamp:      time.Now(),
			ProcessingTime: processingTime,
		},
		Summary:  summary,
		Insights: insights,
	}

	// 5. Save Context
	// Add the current query to the history
	context.QueryHistory = append(context.QueryHistory, &QueryContext{
		Query:     "User query for " + string(request.Query.QueryType), // Placeholder
		Timestamp: time.Now(),
	})
	s.store.SaveContext(request.SessionID, context)

	return mcpResponse, nil
}

// RegisterMCPEndpoints registers the MCP API endpoints with the provided Gin router.
func (s *MCPServer) RegisterMCPEndpoints(router *gin.Engine) {
	router.POST("/mcp/v1/query", s.handleQuery)
}

// validate is the singleton validator instance.
var validate = validator.New()

// handleQuery is the core handler for all MCP requests.
func (s *MCPServer) handleQuery(c *gin.Context) {
	// 1. Decode Request
	var request MCPRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// 2. Process Request
	response, err := s.ProcessMCPRequest(&request)
	if err != nil {
		// Handle different types of errors if needed, for now, a generic internal server error
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 3. Send Response
	c.JSON(http.StatusOK, response)
}

// QueryAllocations translates an MCP query into a CostModel allocation query and transforms the result.
func (s *MCPServer) QueryAllocations(query *OpenCostQueryRequest) (*AllocationResponse, error) {
	// 1. Parse Window
	window, err := opencost.ParseWindowWithOffset(query.Window, 0) // 0 offset for UTC
	// Use the window variable to avoid "declared and not used" error
	_ = window
	if err != nil {
		return nil, fmt.Errorf("failed to parse window '%s': %w", query.Window, err)
	}

	// 2. Translate Filter
	// This is a simplified translation. A real implementation would be more comprehensive.
	// For now, we'll pass an empty filter
	// var filters allocation.AllocationFilter

	// 3. Set Query Options
	// We'll call ComputeAllocation directly with start and end times
	start := *window.Start()
	end := *window.End()

	// 4. Call CostModel
	allocationSet, err := s.costModel.ComputeAllocation(start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to compute allocations: %w", err)
	}

	// 5. Transform Response
	return transformAllocationSet(allocationSet), nil
}

// transformAllocationSet converts an opencost.AllocationSet into the MCP's AllocationResponse format.
func transformAllocationSet(allocSet *opencost.AllocationSet) *AllocationResponse {
	if allocSet == nil {
		return &AllocationResponse{Allocations: make(map[string]*AllocationSet)}
	}

	mcpAllocations := make(map[string]*AllocationSet)

	// Create a single set for all allocations
	mcpSet := &AllocationSet{
		Name:        "allocations",
		Properties:  make(map[string]string),
		Allocations: []*Allocation{},
	}

	// Convert each allocation
	for _, alloc := range allocSet.Allocations {
		if alloc == nil {
			continue
		}

		mcpAlloc := &Allocation{
			Name:         alloc.Name,
			CPUCost:      alloc.CPUCost,
			GPUCost:      alloc.GPUCost,
			RAMCost:      alloc.RAMCost,
			PVCost:       alloc.PVCost(), // Call the method
			NetworkCost:  alloc.NetworkCost,
			SharedCost:   alloc.SharedCost,
			ExternalCost: alloc.ExternalCost,
			TotalCost:    alloc.TotalCost(),
			CPUCoreHours: alloc.CPUCoreHours,
			RAMByteHours: alloc.RAMByteHours,
			GPUHours:     alloc.GPUHours,
			PVByteHours:  alloc.PVBytes(), // Use the method directly
			Start:        alloc.Start,
			End:          alloc.End,
		}
		mcpSet.Allocations = append(mcpSet.Allocations, mcpAlloc)
	}

	mcpAllocations["allocations"] = mcpSet

	return &AllocationResponse{
		Allocations: mcpAllocations,
	}
}

// QueryAssets translates an MCP query into a Provider asset query and transforms the result.
func (s *MCPServer) QueryAssets(query *OpenCostQueryRequest) (*AssetResponse, error) {
	// 1. Parse Window
	window, err := opencost.ParseWindowWithOffset(query.Window, 0) // 0 offset for UTC
	// Use the window variable to avoid "declared and not used" error
	_ = window
	if err != nil {
		return nil, fmt.Errorf("failed to parse window '%s': %w", query.Window, err)
	}

	// 2. Translate Filter (Future Work)
	// For now, we will pass nil for options.
	// TODO: A more robust filter translation is needed here.
	var options interface{}
	// Use the options variable to avoid "declared and not used" error
	_ = options

	// 3. Call Provider
	// For now, we'll return an empty asset set since we don't have a proper implementation
	assetSet := &opencost.AssetSet{
		Assets: make(map[string]opencost.Asset),
	}

	// 4. Transform Response
	return transformAssetSet(assetSet), nil
}

// transformAssetSet converts a models.AssetSet into the MCP's AssetResponse format.
func transformAssetSet(assetSet *opencost.AssetSet) *AssetResponse {
	if assetSet == nil {
		return &AssetResponse{Assets: make(map[string]*AssetSet)}
	}

	mcpAssets := make(map[string]*AssetSet)

	// Create a single set for all assets
	mcpSet := &AssetSet{
		Name:       "assets",
		Properties: make(map[string]string),
		Assets:     []*Asset{},
	}

	// Since we're returning an empty asset set, we'll keep the assets array empty
	mcpAssets["assets"] = mcpSet

	return &AssetResponse{
		Assets: mcpAssets,
	}
}

// QueryCloudCosts translates an MCP query into a CloudCost integration query and transforms the result.
func (s *MCPServer) QueryCloudCosts(query *OpenCostQueryRequest) (*CloudCostResponse, error) {
	// 1. Parse Window
	window, err := opencost.ParseWindowWithOffset(query.Window, 0) // 0 offset for UTC
	// Use the window variable to avoid "declared and not used" error
	_ = window
	if err != nil {
		return nil, fmt.Errorf("failed to parse window '%s': %w", query.Window, err)
	}

	// 2. Set Query Options
	start := *window.Start()
	end := *window.End()
	// Use the start and end variables to avoid "declared and not used" error
	_ = start
	_ = end
	// TODO: Use the aggregate and filter parameters

	// 3. Call Integration
	// For now, we'll return an empty cloud cost set since we don't have a proper implementation
	cloudCostSet := &opencost.CloudCostSet{
		CloudCosts: make(map[string]*opencost.CloudCost),
	}

	// 4. Transform Response
	return transformCloudCostSet(cloudCostSet), nil
}

// transformCloudCostSet converts a opencost.CloudCostSet into the MCP's CloudCostResponse format.
func transformCloudCostSet(ccSet *opencost.CloudCostSet) *CloudCostResponse {
	if ccSet == nil || ccSet.CloudCosts == nil {
		return &CloudCostResponse{CloudCosts: make(map[string]*CloudCostSet)}
	}

	// The MCP response is aggregated by a key. The source data is a flat list.
	// A real implementation would need to respect the aggregation parameters.
	// For now, we will group all results under a single key.
	mcpCloudCosts := make(map[string]*CloudCostSet)
	mcpSet := &CloudCostSet{
		Name:       "unaggregated",
		Properties: make(map[string]string),
		CloudCosts: []*CloudCost{},
	}

	for _, item := range ccSet.CloudCosts {
		if item == nil {
			continue
		}

		mcpCC := &CloudCost{
			Start: *item.Window.Start(),
			End:   *item.Window.End(),
			Properties: CloudCostProperties{
				Provider: item.Properties.Provider,
				Account:  item.Properties.AccountID,
				Service:  item.Properties.Service,
				Category: item.Properties.Category,
				Region:   item.Properties.RegionID,
			},
			NetCost:           item.NetCost.Cost,
			AmortizedNetCost:  item.AmortizedNetCost.Cost,
			InvoicedCost:      item.InvoicedCost.Cost,
			KubernetesPercent: item.NetCost.KubernetesPercent,
		}
		mcpSet.CloudCosts = append(mcpSet.CloudCosts, mcpCC)
	}
	mcpCloudCosts["unaggregated"] = mcpSet

	return &CloudCostResponse{
		CloudCosts: mcpCloudCosts,
	}
}
