package main

import "testing"

func countTree(tree *Tree) (nodes, internal int, rootLeaf bool) {
	rootLeaf = tree.root.children[0] == nil
	var walk func(*node)
	walk = func(n *node) {
		nodes++
		if n.children[0] != nil {
			internal++
			for _, c := range n.children {
				walk(c)
			}
		}
	}
	walk(tree.root)
	return
}

// TestSplitMergeThresholds 四：精确验证分裂/合并阈值的切换时机。
func TestSplitMergeThresholds(t *testing.T) {
	cfg := Config{
		Bounds:         Rect{0, 0, 1000, 1000},
		SplitThreshold: 4, MergeThreshold: 2, MaxDepth: 6,
	}
	tree := NewTree(cfg, nil)

	// 用足够小、彼此分散的对象保证它们全部落入同一象限路径，
	// 从而在一个叶子上累计对象数（角落 TL 区域内再聚集）。
	mk := func(i int) Rect {
		// 放在 (10,10) 附近极小范围，沿同一路径下行。
		x := 10 + float64(i)*0.4
		y := 10 + float64(i)*0.4
		return Rect{x, y, x + 0.2, y + 0.2}
	}

	// 1) 恰好等于阈值：不分裂（条件是 count > SplitThreshold）。
	for i := 0; i < cfg.SplitThreshold; i++ {
		tree.Add(mk(i))
	}
	if n, internal, leaf := countTree(tree); !leaf || internal != 0 || n != 1 {
		t.Fatalf("count == SplitThreshold 时不应分裂：nodes=%d internal=%d", n, internal)
	}

	// 2) 超过阈值 1 个：立即分裂。
	tree.Add(mk(cfg.SplitThreshold))
	if tree.root.children[0] == nil {
		t.Fatal("count = SplitThreshold+1 时必须分裂")
	}

	// 3) 对象继续增加不会出错（与暴力法一致）。
	for i := cfg.SplitThreshold + 1; i < 20; i++ {
		tree.Add(mk(i))
		assertNoDiff(t, tree, "增长中")
	}

	// 4) 逐个删除直到子树对象数 <= MergeThreshold，必须合并回叶子。
	// 直接按 id 全量删除，边删边验证一致性；最终必须只剩根叶子。
	for id := 19; id >= 0; id-- {
		tree.Remove(id)
		if id > 0 {
			// 仍有对象时与暴力法一致（active 由 Remove 维护）。
			got, _ := tree.Detect()
			want, _ := BruteForce(tree.Rects, tree.active)
			if len(got) != len(want) {
				t.Fatalf("删除到 id=%d 时碰撞对数不一致", id)
			}
		}
	}
	if n, internal, leaf := countTree(tree); !leaf || n != 1 {
		t.Fatalf("清空后应只剩根叶子：nodes=%d internal=%d", n, internal)
	}
}

// TestMergeHysteresis 迟滞区间 (MergeThreshold, SplitThreshold] 内不抖动：
// 对象数降回分裂阈值但仍高于合并阈值时保持已分裂结构。
func TestMergeHysteresis(t *testing.T) {
	cfg := Config{
		Bounds:         Rect{0, 0, 1000, 1000},
		SplitThreshold: 4, MergeThreshold: 2, MaxDepth: 6,
	}
	tree := NewTree(cfg, nil)
	mk := func(i int) Rect {
		x := 10 + float64(i)*0.3
		y := 10 + float64(i)*0.3
		return Rect{x, y, x + 0.15, y + 0.15}
	}
	for i := 0; i < 8; i++ {
		tree.Add(mk(i))
	}
	splitOnce := tree.root.children[0] != nil
	if !splitOnce {
		t.Fatal("8 个对象应已分裂")
	}

	// 删到只剩 4 个（== SplitThreshold，但 > MergeThreshold=2）：不应合并。
	for id := 7; id >= 4; id-- {
		tree.Remove(id)
	}
	if tree.root.children[0] == nil {
		t.Fatal("对象数处于迟滞区间 (merge, split] 时不应合并")
	}

	// 再删到 2 个（<= MergeThreshold）：合并。
	tree.Remove(3)
	tree.Remove(2)
	if tree.root.children[0] != nil {
		t.Fatal("对象数 <= MergeThreshold 时必须合并")
	}
	assertNoDiff(t, tree, "迟滞")
}

// TestSplitRedistributesCrossers 分裂时跨界对象上提保留，象限对象下放。
func TestSplitRedistributesCrossers(t *testing.T) {
	cfg := Config{
		Bounds:         Rect{0, 0, 100, 100},
		SplitThreshold: 2, MergeThreshold: 1, MaxDepth: 5,
	}
	tree := NewTree(cfg, nil)
	// 两个触发分裂的小对象在 TL，加一个跨中线的大对象。
	tree.Add(Rect{10, 10, 12, 12})
	tree.Add(Rect{14, 14, 16, 16})
	tree.Add(Rect{40, 40, 60, 60}) // 跨界，应留在根
	if tree.root.children[0] == nil {
		t.Fatal("应已分裂")
	}
	foundCross := false
	for _, id := range tree.root.items {
		if id == 2 {
			foundCross = true
		}
	}
	if !foundCross {
		t.Fatal("跨界对象在分裂后必须保留在内部节点")
	}
	// 子象限中的小对象归属必须下放到子节点。
	if tree.owner[0] == tree.root || tree.owner[1] == tree.root {
		t.Fatal("完整落入子象限的对象必须下放")
	}
	assertNoDiff(t, tree, "分裂再分配")
}
