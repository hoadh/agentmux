package dag

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hoadh/agentmux/internal/config"
)

// Graph represents a directed acyclic graph of agent dependencies.
type Graph struct {
	nodes map[string]bool
	edges map[string][]string // node → what it depends ON
	rdeps map[string][]string // node → what depends on IT
	inDeg map[string]int
}

// New creates an empty graph.
func New() *Graph {
	return &Graph{
		nodes: make(map[string]bool),
		edges: make(map[string][]string),
		rdeps: make(map[string][]string),
		inDeg: make(map[string]int),
	}
}

// AddNode registers a node in the graph.
func (g *Graph) AddNode(name string) {
	g.nodes[name] = true
	if _, ok := g.inDeg[name]; !ok {
		g.inDeg[name] = 0
	}
}

// AddEdge adds a dependency: "from" depends on "to".
func (g *Graph) AddEdge(from, to string) {
	g.edges[from] = append(g.edges[from], to)
	g.rdeps[to] = append(g.rdeps[to], from)
	g.inDeg[from]++
}

// Validate checks for cycles using Kahn's algorithm.
func (g *Graph) Validate() error {
	if len(g.nodes) == 0 {
		return nil
	}

	// Work on a copy of in-degrees
	inDeg := make(map[string]int, len(g.inDeg))
	for k, v := range g.inDeg {
		inDeg[k] = v
	}

	// Queue all nodes with in-degree 0
	queue := make([]string, 0)
	for name := range g.nodes {
		if inDeg[name] == 0 {
			queue = append(queue, name)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++

		for _, dep := range g.rdeps[node] {
			inDeg[dep]--
			if inDeg[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if visited != len(g.nodes) {
		// Find cycle participants
		cycleNodes := make([]string, 0)
		for name := range g.nodes {
			if inDeg[name] > 0 {
				cycleNodes = append(cycleNodes, name)
			}
		}
		sort.Strings(cycleNodes)
		return fmt.Errorf("cycle detected involving: %s", strings.Join(cycleNodes, ", "))
	}

	return nil
}

// Roots returns nodes with no dependencies (in-degree 0), sorted alphabetically.
func (g *Graph) Roots() []string {
	roots := make([]string, 0)
	for name := range g.nodes {
		if g.inDeg[name] == 0 {
			roots = append(roots, name)
		}
	}
	sort.Strings(roots)
	return roots
}

// Dependents returns nodes that depend on the given node.
func (g *Graph) Dependents(name string) []string {
	return g.rdeps[name]
}

// Dependencies returns nodes that the given node depends on.
func (g *Graph) Dependencies(name string) []string {
	return g.edges[name]
}

// InDegree returns the in-degree of a node.
func (g *Graph) InDegree(name string) int {
	return g.inDeg[name]
}

// Nodes returns all node names sorted alphabetically.
func (g *Graph) Nodes() []string {
	names := make([]string, 0, len(g.nodes))
	for name := range g.nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// BuildFromConfig constructs a graph from the config's depends_on fields.
func BuildFromConfig(cfg *config.Config) (*Graph, error) {
	g := New()

	if cfg == nil || len(cfg.Agents) == 0 {
		return g, nil
	}

	for name := range cfg.Agents {
		g.AddNode(name)
	}
	for name, agentCfg := range cfg.Agents {
		for _, dep := range agentCfg.DependsOn {
			g.AddEdge(name, dep)
		}
	}

	if err := g.Validate(); err != nil {
		return nil, err
	}

	return g, nil
}
