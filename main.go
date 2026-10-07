package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	out := flag.String("out", "report.html", "输出的静态 HTML 报告路径")
	seed := flag.Int64("seed", 42, "场景随机种子")
	flag.Parse()

	cfg := DefaultSimConfig()
	sim := NewSimulation(cfg, *seed)

	fmt.Printf("动态四叉树碰撞检测模拟：%d 个对象 × %d 帧\n",
		cfg.Movers+cfg.ClusterObjs, cfg.Frames)
	mismatches := sim.Run()

	// 汇总整段模拟的统计区间。
	frames := sim.Frames()
	var sumCmp, sumBrute, maxDeg, maxSaveFrame int
	minSavePct, maxSavePct := 100.0, -100.0
	for fi, f := range frames {
		sumCmp += f.Stats.Comparisons
		sumBrute += f.Stats.BruteComparisons
		if f.Stats.DegradedNodes > maxDeg {
			maxDeg = f.Stats.DegradedNodes
		}
		save := 0.0
		if f.Stats.BruteComparisons > 0 {
			save = (1 - float64(f.Stats.Comparisons)/float64(f.Stats.BruteComparisons)) * 100
		}
		if save < minSavePct {
			minSavePct = save
		}
		if save > maxSavePct {
			maxSavePct, maxSaveFrame = save, fi
		}
	}
	avgSave := (1 - float64(sumCmp)/float64(sumBrute)) * 100

	if len(mismatches) > 0 {
		fmt.Printf("❌ 发现 %d 帧与暴力法结果不一致：\n", len(mismatches))
		for _, mm := range mismatches {
			fmt.Printf("  帧 %d：漏报 %d 对，多报 %d 对\n", mm.Frame, len(mm.Missing), len(mm.Extra))
		}
		os.Exit(1)
	}
	fmt.Println("✅ 全部帧：四叉树碰撞对集合与暴力 O(n²) 完全一致（无漏报、无多报）")
	fmt.Printf("📉 累计比较次数：四叉树 %d vs 暴力 %d，平均减少 %.1f%%\n", sumCmp, sumBrute, avgSave)
	fmt.Printf("   单帧减少区间：%.1f%% ～ %.1f%%（第 %d 帧最佳）\n", minSavePct, maxSavePct, maxSaveFrame)
	fmt.Printf("🌲 最大深度退化叶子数：%d（深度上限=%d，退化节点内暴力比较）\n", maxDeg, cfg.Tree.MaxDepth)

	sim.verified = true
	if err := WriteReport(*out, sim); err != nil {
		fmt.Fprintln(os.Stderr, "报告生成失败:", err)
		os.Exit(1)
	}
	fmt.Printf("📄 静态回放报告已生成：%s（浏览器直接打开即可）\n", *out)
}
