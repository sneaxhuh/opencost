package oracle

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/opencost/opencost/core/pkg/clustercache"
	"github.com/opencost/opencost/pkg/cloud/models"
	"github.com/opencost/opencost/pkg/config"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestGetKey(t *testing.T) {
	var testCases = map[string]struct {
		isVirtual bool
		gpus      int
	}{
		"virtual-node": {
			true,
			0,
		},
		"gpu": {
			false,
			3,
		},
		"node": {
			false,
			0,
		},
	}
	for instanceType, testCase := range testCases {
		t.Run(instanceType, func(t *testing.T) {
			labels := map[string]string{
				v1.LabelInstanceTypeStable: instanceType,
			}
			if testCase.isVirtual {
				labels[virtualNodeLabel] = ""
			}
			key := (&Oracle{}).GetKey(labels, testNode(testCase.gpus))
			assert.NotEmpty(t, key.ID())
			features := strings.Split(key.Features(), ",")
			assert.Len(t, features, 3)
			assert.Equal(t, instanceType, features[0])
			assert.Equal(t, strconv.FormatBool(testCase.isVirtual), features[1])
			assert.Equal(t, testCase.gpus, key.GPUCount())
			if testCase.gpus > 0 {
				assert.Equal(t, "nvidia.com/gpu", key.GPUType())
			} else {
				assert.Equal(t, "", key.GPUType())
			}
		})
	}
}

func TestGetPVKey(t *testing.T) {
	storageClass := "xyz"
	providerID := "ocid.abc"
	pv := &clustercache.PersistentVolume{
		Spec: v1.PersistentVolumeSpec{
			StorageClassName: storageClass,
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: &v1.CSIPersistentVolumeSource{
					VolumeHandle: providerID,
					Driver:       driverOCIBV,
				},
			},
		},
	}
	pvkey := (&Oracle{}).GetPVKey(pv, map[string]string{}, "")
	assert.Equal(t, blockVolumePartNumber, pvkey.Features())
	assert.Equal(t, storageClass, pvkey.GetStorageClass())
	assert.Equal(t, providerID, pvkey.ID())
}

func TestRegions(t *testing.T) {
	regions := (&Oracle{}).Regions()
	assert.Len(t, regions, 39)
}

func testNode(gpus int) *clustercache.Node {
	capacity := map[v1.ResourceName]resource.Quantity{}
	if gpus > 0 {
		capacity["nvidia.com/gpu"] = resource.MustParse(fmt.Sprintf("%d", gpus))
	}
	return &clustercache.Node{
		SpecProviderID: "ocid.abc",
		Status: v1.NodeStatus{
			Capacity: capacity,
		},
	}
}

// mockClusterCache is a mock implementation of clustercache.ClusterCache for testing
type mockClusterCache struct {
	nodes []*clustercache.Node
}

func (m *mockClusterCache) Run()                                                      {}
func (m *mockClusterCache) Stop()                                                     {}
func (m *mockClusterCache) GetAllNamespaces() []*clustercache.Namespace               { return nil }
func (m *mockClusterCache) GetAllNodes() []*clustercache.Node                         { return m.nodes }
func (m *mockClusterCache) GetAllPods() []*clustercache.Pod                           { return nil }
func (m *mockClusterCache) GetAllServices() []*clustercache.Service                   { return nil }
func (m *mockClusterCache) GetAllDaemonSets() []*clustercache.DaemonSet               { return nil }
func (m *mockClusterCache) GetAllDeployments() []*clustercache.Deployment             { return nil }
func (m *mockClusterCache) GetAllStatefulSets() []*clustercache.StatefulSet           { return nil }
func (m *mockClusterCache) GetAllReplicaSets() []*clustercache.ReplicaSet             { return nil }
func (m *mockClusterCache) GetAllPersistentVolumes() []*clustercache.PersistentVolume { return nil }
func (m *mockClusterCache) GetAllPersistentVolumeClaims() []*clustercache.PersistentVolumeClaim {
	return nil
}
func (m *mockClusterCache) GetAllStorageClasses() []*clustercache.StorageClass { return nil }
func (m *mockClusterCache) GetAllJobs() []*clustercache.Job                    { return nil }
func (m *mockClusterCache) GetAllPodDisruptionBudgets() []*clustercache.PodDisruptionBudget {
	return nil
}
func (m *mockClusterCache) GetAllReplicationControllers() []*clustercache.ReplicationController {
	return nil
}

// mockProviderConfig is a mock implementation of models.ProviderConfig for testing
type mockProviderConfig struct {
	customPricing *models.CustomPricing
	shouldError   bool
}

func (m *mockProviderConfig) ConfigFileManager() *config.ConfigFileManager { return nil }
func (m *mockProviderConfig) GetCustomPricingData() (*models.CustomPricing, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock error")
	}
	return m.customPricing, nil
}
func (m *mockProviderConfig) Update(func(*models.CustomPricing) error) (*models.CustomPricing, error) {
	return nil, nil
}
func (m *mockProviderConfig) UpdateFromMap(map[string]string) (*models.CustomPricing, error) {
	return nil, nil
}

// mockRateCardStore is a mock implementation of RateCardStore for testing
type mockRateCardStore struct {
	enhancedClusterPrice float64
}

func (m *mockRateCardStore) ForManagedCluster(clusterType string) float64 {
	if clusterType == "BASIC_CLUSTER" {
		return 0.0
	}
	return m.enhancedClusterPrice
}

func (m *mockRateCardStore) ForKey(key models.Key, defaultPricing DefaultPricing) (*models.Node, models.PricingMetadata, error) {
	return nil, models.PricingMetadata{}, nil
}

func (m *mockRateCardStore) ForPVK(pvk models.PVKey, defaultPricing DefaultPricing) (*models.PV, error) {
	return nil, nil
}

func (m *mockRateCardStore) ForEgressRegion(region string, defaultPricing DefaultPricing) (*models.Network, error) {
	return nil, nil
}

func (m *mockRateCardStore) ForLB(defaultPricing DefaultPricing) (*models.LoadBalancer, error) {
	return nil, nil
}

func (m *mockRateCardStore) Store() map[string]Price {
	return nil
}

func (m *mockRateCardStore) Refresh() (map[string]Price, error) {
	return nil, nil
}

// Create a wrapper to convert mockRateCardStore to *RateCardStore for testing
func createTestOracleProvider(nodes []*clustercache.Node, enhancedClusterPrice float64) *Oracle {
	// We need to create a real RateCardStore and set its prices manually
	rateCardStore := &RateCardStore{
		prices: map[string]Price{
			enhancedClusterPartNumber: {
				UnitPrice: enhancedClusterPrice,
			},
		},
	}

	return &Oracle{
		Clientset:     &mockClusterCache{nodes: nodes},
		RateCardStore: rateCardStore,
	}
}

func TestClusterManagementPricing(t *testing.T) {
	testCases := map[string]struct {
		nodes                []*clustercache.Node
		enhancedClusterPrice float64
		expectedPlatform     string
		expectedCost         float64
		expectError          bool
		description          string
	}{
		"basic_cluster_with_clusterType_label": {
			nodes: []*clustercache.Node{
				{
					Labels: map[string]string{
						"clusterType": "BASIC_CLUSTER",
					},
					Annotations: map[string]string{
						nodePoolIdAnnotation: "test-pool",
					},
				},
			},
			enhancedClusterPrice: 0.1,
			expectedPlatform:     managementPlatformOKE,
			expectedCost:         0.0,
			expectError:          false,
			description:          "Should return 0.0 cost for BASIC_CLUSTER with clusterType label",
		},
		"basic_cluster_with_oke_label": {
			nodes: []*clustercache.Node{
				{
					Labels: map[string]string{
						"oke.oraclecloud.com/basic-cluster": "true",
					},
					Annotations: map[string]string{
						nodePoolIdAnnotation: "test-pool",
					},
				},
			},
			enhancedClusterPrice: 0.1,
			expectedPlatform:     managementPlatformOKE,
			expectedCost:         0.0,
			expectError:          false,
			description:          "Should return 0.0 cost for BASIC_CLUSTER with oke.oraclecloud.com/basic-cluster label",
		},
		"enhanced_cluster_default": {
			nodes: []*clustercache.Node{
				{
					Labels: map[string]string{
						"kubernetes.io/arch": "amd64",
					},
					Annotations: map[string]string{
						nodePoolIdAnnotation: "test-pool",
					},
				},
			},
			enhancedClusterPrice: 0.15,
			expectedPlatform:     managementPlatformOKE,
			expectedCost:         0.15,
			expectError:          false,
			description:          "Should return enhanced cluster pricing when no basic cluster labels found",
		},
		"enhanced_cluster_with_other_labels": {
			nodes: []*clustercache.Node{
				{
					Labels: map[string]string{
						"clusterType":        "ENHANCED_CLUSTER",
						"kubernetes.io/arch": "amd64",
					},
					Annotations: map[string]string{
						nodePoolIdAnnotation: "test-pool",
					},
				},
			},
			enhancedClusterPrice: 0.2,
			expectedPlatform:     managementPlatformOKE,
			expectedCost:         0.2,
			expectError:          false,
			description:          "Should return enhanced cluster pricing when clusterType is ENHANCED_CLUSTER",
		},
		"multiple_nodes_basic_cluster_first": {
			nodes: []*clustercache.Node{
				{
					Labels: map[string]string{
						"clusterType": "BASIC_CLUSTER",
					},
					Annotations: map[string]string{
						nodePoolIdAnnotation: "test-pool",
					},
				},
				{
					Labels: map[string]string{
						"kubernetes.io/arch": "amd64",
					},
					Annotations: map[string]string{
						nodePoolIdAnnotation: "test-pool-2",
					},
				},
			},
			enhancedClusterPrice: 0.1,
			expectedPlatform:     managementPlatformOKE,
			expectedCost:         0.0,
			expectError:          false,
			description:          "Should return 0.0 cost when first node has BASIC_CLUSTER label",
		},
		"multiple_nodes_basic_cluster_second": {
			nodes: []*clustercache.Node{
				{
					Labels: map[string]string{
						"kubernetes.io/arch": "amd64",
					},
					Annotations: map[string]string{
						nodePoolIdAnnotation: "test-pool",
					},
				},
				{
					Labels: map[string]string{
						"oke.oraclecloud.com/basic-cluster": "true",
					},
					Annotations: map[string]string{
						nodePoolIdAnnotation: "test-pool-2",
					},
				},
			},
			enhancedClusterPrice: 0.1,
			expectedPlatform:     managementPlatformOKE,
			expectedCost:         0.0,
			expectError:          false,
			description:          "Should return 0.0 cost when second node has oke.oraclecloud.com/basic-cluster label",
		},
		"non_oke_cluster": {
			nodes: []*clustercache.Node{
				{
					Labels: map[string]string{
						"kubernetes.io/arch": "amd64",
					},
					Annotations: map[string]string{
						// No OKE annotations
					},
				},
			},
			enhancedClusterPrice: 0.1,
			expectedPlatform:     "",
			expectedCost:         0.0,
			expectError:          false,
			description:          "Should return empty platform and 0.0 cost for non-OKE cluster",
		},
		"no_nodes": {
			nodes:                []*clustercache.Node{},
			enhancedClusterPrice: 0.1,
			expectedPlatform:     "",
			expectedCost:         0.0,
			expectError:          false,
			description:          "Should return empty platform and 0.0 cost when no nodes present",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			// Create Oracle provider with mock dependencies
			oracle := createTestOracleProvider(testCase.nodes, testCase.enhancedClusterPrice)

			// Call the method under test
			platform, cost, err := oracle.ClusterManagementPricing()

			// Assertions
			if testCase.expectError {
				assert.Error(t, err, testCase.description)
			} else {
				assert.NoError(t, err, testCase.description)
			}

			assert.Equal(t, testCase.expectedPlatform, platform, testCase.description)
			assert.Equal(t, testCase.expectedCost, cost, testCase.description)
		})
	}
}

func TestGetManagementPlatform(t *testing.T) {
	testCases := map[string]struct {
		nodes            []*clustercache.Node
		expectedPlatform string
		description      string
	}{
		"oke_cluster_with_node_pool_annotation": {
			nodes: []*clustercache.Node{
				{
					Annotations: map[string]string{
						nodePoolIdAnnotation: "test-pool",
					},
				},
			},
			expectedPlatform: managementPlatformOKE,
			description:      "Should return OKE platform when node has node pool annotation",
		},
		"oke_cluster_with_virtual_pool_annotation": {
			nodes: []*clustercache.Node{
				{
					Annotations: map[string]string{
						virtualPoolIdAnnotation: "test-virtual-pool",
					},
				},
			},
			expectedPlatform: managementPlatformOKE,
			description:      "Should return OKE platform when node has virtual pool annotation",
		},
		"non_oke_cluster": {
			nodes: []*clustercache.Node{
				{
					Annotations: map[string]string{
						"some-other-annotation": "value",
					},
				},
			},
			expectedPlatform: "",
			description:      "Should return empty platform for non-OKE cluster",
		},
		"no_nodes": {
			nodes:            []*clustercache.Node{},
			expectedPlatform: "",
			description:      "Should return empty platform when no nodes present",
		},
		"multiple_nodes_with_oke_annotation": {
			nodes: []*clustercache.Node{
				{
					Annotations: map[string]string{
						"some-other-annotation": "value",
					},
				},
				{
					Annotations: map[string]string{
						nodePoolIdAnnotation: "test-pool",
					},
				},
			},
			expectedPlatform: managementPlatformOKE,
			description:      "Should return OKE platform when any node has OKE annotation",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			// Create Oracle provider with mock cluster cache
			oracle := &Oracle{
				Clientset: &mockClusterCache{nodes: testCase.nodes},
			}

			// Call the method under test
			platform, err := oracle.GetManagementPlatform()

			// Assertions
			assert.NoError(t, err, testCase.description)
			assert.Equal(t, testCase.expectedPlatform, platform, testCase.description)
		})
	}
}

func TestClusterManagementPricing_ErrorCases(t *testing.T) {
	t.Run("pricing_data_not_available", func(t *testing.T) {
		// Create Oracle provider without RateCardStore (nil) but with a mock config that returns error
		oracle := &Oracle{
			Clientset: &mockClusterCache{
				nodes: []*clustercache.Node{
					{
						Annotations: map[string]string{
							nodePoolIdAnnotation: "test-pool",
						},
					},
				},
			},
			RateCardStore: nil,                                    // This will cause ensurePricingData to fail
			Config:        &mockProviderConfig{shouldError: true}, // This will cause GetConfig to fail
		}

		// Call the method under test
		platform, cost, err := oracle.ClusterManagementPricing()

		// Assertions - should return error when pricing data is not available
		assert.Error(t, err, "Should return error when pricing data is not available")
		assert.Equal(t, "", platform, "Should return empty platform on error")
		assert.Equal(t, 0.0, cost, "Should return 0.0 cost on error")
	})
}
