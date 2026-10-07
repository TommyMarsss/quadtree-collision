package main

import (
	"math/rand"
	"testing"
)

// newTestTree 构造常用配置的树。
func newTestTree(rects []Rect) *Tree {
	t := NewTree(Config{
		Bounds:         Rect{0, 0, 100, 100},
		SplitThreshold: 4,
		MergeThreshold: 3,
		MaxDepth:       5,
	}, rects)
	t.InsertAll()
	return t
}

// diffPairs 返回四叉树相对暴力法的漏报与多报。
func diffPairs(t *testing.T, tree *Tree) (missing, extra []Pair) {
	t.Helper()
	got, _ := tree.Detect()
	want, _ := BruteForce(tree.Rects, tree.active)
	g, w := PairSet(got), PairSet(want)
	for p := range w {
		if !g[p] {
			missing = append(missing, p)
		}
	}
	for p := range g {
		if !w[p] {
			extra = append(extra, p)
		}
	}
	return missing, extra
}

func assertNoDiff(t *testing.T, tree *Tree, name string) {
	t.Helper()
	missing, extra := diffPairs(t, tree)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("%s: 与暴力法不一致：漏报 %v 多报 %v", name, missing, extra)
	}
}

// TestCrossBoundary 一：跨越分割线的对象（面积对象）不会漏检跨界碰撞。
func TestCrossBoundary(t *testing.T) {
	bounds := Rect{0, 0, 100, 100}
	// 中线在 (50,50)：big 同时跨越两条分割线，宽对象跨越竖线，高对象跨越横线。
	rects := []Rect{
		{40, 40, 60, 60}, // 0: 跨两条中线
		{55, 55, 65, 65}, // 1: BR 象限，与 0 相交
		{30, 55, 45, 65}, // 2: BL 象限，与 0 相交
		{55, 30, 65, 45}, // 3: TR 象限，与 0 相交
		{30, 30, 45, 45}, // 4: TL 象限，与 0 相交
		{48, 10, 52, 40}, // 5: 跨竖线的高瘦对象（恰好与 0 在 y=40 相切）
		{10, 48, 40, 52}, // 6: 跨横线的宽扁对象（恰好与 0 在 x=40 相切）
		{48, 70, 52, 90}, // 7: 跨竖线，不与 0 相交
	}
	tree := newTestTree(rects)
	_ = bounds

	pairs, _ := tree.Detect()
	ps := PairSet(pairs)

	// 跨界对象 0 必须能与四个象限内的对象全部配对。
	for _, j := range []int{1, 2, 3, 4} {
		if !ps[Pair{0, j}] {
			t.Errorf("跨两条中线的对象漏检与象限内对象 %d 的碰撞", j)
		}
	}
	// 相切也算碰撞（闭区间语义，四叉树与暴力法一致）。
	if !ps[Pair{0, 5}] || !ps[Pair{0, 6}] {
		t.Errorf("跨界对象边界相切的碰撞漏检")
	}
	// 不相交的跨界对象不能多报。
	if ps[Pair{0, 7}] || ps[Pair{5, 7}] {
		t.Errorf("多报了不相交的跨界对象对")
	}

	// 黄金标准：整体集合与暴力法完全一致。
	assertNoDiff(t, tree, "跨界场景")
}

// TestCrossBoundaryAcrossDepths 验证在更深层级跨越分割线同样正确。
func TestCrossBoundaryAcrossDepths(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	rects := make([]Rect, 0, 200)
	// 放置大量跨多级分割线的对象：随机中心点 + 较大尺寸。
	for i := 0; i < 120; i++ {
		size := 8 + rng.Float64()*14
		x := rng.Float64() * (100 - size)
		y := rng.Float64() * (100 - size)
		rects = append(rects, Rect{x, y, x + size, y + size})
	}
	tree := newTestTree(rects)
	assertNoDiff(t, tree, "多级跨界")
	if tree.root.children[0] == nil {
		t.Fatal("场景应当触发了分裂")
	}
}

// TestBoundaryOwnershipDeterminism 恰好落在分割线上的对象只归入一个子象限，
// 不能被复制到多个子节点（owner 唯一）。
func TestBoundaryOwnershipDeterminism(t *testing.T) {
	rects := []Rect{
		{50, 10, 60, 20}, // MinX == midX，归右侧
		{40, 50, 50, 60}, // MaxX == midX，归左侧；MaxY? MinY==midY 归下
		{50, 50, 60, 60}, // 角落点落在正中，归 BR
		{40, 40, 50, 50}, // 右上角点落在正中，归 TL
		{5, 5, 9, 9},     // 触发分裂的 TL 小对象
	}
	tree := newTestTree(rects)
	if tree.root.children[0] == nil {
		t.Fatal("应当已分裂")
	}
	for id, n := range tree.owner {
		// 每个对象在树中只出现一次：统计全树出现次数。
		count := 0
		var walk func(*node)
		walk = func(x *node) {
			for _, v := range x.items {
				if v == id {
					count++
				}
			}
			if x.children[0] != nil {
				for _, c := range x.children {
					walk(c)
				}
			}
		}
		walk(tree.root)
		if count != 1 {
			t.Errorf("对象 %d 在树中出现 %d 次（归属节点 %v），应为 1", id, count, n)
		}
	}
	assertNoDiff(t, tree, "贴线归属")
}

// TestOutOfRootBounds 超出根边界的对象常驻根节点，仍参与全部碰撞。
func TestOutOfRootBounds(t *testing.T) {
	rects := []Rect{
		{-20, -20, 20, 20}, // 越出左上
		{10, 10, 30, 30},   // 根内，与 0 相交
		{90, 90, 130, 130}, // 越出右下
		{80, 80, 95, 95},   // 根内，与 2 相交
		{-50, 60, 10, 90},  // 越出左侧的宽对象
		{5, 65, 15, 85},    // 与 4 相交
	}
	tree := newTestTree(rects)
	assertNoDiff(t, tree, "越界对象")
}
