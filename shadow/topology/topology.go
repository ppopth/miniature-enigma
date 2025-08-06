package topology

import (
	"encoding/json"
	"fmt"
	"os"
)

// Connection represents a network connection between two nodes
type Connection struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// Topology represents a network topology as an adjacency list
type Topology struct {
	NodeCount     int          `json:"node_count"`
	Connections   []Connection `json:"connections"`
	adjacencyList map[int][]int
}

// NewTopology creates a new empty topology
func NewTopology(nodeCount int) *Topology {
	return &Topology{
		NodeCount:     nodeCount,
		Connections:   []Connection{},
		adjacencyList: make(map[int][]int),
	}
}

// LoadFromFile loads topology from a JSON file
func LoadFromFile(filename string) (*Topology, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read topology file: %w", err)
	}

	var topo Topology
	if err := json.Unmarshal(data, &topo); err != nil {
		return nil, fmt.Errorf("failed to parse topology JSON: %w", err)
	}

	// Build adjacency list from connections
	topo.buildAdjacencyList()

	return &topo, nil
}

// SaveToFile saves topology to a JSON file
func (t *Topology) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal topology: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write topology file: %w", err)
	}

	return nil
}

// AddConnection adds a bidirectional connection between two nodes
func (t *Topology) AddConnection(from, to int) {
	if from < 0 || from >= t.NodeCount || to < 0 || to >= t.NodeCount || from == to {
		return
	}

	// Check if connection already exists
	for _, conn := range t.Connections {
		if (conn.From == from && conn.To == to) || (conn.From == to && conn.To == from) {
			return
		}
	}

	// Add to connections list (store with lower ID first for consistency)
	if from < to {
		t.Connections = append(t.Connections, Connection{From: from, To: to})
	} else {
		t.Connections = append(t.Connections, Connection{From: to, To: from})
	}

	// Add to adjacency list (bidirectional)
	t.adjacencyList[from] = append(t.adjacencyList[from], to)
	t.adjacencyList[to] = append(t.adjacencyList[to], from)
}

// GetConnections returns all nodes connected to the given node
func (t *Topology) GetConnections(nodeID int) []int {
	return t.adjacencyList[nodeID]
}

// GetAllConnections returns all connections in the topology
func (t *Topology) GetAllConnections() []Connection {
	return t.Connections
}

// buildAdjacencyList builds the adjacency list from the connections slice
func (t *Topology) buildAdjacencyList() {
	t.adjacencyList = make(map[int][]int)

	for _, conn := range t.Connections {
		// Add bidirectional connections to adjacency list
		t.adjacencyList[conn.From] = append(t.adjacencyList[conn.From], conn.To)
		t.adjacencyList[conn.To] = append(t.adjacencyList[conn.To], conn.From)
	}
}

// GetDescription returns a human-readable description of the topology
func (t *Topology) GetDescription() string {
	return fmt.Sprintf("Topology with %d nodes and %d connections", t.NodeCount, len(t.Connections))
}

// IsConnected checks if the topology graph is connected using BFS
func (t *Topology) IsConnected() bool {
	if t.NodeCount <= 1 {
		return true
	}

	visited := make(map[int]bool)
	queue := []int{0} // Start BFS from node 0
	visited[0] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Visit all neighbors
		neighbors := t.GetConnections(current)
		for _, neighbor := range neighbors {
			if !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	// Check if all nodes were visited
	return len(visited) == t.NodeCount
}
