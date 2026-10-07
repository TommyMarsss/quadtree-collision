package main

import (
	"encoding/json"
	"math"
	"os"
	"strings"
)

type statsJSON struct {
	Cmp  int `json:"cmp"`
	Brut int `json:"bru"`
	Node int `json:"nod"`
	Deg  int `json:"deg"`
	Occ  int `json:"occ"`
}

type nodeJSON struct {
	B []float64 `json:"b"`
	D int       `json:"d"`
	G bool      `json:"g"` // degraded
	I []int     `json:"i,omitempty"`
}

type frameJSON struct {
	R []float64  `json:"r"`
	P []int      `json:"p"`
	X []int      `json:"x"`
	N []nodeJSON `json:"n"`
	S statsJSON  `json:"s"`
}

type reportData struct {
	W        float64     `json:"w"`
	H        float64     `json:"h"`
	Kinds    []int       `json:"k"`
	Cfg      interface{} `json:"cfg"`
	Verified bool        `json:"v"`
	Frames   []frameJSON `json:"f"`
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// WriteReport 把整段模拟渲染为单一静态 HTML：数据与逻辑全部内嵌，
// 不依赖任何第三方库，浏览器直接打开即可逐帧回放。
func WriteReport(path string, sim *Simulation) error {
	frames := sim.Frames()
	kinds := sim.ObjectKinds()

	data := reportData{
		W:        sim.world.Width,
		H:        sim.world.Height,
		Kinds:    kinds,
		Verified: sim.verified,
		Cfg: map[string]int{
			"split": sim.cfg.Tree.SplitThreshold,
			"merge": sim.cfg.Tree.MergeThreshold,
			"depth": sim.cfg.Tree.MaxDepth,
		},
		Frames: make([]frameJSON, 0, len(frames)),
	}

	for _, f := range frames {
		fj := frameJSON{
			R: make([]float64, 0, len(f.Rects)*4),
			P: make([]int, 0, len(f.Pairs)*2),
			X: make([]int, 0),
			N: make([]nodeJSON, 0, len(f.Nodes)),
			S: statsJSON{
				Cmp: f.Stats.Comparisons, Brut: f.Stats.BruteComparisons,
				Node: f.Stats.Nodes, Deg: f.Stats.DegradedNodes, Occ: f.Stats.MaxOccupancy,
			},
		}
		for _, r := range f.Rects {
			// 可视化坐标保留 2 位小数即可，显著压缩内嵌 JSON 体积。
			fj.R = append(fj.R, round2(r.MinX), round2(r.MinY), round2(r.MaxX), round2(r.MaxY))
		}
		for _, p := range f.Pairs {
			fj.P = append(fj.P, p.I, p.J)
		}
		for id, isCross := range f.Cross {
			if isCross {
				fj.X = append(fj.X, id)
			}
		}
		for _, n := range f.Nodes {
			nj := nodeJSON{
				B: []float64{round2(n.Bounds.MinX), round2(n.Bounds.MinY), round2(n.Bounds.MaxX), round2(n.Bounds.MaxY)},
				D: n.Depth, G: n.Degraded,
			}
			if !n.Leaf {
				nj.I = n.Items
			}
			fj.N = append(fj.N, nj)
		}
		data.Frames = append(data.Frames, fj)
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	// 纯数值数据本不含 "<"，转义后可安全内嵌进 <script>。
	jsonBlob := strings.ReplaceAll(string(raw), "<", `<`)

	html := reportHTMLStart + "\nconst DATA = " + jsonBlob + ";\n" + reportHTMLJS
	return os.WriteFile(path, []byte(html), 0o644)
}
