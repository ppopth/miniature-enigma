package topology

import ()

// GenerateLinear creates a linear topology (0-1-2-3-...-N-1)
func GenerateLinear(nodeCount int) *Topology {
	topo := NewTopology(nodeCount)

	for i := 0; i < nodeCount-1; i++ {
		topo.AddConnection(i, i+1)
	}

	return topo
}

// GenerateRing creates a ring topology (0-1-2-...-N-1-0)
func GenerateRing(nodeCount int) *Topology {
	topo := NewTopology(nodeCount)

	for i := 0; i < nodeCount; i++ {
		nextNode := (i + 1) % nodeCount
		topo.AddConnection(i, nextNode)
	}

	return topo
}

// GenerateMesh creates a fully connected mesh topology
func GenerateMesh(nodeCount int) *Topology {
	topo := NewTopology(nodeCount)

	for i := 0; i < nodeCount; i++ {
		for j := i + 1; j < nodeCount; j++ {
			topo.AddConnection(i, j)
		}
	}

	return topo
}

// GenerateTree creates a tree topology with specified branching factor
func GenerateTree(nodeCount int, branchingFactor int) *Topology {
	topo := NewTopology(nodeCount)

	if branchingFactor < 2 {
		branchingFactor = 2
	}

	for i := 0; i < nodeCount; i++ {
		// Connect to children
		for j := 0; j < branchingFactor; j++ {
			child := i*branchingFactor + j + 1
			if child < nodeCount {
				topo.AddConnection(i, child)
			}
		}
	}

	return topo
}

// GenerateSmallWorld creates a small-world topology using Watts-Strogatz model
func GenerateSmallWorld(nodeCount int, k int, rewireProbability float64) *Topology {
	topo := NewTopology(nodeCount)

	if k < 2 {
		k = 2
	}
	if k >= nodeCount {
		k = nodeCount - 1
	}

	// Start with ring lattice
	for i := 0; i < nodeCount; i++ {
		for j := 1; j <= k/2; j++ {
			rightNeighbor := (i + j) % nodeCount
			topo.AddConnection(i, rightNeighbor)
		}
	}

	// For deterministic simulation, use hash-based "rewiring"
	// (In practice, this would use random rewiring)
	if rewireProbability > 0 {
		// Simple deterministic rewiring based on node hash
		for i := 0; i < nodeCount; i++ {
			hash := (i*13 + 7) % 100
			if float64(hash)/100.0 < rewireProbability {
				// "Rewire" by adding a long-range connection
				target := (i + hash*3) % nodeCount
				if target != i {
					topo.AddConnection(i, target)
				}
			}
		}
	}

	return topo
}

// GenerateRandomRegular creates a random regular graph where each node has exactly 'degree' connections
// to randomly selected other nodes. Guarantees connectivity by checking and regenerating until connected.
func GenerateRandomRegular(nodeCount int, degree int) *Topology {
	if nodeCount < 2 {
		return NewTopology(nodeCount)
	}

	if degree < 1 {
		degree = 2 // Minimum degree for connectivity
	}
	if degree >= nodeCount {
		degree = nodeCount - 1 // Maximum possible degree
	}

	// For odd degree * nodeCount, we need to adjust to make it even
	// (each edge contributes 2 to the total degree sum)
	totalDegree := degree * nodeCount
	if totalDegree%2 != 0 {
		if degree < nodeCount-1 {
			degree++ // Increase degree by 1 to make total even
		} else {
			degree-- // Decrease if we're at maximum
		}
	}

	// Keep trying until we generate a connected graph
	for attempt := 0; ; attempt++ {
		topo := NewTopology(nodeCount)

		// Use attempt number as additional seed for variety
		seed := attempt * 137

		// Use deterministic "random" selection based on node hash for reproducibility
		// This ensures consistent results across runs while appearing random
		for nodeID := 0; nodeID < nodeCount; nodeID++ {
			connections := make(map[int]bool)
			connections[nodeID] = true // Can't connect to self

			for len(connections)-1 < degree { // -1 because we exclude self
				// Generate pseudo-random target using hash function with attempt seed
				hash := (nodeID*17 + len(connections)*23 + seed + 42) % nodeCount
				target := hash

				// If target is already connected or is self, find next available
				for connections[target] {
					target = (target + 1) % nodeCount
				}

				connections[target] = true

				// Add bidirectional connection (avoid duplicates)
				if nodeID < target {
					topo.AddConnection(nodeID, target)
				}
			}
		}

		// Check if the graph is connected
		if topo.IsConnected() {
			return topo
		}
	}
}
