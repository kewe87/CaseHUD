"use strict";

// ---------- Settings: start image of the LCD fans ----------
Object.assign(STRINGS.de, {
  "tls.title": "CaseHUD-Logo als Startbild",
  "tls.sub": "Wird einmalig in die Lüfter-LCDs (Start- und Ruhebild) und das 8,8\"-Display (Startbild) geschrieben und erscheint, wenn CaseHUD nicht läuft. Beim 8,8\" gibt es keinen Weg zurück zum Hersteller-Logo.",
  "tls.serial": "Seriennummer für neue Displays vergeben",
  "tls.serial.sub": "Ein Lüfter-Display ohne Seriennummer bekommt einmalig eine (dauerhafter Schreibvorgang), damit es eindeutig erkannt wird.",
  "tls.again": "Erneut schreiben",
  "tls.again.sub": "Schreibt das Logo noch einmal in alle angeschlossenen Lüfter-LCDs und das 8,8\"-Display.",
  "tls.confirm": "Das Logo wird dauerhaft in den Speicher der Lüfter-LCDs geschrieben. Das geht nur begrenzt oft. Das 8,8\"-Display wird nur neu beschrieben, wenn sich sein Bild geändert hat; dort gibt es keinen Weg zurück zum Hersteller-Logo. Jetzt schreiben?",
  "tls.started": "Logo wird geschrieben ({n} Display(s))",
  "tls.none": "Keine passenden Displays angeschlossen",
  "tls.err": "Konnte nicht gespeichert werden: {e}",
});
Object.assign(STRINGS.en, {
  "tls.title": "CaseHUD logo as start image",
  "tls.sub": "Written once into the fan LCDs (start and idle image) and the 8.8\" display (start image); it shows when CaseHUD is not running. On the 8.8\" there is no way back to the maker's logo.",
  "tls.serial": "Assign serial numbers to new displays",
  "tls.serial.sub": "A fan display without a serial number gets one once (a permanent write) so it can be told apart reliably.",
  "tls.again": "Write again",
  "tls.again.sub": "Writes the logo once more to all connected fan LCDs and the 8.8\" display.",
  "tls.confirm": "The logo is written permanently into the memory of the fan LCDs. This only works a limited number of times. The 8.8\" display is only rewritten if its picture changed; there is no way back to the maker's logo on it. Write it now?",
  "tls.started": "Writing the logo ({n} display(s))",
  "tls.none": "No suitable displays connected",
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
