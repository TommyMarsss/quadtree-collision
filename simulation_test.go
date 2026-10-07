package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSimulationEndToEnd 演示场景逐帧与暴力法一致，且能成功生成单文件报告。
func TestSimulationEndToEnd(t *testing.T) {
	cfg := DefaultSimConfig()
	sim := NewSimulation(cfg, 123)
	if mm := sim.Run(); len(mm) > 0 {
		t.Fatalf("演示场景 %d 帧与暴力法不一致：首帧=%d", len(mm), mm[0].Frame)
	}

	// 模拟中段（簇最紧）必须出现深度退化叶子，否则场景未覆盖退化路径。
	frames := sim.Frames()
	mid := frames[cfg.Frames/2]
	if mid.Stats.DegradedNodes == 0 {
		t.Fatal("压缩最紧的中段帧未出现深度上限退化叶子")
	}
	// 首帧与末帧簇散开，退化叶子应消失（验证动态合并完整闭环）。
	if frames[0].Stats.DegradedNodes != 0 {
		t.Fatalf("首帧不应存在退化叶子，实际 %d", frames[0].Stats.DegradedNodes)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "report.html")
	if err := WriteReport(path, sim); err != nil {
		t.Fatalf("生成报告失败: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	html := string(raw)
	if !strings.Contains(html, "<!DOCTYPE html>") || !strings.Contains(html, "const DATA =") {
		t.Fatal("报告不是自包含的单文件 HTML")
	}
	for _, banned := range []string{"<script src", "http://", "https://", "<link"} {
		if strings.Contains(html, banned) {
			t.Fatalf("报告包含外部依赖: %s", banned)
		}
	}
}

// TestMultipleSeeds 多个种子下整段模拟均正确。
func TestMultipleSeeds(t *testing.T) {
	for _, seed := range []int64{1, 2, 99, 2026} {
		sim := NewSimulation(DefaultSimConfig(), seed)
		if mm := sim.Run(); len(mm) > 0 {
			t.Fatalf("seed=%d: %d 帧不一致，首帧 %d 漏 %d 多 %d",
				seed, len(mm), mm[0].Frame, len(mm[0].Missing), len(mm[0].Extra))
		}
	}
}
