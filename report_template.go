package main

const reportHTMLStart = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>动态四叉树碰撞检测 · 回放报告</title>
<style>
  :root {
    --bg: #0f1420; --panel: #161d2e; --ink: #e6ebf5; --muted: #8b97ad;
    --line: #2a3550; --accent: #5b8cff; --good: #38d39f; --bad: #ff5d6c;
    --cross: #ffb84d; --mover: #6fa8ff; --cluster: #b48cff;
  }
  * { box-sizing: border-box; }
  body {
    margin: 0; background: var(--bg); color: var(--ink);
    font: 14px/1.5 -apple-system, "Segoe UI", Roboto, "PingFang SC", "Microsoft YaHei", sans-serif;
  }
  header { padding: 18px 22px 10px; }
  header h1 { font-size: 18px; margin: 0 0 4px; }
  header p { margin: 0; color: var(--muted); font-size: 13px; }
  .layout { display: flex; flex-wrap: wrap; gap: 16px; padding: 14px 22px 28px; }
  .stage { flex: 1 1 640px; min-width: 320px; }
  canvas {
    width: 100%; aspect-ratio: 1 / 1; display: block;
    background: #0b101b; border: 1px solid var(--line); border-radius: 10px;
  }
  .side { flex: 0 1 300px; min-width: 260px; display: flex; flex-direction: column; gap: 12px; }
  .card { background: var(--panel); border: 1px solid var(--line); border-radius: 10px; padding: 14px 16px; }
  .card h2 { font-size: 13px; margin: 0 0 10px; color: var(--muted); font-weight: 600; letter-spacing: .04em; }
  .controls { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-top: 10px; }
  button {
    background: var(--accent); color: #fff; border: 0; border-radius: 7px;
    padding: 7px 14px; font-size: 13px; cursor: pointer;
  }
  button.secondary { background: #24304a; color: var(--ink); }
  button:hover { filter: brightness(1.12); }
  input[type=range] { flex: 1 1 160px; accent-color: var(--accent); }
  .speed { display: flex; align-items: center; gap: 6px; color: var(--muted); font-size: 12px; }
  label.toggle { display: flex; align-items: center; gap: 7px; color: var(--muted); font-size: 13px; cursor: pointer; }
  .stat-row { display: flex; justify-content: space-between; padding: 4px 0; font-variant-numeric: tabular-nums; }
  .stat-row span:first-child { color: var(--muted); }
  .stat-row b { font-weight: 600; }
  .good { color: var(--good); } .bad { color: var(--bad); } .warn { color: var(--cross); }
  .legend { display: grid; grid-template-columns: 1fr 1fr; gap: 6px 10px; font-size: 12px; color: var(--muted); }
  .legend i { display: inline-block; width: 12px; height: 12px; border-radius: 3px; margin-right: 6px; vertical-align: -2px; }
  .verify { font-size: 13px; }
  .verify.ok { color: var(--good); } .verify.fail { color: var(--bad); }
  .bar { height: 6px; background: #232d45; border-radius: 4px; overflow: hidden; margin-top: 8px; }
  .bar i { display: block; height: 100%; background: linear-gradient(90deg, var(--accent), var(--good)); }
</style>
</head>
<body>
<header>
  <h1>动态四叉树 2D 碰撞检测 · 逐帧回放</h1>
  <p>大矩形为普通运动体（频繁跨越分割线），中心紫色簇先压缩到最大深度退化为暴力比较，再散开触发逐层合并。碰撞对红色高亮，跨边界对象橙色描边。</p>
</header>
<div class="layout">
  <div class="stage">
    <canvas id="cv" width="1000" height="1000"></canvas>
    <div class="controls">
      <button id="play">暂停</button>
      <button id="prev" class="secondary">◀</button>
      <input id="seek" type="range" min="0" value="0">
      <button id="next" class="secondary">▶</button>
      <span class="speed">速度
        <select id="rate" class="secondary" style="background:#24304a;color:var(--ink);border:1px solid var(--line);border-radius:6px;padding:4px">
          <option value="0.25">0.25×</option><option value="0.5">0.5×</option>
          <option value="1" selected>1×</option><option value="2">2×</option><option value="4">4×</option>
        </select>
      </span>
    </div>
    <div class="controls">
      <label class="toggle"><input type="checkbox" id="showTree" checked> 四叉树节点</label>
      <label class="toggle"><input type="checkbox" id="showHit" checked> 碰撞高亮</label>
      <label class="toggle"><input type="checkbox" id="showCross" checked> 跨边界对象</label>
    </div>
  </div>
  <div class="side">
    <div class="card">
      <h2>校验状态</h2>
      <div id="verify" class="verify"></div>
    </div>
    <div class="card">
      <h2>第 <span id="frameNo">0</span> 帧统计</h2>
      <div class="stat-row"><span>碰撞对</span><b id="pairs">0</b></div>
      <div class="stat-row"><span>四叉树比较次数</span><b id="cmp">0</b></div>
      <div class="stat-row"><span>暴力法比较次数</span><b id="bru">0</b></div>
      <div class="stat-row"><span>比较次数减少</span><b class="good" id="save">0%</b></div>
      <div class="bar"><i id="savebar" style="width:0%"></i></div>
    </div>
    <div class="card">
      <h2>树结构</h2>
      <div class="stat-row"><span>节点总数</span><b id="nodes">0</b></div>
      <div class="stat-row"><span>深度上限退化叶子</span><b class="warn" id="degraded">0</b></div>
      <div class="stat-row"><span>单节点最多对象</span><b id="occ">0</b></div>
      <div class="stat-row"><span>参数</span><b id="cfg"></b></div>
    </div>
    <div class="card">
      <h2>图例</h2>
      <div class="legend">
        <span><i style="background:#6fa8ff55;border:1px solid var(--mover)"></i>普通运动体</span>
        <span><i style="background:#b48cff55;border:1px solid var(--cluster)"></i>高密度簇</span>
        <span><i style="background:#ff5d6c66;border:1px solid var(--bad)"></i>碰撞对象</span>
        <span><i style="border:2px solid var(--cross);background:transparent"></i>跨边界存储</span>
        <span><i style="background:#ff5d6c44;border:1px dashed var(--bad)"></i>退化叶子(暴力)</span>
      </div>
    </div>
  </div>
</div>
<script>
"use strict";
`

const reportHTMLJS = `
const cv = document.getElementById("cv");
const ctx = cv.getContext("2d");
const N = DATA.f.length;
const S = cv.width / DATA.w;

(function initVerify() {
  const el = document.getElementById("verify");
  if (DATA.v) {
    el.className = "verify ok";
    el.textContent = "✅ 全部帧与暴力法 O(n²) 碰撞对集合完全一致（无漏报、无多报）";
  } else {
    el.className = "verify fail";
    el.textContent = "❌ 存在与暴力法不一致的帧";
  }
})();

const $ = id => document.getElementById(id);
const seek = $("seek"); seek.max = N - 1;

let frame = 0, playing = true, rate = 1, lastT = 0, acc = 0;
const FRAME_MS = 60;

// 预计算每帧碰撞集合，O(1) 查询某对象本帧是否碰撞。
const hitSets = DATA.f.map(f => {
  const s = new Uint8Array(DATA.k.length);
  for (let i = 0; i < f.p.length; i += 2) { s[f.p[i]] = 1; s[f.p[i+1]] = 1; }
  return s;
});
const crossSets = DATA.f.map(f => {
  const s = new Uint8Array(DATA.k.length);
  for (const id of f.x) s[id] = 1;
  return s;
});

function draw(idx) {
  const f = DATA.f[idx];
  ctx.clearRect(0, 0, cv.width, cv.height);

  if ($("showTree").checked) {
    for (const nd of f.n) {
      const [x0, y0, x1, y1] = nd.b;
      if (nd.g) {
        ctx.fillStyle = "rgba(255,93,108,0.10)";
        ctx.fillRect(x0*S, y0*S, (x1-x0)*S, (y1-y0)*S);
        ctx.strokeStyle = "rgba(255,93,108,0.75)";
        ctx.setLineDash([5, 4]);
      } else {
        const a = Math.max(0.10, 0.42 - nd.d * 0.045);
        ctx.strokeStyle = "rgba(120,150,210," + a + ")";
        ctx.setLineDash([]);
      }
      ctx.lineWidth = 1;
      ctx.strokeRect(x0*S + .5, y0*S + .5, (x1-x0)*S - 1, (y1-y0)*S - 1);
    }
    ctx.setLineDash([]);
  }

  const hit = hitSets[idx], cross = crossSets[idx];
  for (let id = 0; id < DATA.k.length; id++) {
    const o = id * 4;
    const x = f.r[o]*S, y = f.r[o+1]*S;
    const w = (f.r[o+2]-f.r[o])*S, h = (f.r[o+3]-f.r[o+1])*S;
    let fill, stroke;
    if ($("showHit").checked && hit[id]) {
      fill = "rgba(255,93,108,0.42)"; stroke = "#ff5d6c";
    } else if (DATA.k[id] === 1) {
      fill = "rgba(180,140,255,0.30)"; stroke = "#b48cff";
    } else {
      fill = "rgba(111,168,255,0.22)"; stroke = "#6fa8ff";
    }
    ctx.fillStyle = fill;
    ctx.fillRect(x, y, w, h);
    ctx.strokeStyle = stroke;
    ctx.lineWidth = 1;
    ctx.strokeRect(x + .5, y + .5, w - 1, h - 1);

    if ($("showCross").checked && cross[id]) {
      ctx.strokeStyle = "#ffb84d";
      ctx.lineWidth = 2;
      ctx.strokeRect(x - 1.5, y - 1.5, w + 3, h + 3);
    }
  }

  // 碰撞连线
  if ($("showHit").checked) {
    ctx.strokeStyle = "rgba(255,93,108,0.55)";
    ctx.lineWidth = 1;
    for (let i = 0; i < f.p.length; i += 2) {
      const a = f.p[i]*4, b = f.p[i+1]*4;
      ctx.beginPath();
      ctx.moveTo((f.r[a]+f.r[a+2])/2*S, (f.r[a+1]+f.r[a+3])/2*S);
      ctx.lineTo((f.r[b]+f.r[b+2])/2*S, (f.r[b+1]+f.r[b+3])/2*S);
      ctx.stroke();
    }
  }

  updateHUD(f, idx);
}

function updateHUD(f, idx) {
  $("frameNo").textContent = idx;
  const pairs = f.p.length / 2;
  const save = f.s.bru > 0 ? (1 - f.s.cmp / f.s.bru) * 100 : 0;
  $("pairs").textContent = pairs;
  $("cmp").textContent = f.s.cmp.toLocaleString();
  $("bru").textContent = f.s.bru.toLocaleString();
  const saveEl = $("save");
  saveEl.textContent = save.toFixed(1) + "%";
  saveEl.className = save >= 0 ? "good" : "bad";
  $("savebar").style.width = Math.max(0, Math.min(100, save)) + "%";
  $("nodes").textContent = f.s.nod;
  $("degraded").textContent = f.s.deg;
  $("occ").textContent = f.s.occ;
  $("cfg").textContent =
    "分裂>" + DATA.cfg.split + " · 合并≤" + DATA.cfg.merge + " · 深度≤" + DATA.cfg.depth;
  seek.value = idx;
}

function loop(t) {
  if (playing) {
    if (!lastT) lastT = t;
    acc += (t - lastT) * rate;
    lastT = t;
    if (acc >= FRAME_MS) {
      const step = Math.floor(acc / FRAME_MS);
      acc -= step * FRAME_MS;
      frame = (frame + step) % N;
    }
    draw(frame);
  } else { lastT = 0; }
  requestAnimationFrame(loop);
}

$("play").onclick = () => {
  playing = !playing;
  $("play").textContent = playing ? "暂停" : "播放";
};
$("prev").onclick = () => { frame = (frame - 1 + N) % N; draw(frame); };
$("next").onclick = () => { frame = (frame + 1) % N; draw(frame); };
seek.oninput = () => { frame = +seek.value; draw(frame); };
$("rate").onchange = e => { rate = +e.target.value; };

draw(0);
requestAnimationFrame(loop);
</script>
</body>
</html>
`
