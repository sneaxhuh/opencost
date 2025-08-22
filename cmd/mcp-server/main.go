package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/opencost/opencost/pkg/cmd/costmodel"
	"github.com/opencost/opencost/pkg/costmodel/dependencies"
	"github.com/rs/zerolog/log"
)

// JSONRPCRequest defines the structure of an incoming JSON-RPC request.
type JSONRPCRequest struct {
	Version string            `json:"jsonrpc"`
	Method  string            `json:"method"`
	Params  []json.RawMessage `json:"params"`
	ID      int               `json:"id"`
}

// JSONRPCResponse defines the structure of an outgoing JSON-RPC response.
type JSONRPCResponse struct {
	Version string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      int         `json:"id"`
}

// RPCError defines the structure of a JSON-RPC error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	// 1. Initialize Dependencies
	// This is a complex process that involves setting up the Kubernetes client,
	// cloud provider, and other components. We've encapsulated this logic
	// in a new function for clarity.
	log.Info().Msg("Initializing MCP server dependencies...")
	server, err := dependencies.NewServer()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize server dependencies")
	}
	log.Info().Msg("MCP server initialized successfully.")

	// 2. Start the Stdio Loop
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()

		var rpcReq JSONRPCRequest
		if err := json.Unmarshal(line, &rpcReq); err != nil {
			sendErrorResponse(rpcReq.ID, -32700, "Parse error")
			continue
		}

		if rpcReq.Method != "query" || len(rpcReq.Params) != 1 {
			sendErrorResponse(rpcReq.ID, -32601, "Method not found or invalid params")
			continue
		}

		var mcpReq costmodel.MCPRequest
		if err := json.Unmarshal(rpcReq.Params[0], &mcpReq); err != nil {
			sendErrorResponse(rpcReq.ID, -32602, "Invalid params")
			continue
		}

		// 3. Process the request
		mcpResp, err := server.ProcessMCPRequest(&mcpReq)
		if err != nil {
			sendErrorResponse(rpcReq.ID, -32000, err.Error())
			continue
		}

		// 4. Send the response
		sendSuccessResponse(rpcReq.ID, mcpResp)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal().Err(err).Msg("Error reading from stdin")
	}
}

func sendErrorResponse(id int, code int, message string) {
	resp := JSONRPCResponse{
		Version: "2.0",
		Error:   &RPCError{Code: code, Message: message},
		ID:      id,
	}
	jsonResp, _ := json.Marshal(resp)
	fmt.Println(string(jsonResp))
}

func sendSuccessResponse(id int, result interface{}) {
	resp := JSONRPCResponse{
		Version: "2.0",
		Result:  result,
		ID:      id,
	}
	jsonResp, _ := json.Marshal(resp)
	fmt.Println(string(jsonResp))
}
