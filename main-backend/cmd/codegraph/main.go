// Yosuri CodeGraph - HNSW-style project navigation for agents
// Layers: 0=Symbol, 1=File, 2=Module, 3=Project
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ─── Core Types ───────────────────────────────────────────────────────────────

type NodeType int

const (
	Symbol NodeType = iota
	File
	Module
	Project
)

func (n NodeType) String() string {
	return []string{"Symbol", "File", "Module", "Project"}[n]
}

func (n NodeType) Layer() int {
	return int(n)
}

type EdgeType int

const (
	Contains EdgeType = iota
	Calls
	Sibling
	Semantic
	Import
)

func (e EdgeType) String() string {
	return []string{"Contains", "Calls", "Sibling", "Semantic", "Import"}[e]
}

type Node struct {
	ID        string                 `json:"id"`
	Type      NodeType               `json:"type"`
	Name      string                 `json:"name"`
	Path      string                 `json:"path"`
	Package   string                 `json:"package"`
	Layer     int                    `json:"layer"`
	Neighbors map[int][]string       `json:"neighbors"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type Edge struct {
	From      string   `json:"from"`
	To        string   `json:"to"`
	Type      EdgeType `json:"type"`
	Weight    float64  `json:"weight"`
	FromLayer int      `json:"from_layer"`
	ToLayer   int      `json:"to_layer"`
}

type CodeGraph struct {
	Nodes     map[string]*Node `json:"nodes"`
	Edges     []Edge           `json:"edges"`
	MaxLayer  int              `json:"max_layer"`
	ByPath    map[string]*Node `json:"-"`
	ByPkg     map[string][]*Node `json:"-"`
}

func NewCodeGraph() *CodeGraph {
	return &CodeGraph{
		Nodes:    make(map[string]*Node),
		ByPath:   make(map[string]*Node),
		ByPkg:    make(map[string][]*Node),
		MaxLayer: 3,
	}
}

func (g *CodeGraph) AddNode(n *Node) {
	if n.Neighbors == nil {
		n.Neighbors = make(map[int][]string)
	}
	if n.Metadata == nil {
		n.Metadata = make(map[string]interface{})
	}
	g.Nodes[n.ID] = n
	g.ByPath[n.Path] = n
	g.ByPkg[n.Package] = append(g.ByPkg[n.Package], n)
}

func (g *CodeGraph) AddEdge(from, to string, et EdgeType, w float64) {
	fn, tn := g.Nodes[from], g.Nodes[to]
	if fn == nil || tn == nil {
		return
	}
	g.Edges = append(g.Edges, Edge{
		From: from, To: to, Type: et, Weight: w,
		FromLayer: fn.Layer, ToLayer: tn.Layer,
	})
	fn.Neighbors[fn.Layer] = append(fn.Neighbors[fn.Layer], to)
}

func (g *CodeGraph) Node(id string) *Node { return g.Nodes[id] }

func (g *CodeGraph) FindByName(name string) []*Node {
	var out []*Node
	nl := strings.ToLower(name)
	for _, n := range g.Nodes {
		if strings.Contains(strings.ToLower(n.Name), nl) {
			out = append(out, n)
		}
	}
	return out
}

func (g *CodeGraph) AtLayer(layer int) []*Node {
	var out []*Node
	for _, n := range g.Nodes {
		if n.Layer == layer {
			out = append(out, n)
		}
	}
	return out
}

// ─── Parser ──────────────────────────────────────────────────────────────────

type Parser struct {
	Graph  *CodeGraph
	Root   string
	ProjID string
}

func NewParser(root string) *Parser {
	return &Parser{
		Graph:  NewCodeGraph(),
		Root:   root,
		ProjID: "proj:" + filepath.Base(root),
	}
}

func (p *Parser) Parse() error {
	// Resolve to absolute path to avoid dot-prefix issues
	abs, err := filepath.Abs(p.Root)
	if err == nil {
		p.Root = abs
	}
	proj := &Node{ID: p.ProjID, Type: Project, Name: filepath.Base(p.Root), Path: p.Root, Layer: 3}
	p.Graph.AddNode(proj)

	return filepath.Walk(p.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			b := filepath.Base(path)
			if strings.HasPrefix(b, ".") || b == "vendor" || b == "node_modules" || b == "build" || b == "dist" || b == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		p.parseFile(path, proj)
		return nil
	})
}

func (p *Parser) parseFile(path string, proj *Node) {
	dir := filepath.Dir(path)
	pkg := filepath.Base(dir)

	modID := "mod:" + dir
	mod := p.Graph.Node(modID)
	if mod == nil {
		mod = &Node{ID: modID, Type: Module, Name: pkg, Path: dir, Package: pkg, Layer: 2}
		p.Graph.AddNode(mod)
		p.Graph.AddEdge(proj.ID, modID, Contains, 1)
	}

	fileID := "file:" + path
	fnode := &Node{ID: fileID, Type: File, Name: filepath.Base(path), Path: path, Package: pkg, Layer: 1}
	p.Graph.AddNode(fnode)
	p.Graph.AddEdge(modID, fileID, Contains, 1)

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return
	}

	// Track symbols in this file for sibling edges
	var fileSyms []*Node

	ast.Inspect(f, func(n ast.Node) bool {
		switch d := n.(type) {
		case *ast.FuncDecl:
			name := d.Name.Name
			sid := fmt.Sprintf("sym:%s:%s", path, name)
			snode := &Node{ID: sid, Type: Symbol, Name: name, Path: path, Package: pkg, Layer: 0}
			p.Graph.AddNode(snode)
			p.Graph.AddEdge(fileID, sid, Contains, 1)
			fileSyms = append(fileSyms, snode)
			p.extractCalls(d, snode)

		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					name := ts.Name.Name
					sid := fmt.Sprintf("sym:%s:%s", path, name)
					snode := &Node{ID: sid, Type: Symbol, Name: name, Path: path, Package: pkg, Layer: 0}
					p.Graph.AddNode(snode)
					p.Graph.AddEdge(fileID, sid, Contains, 1)
					fileSyms = append(fileSyms, snode)
				}
			}
		}
		return true
	})

	// Sibling edges between files in same module
	for _, other := range p.Graph.ByPkg[pkg] {
		if other.ID != fileID && other.Type == File {
			p.Graph.AddEdge(fileID, other.ID, Sibling, 0.5)
		}
	}

	_ = fset
}

func (p *Parser) extractCalls(decl *ast.FuncDecl, symNode *Node) {
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		var name string
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			name = fn.Name
		case *ast.SelectorExpr:
			name = fn.Sel.Name
		default:
			return true
		}
		symNode.Metadata["call:"+name] = true
		return true
	})
}

func (p *Parser) ResolveCalls() {
	byName := make(map[string][]*Node)
	for _, n := range p.Graph.Nodes {
		if n.Type == Symbol {
			byName[n.Name] = append(byName[n.Name], n)
		}
	}
	for _, n := range p.Graph.Nodes {
		if n.Type != Symbol {
			continue
		}
		for k := range n.Metadata {
			if strings.HasPrefix(k, "call:") {
				target := k[5:]
				for _, t := range byName[target] {
					if t.ID != n.ID {
						p.Graph.AddEdge(n.ID, t.ID, Calls, 1)
					}
				}
			}
		}
	}
}

func (p *Parser) AddSemanticEdges() {
	nodes := make([]*Node, 0, len(p.Graph.Nodes))
	for _, n := range p.Graph.Nodes {
		nodes = append(nodes, n)
	}
	for i, a := range nodes {
		for j := i + 1; j < len(nodes); j++ {
			b := nodes[j]
			if a.ID == b.ID {
				continue
			}
			s := simScore(a.Name, b.Name)
			if s > 0.6 && s < 1.0 {
				p.Graph.AddEdge(a.ID, b.ID, Semantic, s)
			}
		}
	}
}

func simScore(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return 0.8
	}
	minL := len(a)
	if len(b) < minL {
		minL = len(b)
	}
	pref := 0
	for i := 0; i < minL; i++ {
		if a[i] == b[i] {
			pref++
		} else {
			break
		}
	}
	den := len(a)
	if len(b) > den {
		den = len(b)
	}
	return float64(pref) / float64(den)
}

// ─── HNSW Jumper ─────────────────────────────────────────────────────────────

type Jumper struct {
	Graph *CodeGraph
}

func NewJumper(g *CodeGraph) *Jumper { return &Jumper{Graph: g} }

func (j *Jumper) Jump(startID string, goal func(*Node) bool, maxSteps int) ([]string, *Node) {
	start := j.Graph.Node(startID)
	if start == nil {
		return nil, nil
	}
	visited := map[string]bool{startID: true}
	path := []string{startID}
	cur := start

	for step := 0; step < maxSteps; step++ {
		if goal(cur) {
			return path, cur
		}
		next := j.bestNeighbor(cur, visited, goal)
		if next == nil {
			break
		}
		visited[next.ID] = true
		path = append(path, next.ID)
		cur = next
	}
	return path, cur
}

func (j *Jumper) bestNeighbor(cur *Node, visited map[string]bool, goal func(*Node) bool) *Node {
	// Same layer first
	for _, nb := range j.neighborsAt(cur, cur.Layer) {
		if !visited[nb.ID] && goal(nb) {
			return nb
		}
	}
	// Go up for broader context
	for l := cur.Layer + 1; l <= j.Graph.MaxLayer; l++ {
		for _, nb := range j.neighborsAt(cur, l) {
			if !visited[nb.ID] {
				return nb
			}
		}
	}
	// Go down for specifics
	for l := cur.Layer - 1; l >= 0; l-- {
		for _, nb := range j.neighborsAt(cur, l) {
			if !visited[nb.ID] {
				return nb
			}
		}
	}
	return nil
}

func (j *Jumper) neighborsAt(n *Node, layer int) []*Node {
	var out []*Node
	for _, id := range n.Neighbors[layer] {
		if nb, ok := j.Graph.Nodes[id]; ok {
			out = append(out, nb)
		}
	}
	return out
}

// ─── CLI ──────────────────────────────────────────────────────────────────────

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: codegraph <command> [args]")
		fmt.Println("Commands:")
		fmt.Println("  index <path>      - Index project into graph")
		fmt.Println("  stats             - Show graph statistics")
		fmt.Println("  find <name>       - Find nodes by name")
		fmt.Println("  jump <from> <to>  - Navigate from node to target")
		fmt.Println("  layers            - List nodes per layer")
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "index":
		if len(os.Args) < 3 {
			fmt.Println("Usage: codegraph index <path>")
			os.Exit(1)
		}
		root := os.Args[2]
		p := NewParser(root)
		if err := p.Parse(); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		p.ResolveCalls()
		p.AddSemanticEdges()

		// Save graph
		out := filepath.Base(root) + ".graph.json"
		data, _ := json.MarshalIndent(p.Graph, "", "  ")
		os.WriteFile(out, data, 0644)
		fmt.Printf("Indexed: %s → %s\n", root, out)
		printStats(p.Graph)

	case "stats":
		g := loadGraph()
		printStats(g)

	case "find":
		if len(os.Args) < 3 {
			fmt.Println("Usage: codegraph find <name>")
			os.Exit(1)
		}
		g := loadGraph()
		nodes := g.FindByName(os.Args[2])
		for _, n := range nodes {
			fmt.Printf("[%s] %s — %s\n", n.Type, n.Name, n.Path)
		}

	case "jump":
		if len(os.Args) < 4 {
			fmt.Println("Usage: codegraph jump <from_name> <to_name>")
			os.Exit(1)
		}
		g := loadGraph()
		j := NewJumper(g)
		fromNodes := g.FindByName(os.Args[2])
		toNodes := g.FindByName(os.Args[3])
		if len(fromNodes) == 0 || len(toNodes) == 0 {
			fmt.Println("Node not found")
			os.Exit(1)
		}
		toIDs := map[string]bool{}
		for _, n := range toNodes {
			toIDs[n.ID] = true
		}
		path, final := j.Jump(fromNodes[0].ID, func(n *Node) bool {
			return toIDs[n.ID]
		}, 15)
		if final != nil {
			fmt.Printf("Found: %s\nPath:\n", final.Name)
			printPath(g, path)
		} else {
			fmt.Println("No path found")
		}

	case "layers":
		g := loadGraph()
		for l := 0; l <= 3; l++ {
			nodes := g.AtLayer(l)
			fmt.Printf("\n=== Layer %d (%s) — %d nodes ===\n", l, NodeType(l), len(nodes))
			sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
			limit := 10
			if len(nodes) < limit {
				limit = len(nodes)
			}
			for i := 0; i < limit; i++ {
				n := nodes[i]
				nc := 0
				for _, ids := range n.Neighbors {
					nc += len(ids)
				}
				fmt.Printf("  %-30s %d neighbors\n", n.Name, nc)
			}
			if len(nodes) > limit {
				fmt.Printf("  ... and %d more\n", len(nodes)-limit)
			}
		}

	default:
		fmt.Println("Unknown command:", cmd)
	}
}

func loadGraph() *CodeGraph {
	files, _ := filepath.Glob("*.graph.json")
	if len(files) == 0 {
		fmt.Println("No graph file found. Run 'index' first.")
		os.Exit(1)
	}
	data, _ := os.ReadFile(files[0])
	var g CodeGraph
	json.Unmarshal(data, &g)
	// Rebuild indexes
	g.ByPath = make(map[string]*Node)
	g.ByPkg = make(map[string][]*Node)
	for _, n := range g.Nodes {
		g.ByPath[n.Path] = n
		g.ByPkg[n.Package] = append(g.ByPkg[n.Package], n)
	}
	return &g
}

func printStats(g *CodeGraph) {
	fmt.Println("=== Graph Stats ===")
	for l := 0; l <= 3; l++ {
		fmt.Printf("Layer %d (%s): %d nodes\n", l, NodeType(l), len(g.AtLayer(l)))
	}
	fmt.Printf("Total edges: %d\n", len(g.Edges))
	ec := make(map[EdgeType]int)
	for _, e := range g.Edges {
		ec[e.Type]++
	}
	fmt.Println("Edge types:")
	for t, c := range ec {
		fmt.Printf("  %s: %d\n", t, c)
	}
}

func printPath(g *CodeGraph, path []string) {
	for i, id := range path {
		n := g.Node(id)
		if n != nil {
			indent := strings.Repeat("  ", i)
			fmt.Printf("%s→ [%s] %s\n", indent, n.Type, n.Name)
		}
	}
}
