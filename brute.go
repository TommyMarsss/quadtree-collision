package main

import "sort"

// BruteForcePairs 是 O(n²) 暴力两两比较，作为正确性的黄金标准。
// 返回排序后的碰撞对与执行的比较次数（恒为 n*(n-1)/2）。
func BruteForcePairs(bodies []*Body) (pairs [][2]int, comparisons int) {
	for i := 0; i < len(bodies); i++ {
		for j := i + 1; j < len(bodies); j++ {
			comparisons++
			a, b := bodies[i], bodies[j]
			if a.Box.Intersects(b.Box) {
				lo, hi := a.ID, b.ID
				if lo > hi {
					lo, hi = hi, lo
				}
				pairs = append(pairs, [2]int{lo, hi})
			}
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i][0] != pairs[j][0] {
			return pairs[i][0] < pairs[j][0]
		}
		return pairs[i][1] < pairs[j][1]
	})
	return pairs, comparisons
}

// SamePairs 比较两个碰撞对集合是否完全一致（双方都必须已排序）。
func SamePairs(a, b [][2]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
