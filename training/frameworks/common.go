package frameworks

import (
	"fmt"

	"gpu-orchestrator/core/models"
)

// validateClusterTopology validates that all nodes are in same provider+region+network
// Phase 4: Common validation for all frameworks
func validateClusterTopology(cluster *models.Cluster) error {
	if len(cluster.Nodes) == 0 {
		return fmt.Errorf("empty cluster")
	}

	// Get first node's topology
	firstNode := cluster.Nodes[0]
	expectedProvider := firstNode.Provider
	expectedRegion := firstNode.Region
	expectedVPC := firstNode.VPC

	// Check all other nodes match
	for i, node := range cluster.Nodes {
		if node.Provider != expectedProvider {
			return fmt.Errorf("node %d has provider %s, expected %s", i, node.Provider, expectedProvider)
		}
		if node.Region != expectedRegion {
			return fmt.Errorf("node %d has region %s, expected %s", i, node.Region, expectedRegion)
		}
		if node.VPC != expectedVPC {
			return fmt.Errorf("node %d has VPC %s, expected %s", i, node.VPC, expectedVPC)
		}
	}

	return nil
}
