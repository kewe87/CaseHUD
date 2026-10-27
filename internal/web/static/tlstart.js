"use strict";

// ---------- Settings: start image of the LCD fans ----------
Object.assign(STRINGS.de, {
  "tls.title": "CaseHUD-Logo als Start- und Ruhebild",
  "tls.sub": "Wird einmalig in die Displays geschrieben und erscheint, wenn CaseHUD nicht läuft.",
  "tls.serial": "Seriennummer für neue Displays vergeben",
  "tls.serial.sub": "Ein Lüfter-Display ohne Seriennummer bekommt einmalig eine (dauerhafter Schreibvorgang), damit es eindeutig erkannt wird.",
  "tls.again": "Erneut schreiben",
  "tls.again.sub": "Schreibt Start- und Ruhebild noch einmal in alle angeschlossenen Lüfter-LCDs.",
  "tls.confirm": "Start- und Ruhebild werden dauerhaft in den Speicher der Lüfter-LCDs geschrieben. Das geht nur begrenzt oft. Jetzt schreiben?",
  "tls.started": "Start- und Ruhebild werden geschrieben ({n} Display(s))",
  "tls.none": "Keine Lüfter-LCDs angeschlossen",
  "tls.err": "Konnte nicht gespeichert werden: {e}",
});
Object.assign(STRINGS.en, {
  "tls.title": "CaseHUD logo as start and idle image",
  "tls.sub": "Written once into the displays; it shows when CaseHUD is not running.",
  "tls.serial": "Assign serial numbers to new displays",
  "tls.serial.sub": "A fan display without a serial number gets one once (a permanent write) so it can be told apart reliably.",
  "tls.again": "Write again",
  "tls.again.sub": "Writes the start and idle image once more to all connected fan LCDs.",
  "tls.confirm": "The start and idle image are written permanently into the fan LCDs' memory, which only works a limited number of times. Write it now?",
  "tls.started": "Writing the start and idle image ({n} display(s))",
  "tls.none": "No fan LCDs connected",
  "tls.err": "Could not save: {e}",
});
applyI18n();

async function loadTLStart() {
  try { $("#set-tls").checked = !!(await api("GET", "/api/tl-start-image")).on; } catch { /* the row stays as it is */ }
  try { $("#set-tls-serial").checked = !!(await api("GET", "/api/tl-serial")).on; } catch { /* the row stays as it is */ }
}
$("#set-tls-serial").addEventListener("change", async () => {
  try { await api("PUT", "/api/tl-serial", { on: $("#set-tls-serial").checked }); }
  catch (e) { toast(t("tls.err", { e: e.message || e }), true); loadTLStart(); }
});
$("#set-tls").addEventListener("change", async () => {
  try { await api("PUT", "/api/tl-start-image", { on: $("#set-tls").checked }); }
  catch (e) { toast(t("tls.err", { e: e.message || e }), true); loadTLStart(); }
});
$("#tls-again").addEventListener("click", async () => {
  if (!confirm(t("tls.confirm"))) return;
  try {
    const r = await api("POST", "/api/tl-start-image/rewrite", { confirm: true });
    toast(r.fans ? t("tls.started", { n: r.fans }) : t("tls.none"));
  } catch (e) { toast(t("tls.err", { e: e.message || e }), true); }
});
loadTLStart();
