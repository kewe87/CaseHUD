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
  "cap.title": "Bildschirm- und Audio-Effekte erlauben",
  "cap.sub": "Bildschirm-Ambilight und Audio-Visualizer lesen Bildschirm und Audioausgabe. Ausschalten widerruft die Erlaubnis und stellt einen laufenden Effekt auf den Standardeffekt zurück.",
  "ack.cap.title": "Bildschirm und Audio", "ack.cap.sub": "Einmalige Bestätigung", "ack.cap.ok": "Verstanden – erlauben",
  "ack.cap.p1": "Diese Effekte lesen den Bildschirminhalt (Bildschirm-Ambilight) beziehungsweise die Audioausgabe (Audio-Visualizer) und leiten daraus die LED-Farben ab.",
  "ack.cap.p2": "Bilder und Töne werden nur im Arbeitsspeicher verarbeitet, nie gespeichert, protokolliert, exportiert oder über das Netzwerk gesendet. Gelesen wird nur, solange der Effekt läuft.",
  "ack.cap.p3": "Auch die LED-Farben spiegeln den Inhalt wider. Darum nimmt die Diagnose bei diesen Effekten keinen Datenverkehr der Beleuchtungsgeräte auf.",
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
  "cap.title": "Allow screen and audio effects",
  "cap.sub": "Screen ambilight and the audio visualizer read the screen and the audio output. Switching off withdraws the permission and returns a running effect to the default one.",
  "ack.cap.title": "Screen and audio", "ack.cap.sub": "One-time confirmation", "ack.cap.ok": "Understood – allow",
  "ack.cap.p1": "These effects read what is on the screen (screen ambilight) or what the computer plays (audio visualizer) and turn it into LED colors.",
  "ack.cap.p2": "Pictures and sound are only processed in memory and are never stored, logged, exported or sent over the network. Nothing is read while the effect is not running.",
  "ack.cap.p3": "The LED colors also reflect that content, so diagnostics do not record the traffic to the lighting devices while one of these effects is in use.",
  "ack.err": "Could not save: {e}",
});
applyI18n();

// notice shows the shared dialog and resolves true (ok) or false (cancel).
// With locked set, Esc does nothing: the first-start dialog has to be answered.
/** @param {{ title: string, sub: string, paras: string[], ok: string, cancel: string, locked?: boolean }} o */
function notice({ title, sub, paras, ok, cancel, locked }) {
  const dlg = $("#notice");
  return new Promise((resolve) => {
    const set = (id, key) => { const e = $(id); e.dataset.i18n = key; e.textContent = t(key); };
    set("#notice-title", title); set("#notice-sub", sub); set("#notice-ok", ok); set("#notice-no", cancel);
    $("#notice-body").replaceChildren(...paras.map((k) => el("p", { "data-i18n": k }, t(k))));
    const done = (v) => {
      dlg.removeEventListener("cancel", onCancel);
      dlg.removeEventListener("close", onClose);
      $("#notice-ok").onclick = $("#notice-no").onclick = null;
      dlg.close();
      resolve(v);
    };
    const onCancel = (e) => { e.preventDefault(); if (!locked) done(false); };
    // A second Esc closes a modal dialog without a cancel event: a locked one comes straight back.
    const onClose = () => { if (locked && dlg.isConnected && !dlg.open) dlg.showModal(); };
    dlg.addEventListener("cancel", onCancel);
    if (locked) dlg.addEventListener("close", onClose);
    $("#notice-ok").onclick = () => done(true);
    $("#notice-no").onclick = () => done(false);
    dlg.showModal();
    $("#notice-ok").focus();
  });
}

// ensureAck asks for a one-time confirmation ("experimental", "lighting" or
// "capture") unless it is on record, and records it. False: the user declined.
const ACKS = {
  experimental: { flag: "ackExperimental", key: "ack.exp", n: 3 },
  lighting: { flag: "ackLighting", key: "ack.light", n: 2 },
  capture: { flag: "ackCapture", key: "ack.cap", n: 3 },
};
async function ensureAck(what) {
  const a = state && state.acks;
  const { flag, key: k, n } = ACKS[what];
  if (a && a[flag]) return true;
  const paras = Array.from({ length: n }, (_, i) => k + ".p" + (i + 1));
  if (!(await notice({ title: k + ".title", sub: k + ".sub", paras, ok: k + ".ok", cancel: "ack.cancel" }))) return false;
  try {
    const next = await api("POST", "/api/ack", { what });
    if (state) state.acks = next;
    renderCapConsent();
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

// The switch shows the permission for the screen and audio effects. Off:
// withdraw it (the running capture effect ends); on: ask first.
function renderCapConsent() {
  const sw = $("#cap-consent");
  if (sw && state && state.acks) sw.checked = !!state.acks.ackCapture;
}
$("#cap-consent").addEventListener("change", async (e) => {
  const on = e.target.checked;
  if (on) {
    if (!(await ensureAck("capture"))) e.target.checked = false;
    return;
  }
  try {
    state.acks = await api("DELETE", "/api/ack/capture");
    if (typeof loadLight === "function") loadLight().catch(() => {});
  } catch (err) {
    toast(t("ack.err", { e: err.message }), true);
  }
  renderCapConsent();
});

function renderDisclaimer() {
  renderCapConsent();
  const a = state && state.acks, s = $("#disc-status");
  if (!a || !s) return;
  const d = a.acceptedAt ? new Date(a.acceptedAt) : null;
  s.textContent = a.accepted && d ? t("disc.status", { d: d.toLocaleDateString(LANG === "de" ? "de-DE" : "en-GB"), v: a.version }) : t("disc.status.none");
}
for (const b of $$("[data-lang]")) b.addEventListener("click", renderDisclaimer);
if (state) checkDisclaimer();
