package dag

import (
	"sort"
	"strings"
	"testing"

	"github.com/hoadh/agentmux/internal/config"
)

func TestNew(t *testing.T) {
	g := New()

	if g == nil {
		t.Fatal("New returned nil")
	}

	if len(g.nodes) != 0 {
		t.Errorf("nodes count: got %d, want 0", len(g.nodes))
	}

	if len(g.edges) != 0 {
		t.Errorf("edges count: got %d, want 0", len(g.edges))
	}
}

func TestAddNode(t *testing.T) {
	g := New()

	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")

	if len(g.nodes) != 3 {
		t.Errorf("nodes count: got %d, want 3", len(g.nodes))
	}

	if !g.nodes["a"] || !g.nodes["b"] || !g.nodes["c"] {
		t.Fatal("nodes not properly added")
	}

	if g.inDeg["a"] != 0 || g.inDeg["b"] != 0 || g.inDeg["c"] != 0 {
		t.Fatal("in-degree not initialized to 0")
	}
}

func TestAddEdge(t *testing.T) {
	g := New()

	g.AddNode("a")
	g.AddNode("b")

	g.AddEdge("a", "b")

	if len(g.edges["a"]) != 1 || g.edges["a"][0] != "b" {
		t.Fatal("edge from a->b not properly added")
	}

	if len(g.rdeps["b"]) != 1 || g.rdeps["b"][0] != "a" {
		t.Fatal("reverse dep b<-a not properly added")
	}

	if g.inDeg["a"] != 1 {
		t.Errorf("in-degree of a: got %d, want 1", g.inDeg["a"])
	}
}

func TestValidate_EmptyGraph(t *testing.T) {
	g := New()

	err := g.Validate()
	if err != nil {
		t.Fatalf("Validate failed on empty graph: %v", err)
	}
}

func TestValidate_SingleNode(t *testing.T) {
	g := New()
	g.AddNode("a")

	err := g.Validate()
	if err != nil {
		t.Fatalf("Validate failed on single node: %v", err)
	}
}

func TestValidate_LinearChain(t *testing.T) {
	g := New()

	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")

	g.AddEdge("b", "a")
	g.AddEdge("c", "b")

	err := g.Validate()
	if err != nil {
		t.Fatalf("Validate failed on linear chain: %v", err)
	}
}

func TestValidate_Diamond(t *testing.T) {
	g := New()

	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddNode("d")

	g.AddEdge("c", "a")
	g.AddEdge("c", "b")
	g.AddEdge("d", "a")
	g.AddEdge("d", "b")

	err := g.Validate()
	if err != nil {
		t.Fatalf("Validate failed on diamond: %v", err)
	}
}

func TestValidate_Parallel(t *testing.T) {
	g := New()

	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddNode("d")

	g.AddEdge("c", "a")
	g.AddEdge("d", "b")

	err := g.Validate()
	if err != nil {
		t.Fatalf("Validate failed on parallel: %v", err)
	}
}

func TestValidate_CycleSimple(t *testing.T) {
	g := New()

	g.AddNode("a")
	g.AddNode("b")

	g.AddEdge("a", "b")
	g.AddEdge("b", "a")

	err := g.Validate()
	if err == nil {
		t.Fatal("expected error for cycle")
	}

	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle, got: %v", err)
	}
}

func TestValidate_CycleSelfLoop(t *testing.T) {
	g := New()

	g.AddNode("a")
	g.AddEdge("a", "a")

	err := g.Validate()
	if err == nil {
		t.Fatal("expected error for self-loop cycle")
	}

	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle, got: %v", err)
	}
}

func TestValidate_CycleComplex(t *testing.T) {
	g := New()

	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddNode("d")

	g.AddEdge("b", "a")
	g.AddEdge("c", "b")
	g.AddEdge("d", "c")
	g.AddEdge("a", "d")

	err := g.Validate()
	if err == nil {
		t.Fatal("expected error for complex cycle")
	}

	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle, got: %v", err)
	}
}

func TestRoots_Empty(t *testing.T) {
	g := New()

	roots := g.Roots()
	if len(roots) != 0 {
		t.Errorf("roots count: got %d, want 0", len(roots))
	}
}

func TestRoots_SingleNode(t *testing.T) {
	g := New()
	g.AddNode("a")

	roots := g.Roots()
	if len(roots) != 1 || roots[0] != "a" {
		t.Fatalf("roots: got %v, want [a]", roots)
	}
}

func TestRoots_LinearChain(t *testing.T) {
	g := New()

	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")

	g.AddEdge("b", "a")
	g.AddEdge("c", "b")

	// AddEdge(from, to) means "from depends on to", so:
	// b depends on a, c depends on b → root is a (in-degree 0)
	roots := g.Roots()
	if len(roots) != 1 || roots[0] != "a" {
		t.Fatalf("roots: got %v, want [a]", roots)
	}
}

func TestRoots_MultipleRoots(t *testing.T) {
	g := New()

	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddNode("d")

	// c depends on a, d depends on b → roots are a and b
	g.AddEdge("c", "a")
	g.AddEdge("d", "b")

	roots := g.Roots()
	if len(roots) != 2 {
		t.Fatalf("roots count: got %d, want 2", len(roots))
	}

	expected := []string{"a", "b"}
	sort.Strings(roots)
	sort.Strings(expected)

	if roots[0] != expected[0] || roots[1] != expected[1] {
		t.Fatalf("roots: got %v, want %v", roots, expected)
	}
}

func TestDependents(t *testing.T) {
	g := New()

	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")

	g.AddEdge("b", "a")
	g.AddEdge("c", "a")

	dependents := g.Dependents("a")
	if len(dependents) != 2 {
		t.Fatalf("dependents count: got %d, want 2", len(dependents))
	}

	expected := map[string]bool{"b": true, "c": true}
	for _, dep := range dependents {
		if !expected[dep] {
			t.Errorf("unexpected dependent: %v", dep)
		}
	}
}

func TestDependencies(t *testing.T) {
	g := New()

	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")

	g.AddEdge("c", "a")
	g.AddEdge("c", "b")

	deps := g.Dependencies("c")
	if len(deps) != 2 {
		t.Fatalf("dependencies count: got %d, want 2", len(deps))
	}

	expected := map[string]bool{"a": true, "b": true}
	for _, d := range deps {
		if !expected[d] {
			t.Errorf("unexpected dependency: %v", d)
		}
	}
}

func TestInDegree(t *testing.T) {
	g := New()

	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")

	g.AddEdge("b", "a")
	g.AddEdge("c", "a")
	g.AddEdge("c", "b")

	if g.InDegree("a") != 0 {
		t.Errorf("a in-degree: got %d, want 0", g.InDegree("a"))
	}

	if g.InDegree("b") != 1 {
		t.Errorf("b in-degree: got %d, want 1", g.InDegree("b"))
	}

	if g.InDegree("c") != 2 {
		t.Errorf("c in-degree: got %d, want 2", g.InDegree("c"))
	}
}

func TestNodes(t *testing.T) {
	g := New()

	g.AddNode("c")
	g.AddNode("a")
	g.AddNode("b")

	nodes := g.Nodes()

	expected := []string{"a", "b", "c"}
	if len(nodes) != len(expected) {
		t.Fatalf("nodes count: got %d, want %d", len(nodes), len(expected))
	}

	for i, node := range nodes {
		if node != expected[i] {
			t.Errorf("node[%d]: got %q, want %q", i, node, expected[i])
		}
	}
}

func TestBuildFromConfig_Empty(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Agents:  make(map[string]config.AgentConfig),
	}

	g, err := BuildFromConfig(cfg)
	if err != nil {
		t.Fatalf("BuildFromConfig failed: %v", err)
	}

	if len(g.Nodes()) != 0 {
		t.Errorf("nodes count: got %d, want 0", len(g.Nodes()))
	}
}

func TestBuildFromConfig_Nil(t *testing.T) {
	g, err := BuildFromConfig(nil)
	if err != nil {
		t.Fatalf("BuildFromConfig failed: %v", err)
	}

	if len(g.Nodes()) != 0 {
		t.Errorf("nodes count: got %d, want 0", len(g.Nodes()))
	}
}

func TestBuildFromConfig_SingleAgent(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Agents: map[string]config.AgentConfig{
			"a": {
				Prompt: "Task A",
			},
		},
	}

	g, err := BuildFromConfig(cfg)
	if err != nil {
		t.Fatalf("BuildFromConfig failed: %v", err)
	}

	nodes := g.Nodes()
	if len(nodes) != 1 || nodes[0] != "a" {
		t.Fatalf("nodes: got %v, want [a]", nodes)
	}
}

func TestBuildFromConfig_DependencyChain(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Agents: map[string]config.AgentConfig{
			"a": {
				Prompt: "Task A",
			},
			"b": {
				Prompt:    "Task B",
				DependsOn: []string{"a"},
			},
			"c": {
				Prompt:    "Task C",
				DependsOn: []string{"b"},
			},
		},
	}

	g, err := BuildFromConfig(cfg)
	if err != nil {
		t.Fatalf("BuildFromConfig failed: %v", err)
	}

	// b depends on a, c depends on b → root is a (in-degree 0)
	roots := g.Roots()
	if len(roots) != 1 || roots[0] != "a" {
		t.Fatalf("roots: got %v, want [a]", roots)
	}

	if g.InDegree("a") != 0 {
		t.Errorf("a in-degree: got %d, want 0", g.InDegree("a"))
	}

	if g.InDegree("b") != 1 {
		t.Errorf("b in-degree: got %d, want 1", g.InDegree("b"))
	}

	if g.InDegree("c") != 1 {
		t.Errorf("c in-degree: got %d, want 1", g.InDegree("c"))
	}
}

func TestBuildFromConfig_WithCycle(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Agents: map[string]config.AgentConfig{
			"a": {
				Prompt:    "Task A",
				DependsOn: []string{"b"},
			},
			"b": {
				Prompt:    "Task B",
				DependsOn: []string{"a"},
			},
		},
	}

	_, err := BuildFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error for cycle")
	}

	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle, got: %v", err)
	}
}

func TestBuildFromConfig_Diamond(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Agents: map[string]config.AgentConfig{
			"a": {
				Prompt: "Task A",
			},
			"b": {
				Prompt: "Task B",
			},
			"c": {
				Prompt:    "Task C",
				DependsOn: []string{"a", "b"},
			},
			"d": {
				Prompt:    "Task D",
				DependsOn: []string{"a", "b"},
			},
		},
	}

	g, err := BuildFromConfig(cfg)
	if err != nil {
		t.Fatalf("BuildFromConfig failed: %v", err)
	}

	roots := g.Roots()
	if len(roots) != 2 {
		t.Fatalf("roots count: got %d, want 2", len(roots))
	}

	dependents_a := g.Dependents("a")
	if len(dependents_a) != 2 {
		t.Fatalf("dependents of a: got %d, want 2", len(dependents_a))
	}
}
