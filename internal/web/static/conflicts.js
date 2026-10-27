"use strict";

// ---------- Other programs on the same devices: banner, dialog, settings ----------
Object.assign(STRINGS.de, {
  "conf.title": "Andere Software steuert dieselben Geräte",
  "conf.sub": "Zwei Programme gleichzeitig stören sich: das eine sperrt das Gerät oder überschreibt die Anzeige. CaseHUD schließt nichts von allein.",
  "conf.banner": "Andere Software steuert dieselben Geräte: {names}",
  "conf.show": "Anzeigen",
  "conf.later": "Später",
  "conf.close": "Schließen",
  "conf.closeAll": "Alle schließen",
  "conf.ignore": "Ignorieren",
  "conf.help": "Hilfe",
  "conf.affects": "Betrifft: {d}",
  "conf.busy": "Wird geschlossen …",
  "conf.done": "{n} wurde geschlossen",
  "conf.hard.ask": "{n} läuft noch. Hart beenden? Ungespeicherte Daten dieses Programms gehen verloren.",
  "conf.hard": "Hart beenden",
  "conf.hard.no": "Nein",
  "conf.path": "Pfad: {p}",
  "conf.unusual": "Ungewöhnlicher Speicherort: dieses Programm läuft nicht aus einem üblichen Installationsordner.",
  "conf.unusual.ask": "Ungewöhnlicher Speicherort: {n} läuft von\n{p}\nDas ist kein üblicher Installationsordner (Programme, ProgramData, AppData\\Local\\Programs). Trotzdem schließen?",
  "conf.service": "Läuft als Dienst – bitte in der Dienste-Verwaltung beenden.",
  "conf.elevated": "Läuft mit Admin-Rechten – bitte als Administrator beenden.",
  "conf.services": "Dienste öffnen",
  "conf.gone": "{n} läuft nicht mehr",
  "conf.err": "Das ging nicht: {e}",
  "conf.helptext": "CaseHUD schließt nur Programme dieser Liste und nur nach deinem Klick: erst höflich (das Fenster wird um Schließen gebeten, bis zu 5 Sekunden), danach auf Wunsch hart. Dienste und Programme mit Admin-Rechten kann CaseHUD nicht beenden und versucht es auch nicht. Damit ein Programm beim nächsten Start nicht wieder mitläuft: Windows-Einstellungen → Apps → Autostart, oder Task-Manager → Autostart. Dienste lassen sich in der Dienste-Verwaltung (services.msc) beenden oder auf „Manuell“ stellen.",
  "conf.set.title": "Andere Software",
  "conf.set.sub": "Programme, die dieselben Geräte steuern können. CaseHUD fragt, bevor etwas geschlossen wird.",
  "conf.set.ask": "Beim Start und bei neuen Programmen fragen",
  "conf.set.ask.sub": "Zeigt oben einen Hinweis, sobald eines dieser Programme läuft. Aus: kein Hinweis, die Liste hier bleibt.",
  "conf.set.running": "läuft",
  "conf.set.idle": "nicht gestartet",
  "conf.set.keep": "Nicht schließen",
  "conf.set.service": "Dienst",
  "conf.set.open": "Anzeigen",
  "conf.dev.tllcd": "Lüfter-LCDs", "conf.dev.us88": "Universal Screen 8.8\"", "conf.dev.aio": "Pumpen-Display",
  "conf.dev.fanled": "Lüfter-Beleuchtung", "conf.dev.ringled": "8.8\"-Rand", "conf.dev.boardled": "Mainboard-Beleuchtung", "conf.dev.ramled": "RAM-Beleuchtung",
});
Object.assign(STRINGS.en, {
  "conf.title": "Other software controls the same devices",
  "conf.sub": "Two programs at once get in each other's way: one locks the device or overwrites the picture. CaseHUD never closes anything by itself.",
  "conf.banner": "Other software controls the same devices: {names}",
  "conf.show": "Show",
  "conf.later": "Later",
  "conf.close": "Close",
  "conf.closeAll": "Close all",
  "conf.ignore": "Ignore",
  "conf.help": "Help",
  "conf.affects": "Affects: {d}",
  "conf.busy": "Closing …",
  "conf.done": "{n} was closed",
  "conf.hard.ask": "{n} is still running. End it forcibly? Unsaved data of that program is lost.",
  "conf.hard": "End forcibly",
  "conf.hard.no": "No",
  "conf.path": "Path: {p}",
  "conf.unusual": "Unusual location: this program does not run from a usual install folder.",
  "conf.unusual.ask": "Unusual location: {n} runs from\n{p}\nThis is not a usual install folder (Program Files, ProgramData, AppData\\Local\\Programs). Close it anyway?",
  "conf.service": "Runs as a service – please stop it in the Services manager.",
  "conf.elevated": "Runs with admin rights – please close it as administrator.",
  "conf.services": "Open Services",
  "conf.gone": "{n} is not running any more",
  "conf.err": "That did not work: {e}",
  "conf.helptext": "CaseHUD only closes programs from this list, and only after your click: first politely (the window is asked to close, up to 5 seconds), then forcibly if you want. Services and programs with admin rights cannot be ended by CaseHUD, and it does not try. To keep a program from starting again with Windows: Windows Settings → Apps → Startup, or Task Manager → Startup. Services can be stopped, or set to “Manual”, in the Services manager (services.msc).",
  "conf.set.title": "Other software",
  "conf.set.sub": "Programs that can control the same devices. CaseHUD asks before anything is closed.",
  "conf.set.ask": "Ask at start and when new programs appear",
  "conf.set.ask.sub": "Shows a notice at the top as soon as one of these programs runs. Off: no notice, the list here stays.",
  "conf.set.running": "running",
  "conf.set.idle": "not started",
  "conf.set.keep": "Do not close",
  "conf.set.service": "service",
  "conf.set.open": "Show",
  "conf.dev.tllcd": "Fan LCDs", "conf.dev.us88": "Universal Screen 8.8\"", "conf.dev.aio": "Pump display",
  "conf.dev.fanled": "Fan lighting", "conf.dev.ringled": "8.8\" frame", "conf.dev.boardled": "Mainboard lighting", "conf.dev.ramled": "RAM lighting",
});

const conf = { st: null, busy: {}, hard: {}, msg: {}, dismissed: "", asked: false, help: false, timer: 0 };

(function buildConflictUI() {
  const banner = el("div", { id: "conf-banner", class: "update-banner conf-banner", hidden: true, role: "status" },
    el("span", { class: "dot" }), el("span", { id: "conf-banner-text" }),
    el("button", { id: "conf-banner-show", class: "button primary" }),
    el("button", { id: "conf-banner-x", class: "icon-btn", "aria-label": "Close" }, "✕"));
  const after = $("#update-banner");
  after.parentNode.insertBefore(banner, after.nextSibling);

  const dlg = el("dialog", { id: "conf-dlg", class: "oss notice conf" },
    el("div", { class: "oss-head" },
      el("div", { class: "oss-heart notice-icon", "aria-hidden": "true" }, "!"),
      el("div", {}, el("h2", { id: "conf-dlg-title" }), el("p", { class: "sub", id: "conf-dlg-sub" }))),
    el("div", { class: "notice-body conf-body", id: "conf-dlg-body" }),
    el("p", { class: "conf-help sub", id: "conf-dlg-help", hidden: true }),
    el("div", { class: "notice-actions" },
      el("button", { class: "ghost", id: "conf-dlg-help-btn" }),
      el("button", { class: "ghost", id: "conf-dlg-later" }),
      el("button", { class: "primary", id: "conf-dlg-all" })));
  document.body.append(dlg);
})();

const confText = () => {
  $("#conf-dlg-title").textContent = t("conf.title");
  $("#conf-dlg-sub").textContent = t("conf.sub");
  $("#conf-dlg-help").textContent = t("conf.helptext");
  $("#conf-dlg-help-btn").textContent = t("conf.help");
  $("#conf-dlg-later").textContent = t("conf.later");
  $("#conf-dlg-all").textContent = t("conf.closeAll");
  $("#conf-banner-show").textContent = t("conf.show");
};

const confDevs = (list) => list.map((d) => t("conf.dev." + d)).join(", ");
const confPending = () => (conf.st ? conf.st.items.filter((i) => i.running && !i.ignored && i.affects.length) : []);

async function confLoad() {
  try {
    conf.st = await api("GET", "/api/conflicts");
  } catch { return; }
  confRender();
  const accepted = !state || !state.acks || state.acks.accepted;
  if (!conf.asked && conf.st.ask && accepted && confPending().length && !$("dialog[open]")) {
    conf.asked = true; // the first sight asks once; later programs show the banner
    confOpen();
  }
}

function confRender() {
  confText();
  const pend = conf.st && conf.st.ask ? confPending() : [];
  const key = pend.map((i) => i.exe).sort().join("|");
  if (!pend.length) conf.dismissed = "";
  const banner = $("#conf-banner");
  banner.hidden = !pend.length || conf.dismissed === key;
  if (!banner.hidden) $("#conf-banner-text").textContent = t("conf.banner", { names: pend.map((i) => i.name).join(", ") });
  const dlg = $("#conf-dlg");
  if (dlg.open) {
    if (!confPending().length) dlg.close();
    else confRows();
  }
  confSettings();
}

function confRows() {
  const body = $("#conf-dlg-body");
  const rows = confPending().map((i) => {
    const bad = !i.closable;
    const hard = conf.hard[i.exe];
    const actions = [];
    if (conf.busy[i.exe]) actions.push(el("span", { class: "sub" }, t("conf.busy")));
    else if (hard) {
      actions.push(el("button", { class: "danger", onclick: () => confClose(i.exe, true) }, t("conf.hard")),
        el("button", { class: "ghost", onclick: () => { delete conf.hard[i.exe]; confRows(); } }, t("conf.hard.no")));
    } else {
      if (bad && i.reason === "service") actions.push(el("button", { class: "ghost", onclick: confServices }, t("conf.services")));
      if (!bad) actions.push(el("button", { class: "primary", onclick: () => confClose(i.exe, false) }, t("conf.close")));
      actions.push(el("button", { class: "ghost", onclick: () => confIgnore(i.exe, true) }, t("conf.ignore")));
    }
    return el("div", { class: "conf-row" },
      el("div", { class: "conf-info" },
        el("b", {}, i.name), " ", el("code", {}, i.exe),
        el("small", {}, t("conf.affects", { d: confDevs(i.affects) })),
        ...(i.paths || []).map((p) => el("small", { class: "conf-path" }, t("conf.path", { p }))),
        i.unusual ? el("small", { class: "conf-warn" }, t("conf.unusual")) : null,
        hard ? el("small", { class: "conf-warn" }, t("conf.hard.ask", { n: i.name })) : null,
        bad ? el("small", { class: "conf-warn" }, t(i.reason === "service" ? "conf.service" : "conf.elevated")) : null,
        conf.msg[i.exe] ? el("small", { class: "conf-warn" }, conf.msg[i.exe]) : null),
      el("div", { class: "conf-actions" }, actions));
  });
  body.replaceChildren(...rows);
  $("#conf-dlg-all").disabled = !confPending().some((i) => i.closable) || Object.values(conf.busy).some(Boolean);
}

function confOpen() {
  const dlg = $("#conf-dlg");
  confText();
  confRows();
  if (!dlg.open && confPending().length) dlg.showModal();
}

async function confClose(exe, hard) {
  const item = conf.st.items.find((i) => i.exe === exe);
  const name = item ? item.name : exe;
  let allowUnusual = false;
  if (item && item.unusual) {
    // second, explicit warning: the program runs from an unusual place
    if (!confirm(t("conf.unusual.ask", { n: name, p: (item.paths || []).join("\n") }))) return "cancelled";
    allowUnusual = true;
  }
  conf.busy[exe] = true; delete conf.msg[exe]; delete conf.hard[exe];
  confRows();
  try {
    const r = await api("POST", "/api/conflicts/close", { exe, hard, allowUnusual });
    if (r.status === "closed") {
      toast(t("conf.done", { n: name }));
      setTimeout(() => { if (typeof refresh === "function") refresh().catch(() => {}); }, 4000); // the devices come back
    } else if (r.status === "still_running") conf.hard[exe] = true;
    else if (r.status === "not_running") toast(t("conf.gone", { n: name }));
    else if (r.status === "needs_admin") conf.msg[exe] = t(r.reason === "service" ? "conf.service" : "conf.elevated");
    return r.status;
  } catch (e) {
    conf.msg[exe] = t("conf.err", { e: e.message || e });
  } finally {
    delete conf.busy[exe];
    await confLoad();
  }
}

async function confCloseAll() {
  for (const i of confPending().filter((x) => x.closable)) {
    const s = await confClose(i.exe, false);
    if (s === "still_running") break; // ask about this one before going on
  }
}

async function confIgnore(exe, on) {
  try { conf.st = await api("PUT", "/api/conflicts/ignore", { exe, on }); confRender(); }
  catch (e) { toast(t("conf.err", { e: e.message || e }), true); }
}

async function confServices() {
  try { await api("POST", "/api/conflicts/open-services"); }
  catch (e) { toast(t("conf.err", { e: e.message || e }), true); }
}

function confSettings() {
  const card = $("#conf-card");
  if (!card || !conf.st) return;
  $("#conf-set-ask").checked = conf.st.ask;
  const list = $("#conf-set-list");
  list.replaceChildren(...conf.st.items.map((i) => el("div", { class: "row conf-set-row" },
    el("div", { class: "row-text" },
      el("b", {}, i.name, " ", el("code", {}, i.exe)),
      el("small", {}, confDevs(i.devices) + (i.service ? " · " + t("conf.set.service") : ""))),
    el("span", { class: "conf-badge" + (i.running ? " on" : "") }, t(i.running ? "conf.set.running" : "conf.set.idle")),
    el("label", { class: "conf-keep" }, el("span", {}, t("conf.set.keep")),
      el("input", { type: "checkbox", class: "switch", checked: i.ignored, "aria-label": t("conf.set.keep") + ": " + i.name,
        onchange: (ev) => confIgnore(i.exe, ev.target.checked) })))));
  $("#conf-set-title").textContent = t("conf.set.title");
  $("#conf-set-sub").textContent = t("conf.set.sub");
  $("#conf-set-ask-t").textContent = t("conf.set.ask");
  $("#conf-set-ask-s").textContent = t("conf.set.ask.sub");
}

(function buildConflictSettings() {
  const group = $("#set-devices");
  if (!group) return;
  group.append(el("div", { class: "card rows", id: "conf-card" },
    el("div", { class: "row-block" }, el("b", { id: "conf-set-title" }), el("p", { class: "sub", id: "conf-set-sub" })),
    el("label", { class: "row" }, el("div", { class: "row-text" }, el("b", { id: "conf-set-ask-t" }), el("small", { id: "conf-set-ask-s" })),
      el("input", { type: "checkbox", class: "switch", id: "conf-set-ask" })),
    el("div", { id: "conf-set-list" })));
  $("#conf-set-ask").addEventListener("change", async (ev) => {
    try { conf.st = await api("PUT", "/api/conflicts/ask", { on: ev.target.checked }); confRender(); }
    catch (e) { toast(t("conf.err", { e: e.message || e }), true); confLoad(); }
  });
})();

$("#conf-banner-show").addEventListener("click", confOpen);
$("#conf-banner-x").addEventListener("click", () => {
  conf.dismissed = confPending().map((i) => i.exe).sort().join("|");
  confRender();
});
$("#conf-dlg-later").addEventListener("click", () => $("#conf-dlg").close());
$("#conf-dlg-all").addEventListener("click", confCloseAll);
$("#conf-dlg-help-btn").addEventListener("click", () => {
  conf.help = !conf.help;
  $("#conf-dlg-help").hidden = !conf.help;
});
for (const b of $$("[data-lang]")) b.addEventListener("click", () => setTimeout(confRender, 0));
applyI18n();
confText();
confLoad();
conf.timer = setInterval(() => { if (!document.hidden) confLoad(); }, 5000);
