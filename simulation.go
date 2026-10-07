package main

import (
	"math"
	"math/rand"
)

// World 是仿真的基本参数。
type World struct {
	Width  float64
	Height float64
}

// SimConfig 描述运动模拟场景。
type SimConfig struct {
	Frames      int
	Movers      int // 普通匀速运动体（尺寸较大，频繁跨越分割线）
	ClusterObjs int // 高密度簇对象数量
	ClusterTiny int // 其中极小对象数（确保能下降到最大深度触发退化）
	Tree        Config
}

// Object 是仿真对象的运行时状态。
type Object struct {
	Rect    Rect
	VX, VY  float64 // movers 的速度
	Size    float64
	Kind    int     // 0=普通运动体 1=高密度簇
	AnchorX float64 // 簇对象的锚点方向（单位向量）
	AnchorY float64
	AnchorR float64 // 锚点半径系数
}

// Simulation 持有整段运动模拟的状态与逐帧结果。
type Simulation struct {
	world    World
	cfg      SimConfig
	rng      *rand.Rand
	objects  []Object
	tree     *Tree
	frames   []Frame
	verified bool // Run 之后是否全部帧与暴力法一致
}

// Frame 是一帧的完整快照，供回放与校验使用。
type Frame struct {
	Rects []Rect
	Pairs []Pair
	Stats Stats
	Nodes []NodeSnap
	Cross []bool // 该帧中作为跨边界对象存储在内部节点的对象
}

// Mismatch 记录一帧中四叉树与暴力法结果不一致的碰撞对。
type Mismatch struct {
	Frame   int
	Missing []Pair // 四叉树漏报
	Extra   []Pair // 四叉树多报
}

func DefaultSimConfig() SimConfig {
	return SimConfig{
		Frames:      150,
		Movers:      90,
		ClusterObjs: 70,
		ClusterTiny: 40,
		Tree: Config{
			Bounds:         Rect{0, 0, 1000, 1000},
			SplitThreshold: 8,
			MergeThreshold: 6,
			MaxDepth:       7,
		},
	}
}

// NewSimulation 以固定种子构建确定性场景。
func NewSimulation(cfg SimConfig, seed int64) *Simulation {
	rng := rand.New(rand.NewSource(seed))
	w := World{Width: cfg.Tree.Bounds.Width(), Height: cfg.Tree.Bounds.Height()}
	s := &Simulation{world: w, cfg: cfg, rng: rng}

	total := cfg.Movers + cfg.ClusterObjs
	rects := make([]Rect, 0, total)
	for i := 0; i < cfg.Movers; i++ {
		size := 16 + rng.Float64()*26 // 16~42：大尺寸频繁跨越分割线
		r := Rect{
			MinX: rng.Float64() * (w.Width - size),
			MinY: rng.Float64() * (w.Height - size),
		}
		r.MaxX = r.MinX + size
		r.MaxY = r.MinY + size
		speed := 0.6 + rng.Float64()*1.6
		ang := rng.Float64() * 2 * math.Pi
		s.objects = append(s.objects, Object{
			Rect: r, Size: size, Kind: 0,
			VX: math.Cos(ang) * speed,
			VY: math.Sin(ang) * speed,
		})
		rects = append(rects, r)
	}

	cx, cy := w.Width/2, w.Height/2
	for i := 0; i < cfg.ClusterObjs; i++ {
		var size float64
		if i < cfg.ClusterTiny {
			size = 1.6 + rng.Float64()*1.8 // 极小对象，可下降到最大深度
		} else {
			size = 5 + rng.Float64()*4
		}
		ang := rng.Float64() * 2 * math.Pi
		rr := 0.25 + rng.Float64()*0.75
		r := Rect{MinX: cx - size/2, MinY: cy - size/2, MaxX: cx + size/2, MaxY: cy + size/2}
		s.objects = append(s.objects, Object{
			Rect: r, Size: size, Kind: 1,
			AnchorX: math.Cos(ang), AnchorY: math.Sin(ang), AnchorR: rr,
		})
		rects = append(rects, r)
	}

	s.tree = NewTree(cfg.Tree, rects)
	s.tree.InsertAll()
	return s
}

// clusterScale 在模拟中段把簇压缩到极小（触发深度上限退化），
// 首尾散开（触发分裂后的逐层合并）。1 -> ~0 -> 1。
func clusterScale(frame, total int) float64 {
	const pad = 0.03
	t := float64(frame) / float64(total-1)
	return pad + (1-pad)*(0.5+0.5*math.Cos(2*math.Pi*t))
}

// Step 推进一帧：更新所有对象位置（树随之 Remove/Insert，实时分裂合并），
// 然后同时跑四叉树检测与暴力检测。
func (s *Simulation) Step(frame int) {
	w := s.world
	scale := clusterScale(frame, s.cfg.Frames)
	spread := 320.0
	cx, cy := w.Width/2, w.Height/2

	for i := range s.objects {
		o := &s.objects[i]
		var r Rect
		if o.Kind == 0 {
			r = Rect{
				MinX: o.Rect.MinX + o.VX,
				MinY: o.Rect.MinY + o.VY,
				MaxX: o.Rect.MaxX + o.VX,
				MaxY: o.Rect.MaxY + o.VY,
			}
			// 碰墙反弹
			if r.MinX < 0 {
				r.MinX, r.MaxX = 0, o.Size
				o.VX = math.Abs(o.VX)
			} else if r.MaxX > w.Width {
				r.MinX, r.MaxX = w.Width-o.Size, w.Width
				o.VX = -math.Abs(o.VX)
			}
			if r.MinY < 0 {
				r.MinY, r.MaxY = 0, o.Size
				o.VY = math.Abs(o.VY)
			} else if r.MaxY > w.Height {
				r.MinY, r.MaxY = w.Height-o.Size, w.Height
				o.VY = -math.Abs(o.VY)
			}
		} else {
			px := cx + o.AnchorX*o.AnchorR*spread*scale
			py := cy + o.AnchorY*o.AnchorR*spread*scale
			r = Rect{
				MinX: px - o.Size/2, MinY: py - o.Size/2,
				MaxX: px + o.Size/2, MaxY: py + o.Size/2,
			}
		}
		o.Rect = r
		s.tree.Update(i, r)
	}

	pairs, st := s.tree.Detect()
	snaps := s.tree.Snapshot()
	cross := make([]bool, len(s.objects))
	for _, n := range snaps {
		if !n.Leaf {
			for _, id := range n.Items {
				cross[id] = true
			}
		}
	}

	rectsCopy := make([]Rect, len(s.objects))
	copy(rectsCopy, s.tree.Rects)
	s.frames = append(s.frames, Frame{
		Rects: rectsCopy,
		Pairs: append([]Pair(nil), pairs...),
		Stats: st,
		Nodes: snaps,
		Cross: cross,
	})
}

// Run 执行整段模拟，逐帧校验与暴力法一致；返回所有不一致帧。
func (s *Simulation) Run() []Mismatch {
	var mismatches []Mismatch
	for f := 0; f < s.cfg.Frames; f++ {
		s.Step(f)
		fr := &s.frames[f]

		brute, bruteN := BruteForce(fr.Rects, s.tree.active)
		_ = bruteN
		got := PairSet(fr.Pairs)
		want := PairSet(brute)
		var mm Mismatch
		for p := range want {
			if !got[p] {
				mm.Missing = append(mm.Missing, p)
			}
		}
		for p := range got {
			if !want[p] {
				mm.Extra = append(mm.Extra, p)
			}
		}
		if len(mm.Missing)+len(mm.Extra) > 0 {
			mm.Frame = f
			mismatches = append(mismatches, mm)
		}
	}
	return mismatches
}

func (s *Simulation) Frames() []Frame { return s.frames }

// ObjectKinds 返回每个对象的类型（0 普通 / 1 簇），供回放着色。
func (s *Simulation) ObjectKinds() []int {
	kinds := make([]int, len(s.objects))
	for i, o := range s.objects {
		kinds[i] = o.Kind
	}
	return kinds
}
