"use strict";

// ---------- Settings: experimental devices ----------
Object.assign(STRINGS.de, {
  "set.g.devices": "Geräte",
  "dev.exp.desc": "Manche Geräte hat noch niemand mit CaseHUD auf echter Hardware geprüft. Sie bleiben aus, bis du sie hier freigibst.",
  "dev.exp": "Experimentelle Geräte", "dev.exp.sub": "Erlaubt CaseHUD, die unten genannten Geräte anzusprechen",
  "dev.exp.badge": "experimentell", "dev.exp.hint": "ungetestet – Rückmeldung willkommen",
  "dev.exp.list": "Einzelne Treiber", "dev.exp.sensor": "nur lesen (Sensor)", "dev.exp.display": "Display",
  "dev.exp.report": "Hat ein Gerät funktioniert oder nicht? Ein kurzer Bericht hilft allen.", "dev.report": "Testbericht schreiben",
  "dev.exp.err": "Konnte nicht gespeichert werden: {e}",
});
Object.assign(STRINGS.en, {
  "set.g.devices": "Devices",
  "dev.exp.desc": "Some devices have not been checked with CaseHUD on real hardware yet. They stay off until you allow them here.",
  "dev.exp": "Experimental devices", "dev.exp.sub": "Lets CaseHUD talk to the devices listed below",
  "dev.exp.badge": "experimental", "dev.exp.hint": "untested – feedback welcome",
  "dev.exp.list": "Individual drivers", "dev.exp.sensor": "read only (sensor)", "dev.exp.display": "display",
  "dev.exp.report": "Did a device work or not? A short report helps everyone.", "dev.report": "Write a test report",
  "dev.exp.err": "Could not save: {e}",
});
applyI18n();

// Writes the switches: the global one and the individual ones (a driver is
// "off" when its box is unchecked).
async function saveDevices() {
  const off = $$("#exp-list input[data-driver]").filter((c) => !c.checked).map((c) => c.dataset.driver);
  try {
    const d = await api("PUT", "/api/devices", { experimental: $("#set-exp").checked, off });
    state.devices = d;
    renderDevices();
    // Devices come or go with this change; the list of displays follows shortly.
    setTimeout(() => refresh().catch(() => {}), 1500);
  } catch (e) {
    toast(t("dev.exp.err", { e: e.message }), true);
    renderDevices();
  }
}

function renderDevices() {
  const d = state && state.devices;
  if (!d || !$("#set-exp")) return;
  $("#set-exp").checked = !!d.experimental;
  // Experts only, but stays visible while switched on so it can be switched off.
  $("#exp-card").classList.toggle("expert", !d.experimental);
  if (d.reportUrl) $("#exp-report").href = d.reportUrl;
  const list = $("#exp-list");
  list.replaceChildren(...d.drivers.map((x) => el("label", { class: "row" },
    el("div", { class: "row-text" },
      el("b", {}, x.name, el("span", { class: "badge exp" }, t("dev.exp.badge"))),
      el("small", {}, `${x.kind === "sensor" ? t("dev.exp.sensor") : t("dev.exp.display")} · ${t("dev.exp.hint")}`)),
    el("input", { type: "checkbox", class: "switch", "data-driver": x.id, checked: !x.off, disabled: !d.experimental, onchange: saveDevices }))));
}

$("#set-exp").addEventListener("change", async (e) => {
  if (e.target.checked && !(await ensureAck("experimental"))) { e.target.checked = false; return; }
  saveDevices();
});
for (const b of $$("[data-lang]")) b.addEventListener("click", () => renderDevices());
renderDevices();
