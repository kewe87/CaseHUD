"use strict";

// ---------- Settings: start image of the LCD fans ----------
Object.assign(STRINGS.de, {
  "tls.title": "CaseHUD-Logo als Startbild",
  "tls.sub": "Standardmäßig aus. Schreibt das Logo dauerhaft in die Lüfter-LCDs (Start- und Ruhebild); nur begrenzt oft möglich. Anzeige, wenn CaseHUD nicht läuft.",
  "tls.serial": "Seriennummer für neue Displays vergeben",
  "tls.serial.sub": "Lüfter-Displays ohne Seriennummer erhalten einmalig eine (dauerhafter Schreibvorgang) zur eindeutigen Erkennung.",
  "tls.again": "Erneut schreiben",
  "tls.again.title": "Startbild neu schreiben",
  "tls.again.sub": "Schreibt das Logo noch einmal in alle angeschlossenen Lüfter-LCDs.",
  "tls.again.off": "Nur bei eingeschaltetem Startbild möglich.",
  "tls.confirm": "Das Logo wird dauerhaft in den Speicher der Lüfter-LCDs geschrieben (Start- und Ruhebild), nur begrenzt oft möglich. Jetzt schreiben?",
  "tls.started": "Logo wird geschrieben ({n} Display(s))",
  "tls.none": "Keine passenden Displays angeschlossen",
  "tls.err": "Speichern fehlgeschlagen: {e}",
  "tls.style": "Startbild",
  "tls.style.sub": "Eigenes Logo oder schlichtes Neutralbild ohne Logo.",
  "tls.style.logo": "CaseHUD-Logo",
  "tls.style.neutral": "Neutralbild",
  "tls.style.confirm": "Das Startbild wird gewechselt und dauerhaft in die Lüfter-LCDs geschrieben, nur begrenzt oft möglich. Jetzt wechseln?",
  "boot.title": "Startlogo 8,8\" entfernen",
  "boot.sub": "CaseHUD hat früher ein Startlogo auf dem 8,8\"-Display gespeichert. Entfernt diese Datei dauerhaft vom Gerät.",
  "boot.btn": "Entfernen",
  "boot.confirm": "Das von CaseHUD gespeicherte Startlogo wird dauerhaft vom Display gelöscht. Jetzt entfernen?",
  "boot.done": "Startlogo entfernt ({n} Display(s))",
  "boot.none": "Nichts entfernt",
});
Object.assign(STRINGS.en, {
  "tls.title": "CaseHUD logo as start image",
  "tls.sub": "Off by default. Writes the logo permanently to the fan LCDs (start and idle image); only possible a limited number of times. Shown while CaseHUD is not running.",
  "tls.serial": "Assign serial numbers to new displays",
  "tls.serial.sub": "Fan displays without a serial number receive one once (permanent write) for reliable identification.",
  "tls.again": "Write again",
  "tls.again.title": "Rewrite start image",
  "tls.again.sub": "Writes the logo once more to all connected fan LCDs.",
  "tls.again.off": "Only available while the start image is on.",
  "tls.confirm": "The logo is written permanently to the memory of the fan LCDs (start and idle image), only possible a limited number of times. Write now?",
  "tls.started": "Writing the logo ({n} display(s))",
  "tls.none": "No suitable displays connected",
  "tls.err": "Save failed: {e}",
  "tls.style": "Start image",
  "tls.style.sub": "CaseHUD logo or a plain neutral picture without a logo.",
  "tls.style.logo": "CaseHUD logo",
  "tls.style.neutral": "Neutral picture",
  "tls.style.confirm": "The start image is changed and written permanently to the fan LCDs, only possible a limited number of times. Change now?",
  "boot.title": "Remove 8.8\" start logo",
  "boot.sub": "CaseHUD stored a start logo on the 8.8\" display earlier. Removes that file from the device permanently.",
  "boot.btn": "Remove",
  "boot.confirm": "The start logo stored by CaseHUD is deleted permanently from the display. Remove now?",
  "boot.done": "Start logo removed ({n} display(s))",
  "boot.none": "Nothing removed",
});
applyI18n();

// The rewrite button needs the start image on; the reason is shown next to it.
function tlsAgainSync() {
  const off = !$("#set-tls").checked;
  $("#tls-again").disabled = off;
  $("#tls-again-why").hidden = !off;
}

async function loadTLStart() {
  try {
    const r = await api("GET", "/api/tl-start-image");
    $("#set-tls").checked = !!r.on;
    $("#set-tls-style").value = r.style === "neutral" ? "neutral" : "logo";
  } catch { /* the row stays as it is */ }
  try { $("#boot-row").hidden = !((await api("GET", "/api/boot-logo")).recorded > 0); } catch { /* stays hidden */ }
  tlsAgainSync();
  try { $("#set-tls-serial").checked = !!(await api("GET", "/api/tl-serial")).on; } catch { /* the row stays as it is */ }
}
$("#set-tls-serial").addEventListener("change", async () => {
  try { await api("PUT", "/api/tl-serial", { on: $("#set-tls-serial").checked }); }
  catch (e) { toast(t("tls.err", { e: e.message || e }), true); loadTLStart(); }
});
$("#set-tls").addEventListener("change", async () => {
  tlsAgainSync();
  try { await api("PUT", "/api/tl-start-image", { on: $("#set-tls").checked }); }
  catch (e) { toast(t("tls.err", { e: e.message || e }), true); loadTLStart(); }
});
$("#set-tls-style").addEventListener("change", async () => {
  if ($("#set-tls").checked && !confirm(t("tls.style.confirm"))) { loadTLStart(); return; }
  try { await api("PUT", "/api/tl-start-image", { style: $("#set-tls-style").value }); }
  catch (e) { toast(t("tls.err", { e: e.message || e }), true); loadTLStart(); }
});
$("#boot-remove").addEventListener("click", async () => {
  if (!confirm(t("boot.confirm"))) return;
  try {
    const r = await api("POST", "/api/boot-logo/remove", { confirm: true });
    toast(r.removed ? t("boot.done", { n: r.removed }) : t("boot.none"));
  } catch (e) { toast(t("tls.err", { e: e.message || e }), true); }
  loadTLStart();
});
$("#tls-again").addEventListener("click", async () => {
  if (!confirm(t("tls.confirm"))) return;
  try {
    const r = await api("POST", "/api/tl-start-image/rewrite", { confirm: true });
    toast(r.fans ? t("tls.started", { n: r.fans }) : t("tls.none"));
  } catch (e) { toast(t("tls.err", { e: e.message || e }), true); }
});
loadTLStart();
