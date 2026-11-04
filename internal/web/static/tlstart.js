"use strict";

// ---------- Settings: start image of the LCD fans ----------
Object.assign(STRINGS.de, {
  "tls.title": "CaseHUD-Logo als Startbild",
  "tls.sub": "Standardmäßig aus. Schreibt das Logo dauerhaft in die Lüfter-LCDs (Start- und Ruhebild), das geht nur begrenzt oft. Es erscheint, wenn CaseHUD nicht läuft.",
  "tls.serial": "Seriennummer für neue Displays vergeben",
  "tls.serial.sub": "Ein Lüfter-Display ohne Seriennummer bekommt einmalig eine (dauerhafter Schreibvorgang), damit es eindeutig erkannt wird.",
  "tls.again": "Erneut schreiben",
  "tls.again.sub": "Schreibt das Logo noch einmal in alle angeschlossenen Lüfter-LCDs.",
  "tls.confirm": "Das Logo wird dauerhaft in den Speicher der Lüfter-LCDs geschrieben (Start- und Ruhebild). Das geht nur begrenzt oft. Jetzt schreiben?",
  "tls.started": "Logo wird geschrieben ({n} Display(s))",
  "tls.none": "Keine passenden Displays angeschlossen",
  "tls.err": "Konnte nicht gespeichert werden: {e}",
});
Object.assign(STRINGS.en, {
  "tls.title": "CaseHUD logo as start image",
  "tls.sub": "Off by default. Writes the logo permanently into the fan LCDs (start and idle image), which only works a limited number of times. It shows when CaseHUD is not running.",
  "tls.serial": "Assign serial numbers to new displays",
  "tls.serial.sub": "A fan display without a serial number gets one once (a permanent write) so it can be told apart reliably.",
  "tls.again": "Write again",
  "tls.again.sub": "Writes the logo once more to all connected fan LCDs.",
  "tls.confirm": "The logo is written permanently into the memory of the fan LCDs (start and idle image). This only works a limited number of times. Write it now?",
  "tls.started": "Writing the logo ({n} display(s))",
  "tls.none": "No suitable displays connected",
  "tls.err": "Could not save: {e}",
});
applyI18n();

async function loadTLStart() {
  try { $("#set-tls").checked = !!(await api("GET", "/api/tl-start-image")).on; } catch { /* the row stays as it is */ }
  $("#tls-again").disabled = !$("#set-tls").checked;
  try { $("#set-tls-serial").checked = !!(await api("GET", "/api/tl-serial")).on; } catch { /* the row stays as it is */ }
}
$("#set-tls-serial").addEventListener("change", async () => {
  try { await api("PUT", "/api/tl-serial", { on: $("#set-tls-serial").checked }); }
  catch (e) { toast(t("tls.err", { e: e.message || e }), true); loadTLStart(); }
});
$("#set-tls").addEventListener("change", async () => {
  $("#tls-again").disabled = !$("#set-tls").checked;
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
