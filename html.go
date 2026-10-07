package main

// pageTemplate 是单文件回放页的模板，__FRAMES_DATA__ 会被替换为内嵌的帧数据 JSON。
// 纯原生 HTML/CSS/JavaScript，不引入任何框架或图形库。
const pageTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>四叉树碰撞检测回放</title>
<style>
  :root { --bg:#0f1420; --panel:#1a2233; --fg:#dbe4f0; --muted:#8b98ad; --accent:#4a90d9; }
  * { box-sizing: border-box; }
  body { margin:0; background:var(--bg); color:var(--fg); font:14px/1.5 -apple-system,"PingFang SC","Segoe UI",sans-serif; }
  .wrap { display:flex; gap:16px; padding:16px; max-width:1400px; margin:0 auto; flex-wrap:wrap; }
  canvas { background:#101826; border:1px solid #2a3850; border-radius:6px; max-width:100%; }
  .side { flex:1; min-width:260px; }
  .panel { background:var(--panel); border-radius:8px; padding:14px 16px; margin-bottom:14px; }
  h1 { font-size:18px; margin:0 0 4px; }
  .sub { color:var(--muted); font-size:12px; margin-bottom:12px; }
  .stats { display:grid; grid-template-columns:1fr 1fr; gap:6px 14px; font-variant-numeric:tabular-nums; }
  .stats .k { color:var(--muted); }
  .stats .v { text-align:right; font-weight:600; }
  .bar { height:8px; background:#26314a; border-radius:4px; overflow:hidden; margin-top:6px; }
  .bar i { display:block; height:100%; background:#3fbf7f; width:0; transition:width .15s; }
  .controls { display:flex; align-items:center; gap:10px; flex-wrap:wrap; }
  button { background:var(--accent); color:#fff; border:0; border-radius:6px; padding:7px 16px; font-size:14px; cursor:pointer; }
  button:hover { filter:brightness(1.1); }
  input[type=range] { flex:1; min-width:120px; }
  select { background:#26314a; color:var(--fg); border:0; border-radius:6px; padding:6px; }
  .legend span { display:inline-flex; align-items:center; margin-right:14px; font-size:12px; color:var(--muted); }
  .sw { width:12px; height:12px; border-radius:3px; margin-right:5px; display:inline-block; }
  label.ck { font-size:12px; color:var(--muted); display:inline-flex; align-items:center; gap:4px; margin-right:10px; }
</style>
</head>
<body>
<div class="wrap">
  <div>
    <canvas id="cv" width="960" height="640"></canvas>
    <div class="panel" style="margin-top:14px">
      <div class="controls">
        <button id="play">暂停</button>
        <input type="range" id="slider" min="0" value="0">
        <span id="frameLabel" style="min-width:90px;text-align:right"></span>
        <select id="speed">
          <option value="0.5">0.5×</option>
          <option value="1" selected>1×</option>
          <option value="2">2×</option>
          <option value="4">4×</option>
        </select>
      </div>
      <div style="margin-top:10px">
        <label class="ck"><input type="checkbox" id="showTree" checked>四叉树网格</label>
        <label class="ck"><input type="checkbox" id="showPairs" checked>碰撞连线</label>
        <label class="ck"><input type="checkbox" id="showCross" checked>跨边界高亮</label>
      </div>
    </div>
  </div>
  <div class="side">
    <div class="panel">
      <h1>动态四叉树碰撞检测</h1>
      <div class="sub">空间分区加速 · 跨边界归属 · 深度上限退化 · 与暴力法逐帧校验一致</div>
      <div class="legend">
        <span><i class="sw" style="background:#4a90d9"></i>普通对象</span>
        <span><i class="sw" style="background:#e74c3c"></i>碰撞中</span>
        <span><i class="sw" style="background:#f39c12"></i>跨边界对象</span>
        <span><i class="sw" style="background:#2e4a3a"></i>树节点</span>
      </div>
    </div>
    <div class="panel">
      <div class="stats">
        <span class="k">对象数</span><span class="v" id="sBodies">-</span>
        <span class="k">碰撞对数</span><span class="v" id="sPairs">-</span>
        <span class="k">跨边界对象</span><span class="v" id="sCross">-</span>
        <span class="k">树节点数</span><span class="v" id="sNodes">-</span>
        <span class="k">最大深度</span><span class="v" id="sDepth">-</span>
        <span class="k">四叉树比较次数</span><span class="v" id="sQC">-</span>
        <span class="k">暴力法比较次数</span><span class="v" id="sBC">-</span>
        <span class="k">比较次数减少</span><span class="v" id="sReduce">-</span>
      </div>
      <div class="bar"><i id="reduceBar"></i></div>
    </div>
    <div class="panel" style="font-size:12px;color:var(--muted)">
      归属策略：对象存入能完整包含它的最小节点；跨越分割线的对象留在父节点，
      查询沿重叠子树递归并检查路径上所有节点，保证与暴力 O(n²) 结果完全一致。
      节点对象数超过 8 分裂、子树总数降到 4 及以下合并，最大深度 6，
      达到上限的节点退化为节点内暴力比较。
    </div>
  </div>
</div>
<script>
const FRAMES = __FRAMES_DATA__;
const cv = document.getElementById('cv');
const ctx = cv.getContext('2d');
const slider = document.getElementById('slider');
const playBtn = document.getElementById('play');
const speedSel = document.getElementById('speed');
const showTree = document.getElementById('showTree');
const showPairs = document.getElementById('showPairs');
const showCross = document.getElementById('showCross');
slider.max = FRAMES.length - 1;

let cur = 0, playing = true, acc = 0, last = performance.now();

function draw(f) {
  const fr = FRAMES[f];
  ctx.clearRect(0, 0, cv.width, cv.height);

  // 四叉树节点网格：深度越深颜色越亮。
  if (showTree.checked) {
    for (const n of fr.nodes) {
      const a = 0.25 + n.d * 0.12;
      ctx.strokeStyle = 'rgba(80,200,140,' + Math.min(a, 0.9) + ')';
      ctx.lineWidth = 1;
      ctx.strokeRect(n.x, n.y, n.w, n.h);
    }
  }

  const colliding = new Set();
  for (const p of fr.pairs) { colliding.add(p[0]); colliding.add(p[1]); }
  const cross = new Set(fr.cross);
  const byId = new Map(fr.bodies.map(b => [b.id, b]));

  // 碰撞对连线。
  if (showPairs.checked) {
    ctx.strokeStyle = 'rgba(231,76,60,0.45)';
    ctx.lineWidth = 1;
    for (const p of fr.pairs) {
      const a = byId.get(p[0]), b = byId.get(p[1]);
      if (!a || !b) continue;
      ctx.beginPath();
      ctx.moveTo(a.x + a.w / 2, a.y + a.h / 2);
      ctx.lineTo(b.x + b.w / 2, b.y + b.h / 2);
      ctx.stroke();
    }
  }

  // 对象本体。
  for (const b of fr.bodies) {
    ctx.fillStyle = colliding.has(b.id) ? '#e74c3c' : '#4a90d9';
    ctx.fillRect(b.x, b.y, b.w, b.h);
    if (showCross.checked && cross.has(b.id)) {
      ctx.strokeStyle = '#f39c12';
      ctx.lineWidth = 2;
      ctx.strokeRect(b.x - 1.5, b.y - 1.5, b.w + 3, b.h + 3);
    }
  }

  // 统计面板。
  document.getElementById('sBodies').textContent = fr.bodies.length;
  document.getElementById('sPairs').textContent = fr.pairs.length;
  document.getElementById('sCross').textContent = fr.cross.length;
  document.getElementById('sNodes').textContent = fr.nodes.length;
  document.getElementById('sDepth').textContent = fr.md;
  document.getElementById('sQC').textContent = fr.qc;
  document.getElementById('sBC').textContent = fr.bc;
  const reduce = fr.bc > 0 ? (1 - fr.qc / fr.bc) * 100 : 0;
  document.getElementById('sReduce').textContent = reduce.toFixed(1) + '%';
  document.getElementById('reduceBar').style.width = reduce + '%';
  document.getElementById('frameLabel').textContent = (f + 1) + ' / ' + FRAMES.length;
}

function tick(now) {
  const dt = (now - last) / 1000;
  last = now;
  if (playing) {
    acc += dt * 30 * parseFloat(speedSel.value);
    while (acc >= 1) { acc -= 1; cur = (cur + 1) % FRAMES.length; }
    slider.value = cur;
    draw(cur);
  }
  requestAnimationFrame(tick);
}

playBtn.onclick = () => {
  playing = !playing;
  playBtn.textContent = playing ? '暂停' : '播放';
};
slider.oninput = () => { cur = +slider.value; draw(cur); };
showTree.onchange = showPairs.onchange = showCross.onchange = () => draw(cur);

draw(0);
requestAnimationFrame(tick);
</script>
</body>
</html>
`
