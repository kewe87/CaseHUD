"use strict";

// ---------- AI connection: set up AI apps and tokens for other programs ----------
Object.assign(STRINGS.de, {
  "ai.title": "KI-Verbindung",
  "ai.sub": "Verbindet KI-Apps auf diesem PC mit CaseHUD (MCP). Danach die App neu starten.",
  "ai.empty": "Keine unterstützte KI-App gefunden.",
  "ai.state.missing": "Nicht installiert", "ai.state.found": "Gefunden", "ai.state.connected": "Verbunden",
  "ai.manual": "Befehl wird angezeigt (Claude Code nicht im PATH)",
  "ai.connect": "Verbinden", "ai.change": "Rechte ändern", "ai.disconnect": "Trennen", "ai.cancel": "Abbrechen",
  "ai.connect.title": "{app} verbinden",
  "ai.connect.text": "CaseHUD trägt sich in die Konfiguration von {app} ein. Die Datei wird vorher gesichert, andere Einträge bleiben unverändert. Die MCP-Schnittstelle wird eingeschaltet.",
  "ai.scope.read": "Nur lesen", "ai.scope.read.sub": "Zustand, Sensoren und Vorschauen abrufen.",
  "ai.scope.control": "Steuern", "ai.scope.control.sub": "Zusätzlich Displays, Beleuchtung und Kühlung ändern.",
  "ai.connect.ok": "Verbinden", "ai.change.ok": "Übernehmen",
  "ai.done": "{app} neu starten, dann ist CaseHUD dort verfügbar.",
  "ai.done.backup": "Sicherung angelegt.", "ai.backup.copy": "Pfad kopieren",
  "ai.err.invalid_json": "Die Konfiguration von {app} ist kein gültiges JSON. Nichts geändert.",
  "ai.err.duplicate_keys": "Die Konfiguration von {app} enthält doppelte Schlüssel. Nichts geändert.",
  "ai.err.symlink": "Die Konfiguration von {app} ist eine Verknüpfung und wird nicht beschrieben.",
  "ai.err.not_found": "Die Konfiguration von {app} wurde nicht gefunden.",
  "ai.err.write_failed": "Die Konfiguration von {app} konnte nicht geschrieben werden.",
  "ai.err.cli_failed": "Der Befehl von {app} ist fehlgeschlagen.",
  "ai.err.unsupported": "Für {app} nicht unterstützt.",
  "ai.err.invalid": "Die Konfiguration von {app} ist ungültig. Nichts geändert.",
  "ai.err.generic": "Aktion für {app} fehlgeschlagen.",
  "ai.done.manual": "Diesen Befehl in einer Eingabeaufforderung ausführen, dann {app} neu starten:",
  "ai.off.done": "{app} getrennt. Der Zugriff wurde entzogen.",
  "ai.off.manual": "Zugriff entzogen. Zum Entfernen des Eintrags diesen Befehl ausführen:",
  "ai.off.title": "{app} trennen", "ai.off.text": "Der Eintrag von CaseHUD wird aus der Konfiguration entfernt (mit Sicherung) und der Zugriff entzogen.",
  "ai.off.ok": "Trennen",
  "ai.conflict.title": "Eintrag „casehud“ ersetzen?",
  "ai.conflict.text": "In {app} gibt es bereits einen Eintrag „casehud“, der nicht zu diesem CaseHUD gehört.",
  "ai.conflict.ok": "Ersetzen",
  "ai.err.conflict": "In {app} gibt es einen fremden Eintrag „casehud“. Er wurde nicht geändert.",
  "ai.copy": "Kopieren", "ai.close": "Schließen",
  "ai.other": "Zugang für ein anderes Programm",
  "ai.other.sub": "Für Programme ohne MCP, die die API nutzen. Schaltet die API ein.",
  "ai.other.btn": "Zugang erstellen", "ai.tok.ok": "Erstellen",
  "ai.tok.title": "Zugang erstellen",
  "ai.tok.text": "Der Token wird nur einmal angezeigt. Die API wird eingeschaltet.",
  "ai.tok.name": "Name des Programms", "ai.tok.name.ph": "z. B. Heimautomation",
  "ai.tok.noname": "Name erforderlich.", "ai.tok.reserved": "Der Name darf nicht mit „MCP (“ beginnen.",
  "ai.tok.done.title": "Zugang für „{name}“",
  "ai.tok.done.text": "Diesen Token jetzt kopieren. Er wird nicht noch einmal angezeigt.",
  "ai.tok.addr": "Adresse", "ai.tok.token": "Token", "ai.tok.spec": "Beschreibung (mit Token abrufbar)", "ai.tok.docs": "Dokumentation",
  "ai.hint.text": "KI-App gefunden: {app}.",
  "ai.hint.open": "Verbinden…", "ai.hint.off": "Nicht mehr anzeigen",
});
Object.assign(STRINGS.en, {
  "ai.title": "AI connection",
  "ai.sub": "Connects AI apps on this PC to CaseHUD (MCP). Restart the app afterwards.",
  "ai.empty": "No supported AI app found.",
  "ai.state.missing": "Not installed", "ai.state.found": "Found", "ai.state.connected": "Connected",
  "ai.manual": "Command is shown (Claude Code not on PATH)",
  "ai.connect": "Connect", "ai.change": "Change rights", "ai.disconnect": "Disconnect", "ai.cancel": "Cancel",
  "ai.connect.title": "Connect {app}",
  "ai.connect.text": "CaseHUD adds itself to the configuration of {app}. The file is backed up first, other entries stay as they are. The MCP interface is switched on.",
  "ai.scope.read": "Read only", "ai.scope.read.sub": "Fetch status, sensors and previews.",
  "ai.scope.control": "Control", "ai.scope.control.sub": "Also change displays, lighting and cooling.",
  "ai.connect.ok": "Connect", "ai.change.ok": "Apply",
  "ai.done": "Restart {app}, then CaseHUD is available there.",
  "ai.done.backup": "Backup created.", "ai.backup.copy": "Copy path",
  "ai.err.invalid_json": "The configuration of {app} is not valid JSON. Nothing changed.",
  "ai.err.duplicate_keys": "The configuration of {app} contains duplicate keys. Nothing changed.",
  "ai.err.symlink": "The configuration of {app} is a link and is not written to.",
  "ai.err.not_found": "The configuration of {app} was not found.",
  "ai.err.write_failed": "The configuration of {app} could not be written.",
  "ai.err.cli_failed": "The command of {app} failed.",
  "ai.err.unsupported": "Not supported for {app}.",
  "ai.err.invalid": "The configuration of {app} is invalid. Nothing changed.",
  "ai.err.generic": "Action for {app} failed.",
  "ai.done.manual": "Run this command in a command prompt, then restart {app}:",
  "ai.off.done": "{app} disconnected. Access was revoked.",
  "ai.off.manual": "Access revoked. To remove the entry, run this command:",
  "ai.off.title": "Disconnect {app}", "ai.off.text": "The CaseHUD entry is removed from the configuration (with a backup) and the access is revoked.",
  "ai.off.ok": "Disconnect",
  "ai.conflict.title": "Replace the “casehud” entry?",
  "ai.conflict.text": "{app} already has an entry “casehud” that does not belong to this CaseHUD.",
  "ai.conflict.ok": "Replace",
  "ai.err.conflict": "{app} has a foreign entry “casehud”. It was not changed.",
  "ai.copy": "Copy", "ai.close": "Close",
  "ai.other": "Access for another program",
  "ai.other.sub": "For programs without MCP that use the API. Switches the API on.",
  "ai.other.btn": "Create access", "ai.tok.ok": "Create",
  "ai.tok.title": "Create access",
  "ai.tok.text": "The token is shown only once. The API is switched on.",
  "ai.tok.name": "Name of the program", "ai.tok.name.ph": "e.g. Home automation",
  "ai.tok.noname": "Name required.", "ai.tok.reserved": "The name must not start with “MCP (”.",
  "ai.tok.done.title": "Access for “{name}”",
  "ai.tok.done.text": "Copy this token now. It will not be shown again.",
  "ai.tok.addr": "Address", "ai.tok.token": "Token", "ai.tok.spec": "Description (needs the token)", "ai.tok.docs": "Documentation",
  "ai.hint.text": "AI app found: {app}.",
  "ai.hint.open": "Connect…", "ai.hint.off": "Don't show again",
});
applyI18n();

let ai = { available: false, apps: [], hint: "" };
let aiHintClosed = false;
const AI_DOCS = "https://github.com/kewe87/CaseHUD/blob/main/docs/API.md";

async function loadAI() {
  try { ai = await api("GET", "/api/ai"); } catch (e) { return; }
  renderAI();
}

function renderAIHint() {
  // On the settings page the AI connection is right there: no banner.
  const onSettings = !!$("#tab-settings") && $("#tab-settings").classList.contains("active");
  const show = !!ai.hint && !aiHintClosed && !onSettings;
  $("#ai-hint").hidden = !show;
  if (show) $("#ai-hint-text").textContent = t("ai.hint.text", { app: ai.hint });
}

function aiRadio(name, value, key, checked) {
  const input = el("input", { type: "radio", name, value, id: "ai-scope-" + value });
  input.checked = checked;
  return [input, el("label", { class: "check-row" }, input, el("span", {}, el("b", {}, t(key)), el("small", {}, t(key + ".sub"))))];
}

// aiInfo shows a dialog with a single close button.
function aiInfo({ title, text, extra }) {
  $("#notice-no").hidden = true;
  return askAccess({ title, text, ok: "ai.close", cancel: "ai.cancel", extra }).finally(() => { $("#notice-no").hidden = false; $("#notice-body").replaceChildren(); }); // the token must not stay in the page
}

function aiCopyRow(label, value, id) {
  const pre = el("pre", { class: "snippet", id }, value);
  const btn = el("button", { class: "ghost", onclick: async () => {
    try { await navigator.clipboard.writeText(value); toast(t("copied")); } catch (e) { toast(e.message, true); }
  } }, t("ai.copy"));
  return el("div", { class: "ai-copy" }, el("small", {}, label), el("div", { class: "ai-copy-row" }, pre, btn));
}

function aiShowResult(app, res, off) {
  const box = $("#ai-msg");
  const kids = [];
  if (res.manual) {
    kids.push(el("div", {}, t(off ? "ai.off.manual" : "ai.done.manual", { app: app.name })), aiCopyRow("", res.command, "ai-cmd"));
  } else {
    kids.push(el("div", { "data-ai-done": app.id }, t(off ? "ai.off.done" : "ai.done", { app: app.name })));
  }
  if (res.backup) {
    // The folder is not spelled out in the page; the path can be copied.
    kids.push(el("small", { "data-ai-backup": "", title: res.backup }, t("ai.done.backup"), " ",
      el("button", { class: "ghost ai-small", type: "button", onclick: async () => {
        try { await navigator.clipboard.writeText(res.backup); toast(t("copied")); } catch (e) { toast(e.message, true); }
      } }, t("ai.backup.copy"))));
  }
  kids.push(el("button", { class: "icon-btn ai-msg-close", type: "button", "aria-label": t("ai.close"), title: t("ai.close"), onclick: () => { box.hidden = true; box.replaceChildren(); } }, "✕"));
  box.replaceChildren(...kids);
  box.hidden = false;
}

const AI_CODES = ["conflict", "invalid_json", "duplicate_keys", "symlink", "not_found", "write_failed", "cli_failed", "unsupported"];
// aiErrCode reads the stable code of an error response (JSON {code}, or the code as text).
function aiErrCode(e) {
  if (e.code) return e.code;
  const raw = e.raw || "";
  try { const j = JSON.parse(raw); if (j && typeof j.code === "string") return j.code; } catch { /* plain text */ }
  return AI_CODES.find((c) => raw.includes(c)) || "";
}

// The text comes from the code, never from the server message (it may carry a path in the wrong language).
function aiError(app, e) {
  const code = aiErrCode(e);
  let key = "ai.err.generic";
  if (e.status === 409) key = "ai.err.conflict";
  else if (AI_CODES.includes(code)) key = "ai.err." + code;
  else if (e.status === 404) key = "ai.err.not_found";
  else if (e.status === 422) key = "ai.err.invalid";
  toast(t(key, { app: app.name }), true);
}

// aiAppFor is the connected AI app that a token named "MCP (<app>)" belongs to.
function aiAppFor(clientName) {
  return ai.apps.find((a) => a.state === "connected" && "MCP (" + a.name + ")" === clientName) || null;
}

async function aiConnect(app) {
  const connected = app.state === "connected";
  const [rRead, rowRead] = aiRadio("ai-scope", "read", "ai.scope.read", app.scope !== "control");
  const [rCtl, rowCtl] = aiRadio("ai-scope", "control", "ai.scope.control", app.scope === "control");
  void rRead;
  if (!(await askAccess({ title: t("ai.connect.title", { app: app.name }), text: t("ai.connect.text", { app: app.name }),
    ok: connected ? "ai.change.ok" : "ai.connect.ok", cancel: "ai.cancel", extra: [rowRead, rowCtl] }))) return;
  const scope = rCtl.checked ? "control" : "read";
  let replace = false;
  for (;;) {
    try {
      const res = await api("POST", "/api/ai/" + encodeURIComponent(app.id) + "/connect", { scope, replace });
      ai = res.view;
      renderAI();
      aiShowResult(app, res.result, false);
      loadSettingsSwitches();
      return;
    } catch (e) {
      if (e.status === 409 && !replace) {
        if (!(await askAccess({ title: t("ai.conflict.title"), text: t("ai.conflict.text", { app: app.name }), ok: "ai.conflict.ok", cancel: "ai.cancel", icon: "!" }))) return;
        replace = true;
        continue;
      }
      aiError(app, e);
      return;
    }
  }
}

async function aiDisconnect(app) {
  if (!(await askAccess({ title: t("ai.off.title", { app: app.name }), text: t("ai.off.text"), ok: "ai.off.ok", cancel: "ai.cancel", icon: "!" }))) return;
  try {
    const res = await api("DELETE", "/api/ai/" + encodeURIComponent(app.id));
    ai = res.view;
    renderAI();
    aiShowResult(app, res.result, true);
    pollAccess();
  } catch (e) {
    aiError(app, e);
  }
}

// loadSettingsSwitches refreshes the MCP switch after CaseHUD turned it on.
function loadSettingsSwitches() {
  const mcp = $("#set-mcp");
  if (mcp && ai.mcp) { mcp.checked = true; $("#row-mcp").hidden = false; if (state && state.settings) state.settings.mcp = true; }
  pollAccess();
}

function aiRow(a) {
  const sub = [];
  if (a.state === "connected" && a.scope) sub.push(t("acc.rights", { scope: scopeName(a.scope, a.system) }));
  if (a.manual && a.state !== "missing") sub.push(t("ai.manual"));
  const actions = el("div", { class: "acc-actions" });
  if (a.state === "found") actions.append(el("button", { class: "ghost", "data-ai-act": "connect", onclick: () => aiConnect(a) }, t("ai.connect")));
  if (a.state === "connected") {
    actions.append(
      el("button", { class: "ghost", "data-ai-act": "change", onclick: () => aiConnect(a) }, t("ai.change")),
      el("button", { class: "ghost", "data-ai-act": "disconnect", onclick: () => aiDisconnect(a) }, t("ai.disconnect")),
    );
  }
  const badge = el("span", { class: "badge " + (a.state === "connected" ? "ok" : a.state === "found" ? "accent" : "") }, t("ai.state." + a.state));
  return el("div", { class: "row", "data-ai": a.id, "data-state": a.state },
    el("div", { class: "row-text" }, el("b", {}, a.name), sub.length ? el("small", {}, sub.join(" · ")) : null), badge, actions);
}

function renderAI() {
  const list = $("#ai-list");
  if (!list) return;
  $("#set-ai").hidden = !ai.available;
  const nav = $("#set-nav-ai");
  if (nav) nav.hidden = !ai.available;
  list.replaceChildren(...ai.apps.map(aiRow));
  renderAIHint();
}

async function aiToken() {
  const name = el("input", { type: "text", id: "ai-tok-name", maxlength: "40", placeholder: t("ai.tok.name.ph"), class: "ai-name" });
  const [rRead, rowRead] = aiRadio("ai-tscope", "read", "ai.scope.read", true);
  const [rCtl, rowCtl] = aiRadio("ai-tscope", "control", "ai.scope.control", false);
  void rRead;
  const sys = el("input", { type: "checkbox", class: "switch", id: "ai-tok-system" });
  sys.disabled = true;
  rCtl.addEventListener("change", () => { sys.disabled = !rCtl.checked; });
  rRead.addEventListener("change", () => { sys.disabled = true; sys.checked = false; });
  const sysRow = el("label", { class: "check-row check-sub" }, sys, el("span", {}, el("b", {}, t("acc.sys")), el("small", {}, t("acc.sys.sub"))));
  const missing = el("small", { class: "ai-name-err", role: "alert", hidden: true }, t("ai.tok.noname"));
  const asked = askAccess({ title: t("ai.tok.title"), text: t("ai.tok.text"), ok: "ai.tok.ok", cancel: "ai.cancel",
    extra: [el("label", { class: "ai-name-row" }, el("small", {}, t("ai.tok.name")), name, missing), rowRead, rowCtl, sysRow] });
  // OK stays inert (and looks it) while the name is empty; the dialog stays open and the field gets the focus. Enter sends.
  const ok = $("#notice-ok");
  const sync = () => { const empty = !name.value.trim(); ok.setAttribute("aria-disabled", empty ? "true" : "false"); name.setAttribute("aria-invalid", "false"); if (!empty && !name.value.trim().startsWith("MCP (")) missing.hidden = true; };
  const guard = (ev) => {
    const v = name.value.trim();
    if (v && !v.startsWith("MCP (")) return;
    ev.stopImmediatePropagation();
    missing.textContent = t(v ? "ai.tok.reserved" : "ai.tok.noname");
    missing.hidden = false;
    name.setAttribute("aria-invalid", "true");
    name.focus();
  };
  ok.addEventListener("click", guard, true);
  name.addEventListener("input", sync);
  name.addEventListener("keydown", (ev) => { if (ev.key === "Enter") { ev.preventDefault(); ok.click(); } });
  sync();
  name.focus();
  const go = await asked;
  ok.removeEventListener("click", guard, true); // the button is shared with every other dialog
  ok.removeAttribute("aria-disabled");
  if (!go) return;
  try {
    const res = await api("POST", "/api/access/tokens", { name: name.value.trim(), scope: rCtl.checked ? "control" : "read", system: rCtl.checked && sys.checked });
    access = res.access;
    renderAccess();
    ai.api = true;
    await aiInfo({ title: t("ai.tok.done.title", { name: name.value.trim() }), text: t("ai.tok.done.text"), extra: [
      aiCopyRow(t("ai.tok.addr"), res.address, "ai-tok-addr"),
      aiCopyRow(t("ai.tok.token"), res.token, "ai-tok-token"),
      el("p", {}, el("small", {}, t("ai.tok.spec") + ": "), el("code", { id: "ai-tok-spec" }, res.openapi),
        " · ", el("a", { href: AI_DOCS, target: "_blank", rel: "noopener" }, t("ai.tok.docs"))),
    ] });
  } catch (e) {
    let m = e.message;
    try { const j = JSON.parse(e.raw || ""); if (j && j.message) m = j.message; } catch { /* plain text */ }
    toast(m, true);
  }
}

$("#btn-ai-token").addEventListener("click", aiToken);
$("#ai-hint-close").addEventListener("click", () => { aiHintClosed = true; renderAIHint(); });
$("#ai-hint-open").addEventListener("click", () => { showTab("settings"); const s = $("#set-ai"); if (s) s.scrollIntoView(); aiHintClosed = true; renderAIHint(); });
$("#ai-hint-off").addEventListener("click", async () => {
  try { ai = await api("POST", "/api/ai/hint/dismiss"); } catch (e) { toast(e.message, true); return; }
  renderAI();
});
document.addEventListener("visibilitychange", () => { if (!document.hidden) loadAI(); });
for (const tab of $$("[data-tab]")) tab.addEventListener("click", () => setTimeout(renderAIHint, 0));
$('[data-tab="settings"]').addEventListener("click", loadAI);
loadAI();
