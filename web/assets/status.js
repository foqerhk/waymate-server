const T = {
  zh: {
    ok: "所有系统运行正常",
    bad: "部分服务异常",
    updated: (t) => `更新于 ${t} · 每 30 秒自动刷新`,
    region: { cn: "中国区", net: "国际区" },
    provider: { amap: "高德地图", google: "Google Maps" },
    up: "正常",
    down: "异常",
    off: "未启用",
    online: (n) => `${n} 在线`,
    quotaLeft: (n) => `剩余 ${n}`,
    devices: "设备总数",
    devicesSub: (e, c) => `老人端 ${e} · 家人端 ${c}`,
    elder: "老人端",
    child: "家人端",
    families: "已配对家庭",
    familiesSub: (a) => `近 7 日活跃设备 ${a}`,
    callsNow: "当前通话",
    callsNowSub: (v, d) => `语音 ${v} · 视频 ${d}`,
    peak: "24 小时峰值并发",
    peakSub: (n) => `实时通道在线 ${n}`,
    activeSum: (avg, max) => `日均 ${avg} · 最高 ${max}`,
    callSum: (sum, peak) => `合计 ${sum} 次 · 最高并发 ${peak}`,
    used: "已用",
    monthUsed: (u, q) => `本月 ${fmt(u)} / ${fmt(q)} 次`,
    keys: (k, per) => `${k} 个 Key · 每个 ${fmt(per)}/月`,
    today: (t, d3) => `今日 ${fmt(t)} · 近三日 ${fmt(d3)}`,
    runway: (v) => (v == null ? "近三日几乎未用，额度充足" : v <= 0 ? "额度已耗尽或不足 1 天" : `预计还能用 ${v} 天`),
    exhausted: (t) => `额度曾于 ${t} 耗尽`,
    mini: { search: "关键字搜索", walk: "步行规划", transit: "公交规划" },
    month: "本月",
    capText: (c, s, l) => `按地图额度可服务约 ${c.familiesByMaps || 0} 个家庭，按通话并发约 ${c.familiesByCalls || 0} 个家庭，取二者较小值作为推荐规模。搜索余量 ${s}，路径规划余量 ${l}。`,
    byMaps: "地图额度可服务",
    byCalls: "通话并发可服务",
    bottleneck: "短板",
    load: "并发负载",
    loadSub: (c) => `上限：语音 ${c.maxVoiceCalls || 0} 路 · 视频 ${c.maxVideoCalls || 0} 路（视频按 ${c.videoWeight || 3} 倍计）`,
    days: (v) => (v == null ? "充足" : v <= 0 ? "不足 1 天" : `约 ${v} 天`),
    unit: "次",
    fam: "户",
  },
  en: {
    ok: "All systems operational",
    bad: "Partial service disruption",
    updated: (t) => `Updated ${t} · refreshes every 30s`,
    region: { cn: "China region", net: "Global region" },
    provider: { amap: "Amap", google: "Google Maps" },
    up: "Operational",
    down: "Down",
    off: "Not enabled",
    online: (n) => `${n} online`,
    quotaLeft: (n) => `${n} left`,
    devices: "Devices",
    devicesSub: (e, c) => `${e} senior · ${c} family`,
    elder: "Senior",
    child: "Family",
    families: "Paired families",
    familiesSub: (a) => `${a} devices active in 7d`,
    callsNow: "Calls now",
    callsNowSub: (v, d) => `${v} voice · ${d} video`,
    peak: "24h peak concurrent",
    peakSub: (n) => `${n} realtime sockets`,
    activeSum: (avg, max) => `avg ${avg}/day · max ${max}`,
    callSum: (sum, peak) => `${sum} calls · peak ${peak} concurrent`,
    used: "used",
    monthUsed: (u, q) => `${fmt(u)} / ${fmt(q)} this month`,
    keys: (k, per) => `${k} keys · ${fmt(per)}/key/month`,
    today: (t, d3) => `${fmt(t)} today · ${fmt(d3)} in 3 days`,
    runway: (v) => (v == null ? "Barely used lately—plenty left" : v <= 0 ? "Quota exhausted or < 1 day left" : `~${v} days left`),
    exhausted: (t) => `Quota ran out at ${t}`,
    mini: { search: "Keyword search", walk: "Walking routes", transit: "Transit routes" },
    month: "this month",
    capText: (c, s, l) => `Map quota supports ~${c.familiesByMaps || 0} families and call capacity ~${c.familiesByCalls || 0}; the smaller one is the recommendation. Runway: search ${s}, routing ${l}.`,
    byMaps: "Supported by map quota",
    byCalls: "Supported by call capacity",
    bottleneck: "bottleneck",
    load: "Concurrent load",
    loadSub: (c) => `Caps: ${c.maxVoiceCalls || 0} voice · ${c.maxVideoCalls || 0} video (video weighs ${c.videoWeight || 3}×)`,
    days: (v) => (v == null ? "ample" : v <= 0 ? "< 1 day" : `~${v} days`),
    unit: "",
    fam: "",
  },
};

const ICONS = {
  phone: '<svg viewBox="0 0 24 24"><path d="M7 2h10a2 2 0 0 1 2 2v16a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2Zm0 3v13h10V5H7Zm5 14.2a1 1 0 1 0 0 2 1 1 0 0 0 0-2Z"/></svg>',
  home: '<svg viewBox="0 0 24 24"><path d="M12 3 2 11.5h3V21h5.5v-6h3v6H19v-9.5h3L12 3Z"/></svg>',
  call: '<svg viewBox="0 0 24 24"><path d="M6.6 10.8a15.2 15.2 0 0 0 6.6 6.6l2.2-2.2a1 1 0 0 1 1-.25 11.4 11.4 0 0 0 3.6.57 1 1 0 0 1 1 1V20a1 1 0 0 1-1 1A17 17 0 0 1 3 4a1 1 0 0 1 1-1h3.5a1 1 0 0 1 1 1c0 1.25.2 2.45.57 3.57a1 1 0 0 1-.25 1L6.6 10.8Z"/></svg>',
  pulse: '<svg viewBox="0 0 24 24"><path d="M3 12h4l2.5-6 4 12 2.5-6H21v2h-3.7L13.5 22l-4-12L8.3 14H3v-2Z"/></svg>',
};

let last = null;
let range = 7;
const lang = () => (currentLang() === "en" ? "en" : "zh");
const L = () => T[lang()];
const fmt = (n) => Number(n || 0).toLocaleString(lang() === "zh" ? "zh-CN" : "en-US");
const esc = (s) => String(s ?? "").replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
const $ = (id) => document.getElementById(id);

function dayLabel(day) {
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(day || "");
  if (!m) return day || "";
  return `${Number(m[2])}/${Number(m[3])}`;
}

function niceMax(v) {
  const raw = Math.max(1, v) / 4;
  const p = Math.pow(10, Math.floor(Math.log10(raw)));
  const n = raw / p;
  const step = Math.max(1, (n <= 1 ? 1 : n <= 2 ? 2 : n <= 5 ? 5 : 10) * p);
  return step * 4;
}

const tick = (v) => (v >= 1000 ? `${+(v / 1000).toFixed(1)}k` : String(Math.round(v)));

/* ---------- charts ---------- */

function chart(el, days, layers) {
  const W = Math.max(280, el.clientWidth || 560);
  const H = 220;
  const right = layers.find((l) => l.axis === "right");
  const pl = 36, pr = right ? 30 : 8, pt = 10, pb = 26;
  const n = Math.max(1, days.length);
  const left = layers.filter((l) => l !== right).flatMap((l) => l.values);
  const max = niceMax(Math.max(1, ...left));
  const rmax = right ? niceMax(Math.max(1, ...right.values)) : max;
  const iw = W - pl - pr, ih = H - pt - pb;
  const slot = iw / n;
  const x = (i) => pl + slot * (i + 0.5);
  const yOf = (l) => (v) => pt + ih - (v / (l === right ? rmax : max)) * ih;
  const y = yOf(null);
  const uid = el.id;

  let g = "";
  for (let k = 0; k <= 4; k++) {
    const yy = y((max / 4) * k);
    g += `<line class="grid" x1="${pl}" x2="${W - pr}" y1="${yy}" y2="${yy}"/>`;
    g += `<text class="ax" x="${pl - 8}" y="${yy + 4}" text-anchor="end">${tick((max / 4) * k)}</text>`;
    if (right) g += `<text class="ax" x="${W - pr + 8}" y="${yy + 4}" fill="${right.color}" style="fill:${right.color}">${tick((rmax / 4) * k)}</text>`;
  }
  const every = Math.ceil(n / Math.max(2, Math.floor(iw / 56)));
  days.forEach((d, i) => {
    if ((n - 1 - i) % every === 0) g += `<text class="ax" x="${x(i)}" y="${H - 6}" text-anchor="middle">${dayLabel(d)}</text>`;
  });

  let body = "";
  let defs = "";
  layers.forEach((l, li) => {
    const ly = yOf(l);
    const pts = l.values.map((v, i) => [x(i), ly(v)]);
    if (l.type === "bar") {
      const bw = Math.min(28, slot * 0.56);
      defs += `<linearGradient id="${uid}b${li}" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="${l.color}"/><stop offset="1" stop-color="${l.color}" stop-opacity=".55"/></linearGradient>`;
      body += pts.map(([px, py], i) => {
        const hgt = Math.max(l.values[i] > 0 ? 2 : 0, pt + ih - py);
        return `<rect x="${px - bw / 2}" y="${pt + ih - hgt}" width="${bw}" height="${hgt}" rx="${Math.min(6, bw / 3)}" fill="url(#${uid}b${li})"/>`;
      }).join("");
      return;
    }
    const d = smoothPath(pts);
    if (l.type === "area") {
      defs += `<linearGradient id="${uid}a${li}" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="${l.color}" stop-opacity=".28"/><stop offset="1" stop-color="${l.color}" stop-opacity="0"/></linearGradient>`;
      body += `<path d="${d} L${pts[pts.length - 1][0]},${pt + ih} L${pts[0][0]},${pt + ih} Z" fill="url(#${uid}a${li})"/>`;
    }
    body += `<path d="${d}" fill="none" stroke="${l.color}" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"/>`;
    const [ex, ey] = pts[pts.length - 1];
    body += `<circle cx="${ex}" cy="${ey}" r="5" fill="#fff" stroke="${l.color}" stroke-width="2.6"/>`;
  });

  el.innerHTML = `<svg viewBox="0 0 ${W} ${H}" width="${W}" height="${H}" role="img">
    <defs>${defs}</defs>${g}${body}
    <line class="hover-line" x1="0" x2="0" y1="${pt}" y2="${pt + ih}" visibility="hidden"/>
    <rect class="hit" x="${pl}" y="${pt}" width="${iw}" height="${ih}" fill="transparent"/>
  </svg>`;

  const svg = el.querySelector("svg");
  const hl = svg.querySelector(".hover-line");
  const tip = $("tip");
  svg.querySelector(".hit").addEventListener("pointermove", (ev) => {
    const r = svg.getBoundingClientRect();
    const px = ((ev.clientX - r.left) / r.width) * W;
    const i = Math.min(n - 1, Math.max(0, Math.floor((px - pl) / slot)));
    hl.setAttribute("x1", x(i)); hl.setAttribute("x2", x(i)); hl.setAttribute("visibility", "visible");
    tip.innerHTML = `<small>${esc(days[i])}</small>${layers.map((l) => `<span style="color:${l.tipColor || l.color}">●</span> ${esc(l.label)} ${fmt(l.values[i])}`).join("<br>")}`;
    tip.hidden = false;
    tip.style.left = `${r.left + (x(i) / W) * r.width}px`;
    tip.style.top = `${r.top + (Math.min(...layers.map((l) => yOf(l)(l.values[i]))) / H) * r.height}px`;
  });
  svg.querySelector(".hit").addEventListener("pointerleave", () => {
    hl.setAttribute("visibility", "hidden");
    tip.hidden = true;
  });
}

function smoothPath(pts) {
  if (pts.length === 1) return `M${pts[0][0] - 1},${pts[0][1]} L${pts[0][0] + 1},${pts[0][1]}`;
  let d = `M${pts[0][0]},${pts[0][1]}`;
  for (let i = 0; i < pts.length - 1; i++) {
    const [x0, y0] = pts[i], [x1, y1] = pts[i + 1];
    const cx = (x0 + x1) / 2;
    d += ` C${cx},${y0} ${cx},${y1} ${x1},${y1}`;
  }
  return d;
}

function spark(values, color, id) {
  const W = 300, H = 64, p = 4;
  const max = Math.max(1, ...values);
  const pts = values.map((v, i) => [p + (i * (W - 2 * p)) / Math.max(1, values.length - 1), H - p - (v / max) * (H - 2 * p)]);
  const d = smoothPath(pts);
  return `<svg viewBox="0 0 ${W} ${H}" preserveAspectRatio="none">
    <defs><linearGradient id="${id}" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="${color}" stop-opacity=".25"/><stop offset="1" stop-color="${color}" stop-opacity="0"/></linearGradient></defs>
    <path d="${d} L${W - p},${H} L${p},${H} Z" fill="url(#${id})"/>
    <path d="${d}" fill="none" stroke="${color}" stroke-width="2.2" vector-effect="non-scaling-stroke"/>
  </svg>`;
}

function gauge(el, used, quota, color) {
  const pct = quota > 0 ? Math.min(1, used / quota) : 0;
  const r = 70, c = 2 * Math.PI * r;
  const col = pct >= 0.9 ? "#d9452f" : pct >= 0.7 ? "#d08317" : color;
  el.innerHTML = `<svg viewBox="0 0 168 168">
      <circle cx="84" cy="84" r="${r}" fill="none" stroke="#e6f0ec" stroke-width="14"/>
      <circle cx="84" cy="84" r="${r}" fill="none" stroke="${col}" stroke-width="14" stroke-linecap="round"
        stroke-dasharray="${c}" stroke-dashoffset="${c * (1 - pct)}" style="transition:stroke-dashoffset .9s cubic-bezier(.2,.7,.2,1)"/>
    </svg>
    <div class="g-txt"><b>${quota > 0 ? Math.round(pct * 100) : 0}<small style="font-size:.5em">%</small></b><span>${L().used}</span></div>`;
}

function runwayState(v) {
  if (v == null) return "ok";
  if (v <= 3) return "bad";
  if (v <= 10) return "warn";
  return "ok";
}

/* ---------- render ---------- */

function render(data) {
  const t = L();
  const u = data.users || {};
  const c = data.calls || {};
  const cap = data.capacity || {};
  const m = data.maps || {};
  const h = data.health || {};
  const s = data.series || {};
  const ok = !!(h.api && h.db);

  const card = $("healthCard");
  card.dataset.state = ok ? "ok" : "bad";
  $("healthTitle").textContent = ok ? t.ok : t.bad;
  const when = data.generated ? new Date(data.generated) : new Date();
  const timeStr = when.toLocaleTimeString(lang() === "zh" ? "zh-CN" : "en-US", { hour12: false });
  $("healthMeta").textContent = `${t.region[data.region] || data.region || ""} · ${t.updated(timeStr)}`;

  const searchPct = m.searchQuotaMonth ? m.searchUsedMonth / m.searchQuotaMonth : 0;
  const mapsBad = m.searchDaysLeft != null && m.searchDaysLeft <= 0;
  const comp = {
    api: [h.api ? "ok" : "bad", h.api ? t.up : t.down],
    db: [h.db ? "ok" : "bad", h.db ? t.up : t.down],
    ws: [h.api ? "ok" : "bad", t.online(fmt(c.wsOnline))],
    livekit: [h.livekit ? "ok" : "warn", h.livekit ? t.up : t.off],
    maps: [mapsBad ? "bad" : searchPct >= 0.9 ? "warn" : "ok", t.quotaLeft(`${Math.max(0, Math.round((1 - searchPct) * 100))}%`)],
  };
  document.querySelectorAll("#components li").forEach((li) => {
    const [st, label] = comp[li.dataset.k] || ["", ""];
    li.dataset.s = st;
    li.querySelector("em").textContent = label;
  });

  const el = u.elders || 0, ch = u.children || 0, tot = Math.max(1, el + ch);
  const kpis = [
    { icon: ICONS.phone, label: t.devices, val: fmt(u.devices), extra: `
        <div class="split"><i style="width:${(el / tot) * 100}%;background:var(--accent)"></i><i style="width:${(ch / tot) * 100}%;background:var(--mint)"></i></div>
        <div class="split-legend"><span style="--c:var(--accent)">${t.elder} ${fmt(el)}</span><span style="--c:var(--mint)">${t.child} ${fmt(ch)}</span></div>` },
    { icon: ICONS.home, label: t.families, val: fmt(u.families), sub: t.familiesSub(fmt(u.active7d)) },
    { icon: ICONS.call, label: t.callsNow, val: fmt((c.activeVoice || 0) + (c.activeVideo || 0)), sub: t.callsNowSub(fmt(c.activeVoice), fmt(c.activeVideo)) },
    { icon: ICONS.pulse, label: t.peak, val: fmt(c.peakConcurrent24h), sub: t.peakSub(fmt(c.wsOnline)) },
  ];
  $("kpis").innerHTML = kpis.map((k) => `
    <article class="panel kpi">
      <div class="k-top"><span class="k-ico">${k.icon}</span>${esc(k.label)}</div>
      <div class="k-val">${k.val}</div>
      ${k.sub ? `<div class="k-sub">${esc(k.sub)}</div>` : ""}${k.extra || ""}
    </article>`).join("");

  const act = s.activeDevices || [];
  const actVals = act.map((d) => d.count || 0);
  const avg = actVals.length ? Math.round(actVals.reduce((a, b) => a + b, 0) / actVals.length) : 0;
  $("activeSum").textContent = t.activeSum(fmt(avg), fmt(Math.max(0, ...actVals)));
  chart($("chartActive"), act.map((d) => d.day), [{ type: "area", color: "#0F7366", label: lang() === "zh" ? "活跃设备" : "Active", values: actVals }]);

  const calls = s.calls || [];
  const cnt = calls.map((d) => d.count || 0), pk = calls.map((d) => d.peak || 0);
  $("callSum").textContent = t.callSum(fmt(cnt.reduce((a, b) => a + b, 0)), fmt(Math.max(0, ...pk)));
  chart($("chartCalls"), calls.map((d) => d.day), [
    { type: "bar", color: "#14a08d", tipColor: "#7ddea8", label: lang() === "zh" ? "通话" : "Calls", values: cnt },
    { type: "line", axis: "right", color: "#d08317", tipColor: "#f4c983", label: lang() === "zh" ? "峰值" : "Peak", values: pk },
  ]);

  $("providerChip").textContent = `${t.provider[m.provider] || m.provider || "-"} · ${m.keyCount || 0} Key`;
  gauge($("gaugeSearch"), m.searchUsedMonth || 0, m.searchQuotaMonth || 0, "#0F7366");
  gauge($("gaugeLbs"), m.lbsUsedMonth || 0, m.lbsQuotaMonth || 0, "#14a08d");
  $("searchMeta").innerHTML = `${esc(t.monthUsed(m.searchUsedMonth, m.searchQuotaMonth))}<br>${esc(t.keys(m.keyCount || 0, m.searchQuotaPerKey))}`;
  $("lbsMeta").innerHTML = `${esc(t.monthUsed(m.lbsUsedMonth, m.lbsQuotaMonth))}<br>${esc(t.keys(m.keyCount || 0, m.lbsQuotaPerKey))}`;
  const rw = (id, v) => {
    const e = $(id);
    e.dataset.s = runwayState(v);
    e.textContent = t.runway(v);
  };
  rw("searchRunway", m.searchDaysLeft);
  rw("lbsRunway", m.lbsDaysLeft);
  if (m.quotaExhaustedAt) {
    const e = $("searchRunway");
    e.dataset.s = "bad";
    e.textContent = t.exhausted(new Date(m.quotaExhaustedAt).toLocaleString(lang() === "zh" ? "zh-CN" : "en-US", { hour12: false }));
  }

  const minis = [
    ["search", "#0F7366", s.mapsSearch, m.searchUsedMonth, m.searchUsedToday, m.searchLast3d],
    ["walk", "#2a7ab8", s.mapsWalk, m.walkUsedMonth, m.walkUsedToday, m.walkLast3d],
    ["transit", "#c97812", s.mapsTransit, m.transitUsedMonth, m.transitUsedToday, m.transitLast3d],
  ];
  $("miniGrid").innerHTML = minis.map(([k, col, ser, mon, td, d3]) => `
    <article class="panel mini">
      <div class="m-top"><h4>${esc(t.mini[k])}</h4><span class="chip" style="background:color-mix(in srgb, ${col} 12%, transparent);color:${col}">${esc(t.month)}</span></div>
      <div class="m-month">${fmt(mon)}</div>
      <div class="m-sub">${esc(t.today(td, d3))}</div>
      ${spark((ser || []).map((d) => d.count || 0).concat((ser || []).length ? [] : [0, 0]), col, `sp-${k}`)}
    </article>`).join("");

  $("capFamilies").textContent = fmt(cap.recommendedFamilies);
  $("capText").textContent = t.capText(cap, t.days(m.searchDaysLeft), t.days(m.lbsDaysLeft));
  const fm = cap.familiesByMaps || 0, fc = cap.familiesByCalls || 0, fmax = Math.max(1, fm, fc);
  const bar = (label, v, isB) => `
    <div class="bar-row${isB ? " bottleneck" : ""}">
      <div class="b-top"><span>${esc(label)}${isB ? `<span class="tagb">${esc(t.bottleneck)}</span>` : ""}</span><em>${fmt(v)} ${esc(t.fam)}</em></div>
      <div class="bar-track"><i style="width:${(v / fmax) * 100}%"></i></div>
    </div>`;
  $("capBars").innerHTML = bar(t.byMaps, fm, cap.bottleneck === "maps") + bar(t.byCalls, fc, cap.bottleneck === "calls");

  const lu = cap.loadUnits || 0, lm = cap.maxLoadUnits || 0;
  const on = lm > 0 ? Math.min(20, Math.ceil((lu / lm) * 20)) : 0;
  $("capLoad").innerHTML = `
    <div class="l-top"><span>${esc(t.load)}</span><em>${fmt(lu)} / ${fmt(lm)}</em></div>
    <div class="segs">${Array.from({ length: 20 }, (_, i) => `<i class="${i < on ? "on" : ""}"></i>`).join("")}</div>
    <div class="l-sub">${esc(t.loadSub(cap))}</div>`;
}

async function loadStatus() {
  const errEl = $("error");
  try {
    const res = await fetch(`/v1/public/status?range=${range}d`, { cache: "no-store" });
    if (!res.ok) throw new Error(String(res.status));
    last = await res.json();
    errEl.hidden = true;
    render(last);
  } catch (e) {
    errEl.hidden = false;
    errEl.textContent = (I18N[currentLang()] || I18N.zh).load_fail;
    if (!last) {
      $("healthCard").dataset.state = "bad";
      $("healthTitle").textContent = (I18N[currentLang()] || I18N.zh).load_fail;
    }
  }
}

document.addEventListener("langchange", () => { if (last) render(last); });

document.addEventListener("DOMContentLoaded", () => {
  document.querySelectorAll(".range button").forEach((btn) => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(".range button").forEach((b) => b.classList.toggle("active", b === btn));
      range = Number(btn.dataset.range) || 7;
      loadStatus();
    });
  });

  let rt;
  window.addEventListener("resize", () => {
    clearTimeout(rt);
    rt = setTimeout(() => { if (last) render(last); }, 150);
  });

  loadStatus();
  setInterval(loadStatus, 30000);
});
