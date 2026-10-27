"use strict";

// ---------- API ----------
const TOKEN = new URLSearchParams(location.search).get("t") || "";

async function api(method, path, body, raw) {
  const opts = { method, headers: { "X-Token": TOKEN } };
  if (body instanceof FormData) opts.body = body;
  else if (body !== undefined) {
    opts.body = JSON.stringify(body);
    opts.headers["Content-Type"] = "application/json";
  }
  const res = await fetch(path, opts);
  if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
  if (raw) return res;
  return res.status === 204 ? null : res.json();
}
const url = (path) => `${path}${path.includes("?") ? "&" : "?"}t=${TOKEN}`;

// ---------- helpers ----------
const $ = (sel, root = document) => root.querySelector(sel);
const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];
function el(tag, attrs = {}, ...children) {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (k === "class") e.className = v;
    else if (k === "style" && typeof v === "object") Object.assign(e.style, v);
    else if (k.startsWith("on")) e.addEventListener(k.slice(2), v);
    else if (v === true) e.setAttribute(k, "");
    else if (v !== false && v != null) e.setAttribute(k, v);
  }
  for (const c of children.flat()) if (c != null && c !== false) e.append(c);
  return e;
}
// What the manufacturer offers for a device (firmware check): text plus a link
// to the download page, opened in the browser. Only textContent, no HTML.
function fwNoteNodes(n) {
  if (!n || !n.status) return [];
  const link = (txt) => el("a", { href: n.url, target: "_blank", rel: "noopener" }, txt);
  if (n.status === "update") return [link(t("fw.update", { v: n.version }))];
  if (n.status === "current") return [el("span", {}, "· " + t("fw.current"))];
  const txt = n.version ? t("fw.newest", { v: n.version }) : t("fw.newestDate", { d: n.date || "" });
  return [el("span", {}, "· " + txt + " "), link("↗")];
}
const svg = (paths) => {
  const s = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  s.setAttribute("viewBox", "0 0 24 24");
  s.innerHTML = paths;
  return s;
};
let toastTimer;
function toast(msg, isError) {
  const tEl = $("#toast");
  tEl.textContent = msg;
  tEl.className = "toast" + (isError ? " error" : "");
  tEl.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (tEl.hidden = true), isError ? 6000 : 2200);
}
const debounce = (fn, ms) => {
  let h;
  return (...a) => { clearTimeout(h); h = setTimeout(() => fn(...a), ms); };
};
const slug = (s) => s.toLowerCase().replace(/ä/g, "ae").replace(/ö/g, "oe").replace(/ü/g, "ue").replace(/ß/g, "ss")
  .normalize("NFKD").replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 60) || "design";
const setRangeFill = (r) => r.style.setProperty("--p", `${((r.value - r.min) / (r.max - r.min)) * 100}%`);

// ---------- state ----------
let state = { panels: [], designs: [], media: [], fanSet: [] };

async function refresh() {
  state = await api("GET", "/api/state");
  if (state.language && state.language !== LANG) setLang(state.language);
  renderAll();
  if (typeof checkDisclaimer === "function") checkDisclaimer();
}
function renderAll() {
  renderStatus(); renderPanels(); renderArrange(); renderDesigns(); renderLayouts(); renderMedia(); renderSettings(); if (typeof renderDevices === "function") renderDevices(); loadRotation();
  if (drawerId) renderDrawer();
}

const designById = (id) => state.designs.find((d) => d.id === id);
function designName(id, withSet) {
  const key = "design." + id;
  const fm = /^(?:(neon|retro|cyber)-)?(half|w640|w960|h640|h960|card)-?(.*)$/.exec(id);
  if (fm && isFieldDesign(id) && designById(id)) {
    const kind = fm[3];
    const label = (t("kind." + kind) || kind) + " · " + t("size." + fm[2]);
    const set = { neon: "neon", retro: "retro", cyber: "cyberpunk" }[fm[1]];
    return withSet && set ? `${t("set." + set)} · ${label}` : label;
  }
  const d0 = designById(id);
  if (withSet && d0 && d0.builtin && d0.set && d0.set !== "clean") return `${t("set." + d0.set)} · ${designName(id)}`;
  const d = designById(id);
  if (d && d.builtin && STRINGS.de[key]) return t(key);
  return d ? d.name : id;
}

function renderStatus() {
  const pill = $("#status-pill");
  const n = state.panels.length;
  if (state.demo) { pill.className = "pill warn"; $("#status-text").textContent = t("status.demo", { n }); }
  else if (state.sensorsError) { pill.className = "pill error"; $("#status-text").textContent = t("status.sensorsOff", { n }); }
  else { pill.className = "pill ok"; $("#status-text").textContent = t("status.ok", { n }); }
  pill.title = state.sensorsError || t("status.title");
}
$("#status-pill").addEventListener("click", () => $('[data-tab="settings"]').click());

// ---------- tabs / header ----------
for (const b of $$("#tabs button")) {
  b.addEventListener("click", () => {
    if (drawerId) closeDrawer();
    $$("#tabs button").forEach((x) => x.classList.toggle("active", x === b));
    $$(".tab").forEach((tab) => tab.classList.toggle("active", tab.id === "tab-" + b.dataset.tab));
    b.scrollIntoView({ block: "nearest", inline: "nearest" });
  });
}
const showTab = (name) => $('[data-tab="' + name + '"]').click();
$("#btn-rescan").addEventListener("click", async () => {
  const b = $("#btn-rescan");
  b.disabled = true;
  b.animate([{ transform: "rotate(0)" }, { transform: "rotate(360deg)" }], { duration: 700, iterations: 2 });
  try { await api("POST", "/api/rescan"); await refresh(); toast(t("rescan.done")); }
  catch (e) { toast(e.message, true); }
  b.disabled = false;
});
for (const b of $$("[data-lang]")) {
  b.addEventListener("click", async () => {
    setLang(b.dataset.lang);
    renderAll();
    if (!$("#editor").hidden) { drawDesignForm(); drawLayers(); drawWidgetForm(); }
    try { await api("PUT", "/api/settings", { language: LANG }); } catch { /* not critical */ }
  });
}

// ---------- displays ----------
function panelGroups() {
  const groups = new Map();
  for (const p of state.panels) {
    const m = /^tllcd-p(\d+)-/.exec(p.id);
    const key = m ? "tl-" + m[1] : p.id;
    // The device counts ports from 0, the controller housing from 1.
    if (!groups.has(key)) groups.set(key, { key, port: m && Number(m[1]) + 1, tl: !!m, panels: [] });
    groups.get(key).panels.push(p);
  }
  return [...groups.values()].sort((a, b) => (a.port || 0) - (b.port || 0));
}

function contentLabel(cfg) {
  if (cfg.mode === "off") return t("content.off");
  if (cfg.mode === "media") return cfg.media ? t("content.media", { name: cfg.media }) : t("content.none");
  if (cfg.grid) return t("content.grid", { grid: t("grid." + cfg.grid + ".short"), n: (cfg.tiles || []).length });
  return t("content.design", { name: designName(cfg.design, true) });
}
// Default names count all fans together, matching the number badges and
// the identify screen, whichever controller port they hang on.
const isTL = (p) => /^tllcd-/.test(p.id);
const fanNumber = (p) => p.number || state.panels.filter(isTL).findIndex((x) => x.id === p.id) + 1;
const shortName = (p) => (p.config && p.config.name) ||
  (isTL(p) ? `${LANG === "de" ? "Lüfter" : "Fan"} ${fanNumber(p)}` : p.name.replace(/ \(.*\)$/, ""));
// deviceLabel describes the hardware position in the user's language.
const deviceLabel = (p) => {
  const m = /^UNI FAN TL LCD \(port (\d+), fan (\d+)\)$/.exec(p.name);
  return m ? `UNI FAN TL LCD · ${t("dev.port", { p: m[1], f: m[2] })}` : p.name;
};

function renderPanels() {
  const box = $("#groups");
  renderMonitorMode();
  box.replaceChildren();
  const n = state.panels.length;
  $("#no-panels").hidden = n > 0;
  $("#panel-count").textContent = n ? t("panels.connected", { n }) : "";
  const se = $("#scan-error");
  se.hidden = !((state.scanError && !n) || state.rescanGaveUp);
  se.textContent = (state.scanError && !n ? state.scanError + " " : "") + (state.rescanGaveUp ? t("scan.gaveup") : "");

  const rank = (g) => (g.tl ? 0 : g.panels.every(isWideP) ? 2 : 1);
  for (const g of panelGroups().sort((a, b) => rank(a) - rank(b))) { // stable: ports stay in order
    // A category, not the device name again: the card below carries that.
    const title = g.tl ? t("group.tl", { port: g.port }) : g.panels.every(isWideP) ? t("group.wide") : t("group.other");
    const head = el("h3", { class: "section" }, el("span", {}, title,
      g.tl ? el("span", { class: "count" }, t("group.tlCount", { n: g.panels.length })) : null));
    if (g.tl && g.panels.length >= 2) {
      head.append(el("span", { class: "actions" },
        el("button", { class: "ghost", "data-menu": "1",
          onclick: (e) => showMenu(e.currentTarget, t("menu.fanset"), SETS.map((set) => [t("set." + set), () => applyFanSet(g.panels, set)])) }, t("btn.fanset"))));
    }
    box.append(head, el("div", { class: "grid panels" }, g.panels.map(panelCard)));
  }
}

// Wide panels (the 1920x480 bar) keep their real aspect ratio everywhere.
const isWideSize = (w, h) => h > 0 && w >= h * 1.5;
const isWideP = (p) => isWideSize(p.width, p.height);
const isWideD = (d) => isWideSize(d.w, d.h);
// A design fits a panel unless it is round-only on a square one, or a bar
// design on a screen that is not a bar. Square designs are centred on bars.
// Designs made for the fields of a grid (cards, half fields, wide fields) are
// only offered per field, not as a whole-screen design.
const FIELD_RE = /^(?:(?:neon|retro|cyber)-)?(?:half|w640|w960|h640|h960|card)-/;
const isFieldDesign = (id) => FIELD_RE.test(id);
const isBarP = (p) => isWideSize(p.width, p.height) || isWideSize(p.height, p.width);
const isTallP = (p) => isWideSize(p.height, p.width);
const designFits = (d, p) => !isFieldDesign(d.id) && (isWideD(d) ? isWideP(p) : !p.round || d.round);
// A field of a grid shows a design of its own shape; others are fitted with bars.
const aspectOf = (w, h) => (h > 0 ? w / h : 1);
const fitsCell = (d, cell) => d.w > 0 && Math.abs(aspectOf(d.w, d.h) / aspectOf(cell.w, cell.h) - 1) < 0.03;
let showAllDesigns = false;
const designFitsFor = (d, p) => (p.cell ? showAllDesigns || fitsCell(d, { w: p.width, h: p.height }) : designFits(d, p));
const ratio = (w, h) => (w > 0 && h > 0 ? `${w} / ${h}` : "1");

const panelNumber = (p) => p.number || state.panels.findIndex((x) => x.id === p.id) + 1;

function panelCard(p) {
  const cfg = p.config;
  return el("button", { class: "panel-card" + (isWideP(p) ? " wide" : "") + (isTallP(p) ? " tall" : "") + (drawerId === p.id ? " selected" : ""), "data-panel": p.id, onclick: () => openDrawer(p.id) },
    el("div", { class: "well" },
      el("span", { class: "num-badge", title: t("btn.identify") }, String(panelNumber(p))),
      el("div", { class: "screen" + (p.round ? " round" : "") + (isTallP(p) ? " tall" : ""), style: { aspectRatio: ratio(p.width, p.height) } }, el("img", { alt: "", "data-preview": p.id }))),
    el("div", {},
      el("div", { class: "panel-name" }, shortName(p), p.experimental ? el("span", { class: "badge exp", title: t("dev.exp.hint") }, t("dev.exp.badge")) : null),
      el("div", { class: "panel-meta" }, `${p.width}×${p.height}` + (shortName(p) === p.name.replace(/ \(.*\)$/, "") ? "" : ` · ${p.name.replace(/ \(.*\)$/, "")}`))),
    el("span", { class: "content-pill" + (cfg.mode === "off" ? " off" : ""), title: contentLabel(cfg) }, contentLabel(cfg)),
    p.error ? el("div", { class: "panel-error" }, p.error) : null);
}

async function setPanel(id, cfg, quiet) {
  const p = state.panels.find((x) => x.id === id);
  const turned = p && isBarP(p) && (p.config.rotation || 0) !== (cfg.rotation || 0);
  if (cfg.grid === "") { cfg = { ...cfg }; delete cfg.tiles; }
  try {
    await api("PUT", `/api/panels/${encodeURIComponent(id)}`, cfg);
    if (turned) { await refresh(); toast(t("applied.one", { name: shortName(state.panels.find((x) => x.id === id) || p) })); return; }
    if (p) p.config = { ...cfg };
    if (!quiet) {
      toast(t("applied.one", { name: p ? shortName(p) : id }));
      pulse(id);
    }
    const card = $(`[data-panel="${CSS.escape(id)}"] .content-pill`);
    if (card) { card.textContent = card.title = contentLabel(cfg); card.classList.toggle("off", cfg.mode === "off"); }
  } catch (e) {
    toast(e.message, true);
    // The server kept the old setup: show that again, not the rejected change.
    if (drawerId === id) renderDrawer();
  }
}
const setPanelSoon = debounce((id, cfg) => setPanel(id, cfg, true), 200);
// Brightness and rotation are stored in the device's flash memory: they are
// sent once the user has settled (release of the slider, 700 ms after the
// last click), never for every step on the way. The preview may follow live.
const settleTimers = {};
function setPanelSettled(id, cfg, now) {
  clearTimeout(settleTimers[id]);
  const go = () => { delete settleTimers[id]; setPanel(id, cfg, true); };
  if (now) go(); else settleTimers[id] = setTimeout(go, 700);
}

function pulse(id) {
  const s = $(`[data-panel="${CSS.escape(id)}"] .screen`);
  if (!s) return;
  s.classList.remove("pulse");
  void s.offsetWidth;
  s.classList.add("pulse");
}

async function applyFanSet(panels, set) {
  const trio = (state.fanSets || {})[set] || (state.fanSets || {}).clean;
  for (let i = 0; i < panels.length; i++) {
    const cfg = { ...panels[i].config, mode: "design", design: trio[i % trio.length] };
    await setPanel(panels[i].id, cfg, true);
    pulse(panels[i].id);
  }
  renderPanels();
  toast(t("fanset.done"));
}

$("#btn-identify-all").addEventListener("click", () =>
  api("POST", "/api/identify").then(() => toast(t("identify.done"))).catch((e) => toast(e.message, true)));

// ---------- arrangement (controller schematic) ----------
// Shows the fan controller with its ports and the fans on them. The logical
// number of a fan decides the order of the screens in CaseHUD; it is kept in
// the CaseHUD config only, nothing is written to the devices.
let arranging = false, arrSel = null;
function arrangeOpen(on) {
  arranging = on;
  $("#tab-displays").classList.toggle("arranging", on);
  $("#arrange-view").hidden = !on;
  if (on) { showTab("displays"); arrSel = null; renderArrange(); window.scrollTo(0, 0); }
}
$("#btn-arrange").addEventListener("click", () => arrangeOpen(true));
$("#btn-arrange-back").addEventListener("click", () => arrangeOpen(false));
$("#set-open-arrange").addEventListener("click", () => arrangeOpen(true));
$("#btn-arrange-reset").addEventListener("click", async () => {
  try { await api("POST", "/api/fans/reset"); await refresh(); toast(t("arr.resetDone")); }
  catch (e) { toast(e.message, true); }
});

const arrWarn = (w) => t("arr.warn." + w.code, { port: w.port, n: w.n, lcdPerPort: (state.arrangement.limits || {}).lcdPerPort, lcdTotal: (state.arrangement.limits || {}).lcdTotal, fansPerPort: (state.arrangement.limits || {}).fansPerPort });

function arrFanNode(f) {
  if (!f.lcd || !f.id) { // a fan without display: an empty circle (shown once another module reports it)
    return el("span", { class: "arr-fan empty", title: t("arr.noLCD") });
  }
  const sel = arrSel === f.id;
  return el("button", { class: "arr-fan" + (sel ? " sel" : ""), "data-fan": f.id, "aria-pressed": sel ? "true" : "false",
    title: t("arr.fanTitle", { n: f.number, port: f.port, pos: f.index + 1 }), onclick: () => arrPick(f) },
  el("img", { alt: "", "data-preview": f.id, src: liveFrames[f.id] || null }),
  el("span", { class: "arr-no" }, String(f.number)));
}

function arrPick(f) {
  arrSel = arrSel === f.id ? null : f.id;
  if (arrSel) api("POST", `/api/panels/${encodeURIComponent(f.id)}/identify`).catch(() => {});
  renderArrange();
}

function arrPicker(fans, f) {
  const n = fans.length;
  const owner = (no) => fans.find((x) => x.number === no);
  return el("div", { class: "arr-picker" },
    el("h3", {}, t("arr.pick", { port: f.port, pos: f.index + 1 })),
    el("p", { class: "sub" }, t("arr.pick.sub")),
    el("div", { class: "arr-nums" }, Array.from({ length: n }, (_, i) => i + 1).map((no) => {
      const o = owner(no);
      return el("button", { class: "pick" + (no === f.number ? " active" : ""), title: o && o.id !== f.id ? t("arr.swap", { port: o.port, pos: o.index + 1 }) : "",
        onclick: async () => {
          if (no === f.number) return;
          try { await api("POST", `/api/fans/${encodeURIComponent(f.id)}/number`, { number: no }); await refresh(); api("POST", `/api/panels/${encodeURIComponent(f.id)}/identify`).catch(() => {}); }
          catch (e) { toast(e.message, true); }
        } }, String(no));
    })),
    el("button", { class: "ghost", onclick: () => { arrSel = null; renderArrange(); } }, t("btn.close")));
}

function renderArrange() {
  if (!arranging) return;
  const a = state.arrangement || { ports: [], warnings: [], limits: {} };
  const fans = a.ports.flatMap((p) => p.fans).concat(a.loose || []).filter((f) => f.lcd && f.id);
  if (arrSel && !fans.some((f) => f.id === arrSel)) arrSel = null;
  $("#arr-count").textContent = fans.length ? t("arr.count", { n: fans.length }) : t("arr.none");
  $("#arr-warnings").replaceChildren(...(a.warnings || []).map((w) => el("div", { class: "banner warn" }, arrWarn(w))));

  const tlBlink = !!(state.lighting && state.lighting.tlActive);
  const rows = a.ports.flatMap((p) => {
    const hasLCD = p.fans.some((f) => f.lcd && f.id);
    const nodes = p.fans.length ? p.fans.map(arrFanNode) : [0, 1, 2].map(() => el("span", { class: "arr-fan empty" }));
    const chain = el("div", { class: "arr-chain" });
    nodes.forEach((n, i) => { if (i) chain.append(el("span", { class: "arr-link" })); chain.append(n); });
    const sel = p.fans.find((f) => f.id && f.id === arrSel);
    const row = el("div", { class: "arr-row", "data-port": p.port },
      el("div", { class: "arr-port" }, el("span", { class: "arr-line" }), el("span", { class: "arr-tag" + (p.lcdAllowed ? "" : " plain") }, String(p.port))),
      el("div", {},
        chain,
        p.lcdAllowed ? null : el("div", { class: "arr-note" }, t("arr.port1"))),
      el("button", { class: "ghost arr-blink", disabled: !hasLCD && !tlBlink, title: hasLCD ? "" : tlBlink ? t("arr.blink.fw") : t("arr.blink.none"),
        // Ports with LCD fans identify them on their screens; other ports use the controller's own blink (TL lighting on).
        onclick: () => api("POST", hasLCD ? `/api/identify?port=${p.port}` : `/api/lighting/blink?port=${p.port}`).then(() => toast(t("identify.done"))).catch((e) => toast(e.message, true)) },
      t("btn.identify")));
    return sel ? [row, arrPicker(fans, sel)] : [row];
  });
  if ((a.loose || []).length) {
    const chain = el("div", { class: "arr-chain" });
    a.loose.forEach((f, i) => { if (i) chain.append(el("span", { class: "arr-link" })); chain.append(arrFanNode({ ...f, port: "?", index: i })); });
    rows.push(el("div", { class: "arr-row" }, el("div", { class: "arr-port" }, el("span", { class: "arr-tag plain" }, "?")),
      el("div", {}, chain, el("div", { class: "arr-note" }, t("arr.loose"))), el("span")));
  }
  const loosePick = (a.loose || []).find((f) => f.id === arrSel);
  if (loosePick) rows.push(arrPicker(fans, { ...loosePick, port: "?", index: a.loose.indexOf(loosePick) }));
  $("#arr-ports").replaceChildren(...rows);
  renderArrScan(a.scan);
}

// The fan list comes from the last read, kept in the config. The controller is
// asked only when the user presses the button.
let arrScanning = false;
function renderArrScan(sc) {
  const box = $("#arr-scan");
  if (!sc || !sc.available) { box.replaceChildren(); return; }
  const when = sc.at ? new Date(sc.at).toLocaleString(LANG === "de" ? "de-DE" : "en-GB") : "";
  const busy = arrScanning || sc.busy;
  box.replaceChildren(
    el("button", { class: "ghost", disabled: busy, onclick: async () => {
      if (!confirm(t("arr.scan.confirm"))) return;
      arrScanning = true; renderArrScan(sc);
      try { const a = await api("POST", "/api/arrangement/scan"); state.arrangement = a; }
      catch (e) { toast(e.message, true); }
      arrScanning = false; renderArrange();
    } }, t(sc.at ? "arr.scan.again" : "arr.scan")),
    el("small", { class: "arr-note" }, sc.live ? t("arr.scan.live") : t("arr.scan.hint")),
    el("small", { class: "arr-note" }, when ? t("arr.scan.at", { when }) : t("arr.scan.never")));
}

// Live previews: one event stream carries every frame the panels show, so
// animations look as smooth here as on the LCDs.
let live = null;
const liveFrames = {};
function startLive() {
  if (live) return;
  live = new EventSource(url("/api/live"));
  live.onmessage = (ev) => {
    const f = JSON.parse(ev.data);
    const bin = atob(f.jpg);
    const bytes = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    const src = URL.createObjectURL(new Blob([bytes], { type: "image/jpeg" }));
    const old = liveFrames[f.id];
    liveFrames[f.id] = src;
    for (const img of $$("img[data-preview]")) if (img.dataset.preview === f.id) img.src = src;
    if (old) setTimeout(() => URL.revokeObjectURL(old), 1000); // after the swap has painted
  };
  // EventSource reconnects by itself after errors.
}
function stopLive() { if (live) { live.close(); live = null; } }
document.addEventListener("visibilitychange", () => (document.hidden ? stopLive() : startLive()));
startLive();
// New preview images show the latest frame right away.
new MutationObserver(() => {
  for (const img of $$("img[data-preview]")) {
    const src = liveFrames[img.dataset.preview];
    if (src && img.src !== src) img.src = src;
  }
}).observe(document.body, { childList: true, subtree: true });

// ---------- drawer ----------
let drawerId = null;
function openDrawer(id) {
  drawerId = id;
  $("#drawer").hidden = false;
  $("#scrim").hidden = false;
  document.body.classList.add("drawer-open");
  $$(".panel-card").forEach((c) => c.classList.toggle("selected", c.dataset.panel === id));
  renderDrawer();
}
function closeDrawer() {
  drawerId = null;
  $("#drawer").hidden = true;
  $("#scrim").hidden = true;
  document.body.classList.remove("drawer-open");
  $$(".panel-card").forEach((c) => c.classList.remove("selected"));
}
$("#dr-close").addEventListener("click", closeDrawer);
$("#scrim").addEventListener("click", closeDrawer);
document.addEventListener("keydown", (e) => {
  if (e.key !== "Escape") return;
  if (!$("#menu").hidden) $("#menu").hidden = true;
  else if (drawerId) closeDrawer();
});

function renderDrawer() {
  const p = state.panels.find((x) => x.id === drawerId);
  if ($("#dr-content").dataset.for !== drawerId) { delete $("#dr-content").dataset.scrolled; $("#dr-content").dataset.for = drawerId; }
  if (!p) { closeDrawer(); return; }
  const cfg = { ...p.config };
  const prev = $("#dr-preview");
  prev.className = "drawer-preview" + (p.round ? " round" : "") + (isWideP(p) ? " wide" : "") + (isTallP(p) ? " tall" : "");
  prev.style.aspectRatio = ratio(p.width, p.height);
  prev.querySelector("img").dataset.preview = p.id;
  $("#dr-name").textContent = `${isTL(p) ? "" : t("display.n", { n: panelNumber(p) })}${isTL(p) ? shortName(p) : (cfg.name ? " · " + cfg.name : "")}`;
  const label = $("#dr-label");
  label.value = cfg.name || "";
  label.onchange = () => {
    cfg.name = label.value.trim();
    setPanel(p.id, cfg, true).then(() => { renderPanels(); renderDrawer(); });
  };
  $("#dr-identify").onclick = () => api("POST", `/api/panels/${encodeURIComponent(p.id)}/identify`).then(() => toast(t("identify.done"))).catch((e) => toast(e.message, true));
  $("#dr-meta").textContent = `${deviceLabel(p).replace(/ \(.*\)$/, "")} · ${p.width}×${p.height}`;

  const change = (patch) => { Object.assign(cfg, patch); setPanel(p.id, cfg); renderDrawerContent(p, cfg, change); renderModes(); $("#dr-theme-field").hidden = cfg.mode !== "design"; };

  const renderModes = () => {
    $("#dr-mode").replaceChildren(...["design", "media", "off"].map((m) =>
      el("button", { class: cfg.mode === m ? "active" : "", onclick: () => change({ mode: m }) }, t("mode." + m))));
  };
  renderModes();
  renderDrawerContent(p, cfg, change);

  const bri = $("#dr-bri");
  bri.value = cfg.brightness;
  setRangeFill(bri);
  $("#dr-bri-val").textContent = cfg.brightness + "%";
  bri.oninput = () => {
    cfg.brightness = +bri.value;
    setRangeFill(bri);
    $("#dr-bri-val").textContent = cfg.brightness + "%";
    setPanelSettled(p.id, cfg, false); // keyboard steps and fallback; the release sends right away
  };
  bri.onchange = () => setPanelSettled(p.id, cfg, true);
  $("#dr-theme-field").hidden = cfg.mode !== "design";
  $("#dr-theme").replaceChildren(...["dark", "light"].map((th) =>
    el("button", { class: (cfg.theme || "dark") === th ? "active" : "", onclick: () => { change({ theme: th === "dark" ? "" : th }); renderDrawer(); } }, t("theme." + th))));
  const renderRot = () => $("#dr-rot").replaceChildren(...[0, 90, 180, 270].map((r) =>
    el("button", { class: cfg.rotation === r ? "active" : "", onclick: () => {
      cfg.rotation = r; renderRot(); // the device is written once the clicks stop
      setPanelSettled(p.id, cfg, false);
    } }, r + "°")));
  renderRot();
  $("#dr-rot-hint").hidden = !isBarP(p);
  // The offline clock is a flag the display keeps; it is written only when this switch is flipped.
  const oc = $("#dr-oclock");
  $("#dr-oclock-field").hidden = p.offlineClock === undefined;
  oc.checked = !!p.offlineClock;
  oc.onchange = () => api("POST", `/api/panels/${encodeURIComponent(p.id)}/offline-clock`, { on: oc.checked })
    .then(() => { p.offlineClock = oc.checked; toast(t("saved")); }).catch((e) => { oc.checked = !oc.checked; toast(e.message, true); });
  $("#dr-fw-field").hidden = !p.firmware; // read only, shown when the display reported one
  $("#dr-fw").textContent = p.firmware || "";
  $("#dr-fw-note").replaceChildren(...fwNoteNodes(p.firmwareLatest));
  const fl = $("#dr-flip");
  $("#dr-flip-field").hidden = p.deviceFlip === undefined;
  fl.checked = !!p.deviceFlip;
  fl.onchange = () => api("POST", `/api/panels/${encodeURIComponent(p.id)}/device-flip`, { on: fl.checked })
    .then(() => { p.deviceFlip = fl.checked; toast(t("saved")); }).catch((e) => { fl.checked = !fl.checked; toast(e.message, true); });

  const all = $("#dr-apply-all");
  const d0 = state.designs.find((x) => x.id === cfg.design);
  // A square design would be cut off on round fans: only apply where it fits.
  // A grid layout (all fields) goes to bar displays only, never to single fans.
  const targets = state.panels.filter((q) => (cfg.grid ? isBarP(q) && (q.grids || []).some((g) => g.id === cfg.grid) : cfg.mode !== "design" || !d0 || designFits(d0, q)));
  all.hidden = state.panels.length < 2 || targets.length < 2;
  all.textContent = cfg.grid ? t("btn.applyAllBars", { n: targets.length })
    : t("btn.applyAll." + (cfg.mode === "media" ? "media" : cfg.mode === "off" ? "off" : "design"), { n: targets.length });
  all.title = cfg.grid ? t("btn.applyAllBars.sub") : "";
  all.onclick = async () => {
    for (const q of targets) await setPanel(q.id, { ...cfg }, true);
    targets.forEach((q) => pulse(q.id));
    renderPanels();
    toast(targets.length === state.panels.length ? t("applied.all") : t("applied.some", { n: targets.length }));
  };
}

// ---------- grid layout (bar displays) ----------
const SET_PREFIX = { clean: "", neon: "neon-", retro: "retro-", cyberpunk: "cyber-" };
// The selected field is remembered per display, so a second bar starts clean.
const selFields = new Map();
const getSel = (p) => selFields.get(p.id) || 0;
const setSel = (p, i) => selFields.set(p.id, i);

// The design set the panel mostly uses, for new fields and templates.
function currentSet(cfg) {
  const id = cfg.grid ? (cfg.tiles || []).map((x) => x.design).find(Boolean) : cfg.design;
  return (designById(id) || {}).set || "clean";
}

// A design for a new field of the given shape.
function defaultTile(cell, upright, set, i) {
  const pre = SET_PREFIX[set] || "";
  const squares = ((state.gridTemplates || {})[set] || (state.gridTemplates || {}).clean || [])[0];
  if (cell.w === cell.h && squares) return { mode: "design", design: squares.tiles[i % squares.tiles.length].design };
  if (cell.w === 480 && cell.h === 240) return { mode: "design", design: pre + "half-" + ["cpu", "gpu", "cpu-temp", "gpu-temp", "coolant", "pump", "power", "fps"][i % 8] };
  const tag = (upright ? "h" + cell.h : "w" + cell.w);
  return { mode: "design", design: pre + tag + "-" + ["cpu", "gpu", "coolant", "power", "fps"][i % 5] };
}

// Fields for preset g: keep what fits the new shape, fill up the rest.
function tilesFor(p, cfg, g) {
  const set = currentSet(cfg);
  const upright = p.height > p.width;
  return g.cells.map((cell, i) => {
    const old = (cfg.tiles || [])[i];
    const d = old && old.mode !== "media" && old.mode !== "off" ? designById(old.design) : null;
    if (old && (old.mode === "media" || old.mode === "off" || (d && fitsCell(d, cell)))) return { ...old };
    return defaultTile(cell, upright, set, i);
  });
}

function gridThumb(cells, w, h, active) {
  const ns = "http://www.w3.org/2000/svg";
  const s = document.createElementNS(ns, "svg");
  s.setAttribute("viewBox", `0 0 ${w} ${h}`);
  s.setAttribute("class", "grid-thumb" + (active ? " active" : ""));
  s.setAttribute("aria-hidden", "true");
  const g = 14; // gap in panel pixels
  cells.forEach((c) => {
    const r = document.createElementNS(ns, "rect");
    r.setAttribute("x", c.x + g / 2); r.setAttribute("y", c.y + g / 2);
    r.setAttribute("width", Math.max(1, c.w - g)); r.setAttribute("height", Math.max(1, c.h - g));
    r.setAttribute("rx", 22);
    s.append(r);
  });
  return s;
}

// The layout editor brings its own re-render; the drawer uses renderDrawerContent.
const rerenderTarget = (p, cfg, change) => (p.rerender || renderDrawerContent)(p, cfg, change);

function layoutSection(p, cfg, change) {
  const upright = p.height > p.width;
  const presets = p.grids || [];
  const set = currentSet(cfg);
  const sec = el("section", { class: "dr-section layout-section" },
    el("div", { class: "dr-section-head" }, el("b", {}, t("layout.title")), el("small", {}, t("layout.sub"))));

  // Ready-made compositions: one click and the display shows them.
  const tpls = ((state.gridTemplates || {})[set] || []);
  if (tpls.length) {
    sec.append(el("div", { class: "field" }, el("span", {}, t("layout.templates")),
      el("div", { class: "tpl-row" }, tpls.map((tp) =>
        el("button", { class: "ghost tpl", title: t("tpl." + tp.id + ".sub"), onclick: () => { setSel(p, 0); change({ grid: tp.grid, tiles: tp.tiles.map((x) => ({ ...x })) }); } },
          el("b", {}, t("tpl." + tp.id)), el("small", {}, t("grid." + tp.grid + ".short")))))));
  }

  // Preset choice with thumbnails; "single design" is the old behaviour.
  const single = [{ x: 0, y: 0, w: p.width, h: p.height }];
  const opts = p.layoutEditor ? presets : [{ id: "", cells: single }, ...presets];
  sec.append(el("div", { class: "field" }, el("span", {}, t("layout.presets")),
    el("div", { class: "preset-grid" + (upright ? " tall" : "") }, opts.map((g) =>
      el("button", { class: "preset" + ((cfg.grid || "") === g.id ? " active" : ""), "aria-pressed": String((cfg.grid || "") === g.id),
          onclick: () => {
            if ((cfg.grid || "") === g.id) return;
            setSel(p, 0);
            if (!g.id) change({ grid: "", tiles: [] });
            else change({ grid: g.id, tiles: tilesFor(p, cfg, g) });
          } },
        gridThumb(g.cells, p.width, p.height, (cfg.grid || "") === g.id),
        el("span", { class: "preset-label" }, t(g.id ? "grid." + g.id : "grid.single")),
        g.small ? el("small", { class: "preset-hint" }, t("grid.small")) : null)))));

  if (!cfg.grid) return sec;
  const g = presets.find((x) => x.id === cfg.grid);
  if (!g) return sec;
  const selField = Math.max(0, Math.min(getSel(p), g.cells.length - 1));
  setSel(p, selField);
  const pick = (i) => {
    setSel(p, i);
    rerenderTarget(p, cfg, change);
    requestAnimationFrame(() => { const h = $("#dr-content .field-head"); if (h) h.scrollIntoView({ block: "nearest" }); });
  };

  // The board: the live picture with a button over every field.
  const board = el("div", { class: "board" + (upright ? " tall" : ""), style: { aspectRatio: ratio(p.width, p.height) } },
    el("img", { alt: "", "data-preview": p.id, ...(p.layoutEditor ? { "data-draft": "1" } : {}) }),
    ...g.cells.map((c, i) => el("button", { class: "cell" + (i === selField ? " active" : ""), "aria-label": t("field.n", { n: i + 1 }), title: t("field.n", { n: i + 1 }) + " · " + tileLabel((cfg.tiles || [])[i]),
      style: { left: `${c.x / p.width * 100}%`, top: `${c.y / p.height * 100}%`, width: `${c.w / p.width * 100}%`, height: `${c.h / p.height * 100}%` },
      onclick: () => pick(i) }, el("span", {}, String(i + 1)))));
  // The list does the same for narrow windows (the board is too small there).
  const list = el("div", { class: "cell-list" }, g.cells.map((c, i) => {
    const label = tileLabel((cfg.tiles || [])[i]);
    return el("button", { class: "cell-row" + (i === selField ? " active" : ""), onclick: () => pick(i) },
      el("b", {}, String(i + 1)), el("span", {}, label), el("small", {}, `${c.w}×${c.h}`));
  }));
  sec.append(el("div", { class: "field" }, el("span", {}, t("layout.board")), board, list));
  return sec;
}

// What a field shows, in words.
function tileLabel(tl) {
  tl = tl || {};
  return tl.mode === "off" ? t("mode.off") : tl.mode === "media" ? (tl.media || t("content.none")) : designName(tl.design);
}

// Everything a real display has, for one field: design, theme, background, media.
function fieldEditor(p, cfg, change) {
  const g = (p.grids || []).find((x) => x.id === cfg.grid);
  const selField = Math.max(0, Math.min(getSel(p), g ? g.cells.length - 1 : 0));
  const cell = g && g.cells[selField];
  if (!cell) return el("div");
  const tile = { mode: "design", ...(cfg.tiles || [])[selField] };
  const tchange = (patch) => {
    // A mode switch brings a usable value along: the server refuses a field
    // that shows media or a design without saying which.
    if (patch.mode === "media" && !(patch.media || tile.media)) {
      if (!state.media.length) { toast(t("media.none") + t("media.addLink"), true); return; }
      patch = { ...patch, media: state.media[0].name };
    }
    if (patch.mode === "design" && !(patch.design || tile.design)) {
      const own = (g.cells[selField]);
      patch = { ...patch, design: defaultTile(own, p.height > p.width, currentSet(cfg), selField).design };
    }
    const tiles = (cfg.tiles || []).map((x) => ({ ...x }));
    tiles[selField] = { ...tiles[selField], ...patch };
    change({ tiles });
  };
  // The header stays in view while the designs below scroll, with the other
  // fields one tap away.
  const sec = el("section", { class: "dr-section field-editor" },
    el("div", { class: "dr-section-head field-head" },
      el("div", { class: "field-title" }, el("b", {}, `${t("field.n", { n: selField + 1 })} · ${tileLabel(tile)}`), el("small", {}, `${cell.w}×${cell.h}`)),
      el("div", { class: "field-pick", role: "group", "aria-label": t("layout.board") }, g.cells.map((c, i) =>
        el("button", { class: "ghost" + (i === selField ? " active" : ""), "aria-label": t("field.n", { n: i + 1 }), "aria-pressed": String(i === selField),
          onclick: () => { setSel(p, i); rerenderTarget(p, cfg, change); } }, String(i + 1))))));
  sec.append(el("div", { class: "field" }, el("span", {}, t("field.content")),
    el("div", { class: "seg small" }, ["design", "media", "off"].map((m) =>
      el("button", { class: tile.mode === m ? "active" : "", onclick: () => tchange({ mode: m }) }, t("mode." + m))))));
  if (tile.mode === "design") {
    sec.append(el("div", { class: "field" }, el("span", {}, t("field.scheme")),
      el("div", { class: "seg small" }, [["", t("theme.inherit")], ["dark", t("theme.dark")], ["light", t("theme.light")]].map(([v, label]) =>
        el("button", { class: (tile.theme || "") === v ? "active" : "", onclick: () => tchange({ theme: v }) }, label)))));
    const all = el("input", { type: "checkbox", class: "switch", onchange: (e) => { showAllDesigns = e.target.checked; rerenderTarget(p, cfg, change); } });
    all.checked = showAllDesigns;
    sec.append(el("label", { class: "field check" }, all, el("span", {}, el("b", {}, t("field.showAll")), el("small", {}, t("field.showAll.sub")))));
  }
  // The field's own screen: preview aspect, design filter and sizes follow its shape.
  const fp = { ...p, id: p.id + "#" + selField, width: cell.w, height: cell.h, round: false, cell: true };
  const view = { ...tile, theme: tile.theme || cfg.theme || "" };
  const inner = el("div", {});
  sec.append(inner);
  renderTarget(inner, fp, view, (patch) => {
    // The theme shown is the panel's when the field has none: never write it back.
    if ("theme" in patch && patch.theme === view.theme && !tile.theme) delete patch.theme;
    tchange(patch);
  });
  return sec;
}

// Design sets the viewer has unfolded in the drawer (touched = toggled once).
const openSets = new Set();
function renderDrawerContent(p, cfg, change) {
  change = change || ((patch) => { Object.assign(cfg, patch); setPanel(p.id, cfg); renderDrawer(); });
  const box = $("#dr-content");
  box.replaceChildren();
  if (isBarP(p) && cfg.mode === "design") {
    box.append(assignSection(p, cfg, change));
    if (cfg.grid) return;
  }
  renderTarget(box, p, cfg, change);
}

// renderTarget shows the pickers for what one screen (or one field of a grid)
// displays: design with theme-aware previews and background, or a media file.
function renderTarget(box, p, cfg, change) {
  if (cfg.mode === "design") {
    const wide = isWideP(p);
    const fits = state.designs.filter((d) => designFitsFor(d, p));
    // On a bar the designs made for it come first.
    const wideFirst = (a, b) => (wide ? isWideD(b) - isWideD(a) : 0);
    const groups = [...SETS.map((set) => [t("set." + set), fits.filter((d) => d.builtin && (d.set || "clean") === set).sort(wideFirst)]),
      [t("designs.custom"), fits.filter((d) => !d.builtin)]].filter(([, ds]) => ds.length);
    // Sets fold away so the list stays short; the set with the current
    // design starts open, and what the viewer opens stays open.
    const activeSet = groups.find(([, ds]) => ds.some((d) => d.id === cfg.design));
    box.append(el("div", { class: "field" }, el("span", {}, t("field.design")),
      ...groups.map(([title, ds]) => {
        const open = openSets.has(title) || (!openSets.touched && activeSet && activeSet[0] === title);
        const det = el("details", { class: "pick-group", open },
          el("summary", {}, el("span", {}, title), el("small", {}, ds.some((d) => d.id === cfg.design) ? "✓ " + designName(cfg.design) : String(ds.length))),
          el("div", { class: "pick-grid" + (wide ? " wide" : "") }, ds.map((d) =>
            el("button", { class: "pick" + (d.round && p.round ? " round" : "") + (cfg.design === d.id ? " active" : ""), title: designName(d.id), onclick: () => change({ design: d.id }) },
              el("img", { src: url(`/api/designs/${d.id}/preview?${previewSize(p, 160)}&theme=${cfg.theme || "dark"}`), alt: "", loading: "lazy", style: { aspectRatio: ratio(p.width, p.height) } }),
              el("span", { class: "pick-label" }, hy(designName(d.id)))))));
        if (open) openSets.add(title);
        det.addEventListener("toggle", () => { openSets.touched = true; det.open ? openSets.add(title) : openSets.delete(title); });
        return det;
      })));
    // A field of a grid keeps the layout board in view: no jumping to the active design.
    if (!p.cell) requestAnimationFrame(() => { const a = box.querySelector(".pick.active"); if (a && !box.dataset.scrolled) { a.scrollIntoView({ block: "nearest" }); box.dataset.scrolled = "1"; } });
    // Animated background behind the design.
    const bgs = [["", t("bg.none")], ...(state.backgrounds || []).map((b) => [b, t("bg." + b)]),
      ...state.media.filter((m) => m.kind !== "video" || state.ffmpeg).map((m) => ["media:" + m.name, m.name])];
    const strength = cfg.bg_strength || 35;
    const designSet = (state.designs.find((d) => d.id === cfg.design) || {}).set || "clean";
    const suggested = (state.setBackgrounds || {})[designSet];
    // The background is its own setting: it runs behind whatever design is
    // chosen, so it gets its own box.
    const sec = el("section", { class: "dr-section" },
      el("div", { class: "dr-section-head" }, el("b", {}, t("field.background")), el("small", {}, t("bg.sub"))),
      el("div", { class: "pick-grid bg-grid" + (wide ? " wide2" : "") }, bgs.map(([v, label]) =>
        el("button", { class: "pick" + (p.round ? " round" : "") + ((cfg.background || "") === v ? " active" : ""), title: label, onclick: () => change({ background: v }) },
          el("img", { src: url(`/api/designs/${cfg.design}/preview?${previewSize(p, 120)}&theme=${cfg.theme || "dark"}&bg=${encodeURIComponent(v)}&bgs=${Math.min(80, Math.round(strength * 1.6))}&bloom=${cfg.bg_bloom ? 1 : 0}`), alt: "", loading: "lazy", style: { aspectRatio: ratio(p.width, p.height) } }),
          el("span", { class: "pick-label" }, hy(v && v === suggested ? "★ " + label : label))))),
      suggested ? el("div", { class: "bg-hint" }, t("bg.suggested")) : null);
    box.append(sec);
    if (cfg.background) {
      const r = el("input", { type: "range", min: 10, max: 80, step: 5, value: strength });
      const val = el("b", {}, strength + "%");
      setRangeFill(r);
      r.oninput = () => { setRangeFill(r); val.textContent = r.value + "%"; };
      r.onchange = () => change({ bg_strength: +r.value });
      sec.append(el("div", { class: "field" }, el("span", {}, t("field.bgStrength"), " ", val), r));
      const bloom = el("input", { type: "checkbox", class: "switch", onchange: (e) => change({ bg_bloom: e.target.checked }) });
      bloom.checked = !!cfg.bg_bloom;
      sec.append(el("label", { class: "field check" }, bloom, el("span", {}, el("b", {}, t("field.bloom")), el("small", {}, t("field.bloom.sub")))));
    }
  } else if (cfg.mode === "media") {
    if (!state.media.length) {
      box.append(el("p", { class: "muted" }, t("media.none"),
        el("button", { class: "link", onclick: () => { closeDrawer(); $('[data-tab="media"]').click(); } }, t("media.addLink"))));
    } else {
      box.append(el("div", { class: "field" }, el("span", {}, t("field.file")),
        el("div", { class: "pick-grid" }, state.media.map((m) => {
          const th = el("div", { class: "thumb" }, m.kind === "video" ? "▶" : null);
          if (m.kind !== "video") th.style.backgroundImage = `url("${url(`/api/media/${encodeURIComponent(m.name)}`)}")`;
          return el("button", { class: "pick" + (cfg.media === m.name ? " active" : ""), onclick: () => change({ media: m.name }) },
            th, el("span", { class: "pick-label" }, m.name));
        }))));
    }
  }
}

// ---------- designs ----------
function showMenu(anchor, title, items) {
  const m = $("#menu");
  m.replaceChildren(el("div", { class: "menu-title" }, title),
    ...items.map(([label, fn]) => el("button", { onclick: () => { m.hidden = true; fn(); } }, label)));
  m.hidden = false;
  m.style.left = "0px";
  const r = anchor.getBoundingClientRect();
  const top = Math.max(10, Math.min(r.bottom + 6, innerHeight - m.offsetHeight - 10));
  Object.assign(m.style, { left: Math.max(10, Math.min(r.left, document.documentElement.clientWidth - m.offsetWidth - 10)) + "px", top: top + "px" });
}
document.addEventListener("pointerdown", (e) => {
  if (!$("#menu").hidden && !$("#menu").contains(e.target) && !e.target.closest("[data-menu]")) $("#menu").hidden = true;
});

function designCard(d) {
  return el("div", { class: "card design-card" + (d.round ? " round" : "") + (isWideD(d) ? " wide" : "") },
    el("div", { class: "thumb-wrap" }, el("img", { src: url(`/api/designs/${d.id}/preview?w=${isWideD(d) ? 720 : 360}&theme=${galleryTheme}&_=${Date.now()}`), alt: "", loading: "lazy", style: { aspectRatio: ratio(d.w, d.h) } })),
    el("div", { class: "meta" }, designName(d.id)),
    el("div", { class: "row-buttons" },
      el("button", { class: "ghost", onclick: () => openEditor(d.id, d.builtin) }, d.builtin ? t("btn.customize") : t("btn.edit")),
      d.builtin ? null : el("button", { class: "ghost icon-btn", title: t("btn.duplicate"), "aria-label": t("btn.duplicate"), onclick: () => openEditor(d.id, true) }, "⧉"),
      d.builtin ? null : el("button", { class: "danger icon-btn", title: t("btn.delete"), "aria-label": t("btn.delete"), onclick: () => deleteDesign(d) }, "✕")));
}

let galleryTheme = "dark";
function renderGalleryTheme() {
  $("#gallery-theme").replaceChildren(...["dark", "light"].map((th) =>
    el("button", { class: galleryTheme === th ? "active" : "", onclick: () => { galleryTheme = th; renderDesigns(); } }, t("theme." + th))));
}

// Preview size (query) that shows a design as the panel p would: its own
// aspect ratio, wide bars in a bigger image so text stays readable.
function previewSize(p, w) {
  if (!isWideP(p) && !p.cell) return `w=${w}`;
  const ww = isWideP(p) ? w * 2 : w;
  return `w=${ww}&h=${Math.round(ww * p.height / p.width)}`;
}

const SETS = ["clean", "neon", "retro", "cyberpunk"];

function renderDesigns() {
  renderGalleryTheme();
  const custom = state.designs.filter((d) => !d.builtin);
  $("#designs-sets").replaceChildren(...SETS.flatMap((set) => {
    const all = state.designs.filter((d) => d.builtin && (d.set || "clean") === set);
    const ds = all.filter((d) => !isFieldDesign(d.id));
    const fd = all.filter((d) => isFieldDesign(d.id));
    if (!all.length) return [];
    return [el("h3", { class: "section" }, el("span", {}, t("set." + set),
      el("span", { class: "count" }, t("set." + set + ".sub")))),
      el("div", { class: "grid designs" }, ds.map(designCard)),
      fd.length ? el("details", { class: "field-designs" }, el("summary", { class: "section" }, el("span", {}, t("designs.fields"), el("span", { class: "count" }, String(fd.length)))),
        el("p", { class: "sub" }, t("designs.fields.sub")),
        el("div", { class: "grid designs" }, fd.map(designCard))) : null];
  }));
  $("#designs-custom").replaceChildren(...custom.map(designCard));
  $("#custom-head").hidden = !custom.length;
}

async function deleteDesign(d) {
  if (!confirm(t("confirm.deleteDesign", { name: d.name }))) return;
  try { await api("DELETE", `/api/designs/${d.id}`); await refresh(); }
  catch (e) { toast(e.message, true); }
}
$("#btn-new-design").addEventListener("click", () => openEditor("loop", true, true));


// ---------- assigning a layout to a display (drawer) ----------
const layoutName = (l) => (l.builtin ? `${t("tpl." + l.tpl)} · ${t("set." + l.set)}` : l.name);
const layoutById = (id) => (state.layouts || []).find((l) => l.id === id);
const layoutUsers = (id) => state.panels.filter((q) => q.config.layout === id).length;

function assignSection(p, cfg, change) {
  const sec = el("section", { class: "dr-section assign-section" },
    el("div", { class: "dr-section-head" }, el("b", {}, t("assign.title")), el("small", {}, t("assign.sub"))));
  const list = el("div", { class: "assign-list" });
  list.append(el("button", { class: "ghost assign-opt" + (!cfg.grid ? " active" : ""), "aria-pressed": String(!cfg.grid),
    onclick: () => { if (cfg.grid) change({ layout: "", grid: "", tiles: [] }); } }, t("assign.single")));
  const own = cfg.grid && !cfg.layout;
  if (own) {
    list.append(el("div", { class: "assign-row" },
      el("button", { class: "ghost assign-opt active", "aria-pressed": "true" }, t("assign.custom")),
      el("button", { class: "ghost", onclick: () => openLayoutEditor({ from: { grid: cfg.grid, tiles: cfg.tiles }, assign: p.id }) }, t("assign.save"))));
  }
  sec.append(list);

  const all = (state.layouts || []).filter((l) => (p.grids || []).some((g) => g.id === l.grid));
  const groups = [[t("layouts.mine"), all.filter((l) => !l.builtin)],
    ...SETS.map((set) => [t("set." + set), all.filter((l) => l.builtin && l.set === set)])].filter(([, ls]) => ls.length);
  const activeGroup = groups.find(([, ls]) => ls.some((l) => l.id === cfg.layout));
  sec.append(el("div", { class: "field" }, el("span", {}, t("assign.layouts")),
    ...groups.map(([title, ls]) => {
      const open = openSets.has("L" + title) || (!openSets.touched && activeGroup && activeGroup[0] === title);
      const det = el("details", { class: "pick-group", open },
        el("summary", {}, el("span", {}, title), el("small", {}, ls.some((l) => l.id === cfg.layout) ? "✓ " + layoutName(layoutById(cfg.layout)) : String(ls.length))),
        el("div", { class: "pick-grid layout-pick" }, ls.map((l) =>
          el("button", { class: "pick" + (cfg.layout === l.id ? " active" : ""), title: layoutName(l),
            onclick: () => change({ layout: l.id, grid: l.grid, tiles: l.tiles.map((x) => ({ ...x })) }) },
            el("img", { src: url(`/api/layouts/${encodeURIComponent(l.id)}/preview?w=480&theme=${cfg.theme || "dark"}&_=${layoutStamp}`), alt: "", loading: "lazy" }),
            el("span", { class: "pick-label" }, hy(layoutName(l)))))));
      if (open) openSets.add("L" + title);
      det.addEventListener("toggle", () => { openSets.touched = true; det.open ? openSets.add("L" + title) : openSets.delete("L" + title); });
      return det;
    })));
  if (cfg.layout && layoutById(cfg.layout)) {
    sec.append(el("button", { class: "ghost", onclick: () => openLayoutEditor({ id: cfg.layout }) }, t("assign.edit")));
  }
  return sec;
}

// ---------- layouts (Designs tab: workshop) ----------
let layoutStamp = Date.now(); // changes when a layout is saved, so previews reload
let designsView = "designs";
function renderDesignsView() {
  $("#designs-view").replaceChildren(...["designs", "layouts"].map((v) =>
    el("button", { class: designsView === v ? "active" : "", onclick: () => { designsView = v; renderDesignsView(); } }, t("designs.view." + v))));
  $("#designs-pane").hidden = designsView !== "designs";
  $("#layouts-pane").hidden = designsView !== "layouts";
  $("#designs-actions").hidden = designsView !== "designs";
  $("#btn-new-layout").hidden = designsView !== "layouts";
}

function layoutCard(l) {
  const n = layoutUsers(l.id);
  return el("div", { class: "card layout-card" },
    el("div", { class: "thumb-wrap" }, el("img", { src: url(`/api/layouts/${encodeURIComponent(l.id)}/preview?w=720&theme=${galleryTheme}&_=${layoutStamp}`), alt: "", loading: "lazy" })),
    el("div", { class: "meta" }, layoutName(l),
      el("small", {}, `${t("grid." + l.grid + ".short")} · ${n ? (n === 1 ? t("layouts.used1") : t("layouts.used", { n })) : t("layouts.unused")}`)),
    el("div", { class: "row-buttons" },
      el("button", { class: "ghost", onclick: () => openLayoutEditor({ id: l.id }) }, l.builtin ? t("btn.customize") : t("btn.edit")),
      l.builtin ? null : el("button", { class: "ghost icon-btn", title: t("btn.duplicate"), "aria-label": t("btn.duplicate"), onclick: () => openLayoutEditor({ id: l.id, copy: true }) }, "⧉"),
      l.builtin ? null : el("button", { class: "danger icon-btn", title: t("btn.delete"), "aria-label": t("btn.delete"), onclick: () => deleteLayout(l) }, "✕")));
}

function renderLayouts() {
  renderDesignsView();
  const ls = state.layouts || [];
  const mine = ls.filter((l) => !l.builtin);
  const head = (title, count) => el("h3", { class: "section" }, el("span", {}, title, count != null ? el("span", { class: "count" }, String(count)) : null));
  $("#layouts-list").replaceChildren(
    head(t("layouts.mine"), mine.length),
    mine.length ? el("div", { class: "grid layouts" }, mine.map(layoutCard)) : el("p", { class: "layouts-empty" }, t("layouts.empty")),
    ...SETS.flatMap((set) => {
      const bs = ls.filter((l) => l.builtin && l.set === set);
      return bs.length ? [el("h3", { class: "section" }, el("span", {}, t("layouts.builtin") + " · " + t("set." + set))),
        el("div", { class: "grid layouts" }, bs.map(layoutCard))] : [];
    }));
}

async function deleteLayout(l) {
  if (!confirm(t("confirm.deleteLayout", { name: layoutName(l) }))) return;
  try { await api("DELETE", `/api/layouts/${encodeURIComponent(l.id)}`); layoutStamp = Date.now(); await refresh(); toast(t("le.deleted")); }
  catch (e) { toast(e.message, true); }
}
$("#btn-new-layout").addEventListener("click", () => openLayoutEditor({}));

// The editor works on a copy: nothing changes until it is saved.
let le = null;
function newLayoutId(name) {
  const slug = name.toLowerCase().normalize("NFKD").replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 40) || "layout";
  let id = slug, i = 2;
  while (layoutById(id)) id = `${slug}-${i++}`;
  return id;
}
function openLayoutEditor(o) {
  // switch to the workshop first: the editor belongs to the Designs tab
  designsView = "layouts";
  $('[data-tab="designs"]').click();
  renderDesignsView();
  const src = o.id ? layoutById(o.id) : null;
  let tpl = null;
  if (!src && !o.from) tpl = ((state.gridTemplates || {}).clean || [])[0];
  const grid = src ? src.grid : o.from ? o.from.grid : tpl ? tpl.grid : "4";
  const tiles = (src ? src.tiles : o.from ? o.from.tiles : tpl ? tpl.tiles : []).map((x) => ({ ...x }));
  const keep = src && !src.builtin && !o.copy; // editing one of your own: same id
  le = {
    id: keep ? src.id : null,
    name: keep ? src.name : src ? (src.builtin ? layoutName(src) : t("le.copy", { name: src.name })) : o.from ? t("le.custom") : t("le.new"),
    cfg: { mode: "design", grid, tiles },
    assign: o.assign || null,
    dirty: !keep,
    p: { id: "layout-editor", name: "", width: 1920, height: 480, round: false, layoutEditor: true, grids: state.barGrids || [] },
  };
  le.p.rerender = renderLayoutEditor;
  selFields.set("layout-editor", 0);
  $("#le-name").value = le.name;
  $("#layout-editor").hidden = false;
  document.body.classList.add("editing");
  renderLayoutEditor();
}
function closeLayoutEditor(force) {
  if (!force && le && le.dirty && !confirm(t("le.discard"))) return;
  le = null;
  $("#layout-editor").hidden = true;
  document.body.classList.remove("editing");
}
$("#le-close").addEventListener("click", () => closeLayoutEditor(false));
$("#le-name").addEventListener("input", () => { if (le) { le.name = $("#le-name").value; le.dirty = true; } });

const refreshLayoutPreview = debounce(async () => {
  if (!le) return;
  try {
    const r = await fetch("/api/layouts/preview?w=960&theme=", { method: "POST", headers: { "X-Token": TOKEN }, body: JSON.stringify({ grid: le.cfg.grid, tiles: le.cfg.tiles }) });
    if (!r.ok) return;
    const src = URL.createObjectURL(await r.blob());
    for (const img of $$("img[data-draft]")) { const old = img.src; img.src = src; if (old.startsWith("blob:")) setTimeout(() => URL.revokeObjectURL(old), 500); }
  } catch { /* the next change redraws */ }
}, 250);

function renderLayoutEditor() {
  if (!le) return;
  const { p, cfg } = le;
  const change = (patch) => { Object.assign(cfg, patch); le.dirty = true; renderLayoutEditor(); };
  const body = $("#le-body");
  const keepScroll = body.scrollTop;
  body.replaceChildren(layoutSection(p, cfg, change));
  if (cfg.grid) body.append(fieldEditor(p, cfg, change));
  body.scrollTop = keepScroll;
  refreshLayoutPreview();
}

$("#le-save").addEventListener("click", async () => {
  if (!le) return;
  const name = le.name.trim();
  if (!name) { toast(t("le.nameRequired"), true); $("#le-name").focus(); return; }
  const id = le.id || newLayoutId(name);
  try {
    await api("PUT", `/api/layouts/${encodeURIComponent(id)}`, { name, grid: le.cfg.grid, tiles: le.cfg.tiles });
    const target = le.assign;
    const used = state.panels.filter((q) => q.config.layout === id).length;
    layoutStamp = Date.now();
    closeLayoutEditor(true);
    await refresh();
    if (target) {
      const q = state.panels.find((x) => x.id === target);
      const l = layoutById(id);
      if (q && l) { await setPanel(q.id, { ...q.config, layout: id, grid: l.grid, tiles: l.tiles.map((x) => ({ ...x })) }, true); await refresh(); }
    }
    toast(used && !target ? t("le.savedUsed", { n: used }) : t("le.saved"));
  } catch (e) { toast(e.message, true); }
});

// ---------- scenes, automation, rotation ----------
// Scenes are saved display setups. The automation shows one while a game
// runs; the rotation cycles through the ones marked "in rotation".
let rot = { config: {}, status: {} };
let auto = { config: { rules: [] }, status: { activeRule: -1, pending: -1 }, gaming: {} };
let renamingScene = null;

async function loadRotation() {
  try { rot = await api("GET", "/api/rotation"); } catch { return; }
  try { auto = await api("GET", "/api/automation"); } catch { /* keep the old one */ }
  auto.config.rules = auto.config.rules || [];
  renderScenes(); renderAutomation(); renderRotation(); renderStateChip();
}
// Displays tab: one line says when the automation or rotation is in charge.
function renderStateChip() {
  const chip = $("#state-chip");
  const a = auto.config && auto.config.enabled && auto.status && auto.status.activeRule >= 0;
  const r = rot.config && rot.config.enabled && rot.status && rot.status.active;
  let text = "";
  if (a) text = auto.status.scene ? t("auto.banner.scene", { s: auto.status.scene }) : t("auto.banner.normal");
  else if (r) text = rot.status.gaming ? t("chip.rot.gaming") : rot.status.scene ? t("chip.rot.scene", { s: rot.status.scene }) : t("chip.rot");
  chip.hidden = !text || !state.panels.length;
  chip.textContent = text;
  chip.title = t("chip.open");
}
$("#state-chip").addEventListener("click", () => showTab("scenes"));
// The engine's own view changes by itself (a game starts): poll it cheaply.
async function pollAutomation() {
  if (document.hidden || !$("#tab-displays").classList.contains("active") && !$("#tab-scenes").classList.contains("active")) return;
  if (document.activeElement && document.activeElement.closest("#tab-scenes")) return;
  const before = JSON.stringify([auto.status, auto.gaming]);
  try {
    const a = await api("GET", "/api/automation");
    const r = await api("GET", "/api/rotation");
    if (JSON.stringify([a.status, a.gaming]) === before) return;
    auto.status = a.status; auto.gaming = a.gaming; rot.status = r.status;
    renderScenes(); renderAutomation(); renderRotation(); renderStateChip();
  } catch { /* next time */ }
}
// The whole rotation config goes back in one piece, so it is read again right
// before writing: a scene saved meanwhile must not be wiped out by a stale
// copy. patch is an object or a function of the fresh config.
async function saveRotation(patch) {
  try {
    const fresh = (await api("GET", "/api/rotation")).config || {};
    const change = typeof patch === "function" ? patch(fresh) : patch;
    rot.config = { ...fresh, ...(change || {}) };
    await api("PUT", "/api/rotation", rot.config);
  } catch (e) { toast(e.message, true); }
  setTimeout(loadRotation, 400);
}
async function saveAutomation(patch) {
  Object.assign(auto.config, patch || {});
  // A rule still waiting for its game name stays local until it has one.
  const cfg = { ...auto.config, rules: auto.config.rules.filter((r) => r.when !== "game:") };
  try { await api("PUT", "/api/automation", cfg); } catch (e) { toast(e.message, true); }
  setTimeout(loadRotation, 300);
}
const scenesOf = () => rot.config.scenes || [];
const rulesUsing = (name) => (auto.config.rules || []).filter((r) => r.scene === name);

// Edge/WebView2 has no hyphenation dictionary offline (hyphens:auto does
// nothing, checked), so long German words get soft hyphens at word parts,
// so a label never breaks in the middle of a word without a hyphen.
const HY_PARTS = /(?<=\p{L}{3})(kreislauf|leistung|speicher|last|stellungen|zusammen|temperatur|auslastung|spannung|frequenz|durchfluss|kühlmittel|lüfter|drehzahl|übersicht|kühlung|wasser|grafik|gesamt|prozessor|hotspot|takt)/giu;
const hy = (s) => (LANG === "de" ? String(s).replace(HY_PARTS, "­$1") : String(s));

function chip(label, onRemove, extra) {
  return el("span", { class: "chip" }, extra || null, label, onRemove ? el("button", { class: "chip-x", title: t("btn.delete"), onclick: onRemove }, "✕") : null);
}

function renderScenes() {
  const has = state.panels.length > 0;
  $("#scenes-body").hidden = !has;
  $("#scenes-empty").hidden = has;
  if (!has) return;
  const scenes = scenesOf();
  const autoOn = auto.config.enabled && auto.status && auto.status.activeRule >= 0;
  const shown = autoOn ? auto.status.scene : (rot.status.active ? rot.status.scene : "");
  const box = $("#scene-list");
  if (!scenes.length) { box.replaceChildren(el("p", { class: "empty-note" }, t("scenes.none"))); return; }
  box.replaceChildren(...scenes.map((sc) => {
    const used = rulesUsing(sc.name);
    const row = el("div", { class: "scene-row" + (shown === sc.name ? " active" : "") });
    const name = renamingScene === sc.name
      ? el("input", { class: "scene-rename", value: sc.name, maxlength: "40", "aria-label": t("scenes.rename") })
      : el("b", { class: "scene-name" }, sc.name);
    if (renamingScene === sc.name) {
      const done = async (ok) => {
        if (renamingScene !== sc.name) return;
        const to = name.value.trim();
        renamingScene = null;
        if (ok && to && to !== sc.name) {
          try { await api("POST", "/api/scenes/rename", { from: sc.name, to }); } catch (e) { toast(e.message, true); }
          await loadRotation();
        } else renderScenes();
      };
      name.addEventListener("keydown", (e) => { if (e.key === "Enter") done(true); else if (e.key === "Escape") done(false); });
      name.addEventListener("blur", () => done(true));
      setTimeout(() => { name.focus(); name.select(); }, 0);
    }
    const badges = el("span", { class: "badges" },
      shown === sc.name ? el("span", { class: "badge on" }, t("scenes.showing")) : null,
      used.length ? el("span", { class: "badge", title: t("scenes.usedAuto") }, t("scenes.badge.auto")) : null,
      null);
    const inRot = el("input", { type: "checkbox", class: "switch", "aria-label": t("scenes.inRotation") });
    inRot.checked = !sc.no_rotation;
    inRot.addEventListener("change", () => saveRotation((c) => ({ scenes: (c.scenes || []).map((x) => (x.name === sc.name ? { ...x, no_rotation: !inRot.checked } : x)) })));
    row.append(
      el("div", { class: "scene-main" }, name, badges),
      el("label", { class: "scene-rot", title: t("scenes.inRotation.sub") }, inRot, el("span", {}, t("scenes.inRotation"))),
      el("div", { class: "scene-actions" },
        el("button", { class: "ghost", title: t("scenes.apply.sub"), onclick: () => applyScene(sc) }, t("scenes.apply")),
        el("button", { class: "ghost", onclick: () => { renamingScene = sc.name; renderScenes(); } }, t("scenes.rename")),
        el("button", { class: "danger icon-danger", title: t("btn.delete"), "aria-label": t("btn.delete") + ": " + sc.name, onclick: () => deleteScene(sc, used) }, trashIcon())));
    return row;
  }));
}

async function applyScene(sc) {
  if (!confirm(t("scenes.apply.confirm", { s: sc.name }))) return;
  try { await api("POST", "/api/scenes/apply", { name: sc.name }); toast(t("scenes.applied", { s: sc.name })); await refresh(); }
  catch (e) { toast(e.message, true); }
}
async function deleteScene(sc, used) {
  if (!confirm(t("scenes.delete.confirm", { s: sc.name }) + (used.length ? "\n\n" + t("scenes.delete.rules", { n: used.length }) : ""))) return;
  await saveRotation((c) => ({ scenes: (c.scenes || []).filter((x) => x.name !== sc.name) }));
}
const trashIcon = () => svg('<path d="M4 7h16M10 11v6M14 11v6M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12M9 7V4h6v3"/>');

$("#btn-save-scene").addEventListener("click", async () => {
  const name = $("#rot-scene-name").value.trim();
  if (!name) { $("#rot-scene-name").focus(); return; }
  if (scenesOf().some((s) => s.name === name) && !confirm(t("scenes.overwrite", { s: name }))) return;
  try { await api("POST", "/api/rotation/scene", { name }); $("#rot-scene-name").value = ""; toast(t("rot.saved", { s: name })); loadRotation(); }
  catch (e) { toast(e.message, true); }
});
$("#rot-scene-name").addEventListener("keydown", (e) => { if (e.key === "Enter") $("#btn-save-scene").click(); });
$("#btn-gaming-scene").addEventListener("click", async () => {
  if (scenesOf().some((s) => s.name === "Gaming") && !confirm(t("scenes.overwrite", { s: "Gaming" }))) return;
  try {
    await api("POST", "/api/scenes/gaming");
    toast(t("scenes.gaming.done"));
    await loadRotation();
    // Offer it to the automation right away when no rule reacts to games yet.
    if (!(auto.config.rules || []).some((r) => r.when === "game")) {
      auto.config.rules = [{ when: "game", scene: "Gaming" }, ...auto.config.rules];
      await saveAutomation({});
    }
  } catch (e) { toast(e.message, true); }
});

// ----- automation -----
const sameExeName = (a, b) => String(a).trim().toLowerCase() === String(b).trim().toLowerCase();
const exeOf = (w) => (w.startsWith("game:") ? w.slice(5) : "");
// Games seen running, newest first, for the "pick a program" list (kept in
// this browser only; the app itself needs no list).
function recentGames() {
  try { return JSON.parse(localStorage.getItem("casehud.games") || "[]").filter((x) => typeof x === "string"); } catch { return []; }
}
function rememberGame(name) {
  if (!name) return;
  const list = recentGames().filter((x) => !sameExeName(x, name));
  list.unshift(name);
  try { localStorage.setItem("casehud.games", JSON.stringify(list.slice(0, 12))); } catch { /* storage blocked */ }
}
function renderAutomation() {
  const c = auto.config, st = auto.status || {}, g = auto.gaming || {};
  const rules = c.rules || [];
  if (g.active && g.game) rememberGame(g.game);
  $("#auto-enabled").checked = !!c.enabled;
  $("#auto-body").classList.toggle("off", !c.enabled);
  const act = st.activeRule >= 0;
  $("#auto-status").textContent = !c.enabled ? t("auto.off")
    : act ? (st.scene ? t("auto.on.scene", { s: st.scene }) : t("auto.on.normal"))
    : st.pending >= 0 ? t("auto.soon") : st.pending === -2 ? t("auto.leaving") : t("auto.waiting");
  const ban = $("#auto-banner");
  ban.hidden = !(c.enabled && act);
  if (!ban.hidden) {
    ban.replaceChildren(
      el("b", {}, st.scene ? t("auto.banner.scene", { s: st.scene }) : t("auto.banner.normal")),
      el("span", {}, t("auto.banner.why", { n: st.activeRule + 1 }) + (st.game || g.game ? " · " + (st.game || g.game) : "")),
      el("span", { class: "sub" }, t(st.scene ? "auto.banner.manual" : "auto.banner.manual.normal", { s: st.scene || "" })),
      el("div", { class: "inline-form" },
        st.scene ? el("button", { class: "ghost", onclick: () => editAutoScene(st.scene) }, t("auto.banner.edit")) : null,
        el("button", { class: "ghost", onclick: () => saveAutomation({ enabled: false }) }, t("auto.banner.pause"))));
  }
  const scenes = scenesOf();
  const seen = [...new Set([...rules.map((r) => exeOf(r.when)).filter(Boolean), ...recentGames()])];
  $("#auto-rules").replaceChildren(...(rules.length ? rules.map((r, i) => {
    const specific = r.when.startsWith("game:");
    const missing = r.scene && !scenes.some((s) => s.name === r.scene);
    const live = c.enabled && st.activeRule === i;
    const upd = (patch) => { rules[i] = { ...rules[i], ...patch }; saveAutomation({ rules }); };
    const when = el("select", { class: "rule-when", "aria-label": t("auto.when"), onchange: (e) => {
      if (e.target.value === "game") upd({ when: "game" });
      else { rules[i] = { ...rules[i], when: "game:" }; renderAutomation(); setTimeout(() => { const x = $$("#auto-rules .rule-exe")[0]; if (x) x.focus(); }, 0); }
    } },
      el("option", { value: "game", selected: !specific }, t("auto.when.game")),
      el("option", { value: "game:", selected: specific }, t("auto.when.exe")));
    // A specific game: choose from the ones seen lately, or type the program name.
    let program = null;
    if (specific) {
      const exe = el("input", { class: "rule-exe", value: exeOf(r.when), placeholder: t("auto.exe.ph"), maxlength: "100", "aria-label": t("auto.exe"),
        onchange: (e) => { const v = e.target.value.trim(); if (v) upd({ when: "game:" + v }); } });
      const known = seen.filter((x) => !sameExeName(x, exeOf(r.when)));
      const pickList = known.length ? el("select", { class: "rule-pick", "aria-label": t("auto.pick"), onchange: (e) => { if (e.target.value) upd({ when: "game:" + e.target.value }); } },
        el("option", { value: "" }, t("auto.pick")), ...known.map((x) => el("option", { value: x }, x))) : null;
      program = el("div", { class: "rule-program" }, pickList, exe);
    }
    const scene = el("select", { class: "rule-scene", "aria-label": t("auto.then") },
      el("option", { value: "" }, t("auto.normal")),
      ...scenes.map((sc) => el("option", { value: sc.name, selected: r.scene === sc.name }, sc.name)),
      missing ? el("option", { value: r.scene, selected: true }, r.scene + " ⚠") : null);
    scene.addEventListener("change", () => upd({ scene: scene.value }));
    const on = el("input", { type: "checkbox", class: "switch", title: t("auto.ruleOn"), "aria-label": t("auto.ruleOn") });
    on.checked = !r.off;
    on.addEventListener("change", () => upd({ off: !on.checked }));
    const move = (d) => { [rules[i], rules[i + d]] = [rules[i + d], rules[i]]; saveAutomation({ rules }); };
    return el("div", { class: "rule" + (r.off ? " off" : "") + (live ? " active" : "") },
      el("b", { class: "rule-n", title: t("auto.order") }, String(i + 1)),
      el("div", { class: "rule-body" },
        el("div", { class: "rule-line" }, el("span", { class: "rule-word" }, t("auto.if")), when, program),
        el("div", { class: "rule-line" }, el("span", { class: "rule-word" }, t("auto.show")), scene,
          live ? el("span", { class: "chip-live" }, t("auto.live")) : null),
        missing ? el("small", { class: "warn-text" }, t("auto.missing")) : null),
      el("div", { class: "rule-actions" }, on,
        el("button", { class: "ghost icon-btn", disabled: i === 0, title: t("auto.up"), "aria-label": t("auto.up"), onclick: () => move(-1) }, "▲"),
        el("button", { class: "ghost icon-btn", disabled: i === rules.length - 1, title: t("auto.down"), "aria-label": t("auto.down"), onclick: () => move(1) }, "▼"),
        el("button", { class: "ghost icon-btn", title: t("btn.delete"), "aria-label": t("btn.delete"), onclick: () => { rules.splice(i, 1); saveAutomation({ rules }); } }, "✕")));
  }) : [el("p", { class: "empty-note" }, t("auto.none"))]));
  // What the "otherwise" case does.
  $("#auto-else-text").textContent = " " + (rot.config.enabled ? t("auto.else.rot") : t("auto.else.normal"));
  // Transparency: what the engine sees right now.
  $("#auto-detected").textContent = g.active
    ? t("auto.detected", { game: g.game || t("auto.unknown"), src: t("auto.src." + (g.source || "fps")) })
    : t("auto.detected.none");
  const add = $("#auto-add-detected");
  const known = g.active && g.game && !rules.some((r) => r.when.toLowerCase() === ("game:" + g.game).toLowerCase());
  add.hidden = !known;
  if (known) add.textContent = t("auto.addDetected", { game: g.game });
  add.onclick = () => saveAutomation({ enabled: true, rules: [{ when: "game:" + g.game, scene: "" }, ...rules] });
  // A wrongly detected program can be told it is no game (not for outside hints).
  const ign = c.ignore_games || [];
  const ib = $("#auto-ignore-detected");
  const canIgnore = g.active && g.source !== "external" && g.game && !ign.some((x) => sameExeName(x, g.game));
  ib.hidden = !canIgnore;
  ib.textContent = t("auto.ignore.btn");
  ib.onclick = () => saveAutomation({ ignore_games: [...ign, g.game] });
  $("#auto-ignore").replaceChildren(...(ign.length
    ? ign.map((x) => chip(x, () => saveAutomation({ ignore_games: ign.filter((y) => y !== x) })))
    : [el("small", { class: "muted" }, t("auto.ignore.none"))]));
  $("#auto-enter").value = String(c.enter_s || 3);
  $("#auto-leave").value = String(c.leave_s || 10);
}
$("#auto-enabled").addEventListener("change", (e) => saveAutomation({ enabled: e.target.checked }));
$("#auto-enter").addEventListener("change", (e) => saveAutomation({ enter_s: +e.target.value }));
$("#auto-leave").addEventListener("change", (e) => saveAutomation({ leave_s: +e.target.value }));
$("#auto-add").addEventListener("click", () => {
  const scene = scenesOf().some((s) => s.name === "Gaming") ? "Gaming" : "";
  saveAutomation({ rules: [...(auto.config.rules || []), { when: "game", scene }] });
});

// Edit a scene the automation is showing: the displays take it over as their
// normal setup; what they showed before is kept as a scene of its own.
async function editAutoScene(name) {
  if (!confirm(t("auto.edit.confirm", { s: name, b: t("auto.backup") }))) return;
  try {
    await api("POST", "/api/rotation/scene", { name: t("auto.backup"), no_rotation: true });
    await api("POST", "/api/scenes/apply", { name });
    await refresh();
    $("#rot-scene-name").value = name;
    showTab("scenes");
    $("#scene-card").scrollIntoView({ block: "center", behavior: "smooth" });
    toast(t("auto.edit.done", { s: name }));
  } catch (e) { toast(e.message, true); }
}

// ----- rotation -----
function renderRotation() {
  const c = rot.config, st = rot.status || {};
  $("#rot-enabled").checked = !!c.enabled;
  const held = auto.config.enabled && auto.status && auto.status.activeRule >= 0;
  $("#rot-status").textContent = !c.enabled ? t("rot.off") : held ? t("rot.held", { s: auto.status.scene || t("auto.normal") }) : st.active ? (st.scene ? t("rot.scene", { s: st.scene }) : t("rot.running")) : t("rot.waiting");
  $("#rot-mode").replaceChildren(...["scenes", "conveyor"].map((m) =>
    el("button", { class: (c.mode || "scenes") === m ? "active" : "", onclick: () => saveRotation({ mode: m }) }, t("rot.mode." + m))));
  $("#rot-interval").value = String(c.interval_s || 15);
  $("#rot-mode-hint").textContent = t("rot.mode." + (c.mode || "scenes") + ".hint");
  $("#rot-effect").replaceChildren(...(state.transitions || []).map((fx) => el("option", { value: fx, selected: c.effect === fx }, t("fx." + fx))));
  $("#rot-cascade").checked = !!c.cascade;
  $("#rot-scenes-box").hidden = (c.mode || "scenes") !== "scenes";
  $("#rot-conveyor-box").hidden = c.mode !== "conveyor";
  const inRot = scenesOf().filter((s) => !s.no_rotation);
  $("#rot-scenes").replaceChildren(...(inRot.length ? inRot.map((sc, i) => {
    const ch = chip(sc.name, null, el("b", { class: "chip-n" }, i + 1));
    if (c.enabled && !held && st.scene === sc.name) ch.classList.add("active");
    return ch;
  }) : [el("small", { class: "muted" }, t("rot.noscenes"))]));
  // Switched off, the options fold away: nothing to set until it runs.
  $("#rot-body").hidden = !c.enabled;
  $("#rot-order-box").hidden = !(c.mode === "conveyor" || c.cascade);
  const conv = c.conveyor || [];
  $("#rot-conveyor").replaceChildren(...(conv.length ? conv.map((id, i) =>
    chip(designName(id), () => saveRotation((cf) => ({ conveyor: (cf.conveyor || []).filter((_, j) => j !== i) })), el("b", { class: "chip-n" }, i + 1))) : [el("small", { class: "muted" }, t("rot.noconveyor"))]));
  $("#rot-add-design").replaceChildren(el("option", { value: "" }, t("rot.adddesign")),
    ...state.designs.map((d) => el("option", { value: d.id }, designName(d.id, true))));
  // Display order (top to bottom) for cascade and conveyor.
  const ids = state.panels.map((p) => p.id);
  const order = [...(c.order || []).filter((id) => ids.includes(id)), ...ids.filter((id) => !(c.order || []).includes(id))];
  $("#rot-order").replaceChildren(...order.map((id, i) => {
    const p = state.panels.find((x) => x.id === id);
    const move = (d) => { const o = [...order]; [o[i], o[i + d]] = [o[i + d], o[i]]; saveRotation({ order: o }); };
    return el("div", { class: "order-row" }, el("b", {}, i + 1), el("span", {}, shortName(p)),
      el("button", { class: "ghost icon-btn", disabled: i === 0, onclick: () => move(-1) }, "▲"),
      el("button", { class: "ghost icon-btn", disabled: i === order.length - 1, onclick: () => move(1) }, "▼"));
  }));
}
$("#rot-enabled").addEventListener("change", (e) => saveRotation({ enabled: e.target.checked }));
$("#rot-interval").addEventListener("change", (e) => saveRotation({ interval_s: +e.target.value }));
$("#rot-effect").addEventListener("change", (e) => saveRotation({ effect: e.target.value }));
$("#rot-cascade").addEventListener("change", (e) => saveRotation({ cascade: e.target.checked }));
$("#rot-add-design").addEventListener("change", (e) => {
  if (!e.target.value) return;
  const id = e.target.value;
  saveRotation((cf) => ({ conveyor: [...(cf.conveyor || []), id] }));
});
$("#rot-next").addEventListener("click", () => api("POST", "/api/rotation/next").then(() => setTimeout(loadRotation, 300)).catch((e) => toast(e.message, true)));
setInterval(pollAutomation, 3000);

// ---------- community gallery ----------
async function openCommunity() {
  const body = $("#community-body");
  body.replaceChildren(el("p", { class: "empty" }, t("comm.loading")));
  $("#community").showModal();
  let entries;
  try { entries = await api("GET", "/api/community"); }
  catch (e) {
    console.warn("community gallery:", e.message);
    body.replaceChildren(el("div", { class: "empty" }, el("p", {}, t("comm.error")),
      el("button", { class: "ghost", onclick: openCommunity }, t("comm.retry"))));
    return;
  }
  if (!entries.length) { body.replaceChildren(el("p", { class: "empty" }, t("comm.none"))); return; }
  body.replaceChildren(el("div", { class: "grid designs" }, entries.map((c) => {
    const imp = el("button", { class: "primary", onclick: async () => {
      imp.disabled = true;
      try {
        await api("POST", `/api/community/${c.id}/import`);
        toast(t("comm.imported", { name: c.name }));
        await refresh();
        imp.textContent = t("comm.done");
      } catch (e) { toast(e.message, true); imp.disabled = false; }
    } }, t("comm.import"));
    return el("div", { class: "card design-card" + (c.round ? " round" : "") },
      el("div", { class: "thumb-wrap" }, el("img", { src: url(`/api/community/${c.id}/preview?w=360&theme=${galleryTheme}`), alt: "", loading: "lazy" })),
      el("div", { class: "meta", title: c.description || "" }, c.name, el("small", {}, t("comm.by", { a: c.author || "?" }))),
      el("div", { class: "row-buttons" }, imp));
  })));
}
$("#btn-community").addEventListener("click", openCommunity);
$("#community-close").addEventListener("click", () => $("#community").close());
$("#community").addEventListener("click", (e) => { if (e.target === $("#community")) $("#community").close(); });

// Designs are plain JSON: share them, or let an AI write one and import it.
$("#design-import").addEventListener("change", async (e) => {
  const f = e.target.files[0];
  e.target.value = "";
  if (!f) return;
  try {
    const l = JSON.parse(await f.text());
    if (!l || !Array.isArray(l.widgets) || !(l.width > 0) || !(l.height > 0)) throw new Error();
    l.name = l.name || f.name.replace(/\.json$/i, "");
    await openEditor("", false, false, l);
    toast(t("import.done"));
  } catch { toast(t("import.bad"), true); }
});

// ---------- media ----------
function renderMedia() {
  $("#ffmpeg-hint").hidden = state.ffmpeg;
  $("#media-hint").hidden = !state.media.length;
  $("#media").replaceChildren(...state.media.map((m) => {
    const thumb = el("div", { class: "thumb" }, m.kind === "video" ? "▶" : null);
    if (m.kind !== "video") thumb.style.backgroundImage = `url("${url(`/api/media/${encodeURIComponent(m.name)}`)}")`;
    return el("div", { class: "card media-card" }, thumb,
      el("div", { class: "meta" },
        el("div", { class: "meta-text" }, el("div", { class: "name", title: m.name }, m.name), el("div", { class: "kind" }, m.kind)),
        el("button", { class: "danger icon-btn", title: t("btn.delete"), "aria-label": t("btn.delete"), onclick: () => deleteMedia(m) }, "✕")));
  }));
}
async function deleteMedia(m) {
  if (!confirm(t("confirm.deleteMedia", { name: m.name }))) return;
  try { await api("DELETE", `/api/media/${encodeURIComponent(m.name)}`); await refresh(); }
  catch (e) { toast(e.message, true); }
}
async function upload(files) {
  for (const f of files) {
    const fd = new FormData();
    fd.append("file", f);
    toast(t("upload.progress", { name: f.name }));
    try { await api("POST", "/api/media", fd); }
    catch (e) { toast(`${f.name}: ${e.message}`, true); return; }
  }
  await refresh();
  toast(t("upload.done"));
}
for (const id of ["#media-file", "#media-file-2"]) $(id).addEventListener("change", (e) => { upload(e.target.files); e.target.value = ""; });
const dz = $("#dropzone");
dz.addEventListener("dragover", (e) => { e.preventDefault(); dz.classList.add("over"); });
dz.addEventListener("dragleave", () => dz.classList.remove("over"));
dz.addEventListener("drop", (e) => { e.preventDefault(); dz.classList.remove("over"); upload(e.dataTransfer.files); });

// ---------- settings ----------
function renderSettings(test) {
  const ok = !state.sensorsError;
  const card = $("#lhm-card"), st = $("#lhm-status");
  card.classList.toggle("ok", ok);
  st.className = "sub " + (ok ? "ok" : "error");
  const okKey = state.sensorSource === "service" ? "lhm.ok.service" : "lhm.ok.lhm";
  st.textContent = state.demo ? t("lhm.demo") : ok ? t(okKey, { n: test ? test.count : "…" }) : t("lhm.off");
  if (state.demo) { st.className = "sub warn"; card.classList.remove("ok"); }
  if (ok && !test && !state.demo) api("GET", "/api/sensors").then((rs) => { st.textContent = t(okKey, { n: rs.length }); }).catch(() => {});
  const st2 = state.settings || {};
  $("#set-updates").checked = !!st2.updateCheck;
  $("#set-fwcheck").checked = !!st2.firmwareCheck;
  $("#set-source").value = st2.sensorSource || "auto";
  $("#set-url").value = st2.sensorUrl || "";
  $("#row-url").hidden = $("#set-source").value !== "custom";
  $("#set-pm").checked = !!st2.presentmon;
  $("#set-rtss").checked = !!st2.rtss;
  $("#set-anim").checked = !!st2.animation;
  $("#set-devvideo").checked = st2.deviceVideo !== false && !!st2.ffmpeg;
  $("#set-devvideo").disabled = !st2.ffmpeg;
  $("#set-devvideo-sub").textContent = t(st2.ffmpeg ? "set.devvideo.sub" : "set.devvideo.noff");
  $("#set-mcp").checked = !!st2.mcp;
  $("#set-localapi").checked = !!st2.api;
  $("#set-saver").checked = !!st2.screensaverOff;
  $("#set-temp").value = st2.fahrenheit ? "f" : "c";
  $("#set-clock").value = st2.clock12h ? "12" : "24";
  $("#row-mcp").hidden = !st2.mcp;
  $("#mcp-snippet").textContent = JSON.stringify({ mcpServers: { casehud: { command: state.mcpCommand, args: ["mcp"] } } }, null, 2);
  $("#set-interval").value = String(st2.intervalMs || 0);
  renderUpdate();
  const pm = state.demo ? "demo" : state.presentmon || "unsupported";
  $("#pm-status").textContent = t("pm." + pm);
  $("#pm-status").className = "sub " + (pm === "demo" ? "warn" : pm === "running" ? "ok" : pm === "unsupported" ? "" : "error");
  $("#btn-pm-setup").hidden = !(pm === "no-permission" || pm === "stopped");
  $("#row-autostart").hidden = !state.autostart.supported;
  $("#set-autostart").checked = state.autostart.enabled;
  $("#set-datadir").textContent = "\u200E" + state.dataDir; // keep slashes in place under rtl truncation
  $("#set-version").textContent = state.version;
  loadAwtrix();
}
$("#btn-test").addEventListener("click", async () => {
  const b = $("#btn-test");
  b.disabled = true; b.textContent = t("lhm.testing");
  try {
    const r = await api("POST", "/api/sensors/test");
    state.sensorsError = r.ok ? "" : r.error;
    renderStatus(); renderSettings(r);
    if (!r.ok) toast(r.error, true);
  } catch (e) { toast(e.message, true); }
  b.disabled = false; b.textContent = t("lhm.test");
});
$("#set-autostart").addEventListener("change", async (e) => {
  try { await api("PUT", "/api/settings", { autostart: e.target.checked }); toast(t("saved")); }
  catch (err) { toast(err.message, true); e.target.checked = !e.target.checked; }
});
$("#set-mcp").addEventListener("change", (e) => { $("#row-mcp").hidden = !e.target.checked; });
$("#btn-mcp-copy").addEventListener("click", async () => {
  try { await navigator.clipboard.writeText($("#mcp-snippet").textContent); toast(t("copied")); }
  catch (e) { toast(e.message, true); }
});

// ---------- AWTRIX clocks ----------
let aw = { devices: [], status: {}, apps: [] };
async function loadAwtrix() {
  try { aw = await api("GET", "/api/awtrix"); } catch (e) { return; }
  renderAwtrix();
}
async function saveAwtrix(reload = true) {
  try { await api("PUT", "/api/awtrix", aw.devices); toast(t("saved")); }
  catch (e) { toast(e.message, true); }
  if (reload) setTimeout(loadAwtrix, 2500); // status after the next push round
}
const saveAwtrixDebounced = debounce(() => saveAwtrix(false), 600);
// awModel is the display a clock was detected as ("tc001" when unknown).
const awModel = (id) => (aw.info && aw.info[id] && aw.info[id].model) || "tc001";
function awKind(info) {
  if (!info) return "";
  const model = info.model === "tc002" ? "Ulanzi TC002 · 52×16" : info.model === "tc001" ? "Ulanzi TC001 · 32×8" : `${info.w}×${info.h}`;
  return `${model} · ${info.firmware === "ng" ? "AWTRIX NG" : "AWTRIX 3"} ${info.version}`;
}
function renderAwtrix() {
  const list = $("#aw-list");
  list.replaceChildren(...aw.devices.map((d) => {
    const err = aw.status[d.id];
    const known = d.id in aw.status;
    const info = aw.info && aw.info[d.id];
    const dot = el("span", { class: "dot" + (d.disabled ? "" : known ? (err ? " error" : " ok") : ""), title: err || "" });
    const name = el("input", { class: "aw-name", value: d.name || "AWTRIX", oninput: (e) => { d.name = e.target.value; saveAwtrixDebounced(); } });
    const on = el("input", { type: "checkbox", class: "switch", title: t("aw.enabled"), onchange: (e) => { d.disabled = !e.target.checked; saveAwtrix(); } });
    on.checked = !d.disabled;
    const del = el("button", { class: "danger", onclick: () => { aw.devices = aw.devices.filter((x) => x !== d); saveAwtrix(); renderAwtrix(); } }, t("aw.remove"));
    const stateText = el("span", { class: "aw-state" + (d.disabled ? "" : known ? (err ? " error" : " ok") : "") },
      d.disabled ? t("aw.off") : known ? (err ? t("aw.err") : t("aw.ok")) : t("aw.connecting"));
    const model = awModel(d.id);
    const apps = el("div", { class: "aw-apps" + (model === "tc002" ? " big" : "") }, aw.apps.map((a) => {
      const sel = d.apps.includes(a.id);
      return el("button", { type: "button", class: "aw-app" + (sel ? " on" : ""), "aria-pressed": String(sel), onclick: () => {
        d.apps = sel ? d.apps.filter((x) => x !== a.id) : aw.apps.map((x) => x.id).filter((x) => x === a.id || d.apps.includes(x));
        saveAwtrix(); renderAwtrix();
      } },
        el("img", { src: url(`/api/awtrix/preview.png?app=${a.id}&model=${model}&v=${$("#set-temp").value}`), alt: "" }),
        el("span", {}, t("aw.app." + a.id)));
    }));
    return el("div", { class: "card aw-dev" + (d.disabled ? " off" : "") },
      el("div", { class: "aw-dev-head" }, dot, name, on),
      el("div", { class: "aw-meta" }, el("span", { class: "aw-host" }, d.host), stateText),
      info ? el("div", { class: "aw-kind" }, awKind(info)) : null,
      apps,
      err && !d.disabled ? el("div", { class: "aw-err" }, t("aw.unreachable", { e: err })) : null,
      el("div", { class: "aw-dev-foot" }, del));
  }));
}
// addAwtrix stores a clock that answered with info.
async function addAwtrix(host, info) {
  let id = slug(host), n = 2;
  while (aw.devices.some((d) => d.id === id)) id = `${slug(host)}-${n++}`;
  const name = info.model === "tc002" ? "TC002" : info.model === "tc001" ? "TC001" : "AWTRIX";
  aw.devices.push({ id, name, host, apps: ["cpu", "gpu", "coolant"] });
  toast(t("aw.found", { v: awKind(info) }));
  await saveAwtrix();
  renderAwtrix();
}
$("#btn-aw-add").addEventListener("click", async () => {
  const host = $("#aw-host").value.trim();
  if (!host) return;
  const b = $("#btn-aw-add");
  b.disabled = true; b.textContent = t("lhm.testing");
  try {
    const info = await api("POST", "/api/awtrix/test", { host });
    $("#aw-host").value = "";
    await addAwtrix(host, info);
  } catch (e) { toast(t("aw.notfound", { e: e.message }), true); }
  b.disabled = false; b.textContent = t("aw.addbtn");
});
$("#aw-host").addEventListener("keydown", (e) => { if (e.key === "Enter") $("#btn-aw-add").click(); });
$("#btn-aw-search").addEventListener("click", async () => {
  const b = $("#btn-aw-search"), box = $("#aw-found");
  b.disabled = true; b.textContent = t("aw.searching");
  box.hidden = false;
  box.replaceChildren(el("div", { class: "aw-found-empty" }, t("aw.searching.sub")));
  let found = [];
  try { found = await api("POST", "/api/awtrix/discover"); }
  catch (e) { toast(e.message, true); }
  b.disabled = false; b.textContent = t("aw.searchbtn");
  const renderFound = () => {
    if (!found.length) { box.replaceChildren(el("div", { class: "aw-found-empty" }, t("aw.none"))); return; }
    box.replaceChildren(...found.map((f) => {
      const have = aw.devices.some((d) => d.host === f.host || d.host === f.ip || (f.info.name && d.host.replace(/\.local$/, "") === f.info.name));
      const add = el("button", { class: have ? "ghost" : "", disabled: have, onclick: async () => { add.disabled = true; await addAwtrix(f.host, f.info); renderFound(); } },
        have ? t("aw.added") : t("aw.addone"));
      return el("div", { class: "aw-found-row" },
        el("img", { src: url(`/api/awtrix/preview.png?app=power&model=${f.info.model || "tc001"}`), alt: "" }),
        el("span", {}, el("b", {}, f.info.name || f.ip), el("small", {}, `${awKind(f.info)} · ${f.host === f.ip ? f.ip : `${f.host} (${f.ip})`}`)),
        add);
    }));
  };
  renderFound();
});

let updateDismissed = "";
function renderUpdate() {
  const u = state.update;
  const show = u && u.version !== updateDismissed;
  $("#update-banner").hidden = !show;
  if (show) {
    $("#update-text").textContent = t("upd.text", { v: "v" + u.version });
    $("#update-link").href = u.url || state.releasesUrl;
  }
}
$("#update-close").addEventListener("click", () => { updateDismissed = state.update ? state.update.version : ""; renderUpdate(); });
// All advanced settings go through one endpoint and apply immediately.
async function saveSettings() {
  const st0 = state.settings || {};
  const body = {
    sensorSource: $("#set-source").value,
    sensorUrl: $("#set-url").value.trim(),
    presentmon: $("#set-pm").checked,
    rtss: $("#set-rtss").checked,
    animation: $("#set-anim").checked,
    deviceVideo: st0.ffmpeg ? $("#set-devvideo").checked : st0.deviceVideo !== false,
    mcp: $("#set-mcp").checked,
    api: $("#set-localapi").checked,
    screensaverOff: $("#set-saver").checked,
    fahrenheit: $("#set-temp").value === "f",
    clock12h: $("#set-clock").value === "12",
    intervalMs: +$("#set-interval").value,
    updateCheck: $("#set-updates").checked,
    firmwareCheck: $("#set-fwcheck").checked,
  };
  $("#row-url").hidden = body.sensorSource !== "custom";
  if (body.sensorSource === "custom" && !body.sensorUrl) return; // wait for the address
  try {
    await api("PUT", "/api/settings/advanced", body);
    state.settings = { ...body, ffmpeg: st0.ffmpeg };
    toast(t("saved"));
    setTimeout(() => refresh().catch(() => {}), 1500);
  } catch (err) { toast(err.message, true); }
}
for (const id of ["#set-source", "#set-pm", "#set-rtss", "#set-anim", "#set-devvideo", "#set-mcp", "#set-localapi", "#set-saver", "#set-temp", "#set-clock", "#set-interval", "#set-updates", "#set-fwcheck"]) $(id).addEventListener("change", saveSettings);
$("#set-url").addEventListener("change", saveSettings);
$("#btn-open-log").addEventListener("click", () => api("POST", "/api/open-log").catch((e) => toast(e.message, true)));

$("#btn-pm-setup").addEventListener("click", async () => {
  try { await api("POST", "/api/presentmon/setup"); toast(t("pm.setupDone")); }
  catch (e) { toast(e.message, true); }
});
$("#btn-open-data").addEventListener("click", () => api("POST", "/api/open-data").catch((e) => toast(e.message, true)));

// ---------- design editor ----------
const ed = { layout: null, id: "", sel: -1, liveTimer: null, undo: [] };

const WIDGET_DEFAULTS = {
  gauge: { w: 300, h: 300, label: "CPU", source: "Load/CPU Total", unit: "%", max: 100, glow: true, smooth: true, warn: 80, crit: 95 },
  value: { w: 160, h: 100, label: "WASSER", source: "Temperatures/T Sensor", unit: "°C", decimals: 1 },
  bar: { w: 320, h: 80, label: "GPU", source: "Temperatures/GPU Core", unit: "°C", min: 30, max: 90 },
  graph: { w: 400, h: 120, label: "GPU TEMP", source: "Temperatures/GPU Core", unit: "°C", min: 30, max: 90 },
  text: { w: 300, h: 50, text: "Hallo", size: 36 },
  clock: { w: 320, h: 120, text: "15:04", size: 110 },
};
const WIDGET_ICONS = {
  gauge: '<path d="M5 18a8 8 0 1 1 14 0"/>', value: '<path d="M5 7h3v10M11 17h8M15 7v10"/>',
  bar: '<rect x="3" y="10" width="18" height="4" rx="2"/>', graph: '<path d="M3 17l5-6 4 3 5-7 4 4"/>',
  text: '<path d="M5 6h14M12 6v13"/>', clock: '<circle cx="12" cy="12" r="8"/><path d="M12 8v4l3 2"/>',
};

// Field definitions per widget type: [key, i18n label, input type, i18n hint].
const COMMON = [["label", "f.label", "text"], ["color", "f.color", "color"]];
const SENSOR = [["source", "f.source", "sensor", "h.source"], ["unit", "f.unit", "text"], ["decimals", "f.decimals", "number"]];
const RANGE = [["min", "f.min", "number"], ["max", "f.max", "number"], ["warn", "f.warn", "number", "h.warn"], ["crit", "f.crit", "number", "h.crit"], ["invert", "f.invert", "check"]];
const FIELDS = {
  gauge: [...COMMON, ...SENSOR, ...RANGE, ["sub_source", "f.sub_source", "sensor", "h.sub_source"], ["sub_unit", "f.sub_unit", "text"],
    ["thickness", "f.thickness", "range"], ["segments", "f.segments", "number"], ["ticks", "f.ticks", "number"],
    ["glow", "f.glow", "check"], ["smooth", "f.smooth", "check"]],
  value: [...COMMON, ...SENSOR, ["warn", "f.warn", "number"], ["crit", "f.crit", "number"], ["invert", "f.invert", "check"]],
  bar: [...COMMON, ...SENSOR, ...RANGE, ["segments", "f.segments", "number"]],
  graph: [...COMMON, ...SENSOR, ...RANGE],
  text: [["text", "f.text", "text"], ["size", "f.size", "number"], ["color", "f.color", "color"]],
  clock: [["text", "f.format", "text", "h.format"], ["size", "f.size", "number"], ["color", "f.color", "color"]],
};

function snapshot() {
  ed.undo.push(JSON.stringify(ed.layout));
  if (ed.undo.length > 100) ed.undo.shift();
}
const snapshotSoon = debounce(snapshot, 600);
function undo() {
  if (!ed.undo.length) return;
  ed.layout = JSON.parse(ed.undo.pop());
  if (ed.sel >= ed.layout.widgets.length) ed.sel = -1;
  redrawEditor();
}
$("#ed-undo").addEventListener("click", undo);
$("#ed-export").addEventListener("click", () => {
  const blob = new Blob([JSON.stringify(ed.layout, null, 2) + "\n"], { type: "application/json" });
  const a = el("a", { href: URL.createObjectURL(blob), download: slug(ed.layout.name || "design") + ".casehud.json" });
  document.body.append(a);
  a.click();
  setTimeout(() => { URL.revokeObjectURL(a.href); a.remove(); }, 1000);
});

async function openEditor(id, asCopy, isNew, layout) {
  if (layout) ed.layout = layout;
  else {
    try { ed.layout = await api("GET", `/api/designs/${id}`); }
    catch (e) { toast(e.message, true); return; }
  }
  ed.layout.widgets = ed.layout.widgets || [];
  ed.undo = [];
  const d = designById(id);
  if (layout) ed.id = "";
  else if (isNew) { ed.layout.name = t("ed.newName"); ed.layout.widgets = []; ed.id = ""; }
  else if (asCopy) { ed.layout.name = (d && d.builtin ? designName(id) : ed.layout.name || id) + t("ed.copySuffix"); ed.id = ""; }
  else ed.id = id;
  $("#ed-name").value = ed.layout.name || "";
  ed.sel = -1;
  $("#editor").hidden = false;
  loadSensors();
  redrawEditor();
  clearInterval(ed.liveTimer);
  ed.liveTimer = setInterval(renderStage, 2000);
}
function redrawEditor() {
  $("#ed-roundmask").checked = !!ed.layout.round;
  drawDesignForm(); drawOverlay(); drawLayers(); drawWidgetForm(); renderStage();
}

$("#ed-close").addEventListener("click", () => {
  $("#editor").hidden = true;
  clearInterval(ed.liveTimer);
  refresh();
});
$("#ed-name").addEventListener("input", (e) => { snapshotSoon(); ed.layout.name = e.target.value; });
$("#ed-roundmask").addEventListener("change", (e) => { snapshot(); ed.layout.round = e.target.checked; drawOverlay(); });

$("#ed-save").addEventListener("click", async () => {
  const id = ed.id || slug(ed.layout.name || "design");
  const clash = designById(id);
  if (!ed.id && clash && clash.builtin && !confirm(t("ed.overwriteBuiltin"))) return;
  try {
    await api("PUT", `/api/designs/${id}`, ed.layout);
    ed.id = id;
    toast(t("ed.saved"));
    state.designs = (await api("GET", "/api/state")).designs;
  } catch (e) { toast(e.message, true); }
});

let stageURL;
async function renderStage() {
  if (!ed.layout || $("#editor").hidden) return;
  try {
    const res = await api("POST", `/api/render?w=${isWideSize(ed.layout.width, ed.layout.height) ? 1200 : 480}`, ed.layout, true);
    const blob = await res.blob();
    if (stageURL) URL.revokeObjectURL(stageURL);
    stageURL = URL.createObjectURL(blob);
    $("#stage-img").src = stageURL;
  } catch { /* keep last frame */ }
}
const renderStageSoon = debounce(renderStage, 120);

async function loadSensors() {
  try {
    const rs = await api("GET", "/api/sensors");
    $("#sensor-list").replaceChildren(...rs.map((r) => el("option", { value: r.Path }, `${r.Value.toFixed(1)} ${r.Unit}`)));
  } catch { /* sensors offline */ }
}

function colorInput(value, onInput) {
  const hex = el("input", { value });
  const pick = el("input", { type: "color", value });
  pick.addEventListener("input", () => { hex.value = pick.value; onInput(pick.value); });
  hex.addEventListener("input", () => { if (/^#[0-9a-f]{6}$/i.test(hex.value)) { pick.value = hex.value; onInput(hex.value); } });
  return el("div", { class: "color" }, pick, hex);
}

function drawDesignForm() {
  const L = ed.layout;
  const f = $("#design-form");
  const field = (key, label) => el("label", { class: "field" }, el("span", {}, label),
    colorInput(L[key] || "#000000", (v) => { snapshotSoon(); L[key] = v; renderStageSoon(); }));
  f.replaceChildren(
    el("div", { class: "row2" }, field("background", t("ed.bg")), field("accent", t("ed.accent"))),
    el("div", { class: "row2" }, field("foreground", t("ed.text")), field("muted", t("ed.labels"))),
    el("label", { class: "check" },
      el("input", { type: "checkbox", checked: !!L.scanlines, onchange: (e) => { snapshot(); L.scanlines = e.target.checked; renderStageSoon(); } }),
      t("ed.scanlines")),
    el("label", { class: "check" },
      el("input", { type: "checkbox", checked: !!L.panel, onchange: (e) => { snapshot(); L.panel = e.target.checked ? "#10141c" : ""; drawDesignForm(); renderStageSoon(); } }),
      t("ed.showTiles")),
    ...(L.panel ? [el("label", { class: "field" }, el("span", {}, t("ed.tile")),
      colorInput(L.panel, (v) => { snapshotSoon(); L.panel = v; renderStageSoon(); }))] : []),
    el("div", { class: "hint" }, t("ed.sizeHint")));
}

function drawLayers() {
  $("#layers").replaceChildren(...ed.layout.widgets.map((w, i) =>
    el("button", { class: "layer" + (i === ed.sel ? " sel" : ""), onclick: () => select(i) },
      svg(WIDGET_ICONS[w.type] || ""), `${t("w." + w.type)} · ${w.label || w.text || sourceShort(w.source) || "–"}`)));
}
// sourceShort names a sensor source briefly, e.g. "Temperatures/GPU Core|…" -> "GPU Core".
function sourceShort(src) {
  if (!src) return "";
  const first = src.split("|")[0].split(/\s[+-]\s/)[0];
  return first.split("/").pop();
}
function select(i) { ed.sel = i; drawOverlay(); drawLayers(); drawWidgetForm(); }

function stageScale() { return $("#stage").clientWidth / ed.layout.width; }

function drawOverlay() {
  const ov = $("#overlay");
  ov.replaceChildren();
  $("#stage").classList.toggle("round", !!ed.layout.round);
  $("#stage").classList.toggle("wide", isWideSize(ed.layout.width, ed.layout.height));
  $("#stage").style.aspectRatio = ratio(ed.layout.width, ed.layout.height);
  const s = stageScale();
  ed.layout.widgets.forEach((w, i) => {
    const box = el("div", { class: "wbox" + (i === ed.sel ? " sel" : "") }, el("div", { class: "handle" }));
    Object.assign(box.style, { left: w.x * s + "px", top: w.y * s + "px", width: w.w * s + "px", height: w.h * s + "px" });
    box.addEventListener("pointerdown", (e) => startDrag(e, i, e.target.classList.contains("handle")));
    ov.append(box);
  });
}
window.addEventListener("resize", () => ed.layout && !$("#editor").hidden && drawOverlay());

const SNAP = 6;
function startDrag(e, i, resize) {
  e.preventDefault();
  if (ed.sel !== i) select(i);
  snapshot();
  const w = ed.layout.widgets[i];
  const s = stageScale();
  const L = ed.layout;
  const start = { x: e.clientX, y: e.clientY, wx: w.x, wy: w.y, ww: w.w, wh: w.h };
  const box = $("#overlay").children[i];
  const gv = $("#guide-v"), gh = $("#guide-h");
  const move = (ev) => {
    const dx = Math.round((ev.clientX - start.x) / s), dy = Math.round((ev.clientY - start.y) / s);
    if (resize) {
      w.w = Math.max(20, start.ww + dx);
      w.h = Math.max(20, start.wh + dy);
    } else {
      w.x = Math.max(0, Math.min(L.width - w.w, start.wx + dx));
      w.y = Math.max(0, Math.min(L.height - w.h, start.wy + dy));
      // Snap the widget centre to the canvas centre lines.
      const cx = w.x + w.w / 2, cy = w.y + w.h / 2;
      gv.hidden = Math.abs(cx - L.width / 2) > SNAP;
      gh.hidden = Math.abs(cy - L.height / 2) > SNAP;
      if (!gv.hidden) w.x = Math.round(L.width / 2 - w.w / 2);
      if (!gh.hidden) w.y = Math.round(L.height / 2 - w.h / 2);
    }
    Object.assign(box.style, { left: w.x * s + "px", top: w.y * s + "px", width: w.w * s + "px", height: w.h * s + "px" });
    renderStageSoon();
  };
  const up = () => {
    window.removeEventListener("pointermove", move);
    window.removeEventListener("pointerup", up);
    gv.hidden = gh.hidden = true;
    drawWidgetForm();
  };
  window.addEventListener("pointermove", move);
  window.addEventListener("pointerup", up);
}

// Arrow keys nudge (Shift = 10px), Delete removes, Ctrl+Z undoes.
window.addEventListener("keydown", (e) => {
  if ($("#editor").hidden) return;
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "z") { e.preventDefault(); undo(); return; }
  if (ed.sel < 0 || /INPUT|SELECT|TEXTAREA/.test(document.activeElement.tagName)) return;
  const w = ed.layout.widgets[ed.sel];
  const step = e.shiftKey ? 10 : 1;
  const moves = { ArrowLeft: [-step, 0], ArrowRight: [step, 0], ArrowUp: [0, -step], ArrowDown: [0, step] };
  if (moves[e.key]) {
    e.preventDefault();
    snapshotSoon();
    w.x = Math.max(0, w.x + moves[e.key][0]);
    w.y = Math.max(0, w.y + moves[e.key][1]);
    drawOverlay(); drawWidgetForm(); renderStageSoon();
  } else if (e.key === "Delete") {
    $("#w-del").click();
  }
});

function drawWidgetForm() {
  const f = $("#widget-form");
  f.replaceChildren();
  const w = ed.layout.widgets[ed.sel];
  $("#widget-empty").hidden = !!w;
  $("#widget-actions").hidden = !w;
  if (!w) return;

  const num = (k) => el("input", { type: "number", value: w[k] ?? 0, oninput: (e) => { snapshotSoon(); w[k] = +e.target.value; drawOverlay(); renderStageSoon(); } });
  f.append(
    el("div", { class: "row2" }, el("label", { class: "field" }, el("span", {}, t("f.x")), num("x")), el("label", { class: "field" }, el("span", {}, t("f.y")), num("y"))),
    el("div", { class: "row2" }, el("label", { class: "field" }, el("span", {}, t("f.w")), num("w")), el("label", { class: "field" }, el("span", {}, t("f.h")), num("h"))));

  for (const [key, label, type, hint] of FIELDS[w.type] || []) {
    const set = (v) => { snapshotSoon(); if (v === "" || v == null || v === false) delete w[key]; else w[key] = v; renderStageSoon(); };
    let input;
    if (type === "check") {
      f.append(el("label", { class: "check" }, el("input", { type: "checkbox", checked: !!w[key], onchange: (e) => set(e.target.checked) }), t(label)));
      continue;
    }
    if (type === "number") input = el("input", { type: "number", step: "any", value: w[key] ?? "", oninput: (e) => set(e.target.value === "" ? "" : +e.target.value) });
    else if (type === "range") {
      input = el("input", { type: "range", min: 0.02, max: 0.15, step: 0.005, value: w[key] || 0.075, oninput: (e) => { setRangeFill(e.target); set(+e.target.value); } });
      setRangeFill(input);
    } else if (type === "color") input = colorInput(w[key] || ed.layout.accent || "#22d3ee", set);
    else if (type === "sensor") input = el("input", { list: "sensor-list", value: w[key] || "", placeholder: t("ph.source"), oninput: (e) => set(e.target.value) });
    else input = el("input", { value: w[key] || "", oninput: (e) => { set(e.target.value); if (key === "label" || key === "text") drawLayers(); } });
    f.append(el("label", { class: "field" }, el("span", {}, t(label)), input));
    if (hint) f.append(el("div", { class: "hint" }, t(hint)));
  }
}

for (const b of $$("[data-add]")) {
  b.addEventListener("click", () => {
    snapshot();
    const type = b.dataset.add;
    const d = WIDGET_DEFAULTS[type];
    ed.layout.widgets.push({ type, x: Math.round((ed.layout.width - d.w) / 2), y: Math.round((ed.layout.height - d.h) / 2), ...d });
    select(ed.layout.widgets.length - 1);
    renderStageSoon();
  });
}
$("#w-del").addEventListener("click", () => {
  if (ed.sel < 0) return;
  snapshot();
  ed.layout.widgets.splice(ed.sel, 1);
  select(-1);
  renderStageSoon();
});
$("#w-dup").addEventListener("click", () => {
  if (ed.sel < 0) return;
  snapshot();
  const copy = { ...ed.layout.widgets[ed.sel] };
  copy.x += 12; copy.y += 12;
  ed.layout.widgets.push(copy);
  select(ed.layout.widgets.length - 1);
  renderStageSoon();
});

// ---------- open source thanks ----------
const OSS = [
  ["LibreHardwareMonitor", "https://github.com/LibreHardwareMonitor/LibreHardwareMonitor", "MPL-2.0", "oss.lhm"],
  ["PresentMon (Intel)", "https://github.com/GameTechDev/PresentMon", "MIT", "oss.pm"],
  ["RivaTuner Statistics Server", "https://www.guru3d.com/download/rtss-rivatuner-statistics-server-download/", "Freeware", "oss.rtss"],
  ["Go · golang.org/x/image · x/sys", "https://go.dev", "BSD-3-Clause", "oss.go"],
  ["go-webview2", "https://github.com/jchv/go-webview2", "MIT", "oss.wv"],
  ["fyne.io/systray", "https://github.com/fyne-io/systray", "Apache-2.0", "oss.tray"],
  ["Rajdhani (Indian Type Foundry)", "https://fonts.google.com/specimen/Rajdhani", "OFL-1.1", "oss.font"],
  ["FFmpeg", "https://ffmpeg.org", "LGPL / GPL", "oss.ffmpeg"],
];
const openOSS = () => {
  $("#oss-list").replaceChildren(...OSS.map(([name, href, lic, what]) => el("div", { class: "oss-item" },
    el("a", { href, target: "_blank", rel: "noopener" }, name), el("span", { class: "lic" }, lic), el("span", { class: "what" }, t(what)))));
  $("#oss").showModal();
};
$("#btn-oss").addEventListener("click", openOSS);
$("#btn-oss-2").addEventListener("click", openOSS);
$("#oss-close").addEventListener("click", () => $("#oss").close());
$("#oss").addEventListener("click", (e) => { if (e.target === $("#oss")) $("#oss").close(); });

// ---------- expert mode (remembered per viewer) ----------
let expert = false;
try { expert = localStorage.getItem("expert") === "1"; } catch { /* storage blocked */ }
document.body.classList.toggle("expert-mode", expert);
$("#set-expert").checked = expert;
$("#set-expert").addEventListener("change", (e) => {
  expert = e.target.checked;
  document.body.classList.toggle("expert-mode", expert);
  try { localStorage.setItem("expert", expert ? "1" : "0"); } catch { /* ignore */ }
});

// ---------- settings sub-navigation ----------
const setNavButtons = $$("#set-nav button");
for (const b of setNavButtons) {
  b.addEventListener("click", () => {
    $("#" + b.dataset.target).scrollIntoView({ behavior: "smooth", block: "start" });
    setNavButtons.forEach((x) => x.classList.toggle("active", x === b));
  });
}
// Highlight the group that is currently at the top of the page.
const spy = () => {
  if (!$("#tab-settings").classList.contains("active")) return;
  let cur = null;
  for (const b of setNavButtons) {
    const sec = $("#" + b.dataset.target);
    if (sec.offsetParent !== null && sec.getBoundingClientRect().top <= 120) cur = b;
  }
  if (window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 4) {
    cur = [...setNavButtons].reverse().find((b) => $("#" + b.dataset.target).offsetParent !== null) || cur;
  }
  if (cur) setNavButtons.forEach((x) => x.classList.toggle("active", x === cur));
};
window.addEventListener("scroll", spy, { passive: true });

// ---------- boot ----------
applyI18n();
refresh().catch((e) => toast(e.message, true));
setInterval(() => {
  if (document.hidden || !$("#editor").hidden) return;
  api("GET", "/api/state").then((s) => {
    // Re-render only on structural changes so open forms keep focus.
    const sig = (x) => JSON.stringify([x.panels.map((p) => [p.id, p.error, p.firmware, p.firmwareLatest && p.firmwareLatest.status, p.width, p.height, p.number, p.grids, p.experimental, p.deviceFlip, p.offlineClock]), x.monitorMode, x.designs.map((d) => d.id), x.media.map((m) => m.name), x.sensorsError, x.scanError, x.rescanGaveUp]);
    const changed = sig(s) !== sig(state);
    const keepConfigs = state.panels;
    state = s;
    if (!changed) state.panels = keepConfigs; // local edits may be newer
    renderStatus();
    if (changed) renderAll();
  }).catch(() => {});
}, 5000);

// Deep link: index.html#scenes opens that tab.
const openHashTab = () => { const h = location.hash.slice(1); if (h && $('[data-tab="' + h + '"]')) showTab(h); };
window.addEventListener("hashchange", openHashTab);
openHashTab();

// A display in monitor mode is no CaseHUD display; the page offers the way
// back (one restart report, sent only after the confirmation).
function renderMonitorMode() {
  const list = state.monitorMode || [];
  $("#monitor-mode").hidden = !list.length;
  $("#mm-actions").replaceChildren(...list.map((d) => el("button", { class: "ghost", onclick: (ev) => {
    if (!confirm(t("mm.confirm"))) return;
    ev.target.disabled = true;
    api("POST", "/api/monitor-mode/bring-back", { path: d.path })
      .then(() => toast(t("mm.sent"))).catch((e) => { ev.target.disabled = false; toast(e.message, true); });
  } }, t("mm.back"))));
}
