package main

import "sort"

// Rect 是轴对齐包围盒（AABB），坐标为闭区间：边界相切也算相交，
// 与 bruteForce 采用完全一致的语义。
type Rect struct {
	MinX, MinY, MaxX, MaxY float64
}

// Intersects 判断两个矩形是否相交（边/点相切视为相交）。
func (r Rect) Intersects(o Rect) bool {
	return r.MinX <= o.MaxX && r.MaxX >= o.MinX &&
		r.MinY <= o.MaxY && r.MaxY >= o.MinY
}

// Contains 判断 o 是否被 r 完整包含（含边界）。
func (r Rect) Contains(o Rect) bool {
	return o.MinX >= r.MinX && o.MaxX <= r.MaxX &&
		o.MinY >= r.MinY && o.MaxY <= r.MaxY
}

func (r Rect) Width() float64   { return r.MaxX - r.MinX }
func (r Rect) Height() float64  { return r.MaxY - r.MinY }
func (r Rect) CenterX() float64 { return (r.MinX + r.MaxX) / 2 }
func (r Rect) CenterY() float64 { return (r.MinY + r.MaxY) / 2 }

// Pair 是一对碰撞体 id（I < J）。
type Pair struct{ I, J int }

// Config 控制四叉树的分裂 / 合并行为。
type Config struct {
	Bounds         Rect
	SplitThreshold int // 叶子内对象数 > SplitThreshold 时触发分裂
	MergeThreshold int // 子树对象数 <= MergeThreshold 时合并回父节点（须 < SplitThreshold，构成迟滞）
	MaxDepth       int // 最大分裂深度，达到后叶子退化为节点内暴力比较
}

// Stats 是一次检测的统计信息。
type Stats struct {
	Comparisons      int // 四叉树实际执行的 AABB 比较次数（候选去重后）
	BruteComparisons int // 暴力法的比较次数 n*(n-1)/2
	Pairs            int // 碰撞对数量
	Nodes            int // 节点总数
	InternalNodes    int // 内部节点数
	DegradedNodes    int // 达到最大深度、退化为暴力比较的叶子数
	MaxOccupancy     int // 单个节点承载的最多对象数
}

// NodeSnap 是供可视化使用的节点快照。
type NodeSnap struct {
	Bounds   Rect
	Depth    int
	Leaf     bool
	Degraded bool
	Items    []int // 内部节点中存储的是跨分割线对象；叶子为退化节点时承载聚集对象
}

const (
	quadTL = iota // 左上
	quadTR        // 右上
	quadBL        // 左下
	quadBR        // 右下
)

type node struct {
	bounds   Rect
	parent   *node
	children [4]*node // children[0] == nil 表示叶子
	items    []int
	depth    int
}

// Tree 是动态四叉树。对象可以每帧移动：Update 先移除再插回，
// 移动导致的聚集/分散会实时触发分裂与合并。
type Tree struct {
	cfg    Config
	root   *node
	Rects  []Rect        // id -> 当前包围盒
	owner  map[int]*node // id -> 唯一归属节点
	active []bool        // id 是否仍在树中
}

// NewTree 创建一棵空树。rects 可为 nil（后续 Add 会追加）。
func NewTree(cfg Config, rects []Rect) *Tree {
	if cfg.SplitThreshold <= 0 {
		cfg.SplitThreshold = 8
	}
	if cfg.MergeThreshold < 0 || cfg.MergeThreshold >= cfg.SplitThreshold {
		cfg.MergeThreshold = cfg.SplitThreshold * 3 / 4
	}
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = 8
	}
	return &Tree{
		cfg:    cfg,
		root:   &node{bounds: cfg.Bounds, depth: 0},
		Rects:  rects,
		owner:  make(map[int]*node),
		active: make([]bool, len(rects)),
	}
}

// Add 注册一个新包围盒并插入树中，返回其 id。
func (t *Tree) Add(r Rect) int {
	id := len(t.Rects)
	t.Rects = append(t.Rects, r)
	t.active = append(t.active, true)
	t.insert(id)
	return id
}

// InsertAll 把构造时预置、尚未激活的所有包围盒批量插入树中。
func (t *Tree) InsertAll() {
	for id := range t.Rects {
		if t.active[id] || t.owner[id] != nil {
			continue
		}
		t.active[id] = true
		t.insert(id)
	}
}

// Update 移动一个已存在的对象（先按旧位置移除，再按新位置插回）。
func (t *Tree) Update(id int, r Rect) {
	t.Remove(id)
	t.Rects[id] = r
	t.active[id] = true
	t.insert(id)
}

// Remove 从树中移除对象并尝试合并稀疏节点。
func (t *Tree) Remove(id int) {
	n := t.owner[id]
	if n == nil {
		return
	}
	for i, v := range n.items {
		if v == id {
			n.items = append(n.items[:i], n.items[i+1:]...)
			break
		}
	}
	delete(t.owner, id)
	t.active[id] = false

	// 自底向上尝试合并：对象从某节点消失后，其祖先可能降到合并阈值以下。
	for p := n.parent; p != nil; p = p.parent {
		if p.children[0] != nil && t.subtreeCount(p) <= t.cfg.MergeThreshold {
			t.collapse(p)
		}
	}
}

// Reset 清空整棵树（保留已注册的包围盒槽位语义不变则无此需求，仿真中不使用）。
func (t *Tree) Reset() {
	t.root = &node{bounds: t.cfg.Bounds, depth: 0}
	t.owner = make(map[int]*node)
	for i := range t.Rects {
		t.active[i] = false
	}
}

// quadrant 返回 r 唯一归属的子象限；r 跨越本节点分割线（不含"恰好相切"）时返回 -1。
// 调用方保证 r 已被 n.bounds 完整包含。
// 恰好落在中线上的对象按确定性规则归入一侧（MaxX==mid 归左，MinX==mid 归右），
// 因此任何对象至多落入一个子象限，不会被复制。
func (t *Tree) quadrant(n *node, r Rect) int {
	midX := (n.bounds.MinX + n.bounds.MaxX) / 2
	midY := (n.bounds.MinY + n.bounds.MaxY) / 2

	horizontal := -1 // 0=左, 1=右
	switch {
	case r.MaxX <= midX:
		horizontal = 0
	case r.MinX >= midX:
		horizontal = 1
	}
	if horizontal == -1 {
		return -1 // 跨越竖直分割线
	}

	vertical := -1 // 0=上, 1=下
	switch {
	case r.MaxY <= midY:
		vertical = 0
	case r.MinY >= midY:
		vertical = 1
	}
	if vertical == -1 {
		return -1 // 跨越水平分割线
	}

	if vertical == 0 {
		if horizontal == 0 {
			return quadTL
		}
		return quadTR
	}
	if horizontal == 0 {
		return quadBL
	}
	return quadBR
}

// insert 按"能完整包含对象的最小节点"规则归属：
//   - 在内部节点：对象完整落入某个子象限则继续下行；跨越分割线则留在本节点；
//   - 在叶子：追加；超过分裂阈值且未达最大深度则分裂。
func (t *Tree) insert(id int) {
	r := t.Rects[id]
	n := t.root
	for {
		// 未被根边界完整包含的对象常驻根节点：检测时根节点必被访问，
		// 保证不丢比较；同时维持"对象必被其归属节点完整包含"的不变量。
		if n == t.root && !n.bounds.Contains(r) {
			n.items = append(n.items, id)
			t.owner[id] = n
			if n.children[0] == nil && len(n.items) > t.cfg.SplitThreshold && n.depth < t.cfg.MaxDepth {
				t.split(n)
			}
			return
		}
		if n.children[0] != nil {
			q := t.quadrant(n, r)
			if q < 0 {
				n.items = append(n.items, id)
				t.owner[id] = n
				return
			}
			n = n.children[q]
			continue
		}

		n.items = append(n.items, id)
		t.owner[id] = n
		if len(n.items) > t.cfg.SplitThreshold && n.depth < t.cfg.MaxDepth {
			t.split(n)
		}
		return
	}
}

// split 把叶子分裂为四个子节点：能完整进入某个子象限的对象下放，
// 跨越分割线的对象保留在本节点（本节点因此既是内部节点又携带 items）。
// 分裂后递归处理仍然超阈值的子节点；递归深度受 MaxDepth 严格约束。
func (t *Tree) split(n *node) {
	if n.depth >= t.cfg.MaxDepth {
		return
	}
	if n.bounds.Width() <= 1e-9 || n.bounds.Height() <= 1e-9 {
		return // 浮点尺度耗尽，无法继续有意义地分裂
	}

	minX, minY := n.bounds.MinX, n.bounds.MinY
	maxX, maxY := n.bounds.MaxX, n.bounds.MaxY
	midX := (minX + maxX) / 2
	midY := (minY + maxY) / 2

	quads := [4]Rect{
		{minX, minY, midX, midY}, // TL
		{midX, minY, maxX, midY}, // TR
		{minX, midY, midX, maxY}, // BL
		{midX, midY, maxX, maxY}, // BR
	}
	for i, b := range quads {
		n.children[i] = &node{bounds: b, parent: n, depth: n.depth + 1}
	}

	kept := n.items[:0]
	for _, id := range n.items {
		// 未被本节点完整包含的对象（仅可能出现在根）不允许下放。
		if !n.bounds.Contains(t.Rects[id]) {
			kept = append(kept, id)
			continue
		}
		q := t.quadrant(n, t.Rects[id])
		if q < 0 {
			kept = append(kept, id) // 跨界对象留在本节点
			continue
		}
		child := n.children[q]
		child.items = append(child.items, id)
		t.owner[id] = child
	}
	n.items = kept

	for _, c := range n.children {
		if len(c.items) > t.cfg.SplitThreshold {
			t.split(c) // depth+1，达到 MaxDepth 后直接返回 -> 叶子内暴力
		}
	}
}

// subtreeCount 统计子树内的唯一对象数（含各内部节点保留的跨界对象）。
func (t *Tree) subtreeCount(n *node) int {
	count := len(n.items)
	if n.children[0] != nil {
		for _, c := range n.children {
			count += t.subtreeCount(c)
		}
	}
	return count
}

// collapse 把整棵子树收回为一个叶子，所有对象上提并重新指定归属。
func (t *Tree) collapse(n *node) {
	ids := make([]int, 0)
	var walk func(*node)
	walk = func(x *node) {
		ids = append(ids, x.items...)
		if x.children[0] != nil {
			for _, c := range x.children {
				walk(c)
			}
		}
	}
	walk(n)

	n.children = [4]*node{}
	n.items = ids
	for _, id := range ids {
		t.owner[id] = n
	}
}

// Detect 返回所有碰撞对，并给出实际比较次数等统计。
//
// 正确性要点：对每个对象 i 用其包围盒做范围查询，只访问包围盒相交的节点，
// 收集其中对象做 AABB 测试，用 stamp 保证同一候选只测一次，且只在 i<j 时测试。
// 若 A、B 相交，则 B 的归属节点（完整包含 B 的最小节点）所在路径上的每个
// 节点包围盒都与 A 相交，因此查询必然沿该路径到达 B，不会漏检。
func (t *Tree) Detect() ([]Pair, Stats) {
	n := len(t.Rects)
	stamp := make([]int, n)
	var pairs []Pair
	comparisons := 0

	for i := 0; i < n; i++ {
		if !t.active[i] {
			continue
		}
		gen := i + 1
		ri := t.Rects[i]

		var visit func(*node)
		visit = func(x *node) {
			for _, j := range x.items {
				if j <= i || !t.active[j] || stamp[j] == gen {
					continue
				}
				stamp[j] = gen
				comparisons++
				if ri.Intersects(t.Rects[j]) {
					pairs = append(pairs, Pair{i, j})
				}
			}
			if x.children[0] != nil {
				for _, c := range x.children {
					if ri.Intersects(c.bounds) {
						visit(c)
					}
				}
			}
		}
		visit(t.root)
	}

	sort.Slice(pairs, func(a, b int) bool {
		if pairs[a].I != pairs[b].I {
			return pairs[a].I < pairs[b].I
		}
		return pairs[a].J < pairs[b].J
	})

	st := t.computeStats(len(pairs), comparisons)
	return pairs, st
}

func (t *Tree) computeStats(pairCount, comparisons int) Stats {
	st := Stats{
		Pairs:            pairCount,
		Comparisons:      comparisons,
		BruteComparisons: 0,
	}
	active := 0
	for _, a := range t.active {
		if a {
			active++
		}
	}
	st.BruteComparisons = active * (active - 1) / 2

	var walk func(*node)
	walk = func(n *node) {
		st.Nodes++
		if n.children[0] != nil {
			st.InternalNodes++
			for _, c := range n.children {
				walk(c)
			}
		} else if n.depth >= t.cfg.MaxDepth && len(n.items) > 0 {
			st.DegradedNodes++
		}
		if len(n.items) > st.MaxOccupancy {
			st.MaxOccupancy = len(n.items)
		}
	}
	walk(t.root)
	return st
}

// Snapshot 导出当前树结构（供回放可视化）。
func (t *Tree) Snapshot() []NodeSnap {
	snaps := make([]NodeSnap, 0, 64)
	var walk func(*node)
	walk = func(n *node) {
		leaf := n.children[0] == nil
		snap := NodeSnap{
			Bounds:   n.bounds,
			Depth:    n.depth,
			Leaf:     leaf,
			Degraded: leaf && n.depth >= t.cfg.MaxDepth && len(n.items) > 0,
		}
		if !leaf || snap.Degraded {
			// 内部节点的 items 即跨边界对象；退化叶子的 items 也需要高亮。
			snap.Items = append(snap.Items, n.items...)
		}
		snaps = append(snaps, snap)
		if !leaf {
			for _, c := range n.children {
				walk(c)
			}
		}
	}
	walk(t.root)
	return snaps
}
