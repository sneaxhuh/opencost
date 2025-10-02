[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/6219/badge)](https://www.bestpractices.dev/projects/6219)
[![Gurubase](https://img.shields.io/badge/Gurubase-Ask%20OpenCost%20Guru-006BFF)](https://gurubase.io/g/opencost)

![](./opencost-header.png)

# OpenCost — your favorite open source cost monitoring tool for Kubernetes and cloud spend

OpenCost give teams visibility into current and historical Kubernetes and cloud spend and resource allocation.
These models provide cost transparency in Kubernetes environments that support multiple applications, teams, departments, etc.
It also provides visibility into the cloud costs across multiple providers.

OpenCost was originally developed and open sourced by [Kubecost](https://kubecost.com). This project combines a [specification](/spec/) as well as a Golang implementation of these detailed requirements. The web UI is available in the [opencost/opencost-ui](http://github.com/opencost/opencost-ui) repository.

[![OpenCost UI Walkthrough](./ui/src/thumbnail.png)](https://youtu.be/lCP4Ci9Kcdg)
*OpenCost UI Walkthrough*

To see the full functionality of OpenCost you can view [OpenCost features](https://opencost.io). Here is a summary of features enabled:

- Real-time cost allocation by Kubernetes cluster, node, namespace, controller kind, controller, service, or pod
- Multi-cloud cost monitoring for all cloud services on AWS, Azure, GCP
- Dynamic on-demand k8s asset pricing enabled by integrations with AWS, Azure, and GCP billing APIs
- Supports on-prem k8s clusters with custom CSV pricing
- Allocation for in-cluster K8s resources like CPU, GPU, memory, and persistent volumes
- Easily export pricing data to Prometheus with /metrics endpoint ([learn more](https://www.opencost.io/docs/installation/prometheus))
- Carbon costs for cloud resources
- Support for external costs like Datadog through [OpenCost Plugins](https://github.com/opencost/opencost-plugins)
- Free and open source distribution ([Apache2 license](LICENSE))

## Getting Started

You can deploy OpenCost on any Kubernetes 1.20+ cluster in a matter of minutes, if not seconds!

Visit the full documentation for [recommended installation options](https://www.opencost.io/docs/installation/install).

> **Note for sharded Prometheus users:**
> If you run Prometheus in a sharded (HA) setup, set `PROMETHEUS_SERVER_ENDPOINT` to a global query endpoint (e.g., Thanos Query, Cortex, or Mimir). Pointing to a single Prometheus pod may result in incomplete or intermittent export results. See the [Prometheus integration docs](https://www.opencost.io/docs/installation/prometheus) for details.

## Usage

- [Cost APIs](https://www.opencost.io/docs/integrations/api)
- [CLI / kubectl cost](https://www.opencost.io/docs/integrations/kubectl-cost)
- [Prometheus Metrics](https://www.opencost.io/docs/integrations/prometheus)
- [User Interface](https://www.opencost.io/docs/installation/ui)

## MCP Server

The OpenCost MCP (Model Context Protocol) server provides AI agents with access to cost allocation and asset data through a standardized interface.

### Features

- **Allocation Queries**: Retrieve cost allocation data with filtering and aggregation
- **Asset Queries**: Access detailed asset information including nodes, disks, load balancers, and more
- **Type-Specific Fields**: Full support for asset-specific parameters (GPU details, storage classes, etc.)
- **Dynamic Session Management**: Unique session and query ID generation
- **Request Validation**: Built-in validation using go-playground/validator

### Prerequisites

- Prometheus server running and accessible
- OpenCost configured to connect to your Prometheus instance
- MCP client that supports stdio transport (e.g., Cursor, Claude Desktop)

### Building the MCP Server

```bash
# Build the MCP server binary
go build -o mcp-server cmd/mcp-server/main.go
```

### Configuration

Add the following configuration to your MCP client (e.g., Cursor's `mcp.json`):

```json
{
  "mcpServers": {
    "opencost": {
      "type": "stdio",
      "command": "/path/to/opencost/mcp-server",
      "env": {
        "PROMETHEUS_SERVER_ENDPOINT": "https://your-prometheus-endpoint"
      },
      "args": []
    }
  }
}
```

### Available Tools

- **`get_allocation_costs`**: Retrieve allocation cost data with parameters:
  - `window`: Time window (e.g., "7d", "1h", "30m")
  - `aggregate`: Aggregation properties (e.g., "namespace", "pod", "node")
  - `step`: Resolution step size
  - `accumulate`: Whether to accumulate over time
  - `share_idle`: Whether to share idle costs
  - `include_idle`: Whether to include idle resources
  - `idle_by_node`: Whether to calculate idle by node
  - `include_proportional_asset_resource_costs`: Include proportional asset costs
  - `include_aggregated_metadata`: Include aggregated metadata
  - `share_lb`: Whether to share load balancer costs

- **`get_asset_costs`**: Retrieve asset cost data with parameters:
  - `window`: Time window (e.g., "7d", "1h", "30m")

### Supported Asset Types

- **Node**: Compute instances with CPU, RAM, GPU details
- **Disk**: Storage volumes with usage and cost breakdown
- **LoadBalancer**: Load balancer instances with IP and private status
- **Network**: Network-related costs and usage
- **Cloud**: Cloud service costs with credit information
- **ClusterManagement**: Kubernetes cluster management costs

## Contributing

We :heart: pull requests! See [`CONTRIBUTING.md`](CONTRIBUTING.md) for information on building the project from source and contributing changes.

## Community

If you need any support or have any questions on contributing to the project, you can reach us on [CNCF Slack](https://slack.cncf.io/) in the [#opencost](https://cloud-native.slack.com/archives/C03D56FPD4G) channel or attend the biweekly [OpenCost Working Group community meeting](https://bit.ly/opencost-meeting) from the [Community Calendar](https://bit.ly/opencost-calendar) to discuss OpenCost development.

## FAQ

You can view [OpenCost documentation](https://www.opencost.io/docs/FAQ) for a list of commonly asked questions.
