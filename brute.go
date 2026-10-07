package main

// BruteForce 是黄金标准：对所有活跃对象做 O(n²) 两两 AABB 比较。
// 返回的碰撞对经过排序，与 Tree.Detect 的输出可直接按集合比较。
func BruteForce(rects []Rect, active []bool) ([]Pair, int) {
	var ids []int
	for i, a := range active {
		if a {
			ids = append(ids, i)
		}
	}
	var pairs []Pair
	comparisons := 0
	for a := 0; a < len(ids); a++ {
		i := ids[a]
		for b := a + 1; b < len(ids); b++ {
			j := ids[b]
			comparisons++
			if rects[i].Intersects(rects[j]) {
				pairs = append(pairs, Pair{i, j})
			}
		}
	}
	return pairs, comparisons
}

// PairSet 把碰撞对切片转为集合，便于差分测试。
func PairSet(pairs []Pair) map[Pair]bool {
	m := make(map[Pair]bool, len(pairs))
	for _, p := range pairs {
		m[p] = true
	}
	return m
}
