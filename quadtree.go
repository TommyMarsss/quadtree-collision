package main

import "sort"

// Rect 是轴对齐包围盒（AABB）。所有碰撞体与四叉树节点都用它表示。
type Rect struct {
	X, Y, W, H float64
}

func (r Rect) Right() float64  { return r.X + r.W }
func (r Rect) Bottom() float64 { return r.Y + r.H }

// Contains 报告 r 是否完整包含 o（边界相接也算包含）。
func (r Rect) Contains(o Rect) bool {
	return o.X >= r.X && o.Y >= r.Y && o.Right() <= r.Right() && o.Bottom() <= r.Bottom()
}

// Intersects 报告两个矩形是否重叠。仅边界相接（面积为 0）不算碰撞，
// 暴力法与四叉树共用此函数，保证判定口径一致。
func (r Rect) Intersects(o Rect) bool {
	return r.X < o.Right() && o.X < r.Right() && r.Y < o.Bottom() && o.Y < r.Bottom()
}

// Body 是一个运动中的碰撞体（有面积，不是点）。
type Body struct {
	ID     int
	Box    Rect
	VX, VY float64
}

// Config 控制四叉树的分裂/合并行为。
type Config struct {
	// Capacity：叶子节点对象数超过该值时分裂为四个子节点。
	Capacity int
	// MergeThreshold：子树对象总数降到该值及以下时合并回父节点。
	// 取 Capacity 的一半，形成迟滞区间，避免对象数在阈值附近抖动时反复分裂/合并。
	MergeThreshold int
	// MaxDepth：最大分裂深度。达到上限后节点不再分裂，
	// 退化为该节点内的暴力两两比较，防止高密度场景下无限分裂。
	MaxDepth int
}

// DefaultConfig 返回模拟使用的默认配置。
func DefaultConfig() Config {
	return Config{Capacity: 8, MergeThreshold: 4, MaxDepth: 6}
}

// Quadtree 是动态四叉树节点。归属策略：对象存入能完整包含它的最小节点；
// 跨越分割线的对象留在父节点，由查询逻辑保证不漏检。
type Quadtree struct {
	bounds   Rect
	depth    int
	cfg      Config
	objects  []*Body // 完整包含于本节点、但放不进任何子节点的对象
	children [4]*Quadtree
}

// NewQuadtree 创建一棵空树，bounds 通常为整个世界范围。
func NewQuadtree(bounds Rect, cfg Config) *Quadtree {
	return &Quadtree{bounds: bounds, cfg: cfg}
}

// childIndex 返回能完整包含 r 的子节点象限编号；r 跨越分割线时返回 false。
// 象限编号：0=左上 1=右上 2=左下 3=右下。
func (t *Quadtree) childIndex(r Rect) (int, bool) {
	midX := t.bounds.X + t.bounds.W/2
	midY := t.bounds.Y + t.bounds.H/2
	left := r.Right() <= midX
	right := r.X >= midX
	top := r.Bottom() <= midY
	bottom := r.Y >= midY
	switch {
	case left && top:
		return 0, true
	case right && top:
		return 1, true
	case left && bottom:
		return 2, true
	case right && bottom:
		return 3, true
	}
	return 0, false
}

// Insert 将对象插入能完整包含它的最小节点，必要时触发分裂。
func (t *Quadtree) Insert(b *Body) {
	if t.children[0] != nil {
		if idx, ok := t.childIndex(b.Box); ok {
			t.children[idx].Insert(b)
			return
		}
	}
	t.objects = append(t.objects, b)
	if t.children[0] == nil && len(t.objects) > t.cfg.Capacity && t.depth < t.cfg.MaxDepth {
		t.split()
	}
}

// split 分裂为四个子节点，并把能完整落入子节点的对象下移；
// 跨分割线的对象继续留在本节点。
func (t *Quadtree) split() {
	midX := t.bounds.X + t.bounds.W/2
	midY := t.bounds.Y + t.bounds.H/2
	hw, hh := t.bounds.W/2, t.bounds.H/2
	t.children[0] = &Quadtree{bounds: Rect{t.bounds.X, t.bounds.Y, hw, hh}, depth: t.depth + 1, cfg: t.cfg}
	t.children[1] = &Quadtree{bounds: Rect{midX, t.bounds.Y, hw, hh}, depth: t.depth + 1, cfg: t.cfg}
	t.children[2] = &Quadtree{bounds: Rect{t.bounds.X, midY, hw, hh}, depth: t.depth + 1, cfg: t.cfg}
	t.children[3] = &Quadtree{bounds: Rect{midX, midY, hw, hh}, depth: t.depth + 1, cfg: t.cfg}

	kept := t.objects[:0]
	for _, b := range t.objects {
		if idx, ok := t.childIndex(b.Box); ok {
			t.children[idx].Insert(b)
		} else {
			kept = append(kept, b)
		}
	}
	t.objects = kept
}

// Remove 按 ID 移除对象，并自底向上检查是否需要合并。
func (t *Quadtree) Remove(id int) bool {
	node := t
	// 沿"能完整包含"的路径下行，找到对象实际所在节点。
	for node.children[0] != nil {
		found := false
		for _, b := range node.objects {
			if b.ID == id {
				found = true
				break
			}
		}
		if found {
			break
		}
		// 对象必在某个子节点里；逐个尝试。
		next := node
		for _, c := range node.children {
			if c.containsID(id) {
				next = c
				break
			}
		}
		if next == node {
			break
		}
		node = next
	}
	removed := false
	for i, b := range node.objects {
		if b.ID == id {
			node.objects = append(node.objects[:i], node.objects[i+1:]...)
			removed = true
			break
		}
	}
	if removed {
		t.maybeMerge()
	}
	return removed
}

func (t *Quadtree) containsID(id int) bool {
	for _, b := range t.objects {
		if b.ID == id {
			return true
		}
	}
	if t.children[0] != nil {
		for _, c := range t.children {
			if c.containsID(id) {
				return true
			}
		}
	}
	return false
}

// maybeMerge 递归检查：子树对象总数降到 MergeThreshold 及以下时合并回本节点。
func (t *Quadtree) maybeMerge() {
	if t.children[0] == nil {
		return
	}
	for _, c := range t.children {
		c.maybeMerge()
	}
	if t.subtreeCount() <= t.cfg.MergeThreshold {
		var all []*Body
		t.collect(&all)
		t.objects = all
		t.children = [4]*Quadtree{}
	}
}

// subtreeCount 统计以本节点为根的子树中的对象总数。
func (t *Quadtree) subtreeCount() int {
	n := len(t.objects)
	if t.children[0] != nil {
		for _, c := range t.children {
			n += c.subtreeCount()
		}
	}
	return n
}

// collect 收集子树内全部对象。
func (t *Quadtree) collect(out *[]*Body) {
	*out = append(*out, t.objects...)
	if t.children[0] != nil {
		for _, c := range t.children {
			c.collect(out)
		}
	}
}

// queryCandidates 对 r 重叠的子树中每个对象调用 fn。
// 关键正确性保证：跨边界对象存放在祖先节点，而查询会访问所有与 r
// 重叠的节点（含祖先），因此任何与 r 相交的对象都会被访问到，不会漏检。
func (t *Quadtree) queryCandidates(r Rect, fn func(*Body)) {
	if !t.bounds.Intersects(r) {
		return
	}
	for _, b := range t.objects {
		fn(b)
	}
	if t.children[0] != nil {
		for _, c := range t.children {
			c.queryCandidates(r, fn)
		}
	}
}

// FindPairs 返回所有碰撞对（按 ID 升序去重），以及实际执行的 AABB 比较次数。
// 对每个对象查询候选，仅对 ID 更大的候选做重叠测试，保证每对至多比较一次、
// 结果与暴力法完全一致（不多报不漏报）。
func (t *Quadtree) FindPairs() (pairs [][2]int, comparisons int) {
	var all []*Body
	t.collect(&all)
	for _, a := range all {
		t.queryCandidates(a.Box, func(o *Body) {
			if o.ID <= a.ID {
				return
			}
			comparisons++
			if a.Box.Intersects(o.Box) {
				pairs = append(pairs, [2]int{a.ID, o.ID})
			}
		})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i][0] != pairs[j][0] {
			return pairs[i][0] < pairs[j][0]
		}
		return pairs[i][1] < pairs[j][1]
	})
	return pairs, comparisons
}

// CrossBoundaryIDs 返回跨边界对象（存放在有子节点的内部节点上，
// 即无法完整落入任何单一象限的对象）的 ID 列表。
func (t *Quadtree) CrossBoundaryIDs() []int {
	var ids []int
	var walk func(n *Quadtree)
	walk = func(n *Quadtree) {
		if n.children[0] != nil {
			for _, b := range n.objects {
				ids = append(ids, b.ID)
			}
			for _, c := range n.children {
				walk(c)
			}
		}
	}
	walk(t)
	sort.Ints(ids)
	return ids
}

// NodeInfo 是节点的可视化/统计快照。
type NodeInfo struct {
	Bounds Rect
	Depth  int
	Count  int // 直接存放在该节点的对象数
}

// Nodes 返回所有已创建节点（含空节点）的快照，用于绘制树的结构。
func (t *Quadtree) Nodes() []NodeInfo {
	var out []NodeInfo
	var walk func(n *Quadtree)
	walk = func(n *Quadtree) {
		out = append(out, NodeInfo{Bounds: n.bounds, Depth: n.depth, Count: len(n.objects)})
		if n.children[0] != nil {
			for _, c := range n.children {
				walk(c)
			}
		}
	}
	walk(t)
	return out
}

// MaxDepthUsed 返回当前树实际达到的最大深度。
func (t *Quadtree) MaxDepthUsed() int {
	max := t.depth
	if t.children[0] != nil {
		for _, c := range t.children {
			if d := c.MaxDepthUsed(); d > max {
				max = d
			}
		}
	}
	return max
}
