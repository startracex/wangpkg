package deps

import (
	"maps"
	"sort"
	"testing"
)

func TestAddSingle(t *testing.T) {
	g := NewGraph[string]()
	g.Add("a")
	if !g.Has("a") {
		t.Error("expected graph to have 'a'")
	}
	if g.Len() != 1 {
		t.Errorf("expected Len() = 1, got %d", g.Len())
	}
}

func TestAddMultiple(t *testing.T) {
	g := NewGraph[string]()
	g.Add("a")
	g.Add("b")
	g.Add("c")
	if g.Len() != 3 {
		t.Errorf("expected Len() = 3, got %d", g.Len())
	}
}

func TestAddIdempotent(t *testing.T) {
	g := NewGraph[string]()
	g.Add("a")
	g.Add("a")
	g.Add("a")
	if g.Len() != 1 {
		t.Errorf("expected Len() = 1 after duplicate adds, got %d", g.Len())
	}
}

func TestAddEdge(t *testing.T) {
	g := NewGraph[string]()
	g.AddEdge("a", "b")
	if !g.Has("a") || !g.Has("b") {
		t.Error("expected both 'a' and 'b' to exist after AddEdge")
	}
	if g.Len() != 2 {
		t.Errorf("expected Len() = 2, got %d", g.Len())
	}
}

func TestAddEdgeImplicitAdd(t *testing.T) {
	g := NewGraph[string]()
	g.AddEdge("x", "y")
	if !g.Has("x") || !g.Has("y") {
		t.Error("AddEdge should implicitly add both nodes")
	}
}

func TestAddEdgeFromTo(t *testing.T) {
	g := NewGraph[string]()
	g.AddEdge("a", "b")
	g.AddEdge("a", "c")

	fromA := g.From("a")
	sort.Strings(fromA)
	if len(fromA) != 2 || fromA[0] != "b" || fromA[1] != "c" {
		t.Errorf("expected From('a') = [b c], got %v", fromA)
	}

	toB := g.To("b")
	if len(toB) != 1 || toB[0] != "a" {
		t.Errorf("expected To('b') = [a], got %v", toB)
	}
}

func TestAddEdgeDuplicate(t *testing.T) {
	g := NewGraph[string]()
	g.AddEdge("a", "b")
	g.AddEdge("a", "b")

	fromA := g.From("a")
	if len(fromA) != 2 {
		t.Errorf("duplicate edges are stored twice; expected From('a') len = 2, got %d", len(fromA))
	}
}

func TestHasTrue(t *testing.T) {
	g := NewGraph[string]()
	g.Add("a")
	if !g.Has("a") {
		t.Error("expected Has('a') = true")
	}
}

func TestHasFalse(t *testing.T) {
	g := NewGraph[string]()
	if g.Has("missing") {
		t.Error("expected Has('missing') = false for empty graph")
	}
}

func TestHasFalseAfterAddingOther(t *testing.T) {
	g := NewGraph[string]()
	g.Add("a")
	if g.Has("b") {
		t.Error("expected Has('b') = false when only 'a' was added")
	}
}

func TestFromEmpty(t *testing.T) {
	g := NewGraph[string]()
	g.Add("a")
	from := g.From("a")
	if len(from) != 0 {
		t.Errorf("expected empty slice, got %v", from)
	}
}

func TestToEmpty(t *testing.T) {
	g := NewGraph[string]()
	g.Add("a")
	to := g.To("a")
	if len(to) != 0 {
		t.Errorf("expected empty slice, got %v", to)
	}
}

func TestFromNonexistent(t *testing.T) {
	g := NewGraph[string]()
	from := g.From("nope")
	if from != nil {
		t.Errorf("expected nil for nonexistent node, got %v", from)
	}
}

func TestToNonexistent(t *testing.T) {
	g := NewGraph[string]()
	to := g.To("nope")
	if to != nil {
		t.Errorf("expected nil for nonexistent node, got %v", to)
	}
}

func TestChainedEdges(t *testing.T) {
	// a -> b -> c
	g := NewGraph[string]()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")

	fromA := g.From("a")
	if len(fromA) != 1 || fromA[0] != "b" {
		t.Errorf("expected From('a') = [b], got %v", fromA)
	}

	fromB := g.From("b")
	if len(fromB) != 1 || fromB[0] != "c" {
		t.Errorf("expected From('b') = [c], got %v", fromB)
	}

	toC := g.To("c")
	if len(toC) != 1 || toC[0] != "b" {
		t.Errorf("expected To('c') = [b], got %v", toC)
	}
}

func TestLenEmpty(t *testing.T) {
	g := NewGraph[string]()
	if g.Len() != 0 {
		t.Errorf("expected 0, got %d", g.Len())
	}
}

func TestLenAfterEdges(t *testing.T) {
	g := NewGraph[string]()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	g.AddEdge("a", "c")
	if g.Len() != 3 {
		t.Errorf("expected 3 unique nodes, got %d", g.Len())
	}
}

func TestIterateEmpty(t *testing.T) {
	g := NewGraph[string]()
	var collected []string
	for n := range g.Iterate() {
		collected = append(collected, n)
	}
	if len(collected) != 0 {
		t.Errorf("expected no elements, got %v", collected)
	}
}

func TestIterateAll(t *testing.T) {
	g := NewGraph[string]()
	g.AddEdge("a", "b")
	g.AddEdge("c", "d")

	collected := maps.Collect(g.Iterate())
	if len(collected) != 4 {
		t.Errorf("expected 4 elements, got %d", len(collected))
	}
	for _, expected := range []string{"a", "b", "c", "d"} {
		if _, ok := collected[expected]; !ok {
			t.Errorf("expected to iterate over %q", expected)
		}
	}
}

func TestIterateBreakEarly(t *testing.T) {
	g := NewGraph[int]()
	for i := range 10 {
		g.Add(i)
	}

	count := 0
	for range g.Iterate() {
		count++
		if count >= 3 {
			break
		}
	}
	if count != 3 {
		t.Errorf("expected to break at 3, got %d", count)
	}
}

func TestGroupsEmpty(t *testing.T) {
	g := NewGraph[string]()
	groups := g.Groups()
	if len(groups) != 0 {
		t.Errorf("expected 0 groups for empty graph, got %d", len(groups))
	}
}

func TestGroupsSingleNode(t *testing.T) {
	g := NewGraph[string]()
	g.Add("a")
	groups := g.Groups()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0]) != 1 || groups[0][0] != "a" {
		t.Errorf("expected [[a]], got %v", groups)
	}
}

func TestGroupsLinearChain(t *testing.T) {
	// a -> b -> c
	g := NewGraph[string]()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")

	groups := g.Groups()
	if len(groups) != 3 {
		t.Fatalf("expected 3 layers, got %d: %v", len(groups), groups)
	}
	if len(groups[0]) != 1 || groups[0][0] != "a" {
		t.Errorf("expected first layer [a], got %v", groups[0])
	}
	if len(groups[1]) != 1 || groups[1][0] != "b" {
		t.Errorf("expected second layer [b], got %v", groups[1])
	}
	if len(groups[2]) != 1 || groups[2][0] != "c" {
		t.Errorf("expected third layer [c], got %v", groups[2])
	}
}

func TestGroupsParallelNodes(t *testing.T) {
	// a -> c, b -> c
	g := NewGraph[string]()
	g.AddEdge("a", "c")
	g.AddEdge("b", "c")

	groups := g.Groups()
	if len(groups) != 2 {
		t.Fatalf("expected 2 layers, got %d: %v", len(groups), groups)
	}

	sort.Strings(groups[0])
	if len(groups[0]) != 2 {
		t.Errorf("expected first layer to have 2 nodes, got %v", groups[0])
	}
	if groups[0][0] != "a" || groups[0][1] != "b" {
		t.Errorf("expected first layer [a b], got %v", groups[0])
	}
	if len(groups[1]) != 1 || groups[1][0] != "c" {
		t.Errorf("expected second layer [c], got %v", groups[1])
	}
}

func TestGroupsDiamond(t *testing.T) {
	// a -> b, a -> c, b -> d, c -> d
	g := NewGraph[string]()
	g.AddEdge("a", "b")
	g.AddEdge("a", "c")
	g.AddEdge("b", "d")
	g.AddEdge("c", "d")

	groups := g.Groups()
	if len(groups) != 3 {
		t.Fatalf("expected 3 layers, got %d: %v", len(groups), groups)
	}

	// Layer 0: a
	if len(groups[0]) != 1 || groups[0][0] != "a" {
		t.Errorf("expected layer 0 = [a], got %v", groups[0])
	}
	// Layer 1: b, c
	sort.Strings(groups[1])
	if len(groups[1]) != 2 || groups[1][0] != "b" || groups[1][1] != "c" {
		t.Errorf("expected layer 1 = [b c], got %v", groups[1])
	}
	// Layer 2: d
	if len(groups[2]) != 1 || groups[2][0] != "d" {
		t.Errorf("expected layer 2 = [d], got %v", groups[2])
	}
}

func TestGroupsCycleDetection(t *testing.T) {
	// a -> b -> a (cycle), c is independent
	g := NewGraph[string]()
	g.AddEdge("a", "b")
	g.AddEdge("b", "a")
	g.Add("c")

	groups := g.Groups()
	if len(groups) != 2 {
		t.Fatalf("expected 2 layers (one for c, one for cycle), got %d: %v", len(groups), groups)
	}

	if len(groups[0]) != 1 || groups[0][0] != "c" {
		t.Errorf("expected first layer [c], got %v", groups[0])
	}

	sort.Strings(groups[1])
	if len(groups[1]) != 2 || groups[1][0] != "a" || groups[1][1] != "b" {
		t.Errorf("expected cycle layer [a b], got %v", groups[1])
	}
}

func TestGroupsSelfLoop(t *testing.T) {
	g := NewGraph[string]()
	g.AddEdge("a", "a")

	groups := g.Groups()
	if len(groups) != 1 {
		t.Fatalf("expected 1 layer for self-loop, got %d: %v", len(groups), groups)
	}
	if len(groups[0]) != 1 || groups[0][0] != "a" {
		t.Errorf("expected [a], got %v", groups[0])
	}
}

func TestGroupsAllNodesPresent(t *testing.T) {
	g := NewGraph[string]()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	g.AddEdge("d", "e")

	groups := g.Groups()
	all := map[string]bool{}
	for _, layer := range groups {
		for _, n := range layer {
			all[n] = true
		}
	}
	for _, expected := range []string{"a", "b", "c", "d", "e"} {
		if !all[expected] {
			t.Errorf("node %q missing from groups output", expected)
		}
	}
}

func TestGroupsIsolatedNodes(t *testing.T) {
	g := NewGraph[string]()
	g.Add("x")
	g.Add("y")
	g.Add("z")

	groups := g.Groups()
	if len(groups) != 1 {
		t.Fatalf("expected 1 layer for isolated nodes, got %d: %v", len(groups), groups)
	}
	sort.Strings(groups[0])
	if len(groups[0]) != 3 || groups[0][0] != "x" || groups[0][1] != "y" || groups[0][2] != "z" {
		t.Errorf("expected [x y z], got %v", groups[0])
	}
}

func TestGroupsIntType(t *testing.T) {
	g := NewGraph[int]()
	g.AddEdge(1, 2)
	g.AddEdge(2, 3)
	g.AddEdge(1, 3)

	groups := g.Groups()
	if len(groups) != 3 {
		t.Fatalf("expected 3 layers, got %d", len(groups))
	}
	if len(groups[0]) != 1 || groups[0][0] != 1 {
		t.Errorf("expected layer 0 = [1], got %v", groups[0])
	}
}

func TestPruneEmpty(t *testing.T) {
	g := NewGraph[string]()
	result := g.Prune(nil, nil)
	if len(result) != 0 {
		t.Errorf("expected empty result on empty graph, got %v", result)
	}
}

func TestPruneSingleNodeNoPreserve(t *testing.T) {
	g := NewGraph[string]()
	g.Add("a")

	result := g.Prune([]string{"a"}, nil)
	if len(result) != 1 {
		t.Errorf("expected 1 pruned node, got %d", len(result))
	}
	if _, ok := result["a"]; !ok {
		t.Error("expected 'a' to be pruned")
	}
}

func TestPruneSingleNodePreserved(t *testing.T) {
	g := NewGraph[string]()
	g.Add("a")

	result := g.Prune([]string{"a"}, map[string]struct{}{"a": {}})
	if len(result) != 0 {
		t.Errorf("expected 0 pruned nodes when preserved, got %d", len(result))
	}
}

func TestPruneChain(t *testing.T) {
	// a -> b -> c
	g := NewGraph[string]()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")

	result := g.Prune([]string{"a"}, nil)
	if len(result) != 3 {
		t.Errorf("expected 3 pruned nodes (a, b, c), got %d: %v", len(result), result)
	}
	for _, n := range []string{"a", "b", "c"} {
		if _, ok := result[n]; !ok {
			t.Errorf("expected %q to be pruned", n)
		}
	}
}

func TestPruneChainPreserveMiddle(t *testing.T) {
	// a -> b -> c
	// preserve b: a should be pruned, b preserved, c depends on b (indegree from b)
	g := NewGraph[string]()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")

	result := g.Prune([]string{"a"}, map[string]struct{}{"b": {}})
	if _, ok := result["a"]; !ok {
		t.Error("expected 'a' to be pruned")
	}
	if _, ok := result["b"]; ok {
		t.Error("expected 'b' to be preserved")
	}
	if _, ok := result["c"]; ok {
		t.Error("expected 'c' to NOT be pruned (depends on preserved 'b')")
	}
}

func TestPruneBranching(t *testing.T) {
	// a -> b, a -> c
	g := NewGraph[string]()
	g.AddEdge("a", "b")
	g.AddEdge("a", "c")

	result := g.Prune([]string{"a"}, nil)
	if len(result) != 3 {
		t.Errorf("expected 3 pruned, got %d: %v", len(result), result)
	}
}

func TestPruneMultipleRoots(t *testing.T) {
	// a -> c, b -> c
	g := NewGraph[string]()
	g.AddEdge("a", "c")
	g.AddEdge("b", "c")

	result := g.Prune([]string{"a"}, nil)
	if _, ok := result["a"]; !ok {
		t.Error("expected 'a' to be pruned")
	}
	if _, ok := result["c"]; ok {
		t.Error("expected 'c' NOT to be pruned (still depended on by b)")
	}
}

func TestPruneMultipleRootsAll(t *testing.T) {
	// a -> c, b -> c
	g := NewGraph[string]()
	g.AddEdge("a", "c")
	g.AddEdge("b", "c")

	result := g.Prune([]string{"a", "b"}, nil)
	if len(result) != 3 {
		t.Errorf("expected 3 pruned, got %d: %v", len(result), result)
	}
}

func TestPrunePreserveLeaf(t *testing.T) {
	// a -> b
	g := NewGraph[string]()
	g.AddEdge("a", "b")

	result := g.Prune([]string{"a"}, map[string]struct{}{"b": {}})
	if _, ok := result["a"]; !ok {
		t.Error("expected 'a' to be pruned")
	}
	if _, ok := result["b"]; ok {
		t.Error("expected 'b' to be preserved")
	}
}

func TestPruneStartContainsPreserved(t *testing.T) {
	g := NewGraph[string]()
	g.AddEdge("a", "b")

	result := g.Prune([]string{"a", "b"}, map[string]struct{}{"a": {}})
	if _, ok := result["a"]; ok {
		t.Error("expected 'a' to be preserved")
	}
	if _, ok := result["b"]; !ok {
		t.Error("expected 'b' to be pruned (its only upstream is preserved)")
	}
}

func TestPruneDiamondPreserveCenter(t *testing.T) {
	// a -> b, a -> c, b -> d, c -> d
	g := NewGraph[string]()
	g.AddEdge("a", "b")
	g.AddEdge("a", "c")
	g.AddEdge("b", "d")
	g.AddEdge("c", "d")

	// Prune starting from a, preserve d
	result := g.Prune([]string{"a"}, map[string]struct{}{"d": {}})
	if _, ok := result["a"]; !ok {
		t.Error("expected 'a' pruned")
	}
	if _, ok := result["b"]; !ok {
		t.Error("expected 'b' pruned")
	}
	if _, ok := result["c"]; !ok {
		t.Error("expected 'c' pruned")
	}
	if _, ok := result["d"]; ok {
		t.Error("expected 'd' preserved")
	}
}

func TestPruneWithIntType(t *testing.T) {
	g := NewGraph[int]()
	g.AddEdge(1, 2)
	g.AddEdge(2, 3)

	result := g.Prune([]int{1}, nil)
	if len(result) != 3 {
		t.Errorf("expected 3 pruned, got %d: %v", len(result), result)
	}
}

func TestPruneStartNotInGraph(t *testing.T) {
	g := NewGraph[string]()
	g.Add("a")

	result := g.Prune([]string{"z"}, nil)
	if _, ok := result["z"]; !ok {
		t.Error("expected 'z' to be in toDelete even if not in graph")
	}
	if _, ok := result["a"]; ok {
		t.Error("expected 'a' not to be pruned")
	}
}
