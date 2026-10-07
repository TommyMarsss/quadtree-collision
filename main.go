package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

// 运行运动模拟，逐帧校验四叉树与暴力法结果一致（Simulate 内部已断言），
// 并把帧数据内嵌进单一静态 HTML 文件输出。
func main() {
	seed := flag.Int64("seed", 42, "随机种子")
	frames := flag.Int("frames", 300, "模拟帧数")
	out := flag.String("out", "replay.html", "输出 HTML 文件路径")
	flag.Parse()

	rec := Simulate(*seed, *frames)

	data, err := json.Marshal(rec)
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal frames:", err)
		os.Exit(1)
	}
	page := strings.Replace(pageTemplate, "__FRAMES_DATA__", string(data), 1)
	if err := os.WriteFile(*out, []byte(page), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write html:", err)
		os.Exit(1)
	}

	// 汇总统计。
	var totQ, totB, totPairs, maxDepth int
	for _, f := range rec {
		totQ += f.QuadC
		totB += f.BruteC
		totPairs += len(f.Pairs)
		if f.MaxDep > maxDepth {
			maxDepth = f.MaxDep
		}
	}
	fmt.Printf("模拟完成：%d 帧，%d 个对象\n", len(rec), len(rec[0].Bodies))
	fmt.Printf("碰撞对总数：%d，最大深度：%d\n", totPairs, maxDepth)
	fmt.Printf("比较次数：四叉树 %d vs 暴力 %d（减少 %.1f%%）\n",
		totQ, totB, (1-float64(totQ)/float64(totB))*100)
	fmt.Printf("已生成 %s（用浏览器打开即可回放）\n", *out)
}
