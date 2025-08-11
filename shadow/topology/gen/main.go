package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ethp2p/eth-ec-broadcast/shadow/topology"
)

func main() {
	var (
		topologyType = flag.String("type", "linear", "Topology type: linear, ring, mesh, tree, small-world, random-regular")
		nodeCount    = flag.Int("nodes", 10, "Number of nodes")
		output       = flag.String("output", "topology.json", "Output file path")

		// Type-specific parameters
		branchingFactor     = flag.Int("branching", 2, "Branching factor for tree topology")
		smallWorldK         = flag.Int("k", 4, "Number of nearest neighbors for small-world topology")
		smallWorldRewire    = flag.Float64("rewire", 0.1, "Rewire probability for small-world topology")
		randomRegularDegree = flag.Int("degree", 3, "Node degree for random-regular topology")

		// Utility flags
		visualize = flag.Bool("visualize", false, "Print ASCII visualization of the topology")
		stats     = flag.Bool("stats", false, "Print topology statistics")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Topology Generator - Generate network topology files for Shadow simulations\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  Generate linear topology with 20 nodes:\n")
		fmt.Fprintf(os.Stderr, "    %s -type linear -nodes 20 -output linear-20.json\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Generate small-world topology:\n")
		fmt.Fprintf(os.Stderr, "    %s -type small-world -nodes 50 -k 6 -rewire 0.15 -output sw-50.json\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Generate random-regular topology (degree 4):\n")
		fmt.Fprintf(os.Stderr, "    %s -type random-regular -nodes 20 -degree 4 -output rr-20.json\n\n", os.Args[0])
	}

	flag.Parse()

	// Validate node count
	if *nodeCount < 2 {
		log.Fatal("Node count must be at least 2")
	}

	// Create topology based on type
	var topo *topology.Topology

	switch *topologyType {
	case "linear":
		topo = topology.GenerateLinear(*nodeCount)
	case "ring":
		topo = topology.GenerateRing(*nodeCount)
	case "mesh":
		topo = topology.GenerateMesh(*nodeCount)
	case "tree":
		topo = topology.GenerateTree(*nodeCount, *branchingFactor)
	case "small-world":
		topo = topology.GenerateSmallWorld(*nodeCount, *smallWorldK, *smallWorldRewire)
	case "random-regular":
		topo = topology.GenerateRandomRegular(*nodeCount, *randomRegularDegree)
	default:
		log.Fatalf("Unknown topology type: %s", *topologyType)
	}

	if topo == nil {
		log.Fatalf("Failed to create topology of type %s", *topologyType)
	}

	fmt.Printf("Generated: %s\n", topo.GetDescription())

	// Print statistics if requested
	if *stats {
		printTopologyStats(topo)
	}

	// Visualize if requested
	if *visualize {
		visualizeTopology(topo)
	}

	// Save to file
	if err := topo.SaveToFile(*output); err != nil {
		log.Fatalf("Failed to save topology: %v", err)
	}

	fmt.Printf("Topology saved to: %s\n", *output)
}

func printTopologyStats(topo *topology.Topology) {
	fmt.Println("\n=== Topology Statistics ===")

	connections := topo.GetAllConnections()
	nodeCount := topo.NodeCount

	// Count degree of each node
	degree := make(map[int]int)
	for _, conn := range connections {
		degree[conn.From]++
		degree[conn.To]++
	}

	// Calculate statistics
	minDegree := nodeCount
	maxDegree := 0
	totalDegree := 0

	for i := 0; i < nodeCount; i++ {
		d := degree[i]
		if d < minDegree {
			minDegree = d
		}
		if d > maxDegree {
			maxDegree = d
		}
		totalDegree += d
	}

	avgDegree := float64(totalDegree) / float64(nodeCount)

	fmt.Printf("Nodes: %d\n", nodeCount)
	fmt.Printf("Edges: %d\n", len(connections))
	fmt.Printf("Average degree: %.2f\n", avgDegree)
	fmt.Printf("Min degree: %d\n", minDegree)
	fmt.Printf("Max degree: %d\n", maxDegree)

	// Show degree distribution for small networks
	if nodeCount <= 20 {
		fmt.Println("\nDegree distribution:")
		for i := 0; i < nodeCount; i++ {
			fmt.Printf("  Node %2d: degree %d\n", i, degree[i])
		}
	}
}

func visualizeTopology(topo *topology.Topology) {
	connections := topo.GetAllConnections()
	nodeCount := topo.NodeCount

	if nodeCount > 20 {
		fmt.Println("\n(Visualization skipped for networks with more than 20 nodes)")
		return
	}

	fmt.Println("\n=== Topology Visualization ===")
	fmt.Println("Adjacency Matrix (1 = connected, 0 = not connected):")

	// Create adjacency matrix
	matrix := make([][]bool, nodeCount)
	for i := range matrix {
		matrix[i] = make([]bool, nodeCount)
	}

	for _, conn := range connections {
		matrix[conn.From][conn.To] = true
		matrix[conn.To][conn.From] = true
	}

	// Print header
	fmt.Print("    ")
	for i := 0; i < nodeCount; i++ {
		fmt.Printf("%2d ", i)
	}
	fmt.Println()

	// Print matrix
	for i := 0; i < nodeCount; i++ {
		fmt.Printf("%2d: ", i)
		for j := 0; j < nodeCount; j++ {
			if matrix[i][j] {
				fmt.Print(" 1 ")
			} else {
				fmt.Print(" . ")
			}
		}
		fmt.Println()
	}

	// Print edge list for clarity
	fmt.Println("\nEdge List:")
	for _, conn := range connections {
		fmt.Printf("  %d <-> %d\n", conn.From, conn.To)
	}
}
