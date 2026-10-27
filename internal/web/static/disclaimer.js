"use strict";

// ---------- Disclaimer, first-start dialog and one-time notices ----------
Object.assign(STRINGS.de, {
  "disc.first.sub": "Bitte lesen, bevor CaseHUD Geräte ansteuert",
  "disc.ok": "Verstanden – weiter", "disc.quit": "Beenden",
  "disc.status": "Zugestimmt am {d} (Version {v})", "disc.status.none": "Noch nicht zugestimmt – CaseHUD spricht kein Gerät an.",
  "ack.cancel": "Abbrechen",
  "ack.exp.title": "Experimentelle Geräte", "ack.exp.sub": "Einmalige Bestätigung", "ack.exp.ok": "Verstanden – aktivieren",
  "ack.exp.p1": "Die Treiber dieser Geräte sind noch ungetestet. Die Geräte können unerwartet reagieren.",
  "ack.exp.p2": "CaseHUD schreibt dabei nichts dauerhaft in die Geräte.",
  "ack.exp.p3": "Rückmeldungen sind willkommen – ein kurzer Testbericht hilft allen.",
  "ack.light.title": "Beleuchtung", "ack.light.sub": "Einmalige Bestätigung", "ack.light.ok": "Verstanden – einschalten",
  "ack.light.p1": "Damit steuert CaseHUD die Beleuchtung von Mainboard, Lüfter-Controller und RAM.",
  "ack.light.p2": "Je nach Einstellung „Mit Mainboard synchronisieren“ können Beleuchtungseinstellungen auch nach dem Beenden von CaseHUD aktiv bleiben.",
  "ack.err": "Konnte nicht gespeichert werden: {e}",
});
Object.assign(STRINGS.en, {
  "disc.first.sub": "Please read before CaseHUD talks to any device",
  "disc.ok": "Understood – continue", "disc.quit": "Quit",
  "disc.status": "Accepted on {d} (version {v})", "disc.status.none": "Not accepted yet – CaseHUD does not talk to any device.",
  "ack.cancel": "Cancel",
  "ack.exp.title": "Experimental devices", "ack.exp.sub": "One-time confirmation", "ack.exp.ok": "Understood – enable",
  "ack.exp.p1": "The drivers for these devices are untested. The devices may react unexpectedly.",
  "ack.exp.p2": "CaseHUD does not write anything persistent to the devices.",
  "ack.exp.p3": "Feedback is welcome – a short test report helps everyone.",
  "ack.light.title": "Lighting", "ack.light.sub": "One-time confirmation", "ack.light.ok": "Understood – turn on",
  "ack.light.p1": "This lets CaseHUD control the lighting of the mainboard, the fan controller and the RAM.",
  "ack.light.p2": "Depending on “Sync with motherboard”, lighting settings may stay active after you quit CaseHUD.",
  "ack.err": "Could not save: {e}",
});
applyI18n();

// notice shows the shared dialog and resolves true (ok) or false (cancel).
// With locked set, Esc does nothing: the first-start dialog has to be answered.
function notice({ title, sub, paras, ok, cancel, locked }) {
  const dlg = $("#notice");
  return new Promise((resolve) => {
    const set = (id, key) => { const e = $(id); e.dataset.i18n = key; e.textContent = t(key); };
    set("#notice-title", title); set("#notice-sub", sub); set("#notice-ok", ok); set("#notice-no", cancel);
    $("#notice-body").replaceChildren(...paras.map((k) => el("p", { "data-i18n": k }, t(k))));
    const done = (v) => {
      dlg.removeEventListener("cancel", onCancel);
      $("#notice-ok").onclick = $("#notice-no").onclick = null;
      dlg.close();
      resolve(v);
    };
    const onCancel = (e) => { e.preventDefault(); if (!locked) done(false); };
    dlg.addEventListener("cancel", onCancel);
    $("#notice-ok").onclick = () => done(true);
    $("#notice-no").onclick = () => done(false);
    dlg.showModal();
    $("#notice-ok").focus();
  });
}

// ensureAck asks for a one-time confirmation ("experimental" or "lighting")
// unless it is on record, and records it. False: the user declined.
async function ensureAck(what) {
  const a = state && state.acks;
  if (a && (what === "experimental" ? a.ackExperimental : a.ackLighting)) return true;
  const k = what === "experimental" ? "ack.exp" : "ack.light";
  const paras = what === "experimental" ? [k + ".p1", k + ".p2", k + ".p3"] : [k + ".p1", k + ".p2"];
  if (!(await notice({ title: k + ".title", sub: k + ".sub", paras, ok: k + ".ok", cancel: "ack.cancel" }))) return false;
  try {
    const next = await api("POST", "/api/ack", { what });
    if (state) state.acks = next;
    return true;
  } catch (e) {
    toast(t("ack.err", { e: e.message }), true);
    return false;
  }
}

// checkDisclaimer asks for the disclaimer while it is not accepted in its
// current version. Until then the engine sends nothing to any device.
let discOpen = false;
async function checkDisclaimer() {
  renderDisclaimer();
  const a = state && state.acks;
  if (!a || a.accepted || discOpen) return;
  discOpen = true;
  const yes = await notice({ title: "disc.title", sub: "disc.first.sub", paras: ["disc.p1", "disc.p2", "disc.p3", "disc.p4", "disc.p5", "disc.p6", "disc.p7"],
    ok: "disc.ok", cancel: "disc.quit", locked: true });
  if (!yes) {
    try { await api("POST", "/api/quit"); } catch { /* the app is gone */ }
    return; // the dialog stays closed only because the app ends
  }
  try {
    state.acks = await api("POST", "/api/disclaimer/accept", { version: a.version });
    discOpen = false;
    renderDisclaimer();
    setTimeout(() => refresh().catch(() => {}), 1500); // the displays appear now
  } catch (e) {
    toast(t("ack.err", { e: e.message }), true);
    discOpen = false;
    checkDisclaimer();
  }
}

function renderDisclaimer() {
  const a = state && state.acks, s = $("#disc-status");
  if (!a || !s) return;
  const d = a.acceptedAt ? new Date(a.acceptedAt) : null;
  s.textContent = a.accepted && d ? t("disc.status", { d: d.toLocaleDateString(LANG === "de" ? "de-DE" : "en-GB"), v: a.version }) : t("disc.status.none");
}
for (const b of $$("[data-lang]")) b.addEventListener("click", renderDisclaimer);
if (state) checkDisclaimer();
