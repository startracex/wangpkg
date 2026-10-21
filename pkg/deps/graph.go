package deps

import (
	"iter"
	"maps"
)

type Graph[T comparable] struct {
	dependsOn  map[T][]T
	dependents map[T][]T
	indegree   map[T]int
}

func NewGraph[T comparable]() *Graph[T] {
	return &Graph[T]{
		dependsOn:  make(map[T][]T),
		dependents: make(map[T][]T),
		indegree:   make(map[T]int),
	}
}

func (g *Graph[T]) Add(node T) {
	if _, ok := g.dependsOn[node]; !ok {
		g.dependsOn[node] = nil
		g.dependents[node] = nil
	}
}

func (g *Graph[T]) AddEdge(from, to T) {
	g.dependsOn[from] = append(g.dependsOn[from], to)
	g.dependents[to] = append(g.dependents[to], from)
	if _, ok := g.dependsOn[to]; !ok {
		g.dependsOn[to] = nil
	}
	g.indegree[to]++
}

func (g *Graph[T]) Has(node T) bool {
	_, ok := g.dependsOn[node]
	return ok
}

func (g *Graph[T]) From(node T) []T {
	return g.dependsOn[node]
}

func (g *Graph[T]) To(node T) []T {
	return g.dependents[node]
}

func (g *Graph[T]) Len() int {
	return len(g.dependsOn)
}

func (g *Graph[T]) Clone() *Graph[T] {
	return &Graph[T]{
		dependsOn:  maps.Clone(g.dependsOn),
		dependents: maps.Clone(g.dependents),
		indegree:   maps.Clone(g.indegree),
	}
}

func (g *Graph[T]) Iterate() iter.Seq2[T, []T] {
	return func(yield func(T, []T) bool) {
		for k, v := range g.dependsOn {
			if !yield(k, v) {
				return
			}
		}
	}
}

func (g *Graph[T]) Groups() [][]T {
	var layers [][]T
	if len(g.dependsOn) == 0 {
		return layers
	}
	inDegree := make(map[T]int, len(g.dependsOn))
	for k := range g.dependsOn {
		inDegree[k] = g.indegree[k]
	}

	var current []T
	for n := range g.dependsOn {
		if inDegree[n] == 0 {
			current = append(current, n)
		}
	}

	processed := 0

	for len(current) > 0 {
		next := []T{}
		layer := make([]T, 0, len(current))

		for _, n := range current {
			layer = append(layer, n)
			processed++

			for _, dep := range g.dependsOn[n] {
				inDegree[dep]--
				if inDegree[dep] == 0 {
					next = append(next, dep)
				}
			}
		}

		layers = append(layers, layer)
		current = next
	}

	if processed != len(g.dependsOn) {
		cycleLayer := []T{}
		for n := range g.dependsOn {
			if inDegree[n] > 0 {
				cycleLayer = append(cycleLayer, n)
			}
		}
		if len(cycleLayer) > 0 {
			layers = append(layers, cycleLayer)
		}
	}

	return layers
}

func (g *Graph[T]) Prune(start []T, preserve map[T]struct{}) map[T]struct{} {
	deleteSet := map[T]struct{}{}
	queue := make([]T, 0, len(start))

	for _, n := range start {
		if _, ok := preserve[n]; ok {
			continue
		}

		hasUpstream := false
		for _, up := range g.dependents[n] {
			if _, del := deleteSet[up]; !del {
				if _, p := preserve[up]; !p {
					hasUpstream = true
					break
				}
			}
		}

		if !hasUpstream {
			queue = append(queue, n)
		}
	}

	for len(queue) > 0 {
		n := queue[len(queue)-1]
		queue = queue[:len(queue)-1]

		if _, ok := deleteSet[n]; ok {
			continue
		}

		deleteSet[n] = struct{}{}

		for _, dep := range g.dependsOn[n] {
			g.indegree[dep]--

			if g.indegree[dep] != 0 {
				continue
			}

			if _, ok := preserve[dep]; ok {
				continue
			}

			queue = append(queue, dep)
		}
	}

	return deleteSet
}
