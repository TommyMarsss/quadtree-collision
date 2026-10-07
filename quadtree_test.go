package main

import (
	"math/rand"
	"testing"
)

var testWorld = Rect{0, 0, WorldW, WorldH}

// buildTree 把对象逐个插入一棵新树。
func buildTree(bodies []*Body, cfg Config) *Quadtree {
	t := NewQuadtree(testWorld, cfg)
	for _, b := range bodies {
		t.Insert(b)
	}
	return t
}

// assertMatchesBrute 断言四叉树结果与暴力法完全一致。
func assertMatchesBrute(t *testing.T, bodies []*Body, cfg Config) {
	t.Helper()
	tree := buildTree(bodies, cfg)
	qp, _ := tree.FindPairs()
	bp, _ := BruteForcePairs(bodies)
	if !SamePairs(qp, bp) {
		t.Fatalf("碰撞对不一致：四叉树 %d 对，暴力法 %d 对\nquad=%v\nbrute=%v",
			len(qp), len(bp), qp, bp)
	}
}

// TestCrossBoundaryMatchesBruteForce 验证跨分割线对象的碰撞检测与暴力法一致：
// 故意把对象压在世界中线（根节点分割线）和象限分割线上。
func TestCrossBoundaryMatchesBruteForce(t *testing.T) {
	cfg := DefaultConfig()
	var bodies []*Body
	id := 0
	add := func(x, y, w, h float64) {
		bodies = append(bodies, &Body{ID: id, Box: Rect{x, y, w, h}})
		id++
	}
	midX, midY := WorldW/2, WorldH/2
	// 横跨垂直中线的对象，与中线两侧的对象都重叠。
	add(midX-10, 100, 20, 20)
	add(midX-28, 105, 20, 20) // 中线左侧，与上一个重叠
	add(midX+8, 105, 20, 20)  // 中线右侧，与第一个重叠
	// 横跨水平中线。
	add(700, midY-8, 24, 16)
	add(700, midY+6, 24, 16)
	// 正好压在象限交叉点上的大对象。
	add(midX-15, midY-15, 30, 30)
	add(midX+5, midY+5, 30, 30)
	// 一些凑数对象触发分裂，让树真正长出子节点。
	for i := 0; i < 20; i++ {
		add(50+float64(i)*8, 50, 6, 6)
	}

	tree := buildTree(bodies, cfg)
	if len(tree.CrossBoundaryIDs()) == 0 {
		t.Fatal("预期存在跨边界对象，但 CrossBoundaryIDs 为空")
	}
	assertMatchesBrute(t, bodies, cfg)

	// 精确校验：横跨中线的 0 号对象必须与 1、2 号都碰撞（不能漏报跨界对）。
	qp, _ := tree.FindPairs()
	want := map[[2]int]bool{{0, 1}: true, {0, 2}: true}
	got := map[[2]int]bool{}
	for _, p := range qp {
		got[p] = true
	}
	for p := range want {
		if !got[p] {
			t.Fatalf("漏检跨界碰撞对 %v", p)
		}
	}
}

// TestHighDensityDegenerates 验证高密度场景：大量对象挤在极小区域内，
// 树分裂到深度上限后停止（不栈溢出、不无限分裂），结果仍与暴力法一致。
func TestHighDensityDegenerates(t *testing.T) {
	cfg := DefaultConfig()
	rng := rand.New(rand.NewSource(7))
	var bodies []*Body
	for i := 0; i < 500; i++ {
		w := 2 + rng.Float64()*4
		h := 2 + rng.Float64()*4
		bodies = append(bodies, &Body{
			ID:  i,
			Box: Rect{100 + rng.Float64()*40, 100 + rng.Float64()*40, w, h},
		})
	}
	tree := buildTree(bodies, cfg)
	if d := tree.MaxDepthUsed(); d != cfg.MaxDepth {
		t.Fatalf("高密度场景应分裂到深度上限 %d，实际 %d", cfg.MaxDepth, d)
	}
	assertMatchesBrute(t, bodies, cfg)
}

// TestRandomDifferential 差分测试：多个种子、随机尺寸与分布，
// 四叉树碰撞对集合必须与暴力法完全一致。
func TestRandomDifferential(t *testing.T) {
	cfg := DefaultConfig()
	for seed := int64(0); seed < 20; seed++ {
		rng := rand.New(rand.NewSource(seed))
		n := 50 + rng.Intn(150)
		var bodies []*Body
		for i := 0; i < n; i++ {
			// 尺寸最大 80px，保证相当比例的对象跨越分割线。
			w := 4 + rng.Float64()*76
			h := 4 + rng.Float64()*76
			bodies = append(bodies, &Body{
				ID:  i,
				Box: Rect{rng.Float64() * (WorldW - w), rng.Float64() * (WorldH - h), w, h},
			})
		}
		assertMatchesBrute(t, bodies, cfg)
	}
}

// TestSplitMergeThresholds 验证分裂/合并的阈值切换时机：
// 对象数 > Capacity 时分裂；子树总数降到 MergeThreshold 时合并；
// 两者之间（迟滞区间）保持现状。
func TestSplitMergeThresholds(t *testing.T) {
	cfg := Config{Capacity: 8, MergeThreshold: 4, MaxDepth: 6}
	// 所有对象都放在左上角小区域，保证它们始终落在同一棵子树里。
	mk := func(i int) *Body {
		return &Body{ID: i, Box: Rect{10 + float64(i%4)*6, 10 + float64(i/4)*6, 4, 4}}
	}
	tree := NewQuadtree(testWorld, cfg)
	var bodies []*Body
	for i := 0; i < cfg.Capacity; i++ {
		b := mk(i)
		bodies = append(bodies, b)
		tree.Insert(b)
	}
	if tree.children[0] != nil {
		t.Fatalf("对象数等于 Capacity=%d 时不应分裂", cfg.Capacity)
	}

	// 第 Capacity+1 个对象触发分裂。
	b := mk(cfg.Capacity)
	bodies = append(bodies, b)
	tree.Insert(b)
	if tree.children[0] == nil {
		t.Fatalf("对象数超过 Capacity=%d 时应分裂", cfg.Capacity)
	}

	// 逐个移除：总数降到 MergeThreshold+1 时仍应保持分裂（迟滞），
	// 降到 MergeThreshold 时必须合并。
	total := cfg.Capacity + 1
	removed := 0
	for total > cfg.MergeThreshold+1 {
		tree.Remove(bodies[removed].ID)
		removed++
		total--
		if tree.children[0] == nil {
			t.Fatalf("总数 %d 仍大于 MergeThreshold=%d，不应合并", total, cfg.MergeThreshold)
		}
	}
	tree.Remove(bodies[removed].ID)
	total--
	if total != cfg.MergeThreshold {
		t.Fatalf("测试内部计数错误：total=%d", total)
	}
	if tree.children[0] != nil {
		t.Fatalf("总数降到 MergeThreshold=%d 时应合并", cfg.MergeThreshold)
	}
	if got := tree.subtreeCount(); got != cfg.MergeThreshold {
		t.Fatalf("合并后对象丢失：期望 %d，实际 %d", cfg.MergeThreshold, got)
	}
}

// TestComparisonReduction 验证均匀分布场景下四叉树确实减少了比较次数。
func TestComparisonReduction(t *testing.T) {
	cfg := DefaultConfig()
	rng := rand.New(rand.NewSource(99))
	var bodies []*Body
	for i := 0; i < 200; i++ {
		w := 8 + rng.Float64()*12
		h := 8 + rng.Float64()*12
		bodies = append(bodies, &Body{
			ID:  i,
			Box: Rect{rng.Float64() * (WorldW - w), rng.Float64() * (WorldH - h), w, h},
		})
	}
	tree := buildTree(bodies, cfg)
	_, qc := tree.FindPairs()
	_, bc := BruteForcePairs(bodies)
	if qc >= bc {
		t.Fatalf("均匀分布下四叉树比较次数 %d 应显著小于暴力法 %d", qc, bc)
	}
	t.Logf("比较次数：四叉树 %d vs 暴力 %d（减少 %.1f%%）",
		qc, bc, (1-float64(qc)/float64(bc))*100)
}

// TestSimulationConsistency 对模拟器本身做整体验证（Simulate 内部每帧已断言，
// 这里再确认帧数与比较次数统计合理）。
func TestSimulationConsistency(t *testing.T) {
	frames := Simulate(1, 30)
	if len(frames) != 30 {
		t.Fatalf("期望 30 帧，实际 %d", len(frames))
	}
	for i, f := range frames {
		if f.BruteC != len(f.Bodies)*(len(f.Bodies)-1)/2 {
			t.Fatalf("帧 %d：暴力比较次数应为 n(n-1)/2", i)
		}
	}
}
