// "?" links to the manual. HELP_AREAS mirrors the areas of docs/handbuch/links.json ([de, en], paths
// relative to the repository root); a test in internal/web keeps both equal and checks the anchors.
// An element with data-help="<area>" gets a "?" link at its end (again after every applyI18n, which
// replaces its text). The link points to the manual of the installed version (tag v<version>), main
// for development builds.
const HELP_AREAS = {
  "manual": ["docs/handbuch/README.md", "docs/manual/README.md"],
  "start": ["docs/handbuch/erste-schritte.md#erste-schritte", "docs/manual/getting-started.md#getting-started"],
  "start.install": ["docs/handbuch/erste-schritte.md#installation", "docs/manual/getting-started.md#installation"],
  "start.setup": ["docs/handbuch/erste-schritte.md#ersteinrichtung", "docs/manual/getting-started.md#first-run-setup"],
  "start.interface": ["docs/handbuch/erste-schritte.md#oberfläche-im-überblick", "docs/manual/getting-started.md#interface-overview"],
  "start.update": ["docs/handbuch/erste-schritte.md#update", "docs/manual/getting-started.md#update"],
  "start.uninstall": ["docs/handbuch/erste-schritte.md#deinstallation-und-datenordner", "docs/manual/getting-started.md#uninstalling-and-data-folder"],
  "displays": ["docs/handbuch/displays.md#displays", "docs/manual/displays.md#displays"],
  "displays.edit": ["docs/handbuch/displays.md#display-einstellen", "docs/manual/displays.md#setting-up-a-display"],
  "displays.arrange": ["docs/handbuch/displays.md#anordnung", "docs/manual/displays.md#arrangement"],
  "displays.layouts": ["docs/handbuch/displays.md#breitbild-displays-und-aufteilung", "docs/manual/displays.md#wide-displays-and-layouts"],
  "displays.awtrix": ["docs/handbuch/displays.md#pixeluhren", "docs/manual/displays.md#pixel-clocks"],
  "displays.bootimage": ["docs/handbuch/displays.md#startbild-und-ruhebild", "docs/manual/displays.md#start-and-idle-image"],
  "displays.monitormode": ["docs/handbuch/displays.md#monitor-modus", "docs/manual/displays.md#monitor-mode"],
  "designs": ["docs/handbuch/displays.md#designs", "docs/manual/displays.md#designs"],
  "designs.editor": ["docs/handbuch/displays.md#design-bearbeiten", "docs/manual/displays.md#editing-a-design"],
  "designs.community": ["docs/handbuch/displays.md#teilen-und-community", "docs/manual/displays.md#sharing-and-community"],
  "designs.layouts": ["docs/handbuch/displays.md#layouts", "docs/manual/displays.md#layouts"],
  "media": ["docs/handbuch/displays.md#medien", "docs/manual/displays.md#media"],
  "scenes": ["docs/handbuch/szenen.md#szenen", "docs/manual/scenes.md#scenes"],
  "scenes.list": ["docs/handbuch/szenen.md#szenen-speichern-und-anwenden", "docs/manual/scenes.md#saving-and-applying-scenes"],
  "scenes.automation": ["docs/handbuch/szenen.md#automatik", "docs/manual/scenes.md#automation"],
  "scenes.rotation": ["docs/handbuch/szenen.md#rotation", "docs/manual/scenes.md#rotation"],
  "lighting": ["docs/handbuch/beleuchtung.md#beleuchtung", "docs/manual/lighting.md#lighting"],
  "lighting.enable": ["docs/handbuch/beleuchtung.md#beleuchtung-einschalten", "docs/manual/lighting.md#turning-lighting-on"],
  "lighting.case": ["docs/handbuch/beleuchtung.md#aufgeklapptes-gehäuse", "docs/manual/lighting.md#unfolded-case"],
  "lighting.effect": ["docs/handbuch/beleuchtung.md#effekt-wählen", "docs/manual/lighting.md#choosing-an-effect"],
  "lighting.source": ["docs/handbuch/beleuchtung.md#woher-kommt-die-farbe", "docs/manual/lighting.md#where-does-the-colour-come-from"],
  "lighting.connect": ["docs/handbuch/beleuchtung.md#teile-verbinden", "docs/manual/lighting.md#connecting-parts"],
  "lighting.zones": ["docs/handbuch/beleuchtung.md#zonen-und-led-anzahl", "docs/manual/lighting.md#zones-and-led-count"],
  "lighting.editor": ["docs/handbuch/beleuchtung.md#skizze-und-editor", "docs/manual/lighting.md#sketch-and-editor"],
  "lighting.templates": ["docs/handbuch/beleuchtung.md#gehäusevorlagen", "docs/manual/lighting.md#case-templates"],
  "lighting.presets": ["docs/handbuch/beleuchtung.md#effekt-vorlagen", "docs/manual/lighting.md#effect-presets"],
  "lighting.capture": ["docs/handbuch/beleuchtung.md#bildschirm-ambilight-und-audio-visualizer", "docs/manual/lighting.md#screen-ambilight-and-audio-visualizer"],
  "lighting.more": ["docs/handbuch/beleuchtung.md#weitere-einstellungen", "docs/manual/lighting.md#more-settings"],
  "lighting.ram": ["docs/handbuch/beleuchtung.md#ram-beleuchtung", "docs/manual/lighting.md#ram-lighting"],
  "cooling": ["docs/handbuch/kuehlung.md#kühlung", "docs/manual/cooling.md#cooling"],
  "cooling.mode": ["docs/handbuch/kuehlung.md#regelbetrieb-und-testbetrieb", "docs/manual/cooling.md#control-mode-and-test-mode"],
  "cooling.plan": ["docs/handbuch/kuehlung.md#schaltplan", "docs/manual/cooling.md#wiring-plan"],
  "cooling.curves": ["docs/handbuch/kuehlung.md#kurvenarten", "docs/manual/cooling.md#curve-kinds"],
  "cooling.outputs": ["docs/handbuch/kuehlung.md#lüfter-und-pumpe", "docs/manual/cooling.md#fans-and-pump"],
  "cooling.manual": ["docs/handbuch/kuehlung.md#manueller-modus", "docs/manual/cooling.md#manual-mode"],
  "cooling.calibration": ["docs/handbuch/kuehlung.md#kalibrierung", "docs/manual/cooling.md#calibration"],
  "cooling.presets": ["docs/handbuch/kuehlung.md#presets", "docs/manual/cooling.md#presets"],
  "cooling.safety": ["docs/handbuch/kuehlung.md#sicherheit", "docs/manual/cooling.md#safety"],
  "cooling.pump": ["docs/handbuch/kuehlung.md#pumpe", "docs/manual/cooling.md#pump"],
  "cooling.alarms": ["docs/handbuch/kuehlung.md#alarme", "docs/manual/cooling.md#alarms"],
  "cooling.guard": ["docs/handbuch/kuehlung.md#sichere-drehzahl-und-wächterprozess", "docs/manual/cooling.md#safe-speed-and-guard-process"],
  "cooling.otherprograms": ["docs/handbuch/kuehlung.md#andere-lüfterprogramme", "docs/manual/cooling.md#other-fan-programs"],
  "settings": ["docs/handbuch/einstellungen.md#einstellungen", "docs/manual/settings.md#settings"],
  "settings.areas": ["docs/handbuch/einstellungen.md#bereiche", "docs/manual/settings.md#areas"],
  "settings.general": ["docs/handbuch/einstellungen.md#allgemein", "docs/manual/settings.md#general"],
  "settings.sources": ["docs/handbuch/einstellungen.md#datenquellen", "docs/manual/settings.md#data-sources"],
  "settings.sensors": ["docs/handbuch/einstellungen.md#sensoren", "docs/manual/settings.md#sensors"],
  "settings.fps": ["docs/handbuch/sensoren.md#fps-und-latenz", "docs/manual/sensors.md#fps-and-latency"],
  "settings.devices": ["docs/handbuch/einstellungen.md#geräte", "docs/manual/settings.md#devices"],
  "settings.experimental": ["docs/handbuch/einstellungen.md#experimentelle-geräte", "docs/manual/settings.md#experimental-devices"],
  "settings.conflicts": ["docs/handbuch/einstellungen.md#andere-software", "docs/manual/settings.md#other-software"],
  "settings.display": ["docs/handbuch/einstellungen.md#anzeige", "docs/manual/settings.md#display"],
  "settings.api": ["docs/handbuch/einstellungen.md#schnittstellen", "docs/manual/settings.md#interfaces"],
  "settings.ai": ["docs/handbuch/einstellungen.md#ki-verbindung", "docs/manual/settings.md#ai-connection"],
  "settings.access": ["docs/handbuch/einstellungen.md#zugänge", "docs/manual/settings.md#access"],
  "settings.info": ["docs/handbuch/einstellungen.md#info-und-hilfe", "docs/manual/settings.md#info-and-help"],
  "api": ["docs/handbuch/api-mcp.md#api-und-mcp", "docs/manual/api-mcp.md#api-and-mcp"],
  "api.mcp": ["docs/handbuch/api-mcp.md#mcp-für-ki-apps", "docs/manual/api-mcp.md#mcp-for-ai-apps"],
  "sensors": ["docs/handbuch/sensoren.md#sensoren", "docs/manual/sensors.md#sensors"],
  "sensors.fps": ["docs/handbuch/sensoren.md#fps-und-latenz", "docs/manual/sensors.md#fps-and-latency"],
  "cli": ["docs/handbuch/kommandozeile.md#kommandozeile", "docs/manual/command-line.md#command-line"],
  "build": ["docs/handbuch/selbst-bauen.md#selbst-bauen", "docs/manual/building.md#building-from-source"],
  "roadmap": ["docs/handbuch/selbst-bauen.md#roadmap", "docs/manual/building.md#roadmap"],
  "troubleshooting": ["docs/handbuch/fehlerbehebung.md#fehlerbehebung-und-faq", "docs/manual/troubleshooting.md#troubleshooting-and-faq"],
};

/** The manual page and section for an area, for the current language and the installed version. */
function helpURL(area) {
  const p = HELP_AREAS[area] || HELP_AREAS.manual;
  const v = typeof state !== "undefined" && state && /^\d+\.\d+\.\d+$/.test(state.version || "") ? "v" + state.version : "main";
  const path = p[LANG === "de" ? 0 : 1].split("#");
  return "https://github.com/kewe87/CaseHUD/blob/" + v + "/" + path[0] + (path[1] ? "#" + encodeURIComponent(path[1]) : "");
}

function decorateHelp() {
  for (const e of document.querySelectorAll("[data-help]")) {
    let a = e.querySelector(":scope > a.help-link");
    if (!a) {
      a = document.createElement("a");
      a.className = "help-link";
      a.target = "_blank";
      a.rel = "noopener";
      a.addEventListener("click", () => { a.href = helpURL(e.dataset.help); });
      e.append(a);
    }
    a.href = helpURL(e.dataset.help);
    a.title = t("help.open");
    a.setAttribute("aria-label", t("help.open") + ": " + (e.firstChild && e.firstChild.nodeType === 3 ? e.firstChild.textContent : ""));
  }
}
