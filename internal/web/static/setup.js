"use strict";

// ---------- First-run setup: choose the core areas ----------
Object.assign(STRINGS.de, {
  "setup.title": "Einrichtung", "setup.sub": "Bereiche wählen",
  "setup.hint": "Jederzeit in den Einstellungen änderbar.",
  "setup.ok": "Übernehmen", "setup.skip": "Überspringen",
});
Object.assign(STRINGS.en, {
  "setup.title": "Setup", "setup.sub": "Choose areas",
  "setup.hint": "Can be changed at any time in the settings.",
  "setup.ok": "Apply", "setup.skip": "Skip",
});
applyI18n();

// The areas go through the same call as the switches in the settings, so the
// rule (at least one stays on) and the error are the same.
let wizardOpen = false;
function wizardCards() { return $$("#setup-cards [data-setup]"); }
function renderWizard() {
  const cards = wizardCards();
  const left = cards.filter((c) => c.checked).length;
  for (const c of cards) c.disabled = c.checked && left === 1;
  const h = $("#setup-hint");
  const key = left > 1 ? "setup.hint" : "mod.last";
  h.dataset.i18n = key; h.textContent = t(key);
  h.classList.toggle("warn", left <= 1);
}
async function finishWizard(apply) {
  const dlg = $("#setup");
  const body = apply ? Object.fromEntries(wizardCards().map((c) => [c.dataset.setup, c.checked])) : null;
  $("#setup-ok").disabled = $("#setup-skip").disabled = true;
  try {
    const m = await api("POST", body ? "/api/setup" : "/api/setup/skip", body || undefined);
    if (body) state.modules = m;
  } catch (e) {
    // Already answered elsewhere: nothing left to ask. Otherwise the dialog stays.
    if (e.status !== 409) {
      toast(e.message, true);
      $("#setup-ok").disabled = $("#setup-skip").disabled = false;
      return;
    }
  }
  state.setupPending = false;
  wizardOpen = false;
  dlg.close();
  if (body) { try { await refresh(); } catch { renderModules(); } }
}
for (const c of wizardCards()) c.addEventListener("change", renderWizard);
$("#setup-ok").addEventListener("click", () => finishWizard(true));
$("#setup-skip").addEventListener("click", () => finishWizard(false));
$("#setup-close").addEventListener("click", () => finishWizard(false));
$("#setup").addEventListener("cancel", (e) => { e.preventDefault(); finishWizard(false); });

// checkWizard shows the dialog once for a new configuration, after the disclaimer.
function checkWizard() {
  const a = state && state.acks;
  if (!state || !state.setupPending || wizardOpen || discOpen) return;
  if (!state.demo && !(a && a.accepted)) return;
  if (document.querySelector("dialog[open]")) return;
  wizardOpen = true;
  for (const c of wizardCards()) c.checked = true;
  renderWizard();
  $("#setup-ok").disabled = $("#setup-skip").disabled = false;
  $("#setup").showModal();
  $("#setup-ok").focus();
}
