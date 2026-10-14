package arch

import (
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/simple"
)

// gonumView presents a Graph to gonum's algorithms: node i is g.nodes[i],
// and nodes and arcs are listed in sorted order, so an algorithm that
// traverses them visits them in the same order on every run.
type gonumView struct {
	g *Graph
	// in lists, per node, the nodes with an arc to it, sorted.
	in [][]int
}

// asGonum returns g as a gonum directed graph. It is a function rather than
// a method so that Graph does not refer to its view.
func asGonum(g *Graph) *gonumView {
	in := make([][]int, len(g.nodes))
	for from, arcs := range g.out {
		for _, a := range arcs {
			in[a.to] = append(in[a.to], from)
		}
	}
	return &gonumView{g: g, in: in}
}

// Node returns the node with the given ID, or nil.
func (v *gonumView) Node(id int64) graph.Node {
	if id < 0 || id >= int64(len(v.g.nodes)) {
		return nil
	}
	return simple.Node(id)
}

// Nodes returns every node, in sorted order.
func (v *gonumView) Nodes() graph.Nodes {
	nodes := make([]graph.Node, len(v.g.nodes))
	for i := range nodes {
		nodes[i] = simple.Node(int64(i))
	}
	return iterator.NewOrderedNodes(nodes)
}

// From returns the targets of the node's arcs, in sorted order.
func (v *gonumView) From(id int64) graph.Nodes {
	arcs := v.g.out[id]
	nodes := make([]graph.Node, len(arcs))
	for i, a := range arcs {
		nodes[i] = simple.Node(int64(a.to))
	}
	return iterator.NewOrderedNodes(nodes)
}

// To returns the sources of the arcs to the node, in sorted order.
func (v *gonumView) To(id int64) graph.Nodes {
	from := v.in[id]
	nodes := make([]graph.Node, len(from))
	for i, f := range from {
		nodes[i] = simple.Node(int64(f))
	}
	return iterator.NewOrderedNodes(nodes)
}

// HasEdgeBetween reports an arc either way between the nodes.
func (v *gonumView) HasEdgeBetween(xid, yid int64) bool {
	return v.HasEdgeFromTo(xid, yid) || v.HasEdgeFromTo(yid, xid)
}

// HasEdgeFromTo reports an arc from u to v.
func (v *gonumView) HasEdgeFromTo(uid, vid int64) bool {
	return v.g.Weight(v.g.nodes[uid], v.g.nodes[vid]) > 0
}

// Edge returns the arc from u to v, or nil.
func (v *gonumView) Edge(uid, vid int64) graph.Edge {
	w := v.g.Weight(v.g.nodes[uid], v.g.nodes[vid])
	if w == 0 {
		return nil
	}
	return simple.WeightedEdge{F: simple.Node(uid), T: simple.Node(vid), W: float64(w)}
}
