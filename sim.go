package main

import (
	"fmt"
	"math/rand"
)

// 世界尺寸（像素），与 HTML 画布坐标一致。
const (
	WorldW = 960.0
	WorldH = 640.0
)

// BodyState 是一帧中单个碰撞体的快照。
type BodyState struct {
	ID int     `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	W  float64 `json:"w"`
	H  float64 `json:"h"`
}

// NodeState 是一帧中一个四叉树节点的快照。
type NodeState struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Depth int     `json:"d"`
	Count int     `json:"c"`
}

// Frame 是一帧的完整录制：对象位置、碰撞对、跨边界对象、树结构与统计。
type Frame struct {
	Bodies []BodyState `json:"bodies"`
	Pairs  [][2]int    `json:"pairs"`
	Cross  []int       `json:"cross"`
	Nodes  []NodeState `json:"nodes"`
	QuadC  int         `json:"qc"` // 四叉树实际比较次数
	BruteC int         `json:"bc"` // 暴力法比较次数
	MaxDep int         `json:"md"`
}

// Simulate 运行 frames 帧运动模拟并逐帧录制。
// 场景包含两类对象：均匀分布的自由对象 + 一个高密度移动集群，
// 用于同时展示空间分区的加速效果与深度上限下的退化行为。
// 每一帧都会校验四叉树结果与暴力法完全一致，不一致则 panic。
func Simulate(seed int64, frames int) []Frame {
	rng := rand.New(rand.NewSource(seed))
	bodies := spawnBodies(rng)
	cfg := DefaultConfig()
	world := Rect{0, 0, WorldW, WorldH}

	out := make([]Frame, 0, frames)
	for f := 0; f < frames; f++ {
		step(bodies, rng)

		tree := NewQuadtree(world, cfg)
		for _, b := range bodies {
			tree.Insert(b)
		}
		pairs, qc := tree.FindPairs()
		brutePairs, bc := BruteForcePairs(bodies)
		if !SamePairs(pairs, brutePairs) {
			panic(fmt.Sprintf("frame %d: quadtree/brute mismatch: quad=%v brute=%v", f, pairs, brutePairs))
		}

		fr := Frame{
			Pairs:  pairs,
			Cross:  tree.CrossBoundaryIDs(),
			QuadC:  qc,
			BruteC: bc,
			MaxDep: tree.MaxDepthUsed(),
		}
		if fr.Pairs == nil {
			fr.Pairs = [][2]int{}
		}
		if fr.Cross == nil {
			fr.Cross = []int{}
		}
		for _, b := range bodies {
			fr.Bodies = append(fr.Bodies, BodyState{ID: b.ID, X: b.Box.X, Y: b.Box.Y, W: b.Box.W, H: b.Box.H})
		}
		for _, n := range tree.Nodes() {
			fr.Nodes = append(fr.Nodes, NodeState{
				X: n.Bounds.X, Y: n.Bounds.Y, W: n.Bounds.W, H: n.Bounds.H,
				Depth: n.Depth, Count: n.Count,
			})
		}
		out = append(out, fr)
	}
	return out
}

// spawnBodies 生成初始对象：90 个均匀分布 + 30 个高密度集群。
func spawnBodies(rng *rand.Rand) []*Body {
	var bodies []*Body
	id := 0
	add := func(x, y, w, h, vx, vy float64) {
		bodies = append(bodies, &Body{ID: id, Box: Rect{x, y, w, h}, VX: vx, VY: vy})
		id++
	}
	// 均匀分布的自由对象。
	for i := 0; i < 90; i++ {
		w := 8 + rng.Float64()*16
		h := 8 + rng.Float64()*16
		add(rng.Float64()*(WorldW-w), rng.Float64()*(WorldH-h), w, h,
			(rng.Float64()-0.5)*3, (rng.Float64()-0.5)*3)
	}
	// 高密度集群：挤在小区域内整体漂移，触发深度上限退化。
	// 16 个分布在 140x140 区域 + 24 个压在 24x18 的核心区（必然撞满深度上限）。
	cx, cy := WorldW*0.3, WorldH*0.4
	for i := 0; i < 16; i++ {
		w := 6 + rng.Float64()*8
		h := 6 + rng.Float64()*8
		add(cx+rng.Float64()*140, cy+rng.Float64()*140, w, h,
			0.6+(rng.Float64()-0.5)*0.8, 0.4+(rng.Float64()-0.5)*0.8)
	}
	for i := 0; i < 24; i++ {
		w := 4 + rng.Float64()*6
		h := 4 + rng.Float64()*6
		add(cx+40+rng.Float64()*24, cy+40+rng.Float64()*18, w, h,
			0.6+(rng.Float64()-0.5)*0.6, 0.4+(rng.Float64()-0.5)*0.6)
	}
	return bodies
}

// step 推进一帧：匀速运动 + 边界反弹。
func step(bodies []*Body, rng *rand.Rand) {
	for _, b := range bodies {
		b.Box.X += b.VX
		b.Box.Y += b.VY
		if b.Box.X < 0 {
			b.Box.X = 0
			b.VX = -b.VX
		}
		if b.Box.Right() > WorldW {
			b.Box.X = WorldW - b.Box.W
			b.VX = -b.VX
		}
		if b.Box.Y < 0 {
			b.Box.Y = 0
			b.VY = -b.VY
		}
		if b.Box.Bottom() > WorldH {
			b.Box.Y = WorldH - b.Box.H
			b.VY = -b.VY
		}
	}
}
