package main

import (
	"math/rand"
	"sort"
	"testing"
)

// TestRandomDifferential 三：多种随机分布下四叉树碰撞对集合与暴力法完全一致。
// 覆盖均匀分布、高重叠（大尺寸）、簇分布、边界对齐等分布形态。
func TestRandomDifferential(t *testing.T) {
	distros := []struct {
		name string
		make func(rng *rand.Rand, i, n int) Rect
	}{
		{"uniform", func(rng *rand.Rand, _, n int) Rect {
			s := 2 + rng.Float64()*6
			x := rng.Float64() * (100 - s)
			y := rng.Float64() * (100 - s)
			return Rect{x, y, x + s, y + s}
		}},
		{"heavy-overlap", func(rng *rand.Rand, _, n int) Rect {
			// 大尺寸对象，几乎人人跨界、高重叠率。
			s := 15 + rng.Float64()*20
			x := rng.Float64() * (100 - s)
			y := rng.Float64() * (100 - s)
			return Rect{x, y, x + s, y + s}
		}},
		{"clusters", func(rng *rand.Rand, i, n int) Rect {
			cx := []float64{20, 80, 20, 80}[i%4]
			cy := []float64{20, 20, 80, 80}[i%4]
			s := 1 + rng.Float64()*3
			x := cx - 6 + rng.Float64()*12
			y := cy - 6 + rng.Float64()*12
			return Rect{x, y, x + s, y + s}
		}},
		{"grid-snapped", func(rng *rand.Rand, i, n int) Rect {
			// 大量对象边界恰好落在 25/50/75 等各级分割线上。
			snaps := []float64{0, 12.5, 25, 50, 75, 87.5}
			x0 := snaps[rng.Intn(5)]
			y0 := snaps[rng.Intn(5)]
			s := snaps[1+rng.Intn(2)]
			return Rect{x0, y0, x0 + s, y0 + s}
		}},
		{"mixed-scale", func(rng *rand.Rand, _, n int) Rect {
			var s float64
			switch rng.Intn(3) {
			case 0:
				s = 0.5 + rng.Float64()*1.5 // 极小
			case 1:
				s = 3 + rng.Float64()*5
			default:
				s = 12 + rng.Float64()*18 // 极大
			}
			x := rng.Float64() * (100 - s)
			y := rng.Float64() * (100 - s)
			return Rect{x, y, x + s, y + s}
		}},
	}

	for seed := int64(1); seed <= 40; seed++ {
		for _, d := range distros {
			rng := rand.New(rand.NewSource(seed * 1000))
			n := 1 + rng.Intn(150)
			rects := make([]Rect, n)
			for i := 0; i < n; i++ {
				rects[i] = d.make(rng, i, n)
			}
			tree := newTestTree(rects)
			missing, extra := diffPairs(t, tree)
			if len(missing)+len(extra) > 0 {
				t.Fatalf("seed=%d distro=%s n=%d 漏报=%v 多报=%v",
					seed, d.name, n, missing[:min(3, len(missing))], extra[:min(3, len(extra))])
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestRandomDynamic 随机增删与移动的差分测试：每一步操作后树结果必须与暴力法一致。
func TestRandomDynamic(t *testing.T) {
	for seed := int64(100); seed < 120; seed++ {
		rng := rand.New(rand.NewSource(seed))
		cfg := Config{
			Bounds:         Rect{0, 0, 100, 100},
			SplitThreshold: 6, MergeThreshold: 4, MaxDepth: 6,
		}
		tree := NewTree(cfg, nil)
		var live []int

		randRect := func() Rect {
			s := 1 + rng.Float64()*18
			x := -5 + rng.Float64()*(110-s) // 允许部分越出根边界
			y := -5 + rng.Float64()*(110-s)
			return Rect{x, y, x + s, y + s}
		}

		for step := 0; step < 400; step++ {
			switch rng.Intn(10) {
			case 0, 1, 2, 3, 4: // 50% 添加
				live = append(live, tree.Add(randRect()))
			case 5, 6: // 20% 移动
				if len(live) > 0 {
					id := live[rng.Intn(len(live))]
					tree.Update(id, randRect())
				}
			case 7, 8: // 20% 删除
				if len(live) > 0 {
					k := rng.Intn(len(live))
					tree.Remove(live[k])
					live = append(live[:k], live[k+1:]...)
				}
			default: // 10% 同一对象更新到原位（幂等）
				if len(live) > 0 {
					id := live[rng.Intn(len(live))]
					tree.Update(id, tree.Rects[id])
				}
			}

			if step%3 == 0 {
				got, _ := tree.Detect()
				want, _ := BruteForce(tree.Rects, tree.active)
				if !pairsEqual(got, want) {
					t.Fatalf("seed=%d step=%d live=%d: 碰撞对集合不一致", seed, step, len(live))
				}
			}
		}
	}
}

func pairsEqual(a, b []Pair) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Slice(a, func(i, j int) bool {
		if a[i].I != a[j].I {
			return a[i].I < a[j].I
		}
		return a[i].J < a[j].J
	})
	sort.Slice(b, func(i, j int) bool {
		if b[i].I != b[j].I {
			return b[i].I < b[j].I
		}
		return b[i].J < b[j].J
	})
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestComparisonReduction 统计比较次数减少比例：均匀稀疏场景下必须显著少于暴力法。
func TestComparisonReduction(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	n := 300
	rects := make([]Rect, n)
	for i := range rects {
		s := 2 + rng.Float64()*3
		x := rng.Float64() * (100 - s)
		y := rng.Float64() * (100 - s)
		rects[i] = Rect{x, y, x + s, y + s}
	}
	tree := newTestTree(rects)
	_, st := tree.Detect()
	if st.Comparisons >= st.BruteComparisons {
		t.Fatalf("稀疏均匀分布下四叉树比较 %d 未少于暴力 %d", st.Comparisons, st.BruteComparisons)
	}
	reduction := 1 - float64(st.Comparisons)/float64(st.BruteComparisons)
	t.Logf("n=%d: 四叉树 %d 次 vs 暴力 %d 次，减少 %.1f%%",
		n, st.Comparisons, st.BruteComparisons, reduction*100)
	if reduction < 0.5 {
		t.Fatalf("比较次数仅减少 %.1f%%，分区效果异常", reduction*100)
	}
}
