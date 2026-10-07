package main

import "testing"

// TestHighDensityDegradation 二：大量对象聚集在远小于最深层单元的范围内时，
// 树在 MaxDepth 处停止分裂并退化为节点内暴力比较，不崩溃、不漏检。
func TestHighDensityDegradation(t *testing.T) {
	const n = 400
	const maxDepth = 7
	cfg := Config{
		Bounds:         Rect{0, 0, 1000, 1000},
		SplitThreshold: 8,
		MergeThreshold: 6,
		MaxDepth:       maxDepth,
	}
	rects := make([]Rect, n)
	// 全部塞进一个 2×2 的区域：最深层单元约 7.8 宽，无法再有意义地下放。
	for i := range rects {
		x := 499.0 + float64(i%20)*0.09
		y := 499.0 + float64(i/20)*0.09
		s := 0.07
		rects[i] = Rect{x, y, x + s, y + s}
	}
	tree := NewTree(cfg, rects)
	tree.InsertAll()

	// 结构断言：存在达到 MaxDepth 的退化叶子，且树深度绝不超过 MaxDepth。
	var maxSeen int
	degraded := 0
	var walk func(*node)
	walk = func(x *node) {
		if x.depth > maxSeen {
			maxSeen = x.depth
		}
		if x.depth > maxDepth {
			t.Fatalf("节点深度 %d 超过上限 %d（无限分裂）", x.depth, maxDepth)
		}
		if x.children[0] == nil && x.depth == maxDepth && len(x.items) > 0 {
			degraded++
			if len(x.items) <= cfg.SplitThreshold {
				t.Fatalf("退化叶子只含 %d 个对象，说明并非因深度上限而停止", len(x.items))
			}
		}
		if x.children[0] != nil {
			for _, c := range x.children {
				walk(c)
			}
		}
	}
	walk(tree.root)
	if degraded == 0 {
		t.Fatal("高密度聚集未产生任何深度上限退化叶子")
	}
	if maxSeen != maxDepth {
		t.Fatalf("最大深度 = %d，期望 %d", maxSeen, maxDepth)
	}

	// 黄金标准：退化节点内暴力比较，结果仍须与全局暴力法一致。
	assertNoDiff(t, tree, "高密度退化")

	// 高密度下四叉树比较次数允许接近甚至高于暴力（退化是有意为之），
	// 但时间必须可承受（正确性优先），这里只验证结果一致性与深度约束。
	_, st := tree.Detect()
	if st.BruteComparisons != n*(n-1)/2 {
		t.Fatalf("暴力基线比较次数 %d 错误", st.BruteComparisons)
	}
	if st.DegradedNodes != degraded {
		t.Fatalf("统计退化节点数 %d 与结构扫描 %d 不一致", st.DegradedNodes, degraded)
	}
}

// TestHighDensityThenDisperse 簇散开后，退化节点必须能逐层合并恢复。
func TestHighDensityThenDisperse(t *testing.T) {
	cfg := Config{
		Bounds:         Rect{0, 0, 100, 100},
		SplitThreshold: 4, MergeThreshold: 3, MaxDepth: 4,
	}
	tree := NewTree(cfg, nil)
	n := 60
	ids := make([]int, n)
	for i := 0; i < n; i++ {
		x := 49.5 + float64(i%8)*0.05
		y := 49.5 + float64(i/8)*0.05
		ids[i] = tree.Add(Rect{x, y, x + 0.04, y + 0.04})
	}
	if tree.root.depth != 0 {
		t.Fatal("sanity")
	}
	_, before := tree.Detect()
	if before.DegradedNodes == 0 {
		t.Fatal("聚集状态应当出现退化叶子")
	}

	// 散开：每个对象移动到相距很远的位置（移动触发 Remove -> 合并）。
	for i, id := range ids {
		col := i % 8
		row := i / 8
		x := 2.0 + float64(col)*12
		y := 2.0 + float64(row)*12
		tree.Update(id, Rect{x, y, x + 0.04, y + 0.04})
	}
	assertNoDiff(t, tree, "散开后")
	_, after := tree.Detect()
	if after.DegradedNodes != 0 {
		t.Fatalf("散开后仍存在 %d 个退化叶子，合并不足", after.DegradedNodes)
	}
}

// TestZeroSizedNodeGuard 聚集到浮点尺度耗尽时（节点宽度趋零）也不能无限递归。
func TestZeroSizedNodeGuard(t *testing.T) {
	cfg := Config{
		Bounds:         Rect{0, 0, 1, 1},
		SplitThreshold: 2, MergeThreshold: 1, MaxDepth: 60,
	}
	tree := NewTree(cfg, nil)
	for i := 0; i < 50; i++ {
		// 全部放在完全相同的极小范围，中点细分约 50 次后宽度在 float64 下耗尽。
		x := 0.5 + float64(i)*1e-18
		tree.Add(Rect{x, x, x + 1e-18, x + 1e-18})
	}
	assertNoDiff(t, tree, "尺度耗尽")
}
