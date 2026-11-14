"use strict";

// ---------- Access of other programs: the request dialog and the list in the settings ----------
Object.assign(STRINGS.de, {
  "acc.title": "Zugänge",
  "acc.empty": "Kein Programm verbunden. Programme fragen den Zugriff selbst an.",
  "acc.default": "Standard-Zugang", "acc.default.sub": "Token aus api.json, z. B. für CouchHUD",
  "acc.scope.read": "Nur lesen", "acc.scope.control": "Steuern",
  "acc.sys": "Auch System erlauben (Beenden, Updates)", "acc.sys.sub": "Nur mit „Steuern“. Das Programm kann CaseHUD beenden und aktualisieren.",
  "acc.sys.asked": "Angefragt: auch System.",
  "acc.waiting": "wartet auf Abholung",
  "acc.rights": "Rechte: {scope}", "acc.used": "zuletzt verwendet: {when}",
  "acc.never": "nie", "acc.now": "gerade eben", "acc.min": "vor {n} Min.", "acc.hour": "vor {n} Std.", "acc.day": "vor {n} Tagen",
  "acc.temp": "bis CaseHUD beendet wird",
  "acc.change": "Rechte ändern", "acc.revoke": "Entziehen", "acc.renew": "Erneuern", "acc.cancel": "Abbrechen",
  "acc.change.title": "Rechte von „{name}“ ändern", "acc.change.sub": "Aktuell: {scope}",
  "acc.change.apply": "Übernehmen",
  "acc.change.text": "Die Änderung gilt sofort.",
  "acc.revoke.title": "Zugriff entziehen", "acc.revoke.text": "„{name}“ verliert den Zugriff sofort. Eine neue Anfrage muss erneut bestätigt werden.",
  "acc.renew.title": "Standard-Zugang erneuern", "acc.renew.text": "Der Token in api.json wird ersetzt, der alte endet sofort. Programme, die ihn nur beim Start lesen, müssen neu gestartet werden.",
  "acc.req.title": "{name} möchte auf CaseHUD zugreifen", "acc.req.sub": "Anfrage über {via}",
  "acc.req.deny": "Ablehnen",
  "acc.req.hint.read": "Nur lesen: Zustand, Sensoren und Vorschauen abrufen.",
  "acc.req.hint.control": "Steuern: zusätzlich Displays, Beleuchtung und Kühlung ändern. Nur bei bekannten Programmen erlauben.",
  "acc.req.n": "Anfrage {i} von {n}",
  "acc.remember": "Merken", "acc.remember.sub": "Der Zugriff bleibt nach einem Neustart erhalten. Sonst gilt er, bis CaseHUD beendet wird.",
  "acc.via.api": "API", "acc.via.mcp": "MCP",
});
Object.assign(STRINGS.en, {
  "acc.title": "Access",
  "acc.empty": "No program connected. Programs ask for access themselves.",
  "acc.default": "Default access", "acc.default.sub": "Token from api.json, e.g. for CouchHUD",
  "acc.scope.read": "Read only", "acc.scope.control": "Control",
  "acc.sys": "Also allow System (quit, updates)", "acc.sys.sub": "Only with “Control”. The program can quit and update CaseHUD.",
  "acc.sys.asked": "Requested: System as well.",
  "acc.waiting": "waiting to be collected",
  "acc.rights": "Rights: {scope}", "acc.used": "last used: {when}",
  "acc.never": "never", "acc.now": "just now", "acc.min": "{n} min ago", "acc.hour": "{n} h ago", "acc.day": "{n} days ago",
  "acc.temp": "until CaseHUD quits",
  "acc.change": "Change rights", "acc.revoke": "Revoke", "acc.renew": "Renew", "acc.cancel": "Cancel",
  "acc.change.title": "Change rights of “{name}”", "acc.change.sub": "Now: {scope}",
  "acc.change.apply": "Apply",
  "acc.change.text": "The change applies at once.",
  "acc.revoke.title": "Revoke access", "acc.revoke.text": "“{name}” loses its access at once. A new request has to be confirmed again.",
  "acc.renew.title": "Renew default access", "acc.renew.text": "The token in api.json is replaced and the old one ends at once. Programs that read it only at start have to be restarted.",
  "acc.req.title": "{name} wants to access CaseHUD", "acc.req.sub": "Request through {via}",
  "acc.req.deny": "Deny",
  "acc.req.hint.read": "Read only: fetch status, sensors and previews.",
  "acc.req.hint.control": "Control: also change displays, lighting and cooling. Only for programs you know.",
  "acc.req.n": "Request {i} of {n}",
  "acc.remember": "Remember", "acc.remember.sub": "The access stays after a restart. Otherwise it lasts until CaseHUD quits.",
  "acc.via.api": "API", "acc.via.mcp": "MCP",
});
applyI18n();

let access = { clients: [], pending: [], api: false, mcp: false };
let accessReqID = ""; // the request the dialog shows

const scopeName = (s, sys) => t(s === "control" ? "acc.scope.control" : "acc.scope.read") + (sys ? " + System" : "");

function accessWhen(iso) {
  if (!iso) return t("acc.never");
  const sec = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (sec < 60) return t("acc.now");
  if (sec < 3600) return t("acc.min", { n: Math.floor(sec / 60) });
  if (sec < 86400) return t("acc.hour", { n: Math.floor(sec / 3600) });
  return t("acc.day", { n: Math.floor(sec / 86400) });
}

// askAccess shows the shared notice dialog with texts that carry a program name.
function askAccess({ title, sub, text, ok, cancel, extra, icon }) {
  const p = notice({ title: "acc.cancel", sub: "acc.cancel", paras: [], ok, cancel, icon: icon || "i" });
  $("#notice-title").textContent = title;
  $("#notice-sub").textContent = sub || "";
  $("#notice-body").replaceChildren(el("p", {}, text), ...(extra || []));
  return p;
}

async function accessCall(method, path, body) {
  try {
    access = await api(method, path, body);
    renderAccess();
    return true;
  } catch (e) {
    toast(e.message, true);
    return false;
  }
}

function accessRow(c) {
  // An AI app set up through the AI connection appears once, under its own name, and is disconnected there.
  const app = c.interface === "mcp" && typeof aiAppFor === "function" ? aiAppFor(c.name) : null;
  const rights = t("acc.rights", { scope: scopeName(c.scope, c.system) });
  const meta = [t("acc.via." + c.interface), rights, c.waiting ? t("acc.waiting") : t("acc.used", { when: accessWhen(c.lastUsed) })];
  if (!c.remember && !c.waiting) meta.push(t("acc.temp"));
  const name = c.builtin ? t("acc.default") : app ? app.name : c.name;
  const sub = c.builtin ? t("acc.default.sub") + " · " + [rights, meta[2]].join(" · ") : meta.join(" · ");
  const actions = el("div", { class: "acc-actions" });
  const revoke = () => el("button", { class: "ghost", "data-acc": "revoke", onclick: async () => {
    if (!(await askAccess({ title: t("acc.revoke.title"), text: t("acc.revoke.text", { name: c.name }), ok: "acc.revoke", cancel: "acc.cancel", icon: "!" }))) return;
    if (await accessCall("DELETE", "/api/access/clients/" + encodeURIComponent(c.id))) toast(t("saved"));
  } }, t("acc.revoke"));
  if (c.waiting) {
    actions.append(revoke());
  } else if (c.builtin) {
    actions.append(el("button", { class: "ghost", "data-acc": "renew", onclick: async () => {
      if (!(await askAccess({ title: t("acc.renew.title"), text: t("acc.renew.text"), ok: "acc.renew", cancel: "acc.cancel", icon: "!" }))) return;
      if (await accessCall("DELETE", "/api/access/clients/default")) toast(t("saved"));
    } }, t("acc.renew")));
  } else {
    actions.append(
      el("button", { class: "ghost", "data-acc": "change", onclick: async () => {
        // The same choice as everywhere: a pair of radio buttons, System as an indented switch below.
        const radio = (value, on) => {
          const input = el("input", { type: "radio", name: "acc-chg-scope", value, id: "acc-chg-" + value });
          input.checked = on;
          return [input, el("label", { class: "check-row" }, input, el("span", {}, el("b", {}, t("acc.scope." + value)), el("small", {}, t("ai.scope." + value + ".sub"))))];
        };
        const [rd, rdRow] = radio("read", c.scope !== "control");
        const [ctl, ctlRow] = radio("control", c.scope === "control");
        const sys = el("input", { type: "checkbox", class: "switch", id: "acc-chg-system" });
        sys.checked = !!c.system;
        const sysRow = el("label", { class: "check-row check-sub" }, sys, el("span", {}, el("b", {}, t("acc.sys")), el("small", {}, t("acc.sys.sub"))));
        sys.disabled = !ctl.checked;
        const sync = () => { if (!ctl.checked) sys.checked = false; sys.disabled = !ctl.checked; };
        ctl.addEventListener("change", sync);
        rd.addEventListener("change", sync);
        if (!(await askAccess({ title: t("acc.change.title", { name: c.name }), sub: t("acc.change.sub", { scope: scopeName(c.scope, c.system) }),
          text: t("acc.change.text"), ok: "acc.change.apply", cancel: "acc.cancel", extra: [rdRow, ctlRow, sysRow] }))) return;
        const body = { scope: ctl.checked ? "control" : "read", system: ctl.checked && sys.checked };
        if (await accessCall("PUT", "/api/access/clients/" + encodeURIComponent(c.id), body)) toast(t("saved"));
      } }, t("acc.change")),
      app
        ? el("button", { class: "ghost", "data-acc": "revoke", onclick: () => aiDisconnect(app) }, t("ai.disconnect"))
        : revoke(),
    );
  }
  return el("div", { class: "row", "data-client": c.id },
    el("div", { class: "row-text" }, el("b", {}, name), el("small", {}, sub)), actions);
}

function renderAccess() {
  const list = $("#access-list");
  if (!list) return;
  const rows = access.clients.map(accessRow);
  list.replaceChildren(...(rows.length ? rows : [el("div", { class: "row" }, el("div", { class: "row-text" }, el("small", { "data-i18n": "acc.empty" }, t("acc.empty"))))]));
  showAccessRequest();
}

// showAccessRequest asks about the oldest open request; it ends when the request is gone.
function showAccessRequest() {
  const dlg = $("#access-req");
  const req = access.pending[0];
  if (!req) {
    if (dlg.open) dlg.close();
    accessReqID = "";
    return;
  }
  $("#access-count").textContent = access.pending.length > 1 ? t("acc.req.n", { i: 1, n: access.pending.length }) : "";
  if (accessReqID === req.id && dlg.open) return;
  accessReqID = req.id;
  $("#access-title").textContent = t("acc.req.title", { name: req.name });
  $("#access-sub").textContent = t("acc.req.sub", { via: t("acc.via." + req.interface) });
  $("#access-hint").textContent = t(req.scope === "control" ? "acc.req.hint.control" : "acc.req.hint.read") + (req.system ? " " + t("acc.sys.asked") : "");
  $("#access-remember").checked = false;
  $("#access-system").checked = false; // never on by itself
  $("#access-system-row").hidden = req.scope !== "control";
  $("#access-control").hidden = req.scope !== "control";
  if (!dlg.open) dlg.showModal();
  $("#access-deny").focus(); // never a button that grants
}

async function decideAccess(allow, scope) {
  const system = allow && scope === "control" && $("#access-system").checked;
  const id = accessReqID;
  if (!id) return;
  try {
    access = await api("POST", "/api/access/requests/" + encodeURIComponent(id), { allow, scope: scope || "", system, remember: $("#access-remember").checked });
  } catch (e) {
    if (e.status !== 404) toast(e.message, true); // gone: expired or answered elsewhere
    await pollAccess();
    return;
  }
  accessReqID = "";
  renderAccess();
}
$("#access-deny").addEventListener("click", () => decideAccess(false));
$("#access-read").addEventListener("click", () => decideAccess(true, "read"));
$("#access-control").addEventListener("click", () => decideAccess(true, "control"));
// Esc is a refusal: a request is never left open by accident.
$("#access-req").addEventListener("cancel", (e) => { e.preventDefault(); decideAccess(false); });

async function pollAccess() {
  if (document.hidden) return;
  try { access = await api("GET", "/api/access"); } catch (e) { return; }
  renderAccess();
}
setInterval(() => { if (state && state.settings && (state.settings.api || state.settings.mcp || access.pending.length)) pollAccess(); }, 2000);
document.addEventListener("visibilitychange", () => { if (!document.hidden) pollAccess(); });
pollAccess();
