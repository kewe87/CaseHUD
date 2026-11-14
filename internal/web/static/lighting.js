"use strict";

// ---------- Lighting tab ("Beleuchtung") ----------
// Texts live here so the tab stays one self-contained file.
Object.assign(STRINGS.de, {
  "tab.light": "Beleuchtung",
  "light.sub": "Gehäuse-, RAM- und Lüfterbeleuchtung mit einem gemeinsamen Effekt.",
  "light.master": "Beleuchtung steuern", "light.master.on": "Aktiv – {n} Geräte gefunden", "light.master.on1": "Aktiv – 1 Gerät gefunden",
  "light.master.on0": "Aktiv – kein Gerät gefunden", "light.master.off": "Aus: kein Gerätezugriff durch CaseHUD",
  "light.paused": "Pausiert (Displays aus)",
  "light.nobackend": "Keine Beleuchtungs-Hardware freigegeben. Gefundene Geräte unter „Weitere Einstellungen“ aktivieren (z. B. TL-Lüfter-Beleuchtung).",
  "light.look": "Effekt", "light.brightness": "Helligkeit", "light.exit": "Beim Beenden", "light.exit.off": "Aus", "light.exit.keep": "Letzte Farben halten", "light.exit.device": "Eigener Effekt der Geräte", "light.exit.hint": "Gilt beim Beenden von CaseHUD und beim Ausschalten der Beleuchtung. Bei „Eigener Effekt der Geräte“ schalten Geräte ohne eigenen Effekt aus.",
  "light.effect": "Effekt", "light.effect.params": "Einstellungen des Effekts",
  "light.devices.none": "Keine Geräte. Beleuchtung einschalten oder erneut suchen.",
  "light.devoff.sub.tl": "Gerät bleibt dunkel, auch nach einem Neustart. Jeder Wechsel wird einmal in den Speicher des Controllers geschrieben.",
  "light.devoff.blocked": "Aus wird im Controller gespeichert und braucht „Controller-Beleuchtung erlauben“.",
  "light.detected.ring": "LED-Ring erkannt. Zum Bedienen oder Ausschalten „Beleuchtung steuern“ aktivieren.",
  "light.detected.other": "Beleuchtungsgerät erkannt. Zum Bedienen oder Ausschalten „Beleuchtung steuern“ aktivieren.",
  "light.rescan": "Neu suchen",
  "light.state.ok": "Aktiv", "light.state.conflict": "Belegt", "light.state.disabled": "Aus", "light.state.error": "Fehler",
  "light.conflict.by": "Wird von {names} gesteuert", "light.ignore": "Trotzdem ansteuern",
  "light.ignore.sub": "Beide Programme steuern dasselbe Gerät; Flackern möglich.",
  "light.zone.leds": "{n} LEDs", "light.zone.count": "LED-Anzahl", "light.zone.calib": "Kalibrieren", "light.zone.calib.end": "Fertig",
  "light.zone.calib.hint": "Das Testlicht schreibt nur kurz an den Controller und ändert nichts dauerhaft. Die rote LED wandert mit; erlischt sie, ist das Ende erreicht und die Anzahl bestimmt.",
  "light.zone.prev": "Eine LED zurück", "light.zone.next": "Eine LED weiter",

  "light.tl": "TL-Lüfter-Beleuchtung (experimentell)", "light.tl.sub": "Steuert den TL-Lüfter-Controller. Im Standardmodus „Mit Mainboard synchronisieren“ sendet CaseHUD beim Start einen einzelnen Befehl; danach folgen die Lüfter dem Mainboard. Nötig ist dafür das Sync-Kabel vom ARGB-Anschluss des Mainboards zum Controller; ohne Kabel einen anderen Modus wählen. Die Modi „CaseHUD-Effekte“ und „Effekte des Controllers“ speichern jede Änderung im Flash-Speicher des Controllers und erfordern eine Bestätigung. „Blinken“ und das Testlicht schreiben nur kurz an den Controller. Noch nicht auf allen Systemen getestet.",
  "light.tl.modeWarn": "Dieser Modus schreibt Beleuchtungseinstellungen an den Controller; sie bleiben dort eventuell gespeichert.",
  "light.chip.conflict": "Beleuchtung: {n} Gerät(e) von einem anderen Programm belegt", "light.chip.error": "Beleuchtung: {n} Gerät(e) mit Fehler",
  "light.err.load": "Beleuchtung konnte nicht geladen werden: {e}",
  "light.fx.rainbow": "Regenbogen", "light.fx.solid": "Einfarbig", "light.fx.breathing": "Atmen", "light.fx.wave": "Farbwelle",
  "light.fx.sensor": "Temperatur", "light.fx.cycle": "Farbwechsel", "light.fx.meteor": "Meteor", "light.fx.orbit": "Umlauf", "light.fx.fill": "Füllen",
  "light.fx.burst": "Füllen und verblassen", "light.fx.shuttle": "Pendel", "light.fx.bounce": "Springen", "light.fx.stack": "Stapeln",
  "light.fx.sparkle": "Funkeln", "light.fx.stripes": "Streifen", "light.fx.sand": "Sanduhr", "light.fx.spark": "Funke",
  "light.fx.screen": "Bildschirm-Ambilight", "light.fx.audio": "Audio-Visualizer",
  "light.p.display": "Bildschirm (0 = Hauptbildschirm)", "light.p.area": "Bereich", "light.p.saturation": "Sättigung (%)",
  "light.p.fps": "Aufnahmerate (Bilder/s)", "light.p.bands": "Bänder", "light.p.sensitivity": "Empfindlichkeit (%)",
  "light.p.decay": "Abklingen", "light.p.rainbow": "Regenbogenfarben",
  "light.pv.area.edges": "Ränder (Ambilight)", "light.pv.area.full": "Ganzes Bild",
  "light.pv.path.around": "Rund ums Gehäuse", "light.pv.path.horizontal": "Waagerecht", "light.pv.path.vertical": "Senkrecht", "light.pv.path.radial": "Von der Mitte",
  "light.pv.mode.spectrum": "Spektrum", "light.pv.mode.pulse": "Puls",
  "light.capture.note.screen": "Liest den Bildschirminhalt, solange der Effekt läuft. Nichts wird gespeichert oder gesendet.",
  "light.capture.note.audio": "Liest die Audioausgabe, solange der Effekt läuft. Nichts wird gespeichert oder gesendet.",
  "light.fx.meet": "Treffen und mischen", "light.fx.rainbow-gaps": "Regenbogen mit Lücken",
  "light.p.speed": "Geschwindigkeit", "light.p.scale": "Breite", "light.p.vertical": "Senkrecht", "light.p.color": "Farbe",
  "light.p.useAccent": "Akzentfarbe des Designs", "light.p.period": "Dauer (s)", "light.p.background": "Hintergrund", "light.p.width": "Breite",
  "light.p.sensor": "Sensor", "light.p.min": "Wert für niedrige Farbe", "light.p.max": "Wert für hohe Farbe", "light.p.low": "Niedrige Farbe",
  "light.p.high": "Hohe Farbe", "light.p.smoothing": "Glättung", "light.p.colors": "Genutzte Farben", "light.p.path": "Verlauf",
  "light.p.mirror": "In der Mitte spiegeln", "light.p.reverse": "Richtung umkehren", "light.p.tail": "Schweiflänge", "light.p.dot": "Punktgröße", "light.p.dots": "Anzahl Punkte", "light.p.originX": "Mittelpunkt X (%)", "light.p.originY": "Mittelpunkt Y (%)", "light.p.alternate": "Richtung wechseln",
  "light.p.size": "Blockgröße", "light.p.balls": "Bälle", "light.p.density": "Dichte", "light.p.count": "Streifen", "light.p.gaps": "Lücken",
  "light.p.color1": "Farbe 1", "light.p.color2": "Farbe 2", "light.p.color3": "Farbe 3", "light.p.color4": "Farbe 4", "light.p.color5": "Farbe 5", "light.p.color6": "Farbe 6",
  "light.flash": "Der Controller speichert jede Änderung dauerhaft – Änderungen werden gebündelt gesendet.", "light.more": "Weitere: {n}",
  "light.p.mode": "Modus", "light.p.effect": "Hardware-Effekt", "light.p.direction": "Richtung", "light.p.interval": "Canvas: Sekunden zwischen Updates",
  "light.p.minDelta": "Canvas: kleinste Farbänderung", "light.p.merge": "Hardware-Effekt über alle Lüfter", "light.p.portOrder": "Port-Reihenfolge, z. B. 3,1,2",
  "light.p.brightness": "Helligkeit", "light.p.color1x": "Farbe 1",
  "arr.blink.fw": "Lüfter dieses Ports blinken lassen",
  "arr.scan": "Lüfter ohne Display suchen", "arr.scan.again": "Lüfterliste neu lesen",
  "arr.scan.confirm": "Die Lüfterliste wird vom Controller gelesen (nur lesend, keine Änderung). Andere Programme, die den Controller nutzen, können kurz gestört werden. Fortfahren?", "arr.blink.note": "„Identifizieren“ lässt die Lüfter kurz leuchten oder blinken: kurzer Schreibzugriff auf den Controller, keine dauerhafte Änderung.", "arr.scan.hint": "Liest einmalig die Lüfterliste vom Controller (nur lesend, keine Änderung). Andere Programme, die den Controller nutzen, können kurz gestört werden.",
  "arr.scan.live": "Die Beleuchtung hält die Liste über ihre eigene Verbindung aktuell.",
  "arr.scan.at": "Zuletzt gelesen: {when}", "arr.scan.never": "Noch nicht gelesen – gezeigt werden nur Lüfter mit Display.",
  "light.zone.led1": "1 LED",
  "light.reset": "Beleuchtungseinstellungen ungültig und zurückgesetzt (Sicherung: {file}).",
  "light.presets": "Vorlagen", "light.presets.sub": "Vorlagen aus der Community-Bibliothek und eigene. Auswahl setzt Effekt und Einstellungen; Geräte und Layouts bleiben unverändert.",
  "light.presets.none": "Keine Vorlagen. „Bibliothek aktualisieren“ lädt die Community-Vorlagen von GitHub; alternativ die aktuelle Beleuchtung als Vorlage speichern.", "light.presets.count": "{n} Vorlagen",
  "light.tag.calm": "ruhig", "light.tag.party": "Party", "light.tag.sensor": "Sensor", "light.tag.night": "Nacht", "light.tag.neon": "Neon", "light.tag.clean": "klar", "light.tag.gaming": "Gaming", "light.tag.water": "Wasser", "light.tag.retro": "Retro", "light.tag.cyberpunk": "Cyberpunk",
  "light.lib.refresh": "Bibliothek aktualisieren", "light.lib.loading": "Lade …",
  "light.lib.status": "Stand: {when} · {c} Bauteile",
  "light.lib.never": "Noch nicht geladen. Es werden öffentliche Dateien von GitHub abgerufen, es wird nichts gesendet. Offline gelten die eingebauten Bauteile.",
  "light.lib.error": "Bibliothek nicht erreichbar: {e}. Es gilt die zuletzt geladene Kopie.",
  "light.lib.warn": "{n} Eintrag/Einträge der Bibliothek wurden übersprungen.",
  "light.preset.applied": "Vorlage „{n}“ eingestellt.", "light.preset.ignored": "{k} Einstellung(en) der Vorlage kennt der Effekt nicht und wurde(n) ausgelassen.",
  "light.preset.apply": "Vorlage {n} anwenden",
  "light.preset.save": "Als Vorlage speichern", "light.preset.name": "Name der Vorlage", "light.preset.saved": "Vorlage „{n}“ gespeichert.",
  "light.preset.delete": "Vorlage {n} löschen", "light.preset.confirmDelete": "Eigene Vorlage „{n}“ löschen?", "light.preset.deleted": "Vorlage „{n}“ gelöscht.",
  "light.preset.src.library": "Bibliothek", "light.preset.src.user": "Eigene",
  "light.sync.note": "Die Einstellung bleibt nach dem Beenden von CaseHUD bestehen.",
  "light.flash.confirm": "Der Lüfter-Controller speichert jede Beleuchtungsänderung dauerhaft im Flash-Speicher; häufige Änderungen verschleißen ihn. Empfohlen: Quelle „Mainboard“ (nötig: Sync-Kabel vom ARGB-Anschluss des Mainboards zum Controller). Ohne Kabel einen anderen Modus wählen.",
  "light.ram.hint": "RAM-Beleuchtung braucht die SMBus-Brücke im Sensor-Dienst (Einstellungen → Datenquellen; Datei C:\\ProgramData\\CaseHUD\\sensors.json mit \"smbusBridge\": true, siehe docs/SMBUS-BRIDGE.md).",
  "light.syncsrc": "Sync-Anschluss am Mainboard", "light.syncsrc.none": "– nicht gewählt –",
  "light.syncsrc.note": "Der Lüfter-Controller gibt das Signal des ARGB-Anschlusses parallel an alle Ports weiter. Die Auswahl legt diesen Anschluss im Layout mit {n} Lüftern an (je 26 LEDs, in Kettenreihenfolge).",
  "light.syncsrc.nofans": "Lüfteranzahl noch nicht bekannt. Zuerst die Lüfter einlesen (Displays → Anordnung).",
  "light.syncsrc.ports": "Im Sync-Modus zeigen alle Ports dieselbe Kette: Lüfter an gleicher Position leuchten gleich.",
  "light.syncsrc.nohdr": "Kein Mainboard mit ARGB-Anschluss gefunden.",
  "light.tl.ports": "Ports", "light.tl.follow": "Farben und Richtung aus CaseHUD",
  "light.tl.follow.sub": "Farben und Tempo aus dem CaseHUD-Effekt, die Richtung je Port aus der Gehäuseskizze. Eigene Werte je Port haben Vorrang.",
  "light.tl.port": "Port {n}", "light.tl.effect": "Effekt an Port {n}", "light.tl.direction": "Richtung an Port {n}",
  "light.tl.all": "wie alle Ports", "light.tl.sketch": "aus der Skizze",
  "light.tl.src.sketch": "aus der Skizze", "light.tl.src.manual": "eigener Wert", "light.tl.src.all": "wie alle Ports",
  "light.tl.whole": "als Ganzes ({e})", "light.tl.nofans": "keine Lüfter in der Skizze zugeordnet",
  "light.tl.flash": "Jede Änderung wird nur an die betroffenen Ports gesendet und dort dauerhaft gespeichert.",
  "light.pv.direction.clockwise": "im Uhrzeigersinn", "light.pv.direction.counter-clockwise": "gegen den Uhrzeigersinn",
  "light.pv.direction.up": "hoch", "light.pv.direction.down": "runter", "light.pv.direction.spread": "nach außen", "light.pv.direction.gather": "nach innen",
  "light.pv.effect.rainbow": "Regenbogen", "light.pv.effect.static": "Statisch", "light.pv.effect.breathing": "Atmen", "light.pv.effect.wave": "Welle",
  "light.pv.effect.meteor": "Meteor", "light.pv.effect.color-cycle": "Farbwechsel", "light.pv.effect.stack": "Stapeln",
  "light.unsupported": "Nicht unterstützt", "light.ram.title": "RAM-Beleuchtung",

  "diag.title": "Diagnose exportieren", "diag.sub": "Speichert eine ZIP-Datei mit Protokollende, bereinigten Einstellungen, Versionen, Display-Liste und Beleuchtungsbericht. Namen, Pfade, Adressen und Tokens werden entfernt. Es wird nichts gesendet; Datei vor der Weitergabe prüfen.",
  "diag.trace": "Mitschnitt (30 s)", "diag.trace.sub": "Zeichnet 30 Sekunden lang die Datenpakete zwischen CaseHUD und den Beleuchtungsgeräten auf (in der ZIP-Datei). Sie können Hardware-Kennungen des Controllers enthalten. Nur bei aktiver Beleuchtung; währenddessen z. B. den Effekt ändern.",
  "diag.trace.off": "Für den Mitschnitt muss die Beleuchtung aktiv sein.",
  "diag.export": "Exportieren", "diag.running": "Mitschnitt läuft, noch {n} s. Währenddessen z. B. Effekt oder Helligkeit ändern.",
  "diag.done": "Gespeichert als {f}.", "diag.err": "Export fehlgeschlagen: {e}",
});
Object.assign(STRINGS.en, {
  "tab.light": "Lighting",
  "light.sub": "Case, RAM and fan lighting with one shared effect.",
  "light.master": "Control lighting", "light.master.on": "Active – {n} devices found", "light.master.on1": "Active – 1 device found",
  "light.master.on0": "Active – no device found", "light.master.off": "Off: no device access by CaseHUD",
  "light.paused": "Paused (displays off)",
  "light.nobackend": "No lighting hardware enabled. Enable detected devices under “More settings” (e.g. TL fan lighting).",
  "light.look": "Effect", "light.brightness": "Brightness", "light.exit": "On exit", "light.exit.off": "Off", "light.exit.keep": "Keep last colours", "light.exit.device": "Devices' own effect", "light.exit.hint": "Applies when CaseHUD exits and when lighting is switched off. With “Devices' own effect”, devices without an effect of their own turn off.",
  "light.effect": "Effect", "light.effect.params": "Effect settings",
  "light.devices.none": "No devices. Turn lighting on or search again.",
  "light.devoff.sub.tl": "The device stays dark, also after a restart. Every change is written once to the controller memory.",
  "light.devoff.blocked": "Off is stored in the controller and needs \u201callow controller lighting\u201d.",
  "light.detected.ring": "LED ring detected. Enable “Control lighting” to operate or switch it off.",
  "light.detected.other": "Lighting device detected. Enable “Control lighting” to operate or switch it off.",
  "light.rescan": "Search again",
  "light.state.ok": "Active", "light.state.conflict": "In use", "light.state.disabled": "Off", "light.state.error": "Error",
  "light.conflict.by": "Controlled by {names}", "light.ignore": "Drive it anyway",
  "light.ignore.sub": "Both programs control the same device; flicker is possible.",
  "light.zone.leds": "{n} LEDs", "light.zone.count": "LED count", "light.zone.calib": "Calibrate", "light.zone.calib.end": "Done",
  "light.zone.calib.hint": "The test light writes only briefly to the controller and changes nothing permanently. The red LED moves along; when it goes out, the end is reached and the count is determined.",
  "light.zone.prev": "One LED back", "light.zone.next": "One LED forward",

  "light.tl": "TL fan lighting (experimental)", "light.tl.sub": "Controls the TL fan controller. In the default mode “Sync with motherboard” CaseHUD sends a single command at start; afterwards the fans follow the motherboard. This needs the sync cable from the motherboard ARGB header to the controller; without the cable choose another mode. The modes “CaseHUD effects” and “Controller effects” store every change in the flash memory of the controller and require confirmation. “Blink” and the test light write only briefly to the controller. Not yet tested on all systems.",
  "light.tl.modeWarn": "This mode writes light settings to the controller; it may keep them stored.",
  "light.chip.conflict": "Lighting: {n} device(s) in use by another program", "light.chip.error": "Lighting: {n} device(s) with an error",
  "light.err.load": "Could not load lighting: {e}",
  "light.fx.rainbow": "Rainbow", "light.fx.solid": "Solid color", "light.fx.breathing": "Breathing", "light.fx.wave": "Color wave",
  "light.fx.sensor": "Temperature", "light.fx.cycle": "Color cycle", "light.fx.meteor": "Meteor", "light.fx.orbit": "Orbit", "light.fx.fill": "Color fill",
  "light.fx.burst": "Fill and fade", "light.fx.shuttle": "Shuttle", "light.fx.bounce": "Bounce", "light.fx.stack": "Stack",
  "light.fx.sparkle": "Sparkle", "light.fx.stripes": "Stripes", "light.fx.sand": "Sand glass", "light.fx.spark": "Spark",
  "light.fx.screen": "Screen ambilight", "light.fx.audio": "Audio visualizer",
  "light.p.display": "Display (0 = primary)", "light.p.area": "Area", "light.p.saturation": "Saturation (%)",
  "light.p.fps": "Capture rate (frames/s)", "light.p.bands": "Bands", "light.p.sensitivity": "Sensitivity (%)",
  "light.p.decay": "Decay", "light.p.rainbow": "Rainbow colors",
  "light.pv.area.edges": "Edges (ambilight)", "light.pv.area.full": "Whole picture",
  "light.pv.path.around": "Around the case", "light.pv.path.horizontal": "Horizontal", "light.pv.path.vertical": "Vertical", "light.pv.path.radial": "From the centre",
  "light.pv.mode.spectrum": "Spectrum", "light.pv.mode.pulse": "Pulse",
  "light.capture.note.screen": "Reads the screen while the effect runs. Nothing is stored or sent.",
  "light.capture.note.audio": "Reads the audio output while the effect runs. Nothing is stored or sent.",
  "light.fx.meet": "Meet and mix", "light.fx.rainbow-gaps": "Rainbow with gaps",
  "light.p.speed": "Speed", "light.p.scale": "Width", "light.p.vertical": "Vertical", "light.p.color": "Color",
  "light.p.useAccent": "Design accent color", "light.p.period": "Period (s)", "light.p.background": "Background", "light.p.width": "Width",
  "light.p.sensor": "Sensor", "light.p.min": "Value for low color", "light.p.max": "Value for high color", "light.p.low": "Low color",
  "light.p.high": "High color", "light.p.smoothing": "Smoothing", "light.p.colors": "Colors used", "light.p.path": "Path",
  "light.p.mirror": "Mirror at the middle", "light.p.reverse": "Reverse direction", "light.p.tail": "Tail length", "light.p.dot": "Dot size", "light.p.dots": "Number of dots", "light.p.originX": "Centre X (%)", "light.p.originY": "Centre Y (%)", "light.p.alternate": "Alternate direction",
  "light.p.size": "Block size", "light.p.balls": "Balls", "light.p.density": "Density", "light.p.count": "Stripes", "light.p.gaps": "Gaps",
  "light.p.color1": "Color 1", "light.p.color2": "Color 2", "light.p.color3": "Color 3", "light.p.color4": "Color 4", "light.p.color5": "Color 5", "light.p.color6": "Color 6",
  "light.flash": "The controller stores every change permanently – changes are sent in batches.", "light.more": "More: {n}",
  "light.p.mode": "Mode", "light.p.effect": "Hardware effect", "light.p.direction": "Direction", "light.p.interval": "Canvas: seconds between updates",
  "light.p.minDelta": "Canvas: smallest color change", "light.p.merge": "Hardware effect across all fans", "light.p.portOrder": "Port order, e.g. 3,1,2",
  "light.p.brightness": "Brightness", "light.p.color1x": "Color 1",
  "arr.blink.fw": "Blink the fans of this port",
  "arr.scan": "Find fans without display", "arr.scan.again": "Read the fan list again",
  "arr.scan.confirm": "The fan list is read from the controller (read only, no changes). Other programs using the controller may be disturbed briefly. Continue?", "arr.blink.note": "“Identify” makes the fans light up or blink briefly: short write access to the controller, no permanent change.", "arr.scan.hint": "Reads the fan list from the controller once (read only, no changes). Other programs using the controller may be disturbed briefly.",
  "arr.scan.live": "Lighting keeps the list current through its own connection.",
  "arr.scan.at": "Last read: {when}", "arr.scan.never": "Not read yet – only fans with a display are shown.",
  "light.zone.led1": "1 LED",
  "light.reset": "Lighting settings invalid and reset (backup: {file}).",
  "light.presets": "Presets", "light.presets.sub": "Presets from the community library and your own. Selecting one sets effect and settings; devices and layouts stay unchanged.",
  "light.presets.none": "No presets. “Update library” fetches the community presets from GitHub; alternatively save the current lighting as a preset.", "light.presets.count": "{n} presets",
  "light.tag.calm": "calm", "light.tag.party": "party", "light.tag.sensor": "sensor", "light.tag.night": "night", "light.tag.neon": "neon", "light.tag.clean": "clean", "light.tag.gaming": "gaming", "light.tag.water": "water", "light.tag.retro": "retro", "light.tag.cyberpunk": "cyberpunk",
  "light.lib.refresh": "Update library", "light.lib.loading": "Loading …",
  "light.lib.status": "As of: {when} · {c} parts",
  "light.lib.never": "Not loaded yet. Public files are fetched from GitHub; nothing is sent. Offline, the built-in parts apply.",
  "light.lib.error": "Library not reachable: {e}. The last loaded copy stays in use.",
  "light.lib.warn": "{n} library entries were skipped.",
  "light.preset.applied": "Preset \"{n}\" applied.", "light.preset.ignored": "{k} preset setting(s) are unknown to the effect and were left out.",
  "light.preset.apply": "Apply preset {n}",
  "light.preset.save": "Save as preset", "light.preset.name": "Preset name", "light.preset.saved": "Preset \"{n}\" saved.",
  "light.preset.delete": "Delete preset {n}", "light.preset.confirmDelete": "Delete your preset \"{n}\"?", "light.preset.deleted": "Preset \"{n}\" deleted.",
  "light.preset.src.library": "Library", "light.preset.src.user": "Mine",
  "light.sync.note": "The setting stays in place after you quit CaseHUD.",
  "light.flash.confirm": "The fan controller stores every lighting change permanently in flash memory; frequent changes wear it out. Recommended: source “Mainboard” (needs the sync cable from the motherboard ARGB header to the controller). Without the cable choose another mode.",
  "light.ram.hint": "RAM lighting needs the SMBus bridge in the sensor service (Settings > Data sources; file C:\\ProgramData\\CaseHUD\\sensors.json with \"smbusBridge\": true, see docs/SMBUS-BRIDGE.md).",
  "light.syncsrc": "Sync header on the motherboard", "light.syncsrc.none": "- not chosen -",
  "light.syncsrc.note": "The fan controller passes the signal of the ARGB header to all ports in parallel. Selecting a header lays it out with {n} fans (26 LEDs each, in chain order).",
  "light.syncsrc.nofans": "Number of fans not known yet. Read the fans first (Displays → Arrangement).",
  "light.syncsrc.ports": "In sync mode all ports show the same chain: fans in the same position light up alike.",
  "light.syncsrc.nohdr": "No mainboard with an ARGB header found.",
  "light.tl.ports": "Ports", "light.tl.follow": "Colors and direction from CaseHUD",
  "light.tl.follow.sub": "Colors and speed from the CaseHUD effect, the direction per port from the case sketch. Values set per port take precedence.",
  "light.tl.port": "Port {n}", "light.tl.effect": "Effect on port {n}", "light.tl.direction": "Direction on port {n}",
  "light.tl.all": "like all ports", "light.tl.sketch": "from the sketch",
  "light.tl.src.sketch": "from the sketch", "light.tl.src.manual": "set per port", "light.tl.src.all": "like all ports",
  "light.tl.whole": "as a whole ({e})", "light.tl.nofans": "no fans assigned in the sketch",
  "light.tl.flash": "Each change is sent only to the ports it affects and stored there permanently.",
  "light.pv.direction.clockwise": "clockwise", "light.pv.direction.counter-clockwise": "counter-clockwise",
  "light.pv.direction.up": "up", "light.pv.direction.down": "down", "light.pv.direction.spread": "outwards", "light.pv.direction.gather": "inwards",
  "light.pv.effect.rainbow": "Rainbow", "light.pv.effect.static": "Static", "light.pv.effect.breathing": "Breathing", "light.pv.effect.wave": "Wave",
  "light.pv.effect.meteor": "Meteor", "light.pv.effect.color-cycle": "Color cycle", "light.pv.effect.stack": "Stack",
  "light.unsupported": "Not supported", "light.ram.title": "RAM lighting",

  "diag.title": "Export diagnostics", "diag.sub": "Saves a ZIP file with the end of the log, sanitized settings, versions, display list and lighting report. Names, paths, addresses and tokens are removed. Nothing is sent; review the file before sharing it.",
  "diag.trace": "Recording (30 s)", "diag.trace.sub": "Records the data packets between CaseHUD and the lighting devices for 30 seconds (added to the ZIP file). They can contain hardware identifiers of the controller. Only while lighting is active; change the effect during the recording, for example.",
  "diag.trace.off": "Lighting has to be active for a recording.",
  "diag.export": "Export", "diag.running": "Recording, {n} s left. Change the effect or brightness during the recording, for example.",
  "diag.done": "Saved as {f}.", "diag.err": "Export failed: {e}",
});
Object.assign(STRINGS.de, {
  "setup.assist": "LED-Anzahl ermitteln", "setup.parts": "Bauteile wählen",
  "setup.assist.hint": "Zahl erhöhen, bis die letzte LED am Kettenende gerade leuchtet. Die rote LED ist die gewählte, davor leuchten zwei blaue; sie erscheint nur, wenn die LED existiert.",
  "setup.assist.live": "Nur Vorschau – es wird nichts im Gerät gespeichert.",
  "setup.apply": "Übernehmen", "setup.cancel": "Abbrechen",
  "setup.led": "LED", "setup.of": "{n} von {m}",
  "setup.minus1": "Eine weniger", "setup.plus1": "Eine mehr", "setup.minus10": "Zehn weniger", "setup.plus10": "Zehn mehr",
  "setup.assist.ended": "Der Assistent ist abgelaufen oder wurde beendet.",
  "setup.parts.hint": "Bauteile in der Reihenfolge am Anschluss wählen. Die Summe ihrer LEDs ergibt die LED-Anzahl.",
  "setup.parts.add": "Bauteil hinzufügen", "setup.parts.pick": "- Bauteil wählen -", "setup.parts.addbtn": "Hinzufügen",
  "setup.parts.none": "Noch keine Bauteile gewählt.", "setup.parts.up": "Nach oben", "setup.parts.down": "Nach unten", "setup.parts.remove": "Entfernen",
  "setup.parts.parallel": "parallel geschaltet (Splitter)",
  "setup.parts.parallel.sub": "Alle Bauteile hängen am selben Signal und zeigen es ab LED 1. Zählt das längste Bauteil.",
  "setup.parts.sum": "Summe: {n} LEDs", "setup.parts.longest": "Längstes Bauteil: {n} LEDs",
  "setup.parts.over": "Der Anschluss nimmt höchstens {m} LEDs.",
  "setup.parts.lib": "Fehlt ein Bauteil? Unter „Vorlagen“ die Library aktualisieren.",
  "setup.seg.parallel": "parallel (Splitter)", "setup.seg.leds": "{n} LEDs",
  "setup.seg.note": "Parallel geschaltete Bauteile erhalten dasselbe Signal ab LED 1; kürzere zeigen nur die ersten LEDs.",
  "setup.hint.title": "Keine LED-Anzahl gesetzt",
  "setup.hint.text": "ARGB-Anschlüsse des Mainboards melden die LED-Anzahl nicht; ohne Einstellung leuchtet dort nichts. LED-Anzahl je Anschluss ermitteln.",
  "setup.tpl.title": "Vorlage anwenden", "setup.tpl.sub": "Fertige Einstellungen für bekannte Hardware. Setzt LED-Anzahl und Layout der gelisteten Zonen; Effekt und Helligkeit bleiben unverändert.",
  "setup.tpl.btn": "Vorlage anwenden", "setup.tpl.sync": "Der Lüfter-Controller folgt danach dem Anschluss {z}.",
  "setup.tpl.confirm": "Vorlage „{n}“ setzt LED-Anzahl und Layout dieser Zonen neu und ersetzt bestehende Layouts. Fortfahren?",
  "setup.tpl.done": "Vorlage angewendet: {a} Einstellung(en).", "setup.tpl.skipped": "Nicht angewendet: {s}",
  "setup.saved": "LED-Anzahl übernommen.",
  "setup.rename": "Namen ändern", "setup.rename.ph": "Eigener Name",
});
Object.assign(STRINGS.en, {
  "setup.assist": "Find LED count", "setup.parts": "Pick parts",
  "setup.assist.hint": "Increase the number until the last LED at the end of the chain just lights up. The red LED is the selected one, two blue ones precede it; it appears only if the LED exists.",
  "setup.assist.live": "Preview only - nothing is stored in the device.",
  "setup.apply": "Apply", "setup.cancel": "Cancel",
  "setup.led": "LED", "setup.of": "{n} of {m}",
  "setup.minus1": "One less", "setup.plus1": "One more", "setup.minus10": "Ten less", "setup.plus10": "Ten more",
  "setup.assist.ended": "The assistant timed out or was ended.",
  "setup.parts.hint": "Select the parts in the order they are connected to the header. The sum of their LEDs is the LED count.",
  "setup.parts.add": "Add part", "setup.parts.pick": "- pick a part -", "setup.parts.addbtn": "Add",
  "setup.parts.none": "No parts picked yet.", "setup.parts.up": "Move up", "setup.parts.down": "Move down", "setup.parts.remove": "Remove",
  "setup.parts.parallel": "wired in parallel (splitter)",
  "setup.parts.parallel.sub": "All parts share the same signal and show it from LED 1. The longest part counts.",
  "setup.parts.sum": "Sum: {n} LEDs", "setup.parts.longest": "Longest part: {n} LEDs",
  "setup.parts.over": "The header takes at most {m} LEDs.",
  "setup.parts.lib": "A part is missing? Update the library under \"Presets\".",
  "setup.seg.parallel": "parallel (splitter)", "setup.seg.leds": "{n} LEDs",
  "setup.seg.note": "Parts wired in parallel receive the same signal from LED 1; shorter ones show only the first LEDs.",
  "setup.hint.title": "No LED count set",
  "setup.hint.text": "ARGB headers of the mainboard do not report the LED count; nothing lights until it is set. Determine the LED count per header.",
  "setup.tpl.title": "Apply a template", "setup.tpl.sub": "Ready-made settings for known hardware. Sets LED count and layout of the listed zones; effect and brightness stay unchanged.",
  "setup.tpl.btn": "Apply template", "setup.tpl.sync": "The fan controller then follows header {z}.",
  "setup.tpl.confirm": "Template “{n}” resets LED count and layout of these zones and replaces existing layouts. Continue?",
  "setup.tpl.done": "Template applied: {a} setting(s).", "setup.tpl.skipped": "Not applied: {s}",
  "setup.saved": "LED count applied.",
  "setup.rename": "Rename", "setup.rename.ph": "Your own name",
});
Object.assign(STRINGS.de, {
  "lh.title": "Effekt auf dem Gehäuse",
  "lh.sub": "Seitenansicht in der Mitte, Deckel oben, Boden unten, Rückseite und Front daneben. Jedes Teil nimmt die Farbe an seiner Stelle auf.",
  "lh.live": "Live", "lh.live.preview": "Vorschau", "lh.live.off": "Beleuchtung aus", "lh.live.paused": "Pausiert", "lh.live.lost": "Keine Live-Daten", "lh.live.capture": "Keine Vorschau",
  "lh.live.tip": "Farben, die die Geräte gerade bekommen.", "lh.live.preview.tip": "Berechnete Vorschau des Effekts; die Geräte bekommen gerade nichts.",
  "lh.note.capture": "Bildschirm- und Audio-Effekte entstehen erst beim Abspielen und werden nicht vorab berechnet.",
  "lh.note.error": "Der Effekt lässt sich nicht darstellen.",
  "lh.stage": "Aufgeklapptes Gehäuse mit dem Effekt",
  "lh.case": "{c} · {w} × {h} × {d} mm", "lh.case.custom": "Eigenes Gehäuse",
  "lh.legend.bound": "verbunden", "lh.legend.unbound": "nicht verbunden",
  "lh.unbound.chip": "{n} Teile ohne Zone", "lh.unbound.chip1": "1 Teil ohne Zone", "lh.unbound.title": "Teil anklicken, um es mit einer Zone zu verbinden",
  "lh.empty.title": "Noch keine Skizze", "lh.empty.text": "Teile ins Gehäuse setzen und mit den Zonen der Geräte verbinden; dann zeigt diese Fläche, welche Farbe jedes Teil vom Effekt bekommt.",
  "lh.empty.adopt": "Erkannte Geräte übernehmen", "lh.empty.draw": "Skizze zeichnen", "lh.empty.need": "Beleuchtung einschalten, um Geräte zu erkennen.",
  "lh.tip.unbound": "Nicht verbunden", "lh.tip.leds": "{n} LEDs", "lh.tip.led1": "1 LED", "lh.tip.hint": "Klicken zum Verbinden",
  "lh.bind": "Verbinden mit", "lh.bind.none": "– nicht verbunden –", "lh.bind.edit": "Im Editor öffnen", "lh.bind.close": "Schließen",
  "lh.bind.need": "Beleuchtung einschalten, um die Zonen der Geräte zu sehen.", "lh.bind.nozones": "Keine Zonen gefunden.",
  "lh.auto": "Automatisch zuordnen", "lh.auto.title": "Teile mit eindeutigem Anschluss verbinden",
  "lh.auto.done": "{n} Teile automatisch zugeordnet", "lh.auto.done1": "1 Teil automatisch zugeordnet", "lh.auto.none": "Kein Teil eindeutig zuzuordnen",
  "lh.auto.order": "Reihenfolge der Lüfter: nach der Nummer im Namen, sonst von links nach rechts bzw. oben nach unten. Im Editor änderbar.",
  "lh.auto.open": "{n} Teile noch offen, z. B. an ARGB-Anschlüssen.", "lh.auto.open1": "1 Teil noch offen, z. B. an einem ARGB-Anschluss.",
  "lh.auto.undo": "Rückgängig", "lh.auto.undone": "Zuordnung zurückgenommen.", "lh.auto.err": "Automatische Zuordnung fehlgeschlagen: {e}",
  "lh.auto.src": "„{d}“ zeigt gerade: {s}. Die zugeordneten Teile leuchten erst mit der Quelle „CaseHUD-Effekt“.", "lh.auto.src.switch": "Auf CaseHUD-Effekt umstellen",
  "lh.auto.n.tlsame": "Mehrere Ports tragen die gleichen Lüfter. Welche Gruppe an welchem Port hängt, zeigt „Anschlüsse durchgehen“.",
  "lh.auto.n.tlcount": "Die gezeichneten TL-Lüfter passen zu keinem Port (Anzahl oder Display).", "lh.auto.n.tldevs": "Mehr als ein Lüfter-Controller gefunden: Lüfter bitte von Hand zuordnen.",
  "lh.auto.n.ram": "{n} RAM-Riegel gezeichnet, {m} gefunden: bitte von Hand zuordnen.", "lh.auto.n.board": "{n} Mainboards gezeichnet, {m} Onboard-Zonen gefunden: bitte von Hand zuordnen.",
  "lh.auto.n.leds": "„{p}“ hat {n} LEDs, {z} nur {m}: die übrigen bleiben dunkel.",
  "lh.auto.n.ledsmax": "„{p}“ hat {n} LEDs, {z} steuert höchstens {m}: bitte von Hand zuordnen.",
  "lh.auto.n.syncsrc": "Der Lüfter-Controller folgt dem Mainboard, der Sync-Anschluss ist nicht festgelegt. Anschluss im Geräte-Panel des Controllers wählen ({n} LEDs).",
  "lh.auto.n.syncused": "{z} ist bereits belegt: TL-Lüfter bleiben offen.",
  "lh.auto.sync": "Sync-Modus: Alle Ports zeigen dieselbe Kette. Weitere Lüfter übernehmen die Farben des Lüfters an gleicher Position.",
  "lh.auto.sync.ask": "Der Lüfter-Controller folgt dem Mainboard. Passender Sync-Anschluss: {z} ({n} LEDs).",
  "lh.auto.sync.take": "TL-Lüfter dort zuordnen", "lh.auto.parallel": "parallel: {p}",
  "lh.bind.parallel": "Parallel zu „{p}“: zeigt dieselben Farben.",
  "lh.bind.sync": "Sync-Modus: Alle Ports zeigen dieselbe Kette.",
  "lh.or": "Ausrichtung prüfen", "lh.or.need": "Erst einem Anschluss zuordnen.",
  "lh.or.fan": "Am Teil leuchten vier Farben: oben Rot, rechts Grün, unten Blau, links Gelb, LED 1 weiß.",
  "lh.or.strip": "Am Teil leuchtet LED 1 grün und die letzte LED rot, dazwischen ein Verlauf.",
  "lh.or.rings": "Am Teil leuchtet der obere Ring rot und der untere blau.",
  "lh.or.ask": "Zeigt das Teil das Muster wie die Vorschau?",
  "lh.or.ok": "Passt", "lh.or.next": "Passt, weiter", "lh.or.skip": "Weiter",
  "lh.or.rot90": "Um 90° gedreht", "lh.or.rot180": "Um 180° gedreht", "lh.or.rot270": "Um 270° gedreht",
  "lh.or.mirror": "Gespiegelt", "lh.or.reverse": "Anfang am anderen Ende", "lh.or.flip": "Oben und unten vertauscht",
  "lh.or.turn": "Drehung im Uhrzeigersinn, von vorn auf das Teil gesehen.",
  "lh.or.fixed": "Korrigiert: {f}. Muster erneut vergleichen.",
  "lh.or.done": "Ausrichtung von „{p}“ bestätigt.",
  "lh.or.limit": "Das Muster bleibt höchstens 60 s. Im Gerät wird nichts gespeichert.",
  "lh.or.expired": "Das Muster ist abgelaufen.", "lh.or.again": "Erneut zeigen", "lh.or.cancel": "Abbrechen",
  "lh.or.parallel": "Parallel zu „{p}“ angeschlossen: zeigt dessen Farben. Die Ausrichtung lässt sich hier nicht korrigieren.",
  "lh.or.alt": "Alternative: Controller-Modus mit direkter Port-Ansteuerung. Dann nur 2 Farben je Lüfter, die Einstellung wird in den Controller-Speicher geschrieben.",
  "lh.or.err": "Muster nicht möglich: {e}", "lh.or.step": "Teil {i} von {n}", "lh.or.nomatch": "Diese Korrektur passt nicht zu diesem Teil.",
  "lh.or.top": "oben", "lh.or.right": "rechts", "lh.or.bottom": "unten", "lh.or.left": "links", "lh.or.first": "LED 1", "lh.or.last": "letzte LED",
  "lh.auto.orient": "Ausrichtung der zugeordneten Teile prüfen",
  "lh.find.start": "Anschlüsse durchgehen", "lh.find.step": "Anschluss {i} von {n}", "lh.find.blink": "Dieser Anschluss blinkt jetzt weiß.",
  "lh.find.blinkfan": "Die Lüfter an diesem Port blinken jetzt.", "lh.find.err": "Anschluss lässt sich nicht aufleuchten: {e}",
  "lh.find.hint": "Teile anklicken, die blinken. Bei einer Kette in der Reihenfolge ab dem Anschluss.",
  "lh.find.bind": "Zuordnen", "lh.find.bindn": "{n} Teile zuordnen", "lh.find.skip": "Nichts blinkt – weiter", "lh.find.again": "Erneut blinken", "lh.find.end": "Beenden",
  "lh.find.count": "{z} meldet seine LED-Anzahl nicht. Die zugeordneten Teile haben zusammen {n} LEDs.",
  "lh.find.count.take": "{n} LEDs übernehmen", "lh.find.count.assist": "Mit Assistent ermitteln", "lh.find.count.keep": "Beibehalten",
  "lh.find.counted": "{z}: {n} LEDs eingestellt.", "lh.find.done": "{n} Teile über die Suche zugeordnet", "lh.find.done1": "1 Teil über die Suche zugeordnet",
  "lh.bind.done": "„{p}“ ist mit {z} verbunden.", "lh.bind.cleared": "„{p}“ ist nicht mehr verbunden.", "lh.bind.err": "Speichern fehlgeschlagen: {e}", "lh.bind.full": "zu kurz, höchstens {m} LEDs",
  "lh.sug.title": "Erkannte Geräte übernehmen", "lh.sug.ok": "Übernehmen", "lh.sug.no": "Abbrechen",
  "lh.sug.sub": "Passende Teile für die erkannten Geräte. Die Positionen sind ein Vorschlag und lassen sich in der Skizze ändern.",
  "lh.sug.none": "Keine passenden Teile gefunden.", "lh.sug.notes": "Hinweise",
  "lh.sug.add": "{n} Teile werden der Skizze hinzugefügt.", "lh.sug.add1": "1 Teil wird der Skizze hinzugefügt.",
  "lh.sug.done": "In die Skizze übernommen: {n}", "lh.sug.err": "Vorschlag nicht möglich: {e}", "lh.sug.save": "Speichern fehlgeschlagen: {e}",
  "lh.note.board": "Kein Mainboard mit {n} LEDs im Katalog.", "lh.note.ram": "Kein Speichermodul mit {n} LEDs im Katalog.",
  "lh.note.ring": "Kein Display mit {n} LEDs im Ring im Katalog.", "lh.note.nofan": "Kein Lüfterteil im Katalog.",
  "lh.note.unbound": "Lüfter bleiben ungebunden, wenn die Zone des Controllers weniger LEDs hat als das Lüfterteil.",
  "lh.note.limit": "Teilegrenze erreicht, nicht alles wird vorgeschlagen.",
  "lf.title": "Woher kommt die Farbe?", "lf.sub": "Jedes Gerät bekommt seine Farben aus genau einer Quelle. Ein Klick auf ein Gerät öffnet Quelle, Zonen und Einstellungen.",
  "lf.sources": "Quellen", "lf.devices": "Geräte und Zonen", "lf.source": "Quelle",
  "lf.src.effect": "CaseHUD-Effekt", "lf.src.board": "Mainboard", "lf.src.own": "Eigener Effekt", "lf.src.off": "Aus", "lf.src.none": "Nicht angesteuert",
  "lf.src.effect.sub": "Farbe aus dem Effekt, je LED nach ihrer Stelle im Gehäuse.",
  "lf.src.board.sub": "Folgt dem ARGB-Anschluss des Mainboards; bleibt nach dem Beenden bestehen.",
  "lf.src.own.sub": "Effekt, den der Controller selbst erzeugt.",
  "lf.src.off.sub": "Gerät bleibt dunkel; Effekt und Einstellungen bleiben gespeichert.",
  "lf.src.none.sub": "CaseHUD sendet nichts an dieses Gerät.",
  "lf.uses": "{n} Geräte", "lf.uses1": "1 Gerät", "lf.uses0": "nicht genutzt",
  "lf.zone.loose": "kein Teil zugeordnet", "lf.zone.parts": "{n} Teile", "lf.zone.parts1": "1 Teil", "lf.zone.nocount": "LED-Anzahl fehlt",
  "lf.zone.mirror": "spiegelt {z}", "lf.zone.mirror.none": "spiegelt den Mainboard-Anschluss",
  "lf.open": "Details zu {n}", "lf.more": "+{n} weitere Zonen",
  "lf.none": "Keine Geräte gefunden.", "lf.off": "Beleuchtung ist aus. Zum Erkennen der Geräte oben einschalten.",
  "lf.adopt": "Geräte in Skizze übernehmen…",
  "lf.d.source": "Quelle", "lf.d.zones": "Zonen", "lf.d.settings": "Weitere Geräteeinstellungen", "lf.d.layout": "Lage auf der Effektfläche",
  "lf.d.layout.sub": "Jeder Punkt ist eine LED, so wie der Effekt sie sieht.", "lf.d.close": "Schließen",
  "lf.d.loose": "Diese Zone folgt dem Effekt, gehört aber zu keinem Teil der Skizze. Ohne eigenes Layout liegt sie als Linie in der Mitte der Effektfläche.",
  "light.flash.title": "Controller-Speicher", "light.flash.ok": "Verstanden – umstellen",
  "light.more.title": "Weitere Einstellungen",
  "light.preset.delete.title": "Vorlage löschen", "light.preset.delete.ok": "Löschen",
});
Object.assign(STRINGS.en, {
  "lh.title": "Effect on the case",
  "lh.sub": "Side view in the middle, top above, bottom below, rear and front beside it. Every part takes the colour at its place.",
  "lh.live": "Live", "lh.live.preview": "Preview", "lh.live.off": "Lighting off", "lh.live.paused": "Paused", "lh.live.lost": "No live data", "lh.live.capture": "No preview",
  "lh.live.tip": "The colours the devices get right now.", "lh.live.preview.tip": "A computed preview of the effect; the devices get nothing right now.",
  "lh.note.capture": "Screen and audio effects only exist while they play and are not computed in advance.",
  "lh.note.error": "The effect cannot be shown.",
  "lh.stage": "Unfolded case with the effect",
  "lh.case": "{c} · {w} × {h} × {d} mm", "lh.case.custom": "Custom case",
  "lh.legend.bound": "connected", "lh.legend.unbound": "not connected",
  "lh.unbound.chip": "{n} parts without a zone", "lh.unbound.chip1": "1 part without a zone", "lh.unbound.title": "Click a part to connect it to a zone",
  "lh.empty.title": "No sketch yet", "lh.empty.text": "Place parts in the case and connect them to the zones of your devices; this view then shows which colour every part gets from the effect.",
  "lh.empty.adopt": "Use detected devices", "lh.empty.draw": "Draw sketch", "lh.empty.need": "Turn lighting on to detect devices.",
  "lh.tip.unbound": "Not connected", "lh.tip.leds": "{n} LEDs", "lh.tip.led1": "1 LED", "lh.tip.hint": "Click to connect",
  "lh.bind": "Connect to", "lh.bind.none": "– not connected –", "lh.bind.edit": "Open in editor", "lh.bind.close": "Close",
  "lh.bind.need": "Turn lighting on to see the zones of your devices.", "lh.bind.nozones": "No zones found.",
  "lh.auto": "Assign automatically", "lh.auto.title": "Connect parts whose zone is certain",
  "lh.auto.done": "{n} parts assigned automatically", "lh.auto.done1": "1 part assigned automatically", "lh.auto.none": "No part can be assigned for certain",
  "lh.auto.order": "Fan order: by the number in the name, else left to right or top to bottom. Can be changed in the editor.",
  "lh.auto.open": "{n} parts still open, e.g. on ARGB headers.", "lh.auto.open1": "1 part still open, e.g. on an ARGB header.",
  "lh.auto.undo": "Undo", "lh.auto.undone": "Assignment undone.", "lh.auto.err": "Automatic assignment failed: {e}",
  "lh.auto.src": "“{d}” currently shows: {s}. The assigned parts light up once the source is “CaseHUD effect”.", "lh.auto.src.switch": "Switch to CaseHUD effect",
  "lh.auto.n.tlsame": "Several ports carry the same fans. “Go through headers” shows which group is on which port.",
  "lh.auto.n.tlcount": "The TL fans in the sketch match no port (count or display).", "lh.auto.n.tldevs": "More than one fan controller found: please assign the fans by hand.",
  "lh.auto.n.ram": "{n} memory modules drawn, {m} found: please assign by hand.", "lh.auto.n.board": "{n} mainboards drawn, {m} onboard zones found: please assign by hand.",
  "lh.auto.n.leds": "“{p}” has {n} LEDs, {z} only {m}: the rest stays dark.",
  "lh.auto.n.ledsmax": "“{p}” has {n} LEDs, {z} drives at most {m}: please assign by hand.",
  "lh.auto.n.syncsrc": "The fan controller follows the motherboard, but its sync header is not set. Choose the header in the controller's device panel ({n} LEDs).",
  "lh.auto.n.syncused": "{z} is already in use: TL fans stay open.",
  "lh.auto.sync": "Sync mode: all ports show the same chain. Further fans take the colours of the fan in the same position.",
  "lh.auto.sync.ask": "The fan controller follows the motherboard. Matching sync header: {z} ({n} LEDs).",
  "lh.auto.sync.take": "Assign TL fans there", "lh.auto.parallel": "parallel: {p}",
  "lh.bind.parallel": "In parallel with “{p}”: shows the same colours.",
  "lh.bind.sync": "Sync mode: all ports show the same chain.",
  "lh.or": "Check orientation", "lh.or.need": "Connect it to a header first.",
  "lh.or.fan": "The part shows four colours: red at the top, green on the right, blue at the bottom, yellow on the left, LED 1 white.",
  "lh.or.strip": "The part shows LED 1 in green and the last LED in red, with a gradient in between.",
  "lh.or.rings": "The part shows the top ring in red and the bottom ring in blue.",
  "lh.or.ask": "Does the part show the pattern as the preview does?",
  "lh.or.ok": "Correct", "lh.or.next": "Correct, next", "lh.or.skip": "Next",
  "lh.or.rot90": "Turned by 90°", "lh.or.rot180": "Turned by 180°", "lh.or.rot270": "Turned by 270°",
  "lh.or.mirror": "Mirrored", "lh.or.reverse": "Starts at the other end", "lh.or.flip": "Top and bottom swapped",
  "lh.or.turn": "Turned clockwise, looking at the front of the part.",
  "lh.or.fixed": "Corrected: {f}. Compare the pattern again.",
  "lh.or.done": "Orientation of “{p}” confirmed.",
  "lh.or.limit": "The pattern stays for 60 s at most. Nothing is stored on the device.",
  "lh.or.expired": "The pattern has ended.", "lh.or.again": "Show again", "lh.or.cancel": "Cancel",
  "lh.or.parallel": "Connected in parallel with “{p}”: shows its colours. The orientation cannot be corrected here.",
  "lh.or.alt": "Alternative: controller mode with direct port control. Then only 2 colours per fan, and the setting is written to the controller's memory.",
  "lh.or.err": "Pattern not possible: {e}", "lh.or.step": "Part {i} of {n}", "lh.or.nomatch": "This correction does not fit this part.",
  "lh.or.top": "top", "lh.or.right": "right", "lh.or.bottom": "bottom", "lh.or.left": "left", "lh.or.first": "LED 1", "lh.or.last": "last LED",
  "lh.auto.orient": "Check the orientation of the assigned parts",
  "lh.find.start": "Go through headers", "lh.find.step": "Header {i} of {n}", "lh.find.blink": "This header is blinking white now.",
  "lh.find.blinkfan": "The fans on this port are blinking now.", "lh.find.err": "The header cannot be lit: {e}",
  "lh.find.hint": "Click the parts that blink. For a chain, in order from the header.",
  "lh.find.bind": "Assign", "lh.find.bindn": "Assign {n} parts", "lh.find.skip": "Nothing blinks – next", "lh.find.again": "Blink again", "lh.find.end": "Finish",
  "lh.find.count": "{z} does not report its LED count. The assigned parts have {n} LEDs together.",
  "lh.find.count.take": "Use {n} LEDs", "lh.find.count.assist": "Find with the assistant", "lh.find.count.keep": "Keep",
  "lh.find.counted": "{z}: set to {n} LEDs.", "lh.find.done": "{n} parts assigned by the search", "lh.find.done1": "1 part assigned by the search",
  "lh.bind.done": "“{p}” is connected to {z}.", "lh.bind.cleared": "“{p}” is no longer connected.", "lh.bind.err": "Saving failed: {e}", "lh.bind.full": "too short, at most {m} LEDs",
  "lh.sug.title": "Use detected devices", "lh.sug.ok": "Add", "lh.sug.no": "Cancel",
  "lh.sug.sub": "Matching parts for the detected devices. Positions are a suggestion and can be changed in the sketch.",
  "lh.sug.none": "No matching parts found.", "lh.sug.notes": "Notes",
  "lh.sug.add": "{n} parts are added to the sketch.", "lh.sug.add1": "1 part is added to the sketch.",
  "lh.sug.done": "Added to the sketch: {n}", "lh.sug.err": "Suggestion failed: {e}", "lh.sug.save": "Saving failed: {e}",
  "lh.note.board": "No mainboard with {n} LEDs in the catalog.", "lh.note.ram": "No memory module with {n} LEDs in the catalog.",
  "lh.note.ring": "No display with a ring of {n} LEDs in the catalog.", "lh.note.nofan": "No fan part in the catalog.",
  "lh.note.unbound": "Fans stay unbound where the controller zone has fewer LEDs than the fan part.",
  "lh.note.limit": "Part limit reached, not everything is suggested.",
  "lf.title": "Where does the colour come from?", "lf.sub": "Every device gets its colours from exactly one source. Click a device to choose its source and see its zones and settings.",
  "lf.sources": "Sources", "lf.devices": "Devices and zones", "lf.source": "Source",
  "lf.src.effect": "CaseHUD effect", "lf.src.board": "Mainboard", "lf.src.own": "Own effect", "lf.src.off": "Off", "lf.src.none": "Not driven",
  "lf.src.effect.sub": "Colour from the effect, per LED by its place in the case.",
  "lf.src.board.sub": "Follows the ARGB header of the mainboard; stays after CaseHUD exits.",
  "lf.src.own.sub": "An effect the controller generates itself.",
  "lf.src.off.sub": "The device stays dark; effect and settings stay stored.",
  "lf.src.none.sub": "CaseHUD sends nothing to this device.",
  "lf.uses": "{n} devices", "lf.uses1": "1 device", "lf.uses0": "not used",
  "lf.zone.loose": "no part assigned", "lf.zone.parts": "{n} parts", "lf.zone.parts1": "1 part", "lf.zone.nocount": "LED count missing",
  "lf.zone.mirror": "mirrors {z}", "lf.zone.mirror.none": "mirrors the mainboard header",
  "lf.open": "Details of {n}", "lf.more": "+{n} more zones",
  "lf.none": "No devices found.", "lf.off": "Lighting is off. Turn it on above to detect devices.",
  "lf.adopt": "Add devices to sketch…",
  "lf.d.source": "Source", "lf.d.zones": "Zones", "lf.d.settings": "More device settings", "lf.d.layout": "Place on the effect canvas",
  "lf.d.layout.sub": "Every dot is an LED, placed where the effect sees it.", "lf.d.close": "Close",
  "lf.d.loose": "This zone follows the effect but belongs to no part of the sketch. Without a layout of its own it lies as a line in the middle of the effect canvas.",
  "light.flash.title": "Controller memory", "light.flash.ok": "Understood – switch",
  "light.more.title": "More settings",
  "light.preset.delete.title": "Delete preset", "light.preset.delete.ok": "Delete",
});
applyI18n();

let lv = null; // last state from /api/lighting
/** @type {() => any} */
const focused = () => document.activeElement;
const lightCalibPos = {}; // "device/zone" -> marker position
const LIGHT_COLORS = ["#22d3ee", "#a78bfa", "#3ddc84", "#fab005", "#f43f5e", "#38bdf8", "#f472b6", "#fb923c"];
const lightActive = () => $("#tab-light").classList.contains("active");

const lightLabel = (kind, key, fallback) => {
  const k = "light." + kind + "." + key;
  return STRINGS[LANG][k] || STRINGS.de[k] || fallback || key;
};

async function lightPatch(body) {
  try {
    lv = await api("PATCH", "/api/lighting", body);
    lightSig = JSON.stringify(lv);
    renderLight();
    refreshLightChip();
  } catch (e) {
    toast(e.message, true);
    await loadLight(); // show what is really set
    // loadLight keeps the DOM when the state is unchanged, and the focus guards
    // keep the touched control as the user left it: redraw it explicitly.
    forceRender = true;
    try { renderLight(); } finally { forceRender = false; }
  }
}
let forceRender = false;

// Changes of several controls inside one debounce window are merged field by
// field, so the first of two quick changes is not dropped.
const mergePatch = (a, b) => {
  const out = { ...a };
  for (const [k, v] of Object.entries(b)) {
    const o = out[k];
    out[k] = o && v && typeof o === "object" && typeof v === "object" && !Array.isArray(v) ? mergePatch(o, v) : v;
  }
  return out;
};
let soonPatch = null, soonTimer = 0;
function lightPatchSoon(body) {
  soonPatch = soonPatch ? mergePatch(soonPatch, body) : body;
  clearTimeout(soonTimer);
  soonTimer = setTimeout(() => {
    const b = soonPatch;
    soonPatch = null;
    lightPatch(b);
  }, 160);
}

// Controllers like the TL fan hub store every command in flash memory: their
// settings are collected and sent one second after the last change, never per
// slider tick.
const devQueue = { patch: {}, timer: null };
function queueDeviceSetting(devId, values) {
  devQueue.patch[devId] = { ...(devQueue.patch[devId] || {}), ...values };
  clearTimeout(devQueue.timer);
  devQueue.timer = setTimeout(async () => {
    const devices = {};
    for (const [id, v] of Object.entries(devQueue.patch)) devices[id] = { settings: v };
    devQueue.patch = {}; devQueue.timer = null;
    await lightPatch({ devices });
  }, 1000);
}
const devQueued = () => devQueue.timer !== null;

let lightSig = "";
async function loadLight() {
  let next;
  try { next = await api("GET", "/api/lighting"); }
  catch (e) { if (lightActive()) toast(t("light.err.load", { e: e.message }), true); return; }
  const sig = JSON.stringify(next);
  if (sig === lightSig && lv) return; // nothing changed: keep the DOM (and focus) as it is
  lightSig = sig;
  lv = next;
  renderLight();
  refreshLightChip();
}

// One control for an effect or device parameter. The value is sent on change.
function lightParam(p, value, send, soon) {
  const label = lightLabel("p", p.key, p.label);
  const id = "lp-" + Math.random().toString(36).slice(2, 8);
  let ctl;
  if (p.type === "bool") {
    ctl = el("input", { type: "checkbox", class: "switch", id, checked: !!value, onchange: (e) => send(e.target.checked) });
    return el("div", { class: "lp" }, el("label", { for: id, class: "lp-head" }, el("span", { class: "lp-name" }, label)), ctl);
  }
  const head = el("span", { class: "lp-name" }, label);
  if (p.type === "number") {
    const lo = p.min ?? 0, hi = p.max > lo ? p.max : lo + 100;
    const whole = [lo, hi, p.default ?? 0].every(Number.isInteger);
    const step = whole || hi - lo > 20 ? 1 : 0.1;
    const val = el("span", { class: "lp-val" });
    const show = (v) => (val.textContent = String(Math.round(v * 100) / 100));
    ctl = el("input", { type: "range", id, min: lo, max: hi, step, value: value ?? p.default });
    show(ctl.value); setRangeFill(ctl);
    ctl.addEventListener("input", () => { show(ctl.value); setRangeFill(ctl); soon({ [p.key]: Number(ctl.value) }); });
    return el("div", { class: "lp" }, el("label", { for: id, class: "lp-head" }, head, val), ctl);
  }
  if (p.type === "color") {
    ctl = el("input", { type: "color", id, value: /^#[0-9a-f]{6}$/i.test(value) ? value : "#000000" });
    ctl.addEventListener("input", () => soon({ [p.key]: ctl.value }));
    return el("div", { class: "lp" }, el("label", { for: id, class: "lp-head" }, head), ctl);
  }
  if (p.type === "enum" && (p.values || []).length) {
    ctl = el("select", { id, onchange: (e) => send(e.target.value) },
      ...(p.values.includes(value) ? [] : [el("option", { value: value ?? "" }, value || "–")]),
      ...p.values.map((v) => el("option", { value: v, selected: v === value }, lightLabel("pv", p.key + "." + v, v))));
    return el("div", { class: "lp" }, el("label", { for: id, class: "lp-head" }, head), ctl);
  }
  ctl = el("input", { type: "text", id, value: value ?? "", maxlength: 200 });
  ctl.addEventListener("change", () => send(ctl.value));
  return el("div", { class: "lp" }, el("label", { for: id, class: "lp-head" }, head), ctl);
}

// Controllers with the setting allowFlashWrites store every light command in
// flash memory. The two modes that send commands on their own (hardware,
// canvas) are only set after the user confirmed; leaving them withdraws the
// consent so the next switch asks again.
const flashModes = ["hardware", "canvas"];
const hasFlashGate = (d) => (d.settings || []).some((p) => p.key === "allowFlashWrites");
const flashAllowed = (d) => !!(d.settingValues && d.settingValues.allowFlashWrites);
/** The device as it is now: a control may have been drawn from an older state. */
const fresh = (d) => (lv && lv.devices.find((x) => x.id === d.id)) || d;
const askFlash = () => notice({ title: "light.flash.title", paras: ["light.flash.confirm"], ok: "light.flash.ok", cancel: "ack.cancel" });
async function setTlMode(d, mode) {
  d = fresh(d);
  if (focused() && focused().blur) focused().blur();
  if (hasFlashGate(d)) {
    const flash = flashModes.includes(mode);
    if (flash && !flashAllowed(d)) {
      if (!(await askFlash())) { forceRender = true; try { renderLight(); } finally { forceRender = false; } return; }
      return lightPatch({ devices: { [d.id]: { settings: { mode, allowFlashWrites: true } } } });
    }
    if (!flash) return lightPatch({ devices: { [d.id]: { settings: { mode, allowFlashWrites: false } } } });
  }
  return lightPatch({ devices: { [d.id]: { settings: { mode } } } });
}

async function setAllowFlash(d, on) {
  d = fresh(d);
  if (focused() && focused().blur) focused().blur();
  if (on && !(await askFlash())) { forceRender = true; try { renderLight(); } finally { forceRender = false; } return; }
  return lightPatch({ devices: { [d.id]: { settings: { allowFlashWrites: on } } } });
}

async function setSyncSource(d, source) {
  try {
    lv = await api("POST", "/api/lighting/sync", { device: d.id, source });
    lightSig = JSON.stringify(lv);
    renderLight();
    refreshLightChip();
  } catch (e) {
    toast(e.message, true);
    await loadLight();
  }
}

// ---------- Sources: where the colours of a device come from ----------
// effect: the CaseHUD effect (controllers with a lighting mode: "canvas");
// board: the controller follows a mainboard header ("motherboard");
// own: the controller's own effect ("hardware"); off: dark (the device flag
// "off"); none: CaseHUD leaves the device alone (disabled, or "passive").
const SOURCES = ["effect", "board", "own", "off", "none"];
const modeValues = (d) => { const p = (d.settings || []).find((q) => q.key === "mode"); return (p && p.values) || []; };
function sourcesOf(d) {
  if (d.unsupported) return ["none"];
  const m = modeValues(d);
  if (!m.length) return ["effect", "off", "none"];
  return SOURCES.filter((s) => (s === "effect" ? m.includes("canvas") : s === "board" ? m.includes("motherboard") : s === "own" ? m.includes("hardware") : true));
}
function sourceOf(d) {
  if (d.disabled || d.unsupported) return "none";
  if (d.off) return "off";
  if (!modeValues(d).length) return "effect";
  const mode = (d.settingValues && d.settingValues.mode) || "passive";
  return mode === "motherboard" ? "board" : mode === "hardware" ? "own" : mode === "canvas" ? "effect" : "none";
}
async function setSource(d, src) {
  d = fresh(d);
  if (focused() && focused().blur) focused().blur();
  if (src === sourceOf(d)) return;
  const redraw = () => { forceRender = true; try { renderLight(); } finally { forceRender = false; } };
  const p = {};
  if (src === "none") p.disabled = true;
  else {
    p.disabled = false;
    p.off = src === "off";
    const gate = hasFlashGate(d), m = modeValues(d);
    if (m.length && src !== "off") {
      const mode = src === "board" ? "motherboard" : src === "own" ? "hardware" : "canvas";
      p.settings = { mode };
      if (gate && flashModes.includes(mode) && !flashAllowed(d)) {
        if (!(await askFlash())) { redraw(); return; }
        p.settings.allowFlashWrites = true;
      } else if (gate && !flashModes.includes(mode)) p.settings.allowFlashWrites = false;
    } else if (src === "off" && gate && !flashAllowed(d)) { // "off" is stored in the controller
      if (!(await askFlash())) { redraw(); return; }
      p.settings = { allowFlashWrites: true };
    }
  }
  return lightPatch({ devices: { [d.id]: p } });
}
// ---------- LED count of ARGB headers: assistant, parts, templates ----------
// Headers cannot report how many LEDs hang on them. Everything here is a live
// preview or a plain setting; nothing is written to a device permanently.
let editLabel = null; // "device/zone" whose name is being edited
let assist = null; // {device, zone, n, max, live}
let chain = null; // {device, zone, parts: [id], parallel, max}
let partsList = null, tplList = null, tplSig = "";
const isAssist = (d, z) => assist && assist.device === d.id && assist.zone === z.id;
const isChain = (d, z) => chain && chain.device === d.id && chain.zone === z.id;

const assistSend = debounce(async () => {
  if (!assist) return;
  const a = assist;
  try {
    await api("POST", "/api/lighting/assist", { device: a.device, zone: a.zone, n: a.n });
    a.live = true;
    loadLight();
  } catch (e) { toast(e.message, true); assist = null; renderLight(); }
}, 120);

function startAssist(d, z) {
  chain = null;
  const max = z.max > 0 ? Math.min(z.max, 512) : 512;
  assist = { device: d.id, zone: z.id, n: Math.max(1, Math.min(z.leds || 1, max)), max, live: false };
  api("POST", "/api/lighting/assist", { device: d.id, zone: z.id, n: assist.n })
    .then(() => { if (assist) assist.live = true; return loadLight(); })
    .catch((e) => { toast(e.message, true); assist = null; renderLight(); });
  renderLight();
}
function assistStep(delta) {
  assist.n = Math.max(1, Math.min(assist.max, assist.n + delta));
  assist.live = false;
  renderLight();
  assistSend();
}
async function endAssist(apply) {
  const a = assist;
  assist = null;
  try {
    if (apply) {
      lv = await api("POST", "/api/lighting/zone-setup", { device: a.device, zone: a.zone, count: a.n });
      lightSig = JSON.stringify(lv);
      toast(t("setup.saved"));
    } else await api("DELETE", "/api/lighting/calibrate");
  } catch (e) { toast(e.message, true); }
  await loadLight();
  renderLight();
}

function assistPanel(d, z) {
  const a = assist;
  const btn = (label, delta, txt) => el("button", { class: "ghost sm stepper-btn", type: "button", title: label, "aria-label": label,
    disabled: delta < 0 ? a.n <= 1 : a.n >= a.max, onclick: () => assistStep(delta) }, txt);
  const num = el("input", { type: "number", min: 1, max: a.max, value: a.n, "aria-label": t("setup.led"),
    onchange: (e) => assistStep(Math.max(1, Math.min(a.max, Number(e.target.value) || 1)) - a.n) });
  const slider = el("input", { type: "range", min: 1, max: a.max, step: 1, value: a.n, "aria-label": t("setup.led"),
    oninput: (e) => { a.n = Number(e.target.value); a.live = false; setRangeFill(e.target); num.value = a.n; assistSend(); } });
  setRangeFill(slider);
  return el("div", { class: "setup-panel" },
    el("b", { class: "setup-title" }, (z.label || z.name) + " · " + t("setup.assist")),
    el("p", { class: "setup-hint" }, t("setup.assist.hint")),
    el("div", { class: "stepper" }, btn(t("setup.minus10"), -10, "−10"), btn(t("setup.minus1"), -1, "−1"), num,
      btn(t("setup.plus1"), 1, "+1"), btn(t("setup.plus10"), 10, "+10")),
    slider,
    el("small", { class: "setup-live" }, t("setup.of", { n: a.n, m: a.max }) + " · " + t("setup.assist.live")),
    el("div", { class: "setup-actions" },
      el("button", { class: "primary", type: "button", onclick: () => endAssist(true) }, t("setup.apply")),
      el("button", { class: "ghost", type: "button", onclick: () => endAssist(false) }, t("setup.cancel"))));
}

// ---- parts picker
async function openChain(d, z) {
  if (assist) await endAssist(false);
  const own = (z.segments || []).filter((s) => s.shape === "component" && s.component).map((s) => s.component);
  const par = (z.segments || []).length > 1 && (z.segments || []).every((s) => s.start === 0);
  chain = { device: d.id, zone: z.id, parts: own, parallel: par, max: z.max > 0 ? z.max : 1024 };
  renderLight();
  try { partsList = await api("GET", "/api/lighting/parts"); } catch (e) { toast(e.message, true); partsList = []; }
  renderLight();
}
const partInfo = (id) => (partsList || []).find((p) => p.id === id) || { id, label: id, leds: 0 };
const chainTotal = () => {
  const ns = chain.parts.map((id) => partInfo(id).leds);
  return chain.parallel && ns.length > 1 ? Math.max(0, ...ns) : ns.reduce((a, b) => a + b, 0);
};
async function applyChain() {
  const c = chain;
  try {
    lv = await api("POST", "/api/lighting/zone-setup", { device: c.device, zone: c.zone, parts: c.parts, parallel: c.parallel });
    lightSig = JSON.stringify(lv);
    chain = null;
    toast(t("setup.saved"));
  } catch (e) { toast(e.message, true); }
  renderLight(); refreshLightChip();
}
function chainPanel(d, z) {
  const c = chain;
  const ready = partsList !== null;
  const total = chainTotal();
  const over = total > c.max;
  const sid = "cp-" + Math.random().toString(36).slice(2, 7);
  const sel = el("select", { id: sid, "aria-label": t("setup.parts.add") },
    el("option", { value: "" }, t("setup.parts.pick")),
    ...(partsList || []).map((p) => el("option", { value: p.id }, `${p.label} · ${t("setup.seg.leds", { n: p.leds })}`)));
  const move = (i, dlt) => { const j = i + dlt; if (j < 0 || j >= c.parts.length) return; [c.parts[i], c.parts[j]] = [c.parts[j], c.parts[i]]; renderLight(); };
  const items = c.parts.length ? c.parts.map((id, i) => el("li", {},
    el("span", { class: "part-name" }, el("b", {}, partInfo(id).label), el("small", {}, t("setup.seg.leds", { n: partInfo(id).leds }))),
    el("span", { class: "part-btns" },
      el("button", { class: "ghost icon-btn sm", type: "button", title: t("setup.parts.up"), "aria-label": t("setup.parts.up"), disabled: i === 0, onclick: () => move(i, -1) }, "↑"),
      el("button", { class: "ghost icon-btn sm", type: "button", title: t("setup.parts.down"), "aria-label": t("setup.parts.down"), disabled: i === c.parts.length - 1, onclick: () => move(i, 1) }, "↓"),
      el("button", { class: "ghost icon-btn sm", type: "button", title: t("setup.parts.remove"), "aria-label": t("setup.parts.remove"), onclick: () => { c.parts.splice(i, 1); renderLight(); } }, "✕"))))
    : [el("li", { class: "none" }, t("setup.parts.none"))];
  const pid = sid + "-par";
  return el("div", { class: "setup-panel" },
    el("b", { class: "setup-title" }, (z.label || z.name) + " · " + t("setup.parts")),
    el("p", { class: "setup-hint" }, t("setup.parts.hint")),
    el("div", { class: "part-add" }, sel,
      el("button", { class: "ghost", type: "button", disabled: !ready, onclick: () => { if (sel.value && c.parts.length < 16) { c.parts.push(sel.value); renderLight(); } } }, t("setup.parts.addbtn"))),
    el("ol", { class: "part-list" }, items),
    el("label", { class: "check-row", for: pid },
      el("input", { type: "checkbox", class: "switch", id: pid, checked: c.parallel, onchange: (e) => { c.parallel = e.target.checked; renderLight(); } }),
      el("span", {}, el("b", {}, t("setup.parts.parallel")), el("small", {}, t("setup.parts.parallel.sub")))),
    el("p", { class: "setup-sum" + (over ? " over" : "") },
      (c.parallel && c.parts.length > 1 ? t("setup.parts.longest", { n: total }) : t("setup.parts.sum", { n: total })) +
      (over ? " · " + t("setup.parts.over", { m: c.max }) : "")),
    el("small", { class: "setup-live" }, t("setup.parts.lib")),
    el("div", { class: "setup-actions" },
      el("button", { class: "primary", type: "button", disabled: !c.parts.length || over || !ready, onclick: applyChain }, t("setup.apply")),
      el("button", { class: "ghost", type: "button", onclick: () => { chain = null; renderLight(); } }, t("setup.cancel"))));
}

// ---- parts of a zone, shown below it
function zoneParts(z) {
  const segs = z.segments || [];
  if (segs.length < 2 && !(segs[0] && segs[0].shape === "component")) return null;
  let end = 0, anyPar = false;
  const rows = segs.map((s, i) => {
    const start = s.start != null ? s.start : end;
    const par = s.start != null && (start < end || segs.some((o, j) => j > i && o.start != null && o.start === start));
    anyPar = anyPar || par;
    end = Math.max(end, start + (s.count || 0));
    return { s, par };
  });
  return el("div", { class: "zone-parts" },
    el("ul", {}, rows.map(({ s, par }) => {
      const pid = /^sketch\./.test(s.component || "") ? s.component.slice(7) : "";
      const pl = pid && pv.sketch && pv.sketch.parts.find((p) => p.id === pid && p.bind);
      return el("li", {},
        el("span", {}, s.name || s.component || "-"),
        s.count ? el("small", {}, t("setup.seg.leds", { n: s.count })) : null,
        par ? el("em", { class: "par-badge" }, t("setup.seg.parallel")) : null,
        pl ? el("button", { class: "ghost zp-orient", type: "button", onclick: () => startOrient(pid) }, t("lh.or")) : null);
    })),
    anyPar ? el("small", { class: "setup-live" }, t("setup.seg.note")) : null);
}

// ---- hint card and templates
async function ensureTemplates() {
  if (!lv || !lv.active) { tplList = null; tplSig = ""; return; }
  const sig = lv.devices.map((d) => d.id + d.state).join(",");
  if (sig === tplSig) return;
  tplSig = sig;
  try { tplList = await api("GET", "/api/lighting/templates"); } catch (e) { tplList = []; }
  renderLight();
}
const noCounts = () => {
  const zs = lv.devices.filter((d) => d.state === "ok").flatMap((d) => d.zones.filter((z) => z.resizable));
  return zs.length > 0 && zs.every((z) => z.leds === 0);
};
async function applyTemplate(tp) {
  const name = tp.name[LANG] || tp.name.de;
  if (!(await notice({ title: "setup.tpl.title", paras: [["setup.tpl.confirm", { n: name }]], ok: "setup.tpl.btn", cancel: "ack.cancel", icon: "i" }))) return;
  try {
    const r = await api("POST", "/api/lighting/templates/apply", { id: tp.id, lang: LANG });
    lv = r.view; lightSig = JSON.stringify(lv);
    toast(t("setup.tpl.done", { a: r.applied.length }) + (r.skipped.length ? " " + t("setup.tpl.skipped", { s: r.skipped.join("; ") }) : ""));
  } catch (e) { toast(e.message, true); }
  renderLight(); refreshLightChip();
}
function setupCards() {
  const out = [];
  if (!lv.active) return out;
  if (noCounts()) {
    out.push(el("div", { class: "card setup-hint-card", role: "status" }, el("b", {}, t("setup.hint.title")), el("p", { class: "dev-note warn" }, t("setup.hint.text"))));
  }
  for (const tp of tplList || []) {
    out.push(el("div", { class: "card setup-tpl" },
      el("div", { class: "dev-title" }, el("b", {}, t("setup.tpl.title") + ": " + (tp.name[LANG] || tp.name.de)), el("small", {}, t("setup.tpl.sub"))),
      el("p", { class: "sub" }, (tp.description && (tp.description[LANG] || tp.description.de)) || ""),
      el("ul", { class: "tpl-zones" }, tp.zones.map((z) => el("li", {},
        el("span", {}, (z.label && (z.label[LANG] || z.label.de)) || z.zone), el("small", {}, t("setup.seg.leds", { n: z.leds }))))),
      tp.tlSync ? el("p", { class: "sync-note" }, t("setup.tpl.sync", { z: tp.syncZone || "" })) : null,
      el("div", { class: "setup-actions" }, el("button", { class: "primary", type: "button", onclick: () => applyTemplate(tp) }, t("setup.tpl.btn")))));
  }
  return out;
}

function lightZoneRow(d, z) {
  const key = d.id + "/" + z.id;
  const calibrating = lv.calibrating && lv.calibrating.device === d.id && lv.calibrating.zone === z.id && !lv.calibrating.range && !lv.calibrating.identify && !lv.calibrating.part;
  const limit = z.max > 0 ? Math.min(z.max, 512) : z.resizable ? 512 : z.count;
  const pos = Math.min(lightCalibPos[key] ?? 0, Math.max(limit - 1, 0));
  const send = (p) => {
    lightCalibPos[key] = p;
    api("POST", "/api/lighting/calibrate", { device: d.id, zone: z.id, pos: p }).then(loadLight).catch((e) => toast(e.message, true));
  };
  const active = d.state === "ok";
  const commitLabel = (input) => {
    if (editLabel !== key) return;
    editLabel = null;
    const v = input.value.trim();
    input.blur(); // so the list can be rebuilt
    if (v === (z.label || "")) { setTimeout(renderLight, 0); return; }
    lightPatch({ devices: { [d.id]: { zones: { [z.id]: { label: v } } } } });
  };
  const nameTop = editLabel === key
    ? el("input", { class: "label-input", type: "text", maxlength: 40, value: z.label || "", placeholder: z.name, "aria-label": t("setup.rename.ph"),
        onkeydown: (e) => { if (e.key === "Enter") e.target.blur(); else if (e.key === "Escape") { e.stopPropagation(); editLabel = null; e.target.blur(); setTimeout(renderLight, 0); } },
        onchange: (e) => commitLabel(e.target), onblur: (e) => commitLabel(e.target) })
    : el("span", { class: "zone-title" }, el("b", {}, z.label || z.name),
        el("button", { class: "icon-btn sm pencil", type: "button", title: t("setup.rename"), "aria-label": t("setup.rename"),
          onclick: () => { editLabel = key; renderLight(); } }, "✎"));
  const flags = zoneFlags(d, z);
  const row = el("div", { class: "zone" },
    el("div", { class: "zone-name" }, nameTop, el("small", {}, (z.label ? z.name + " · " : "") + (z.leds === 1 ? t("light.zone.led1") : t("light.zone.leds", { n: z.leds })) + " · " + z.order),
      ...flags.map((f) => el("em", { class: "lf-flag " + f.kind }, f.text))),
    z.resizable ? el("label", { class: "zone-count" }, el("span", {}, t("light.zone.count")),
      el("input", { type: "number", min: 0, max: Math.min(z.max || 1024, 1024), value: z.leds, onchange: (e) => {
        const n = Math.max(0, Math.min(Number(e.target.value) || 0, Math.min(z.max || 1024, 1024)));
        lightPatch({ devices: { [d.id]: { zones: { [z.id]: { count: n } } } } });
      } })) : el("span"),
    calibrating
      ? el("button", { class: "ghost", onclick: () => api("DELETE", "/api/lighting/calibrate").then(loadLight) }, t("light.zone.calib.end"))
      : el("button", { class: "ghost", disabled: !active || limit < 1, onclick: () => send(pos) }, t("light.zone.calib")));
  const extras = [zoneParts(z)];
  if (flags.some((f) => f.loose)) extras.push(el("p", { class: "dev-note" }, t("lf.d.loose")));
  if (z.resizable && active) {
    extras.push(el("div", { class: "zone-tools" },
      el("button", { class: "ghost", type: "button", disabled: isAssist(d, z), onclick: () => startAssist(d, z) }, t("setup.assist")),
      el("button", { class: "ghost", type: "button", disabled: isChain(d, z), onclick: () => openChain(d, z) }, t("setup.parts"))));
  }
  if (isAssist(d, z)) extras.push(assistPanel(d, z));
  if (isChain(d, z)) extras.push(chainPanel(d, z));
  if (!calibrating) return el("div", { class: "zone-wrap" }, row, extras);
  return el("div", { class: "zone-wrap" }, row, extras,
    el("div", { class: "calib" },
      el("button", { class: "ghost icon-btn sm", title: t("light.zone.prev"), "aria-label": t("light.zone.prev"), disabled: pos <= 0, onclick: () => send(pos - 1) }, "‹"),
      el("span", { class: "calib-pos" }, `${pos + 1} / ${limit}`),
      el("button", { class: "ghost icon-btn sm", title: t("light.zone.next"), "aria-label": t("light.zone.next"), disabled: pos >= limit - 1, onclick: () => send(pos + 1) }, "›"),
      el("small", {}, t("light.zone.calib.hint"))));
}

/** The zones of one device on the effect canvas (their own layouts and the parts of the sketch). */
function lightCanvas(d) {
  const [cw, ch] = lv.canvas;
  const W = 640, H = Math.round((640 * ch) / cw);
  const c = el("canvas", { width: W, height: H, class: "light-canvas", role: "img", "aria-label": t("lf.d.layout") });
  const g = c.getContext("2d");
  g.fillStyle = "#0a0d13"; g.fillRect(0, 0, W, H);
  g.strokeStyle = "#232c3c"; g.lineWidth = 1;
  for (let i = 1; i < 8; i++) { g.beginPath(); g.moveTo((W * i) / 8 + .5, 0); g.lineTo((W * i) / 8 + .5, H); g.stroke(); }
  for (let i = 1; i < 5; i++) { g.beginPath(); g.moveTo(0, (H * i) / 5 + .5); g.lineTo(W, (H * i) / 5 + .5); g.stroke(); }
  g.strokeStyle = "#334155"; g.strokeRect(.5, .5, W - 1, H - 1);
  g.fillStyle = devHue(d.id) || LIGHT_COLORS[0];
  for (const z of d.zones) {
    for (const [x, y] of z.points) { g.beginPath(); g.arc(x * (W - 12) + 6, y * (H - 12) + 6, 4, 0, Math.PI * 2); g.fill(); }
    const name = z.label || z.name;
    if (name && z.points.length) {
      const xs = z.points.map((q) => q[0]), ys = z.points.map((q) => q[1]);
      g.font = "11px system-ui, sans-serif"; g.textBaseline = "bottom";
      g.fillText(name, Math.min(Math.min(...xs) * (W - 12) + 6, W - g.measureText(name).width - 4), Math.max(12, Math.min(...ys) * (H - 12) - 2));
    }
    // parts in parallel (splitter) repeat LEDs: drawn hollow
    g.strokeStyle = g.fillStyle; g.lineWidth = 1.5;
    for (const [x, y] of z.echo || []) { g.beginPath(); g.arc(x * (W - 12) + 6, y * (H - 12) + 6, 3.5, 0, Math.PI * 2); g.stroke(); }
  }
  return c;
}

// ---------- signal flow: sources, devices, zones ----------
// Drawn like the cooling wiring: sources on the left, devices on the right,
// one cable per device from the source it follows. Clicking a device opens
// its details in the side panel.
const flow = { open: /** @type {string|null} */ (null), sig: "" };
/** Parts of the stored sketch bound to a zone. */
function boundParts(dev, zone) {
  return ((pv.sketch && pv.sketch.parts) || []).filter((p) => p.bind && p.bind.device === dev && p.bind.zone === zone);
}
/** Notes on a zone: mirrored header, missing LED count, effect zone without a part. */
function zoneFlags(d, z) {
  const out = [], src = sourceOf(d);
  if (z.mirrored) out.push({ kind: "info", text: z.mirroredFrom ? t("lf.zone.mirror", { z: z.mirroredFrom }) : t("lf.zone.mirror.none") });
  if (z.resizable && z.leds === 0 && src === "effect") out.push({ kind: "warn", text: t("lf.zone.nocount") });
  const n = boundParts(d.id, z.id).length;
  if (n) out.push({ kind: "ok", text: n === 1 ? t("lf.zone.parts1") : t("lf.zone.parts", { n }) });
  else if (src === "effect" && z.leds > 0 && !z.mirrored && pv.sketch !== undefined) out.push({ kind: "warn", text: t("lf.zone.loose"), loose: true });
  return out;
}
function deviceNote(d) {
  if (d.state === "conflict") return t("light.conflict.by", { names: (d.message || "").replace(/^in use by /, "") });
  if (d.state === "error" || d.unsupported) return d.message || "";
  return "";
}
function sourceNode(src, uses) {
  const n = uses.length;
  const sub = src === "effect" ? lightLabel("fx", lv.effect, lv.effect) + " · " + lv.brightness + " %" : t("lf.src." + src + ".sub");
  return el("div", { class: "cool-node lf-src-node lf-s-" + src + (n ? "" : " unused"), "data-src": src },
    src === "effect" ? el("img", { class: "lf-swatch", alt: "", "aria-hidden": "true", src: pv.png ? "data:image/png;base64," + pv.png : "" }) : null,
    el("div", { class: "cool-node-title" }, t("lf.src." + src)),
    el("div", { class: "cool-node-sub" }, sub),
    el("div", { class: "lf-uses" }, n === 0 ? t("lf.uses0") : n === 1 ? t("lf.uses1") : t("lf.uses", { n })),
    el("span", { class: "cool-port cool-port-out lf-port", "aria-hidden": "true" }));
}
function deviceNode(d, idx) {
  const src = sourceOf(d), note = deviceNote(d), zs = d.zones;
  const shown = zs.slice(0, 6);
  const node = el("div", { class: "cool-node lf-dev lf-s-" + src + (flow.open === d.id ? " cool-selected" : "") + (d.state === "ok" || d.state === "disabled" ? "" : " lf-bad"),
    "data-id": d.id, "data-src": src, role: "button", tabindex: 0, "aria-label": t("lf.open", { n: d.name }), "aria-expanded": String(flow.open === d.id),
    onclick: () => openDrawer(d.id), onkeydown: (e) => { if ((e.key === "Enter" || e.key === " ") && e.target === e.currentTarget) { e.preventDefault(); openDrawer(d.id); } } },
    el("span", { class: "cool-port cool-port-in lf-port", "aria-hidden": "true" }),
    el("div", { class: "lf-dev-head" },
      el("span", { class: "dev-dot", style: { background: LIGHT_COLORS[idx % LIGHT_COLORS.length] } }),
      el("div", { class: "cool-node-title" }, d.name),
      el("span", { class: "badge " + (d.unsupported ? "disabled" : d.state) }, d.unsupported ? t("light.unsupported") : t("light.state." + d.state))),
    note ? el("div", { class: "cool-node-sub lf-note " + d.state }, note) : null,
    d.warning ? el("div", { class: "cool-node-sub lf-note warn" }, d.warning) : null,
    zs.length ? el("ul", { class: "lf-zones" }, shown.map((z) => {
      const flags = zoneFlags(d, z);
      return el("li", { class: flags.some((f) => f.kind === "warn") ? "warn" : "" },
        el("span", { class: "lf-zone-name" }, z.label || z.name),
        el("small", {}, z.leds === 1 ? t("light.zone.led1") : t("light.zone.leds", { n: z.leds })),
        ...flags.map((f) => el("em", { class: "lf-flag " + f.kind }, f.text)));
    }), zs.length > shown.length ? el("li", { class: "more" }, t("lf.more", { n: zs.length - shown.length })) : null) : null);
  return node;
}
function renderFlow() {
  const board = $("#light-flow");
  const on = lv.enabled;
  const devs = lv.devices;
  $("#light-adopt").disabled = !(on && lv.active && devs.length);
  $("#light-rescan").disabled = !on || !lv.active;
  if (!devs.length) {
    board.classList.add("lf-empty");
    const sig = ["empty", LANG, on, lv.detected || ""].join("|");
    if (flow.sig !== sig) {
      flow.sig = sig;
      board.replaceChildren(...(!on && lv.detected ? [el("div", { class: "empty-note detected" }, t("light.detected." + (lv.detected === "ring" ? "ring" : "other")))] : []),
        el("div", { class: "empty-note" }, on ? t("lf.none") : t("lf.off")));
    }
    closeDrawer(true);
    return;
  }
  board.classList.remove("lf-empty");
  const busy = !forceRender && board.contains(focused()) && focused().tagName === "SELECT";
  const sig = JSON.stringify([LANG, lv.devices, lv.effect, lv.brightness, flow.open, !!pv.sketch, ((pv.sketch && pv.sketch.parts) || []).map((p) => p.bind || null)]);
  if (!busy && (sig !== flow.sig || forceRender)) {
    flow.sig = sig;
    const used = new Map(SOURCES.map((s) => [s, []]));
    for (const d of devs) used.get(sourceOf(d)).push(d.id);
    const offered = new Set(devs.flatMap((d) => sourcesOf(d)));
    const srcs = SOURCES.filter((s) => offered.has(s) || used.get(s).length);
    board.replaceChildren(
      svgFlow(),
      el("div", { class: "cool-col lf-col-src" }, el("h3", { class: "cool-col-head" }, t("lf.sources")),
        el("div", { class: "cool-col-body" }, srcs.map((s) => sourceNode(s, used.get(s))))),
      el("div", { class: "cool-col lf-col-dev" }, el("h3", { class: "cool-col-head" }, t("lf.devices")),
        el("div", { class: "cool-col-body" }, devs.map((d, i) => deviceNode(d, i)))));
    requestAnimationFrame(drawFlowCables);
  }
  board.classList.toggle("lf-running", !!(lv.enabled && lv.active && !lv.paused));
  renderDrawer();
}
const svgFlow = () => { const s = document.createElementNS("http://www.w3.org/2000/svg", "svg"); s.setAttribute("class", "cool-cables lf-cables"); s.setAttribute("aria-hidden", "true"); return s; };
function drawFlowCables() {
  const board = $("#light-flow"), svg = board && board.querySelector(".lf-cables");
  if (!svg || !board.offsetWidth) return;
  const box = board.getBoundingClientRect();
  const at = (port) => { const r = port.getBoundingClientRect(); return [r.left + r.width / 2 - box.left, r.top + r.height / 2 - box.top]; };
  const kids = [];
  for (const node of board.querySelectorAll(".lf-dev")) {
    const src = node.dataset.src, from = board.querySelector(`.lf-src-node[data-src="${src}"] .cool-port-out`), to = node.querySelector(".cool-port-in");
    if (!from || !to) continue;
    const d = coolCablePath(at(from), at(to));
    const g = svgEl("g", { class: "cool-cable-group lf-cable lf-s-" + src + (src === "effect" || src === "board" || src === "own" ? " cool-cable-lead" : " cool-cable-dead"), "data-flow": "2" });
    g.append(svgEl("path", { d, class: "cool-cable" }), svgEl("path", { d, class: "cool-cable-flow" }));
    kids.push(g);
  }
  svg.replaceChildren(...kids);
}

// ---- side panel with the details of one device
function openDrawer(id) {
  flow.open = flow.open === id ? null : id;
  flow.sig = "";
  renderFlow();
  if (flow.open) {
    const p = $("#light-drawer");
    requestAnimationFrame(() => { if (window.innerWidth <= 1180) p.scrollIntoView({ block: "nearest", behavior: "smooth" }); $(".cool-panel-close", p)?.focus({ preventScroll: true }); });
  }
}
function closeDrawer(quiet) {
  if (!flow.open) return;
  const id = flow.open;
  flow.open = null;
  $("#light-drawer").hidden = true;
  $("#light-flow-wrap").classList.remove("open");
  if (quiet) return;
  flow.sig = "";
  renderFlow();
  const node = $(`#light-flow .lf-dev[data-id="${CSS.escape(id)}"]`);
  if (node) node.focus({ preventScroll: true });
}
// Per-port firmware effects of the TL controller ("Eigener Effekt"): every
// port can run its own effect and direction. With "follow" the server derives
// colors, speed and the direction per port from the CaseHUD effect and the
// case sketch; d.ports says what each port runs and where that comes from.
const tlPortKey = /^port\d+(Effect|Direction)$/;
const tlPortsShown = (d) => d.backend === "lianlitl" && (d.settings || []).some((p) => p.key === "follow")
  && !(d.settingValues && d.settingValues.merge);
/** "Seite 1", "Seite 2", "Seite 3" -> "Seite 1–3"; other labels are listed. */
function compactLabels(labels) {
  if (!labels || !labels.length) return "";
  const m = labels.map((x) => /^(.*?)(\d+)$/.exec(x));
  const nums = m.map((x) => x && Number(x[2]));
  const run = labels.length > 2 && m.every((x) => x && x[1] === m[0][1]) && nums.every((n, i) => !i || n === nums[i - 1] + 1);
  return run ? m[0][1] + nums[0] + "–" + nums[nums.length - 1] : labels.join(", ");
}
function tlPortRows(d) {
  if (!tlPortsShown(d)) return [];
  const sv = d.settingValues || {}, follow = !!sv.follow;
  const decl = (key) => (d.settings || []).find((p) => p.key === key);
  const nums = (d.settings || []).map((p) => /^port(\d+)Effect$/.exec(p.key)).filter(Boolean).map((x) => Number(x[1])).sort((a, b) => a - b);
  if (!nums.length) return [];
  const send = (v) => queueDeviceSetting(d.id, v);
  const fid = "lf-tlf-" + d.id.replace(/[^\w-]/g, "_");
  const rows = nums.map((n) => {
    const pv = (d.ports || []).find((x) => x.port === n) || {};
    const ek = "port" + n + "Effect", dk = "port" + n + "Direction";
    const sel = (key, first, label, kind) => {
      const p = decl(key), cur = sv[key] || "";
      return el("select", { "aria-label": label, "data-k": key, onchange: (e) => send({ [key]: e.target.value }) },
        el("option", { value: "", selected: !cur }, first),
        ...((p && p.values) || []).filter((v) => v).map((v) => el("option", { value: v, selected: v === cur }, lightLabel("pv", kind + "." + v, v))));
    };
    let hint = "";
    if (pv.whole) hint = t("light.tl.whole", { e: lightLabel("pv", "effect." + pv.effect, pv.effect) }) + ", " + t("light.tl.src." + (pv.effectSource || "sketch"));
    else if (pv.direction) hint = lightLabel("pv", "direction." + pv.direction, pv.direction) + ", " + t("light.tl.src." + (pv.source || "all"));
    if (follow && !(pv.parts || []).length) hint = hint ? hint + " · " + t("light.tl.nofans") : t("light.tl.nofans");
    const where = compactLabels(pv.labels);
    return el("div", { class: "tl-port", "data-port": n },
      el("div", { class: "tl-port-head" }, el("b", {}, t("light.tl.port", { n })), where ? el("span", { class: "tl-port-where" }, " · " + where) : null),
      el("div", { class: "tl-port-sel" },
        sel(ek, t("light.tl.all"), t("light.tl.effect", { n }), "effect"),
        sel(dk, follow ? t("light.tl.sketch") : t("light.tl.all"), t("light.tl.direction", { n }), "direction")),
      hint ? el("small", { class: "cool-hint tl-port-hint" }, hint) : null);
  });
  return [el("div", { class: "lf-d-sec tl-ports" }, el("h4", { class: "light-sub" }, t("light.tl.ports")),
    el("label", { class: "check-row" },
      el("input", { type: "checkbox", class: "switch", id: fid, checked: follow, onchange: (e) => send({ follow: e.target.checked }) }),
      el("span", {}, el("b", {}, t("light.tl.follow")), el("small", {}, t("light.tl.follow.sub")))),
    ...rows,
    el("small", { class: "cool-hint" }, t("light.tl.flash")))];
}
function drawerContent(d) {
  const src = sourceOf(d), note = deviceNote(d);
  const out = [
    el("div", { class: "cool-panel-head" },
      el("h3", { class: "cool-panel-title" }, d.name),
      el("button", { class: "ghost icon-btn sm cool-panel-close", type: "button", title: t("lf.d.close"), "aria-label": t("lf.d.close"), onclick: () => closeDrawer() }, "✕")),
    el("small", { class: "cool-hint", title: d.firmware ? t("field.firmware.sub") : "" }, d.backend + " · " + d.id + (d.firmware ? " · " + t("field.firmware") + " " + d.firmware : "")),
    ...fwNoteNodes(d.firmwareLatest).map((n) => el("small", { class: "cool-hint" }, n)),
  ];
  if (d.warning) out.push(el("p", { class: "dev-note warn", role: "alert" }, d.warning));
  if (note) out.push(el("p", { class: "dev-note " + d.state }, note));
  if (d.state === "conflict" || d.ignoreConflicts) {
    out.push(el("label", { class: "check-row" },
      el("input", { type: "checkbox", class: "switch", checked: d.ignoreConflicts, onchange: (e) => lightPatch({ devices: { [d.id]: { ignoreConflicts: e.target.checked } } }) }),
      el("span", {}, el("b", {}, t("light.ignore")), el("small", {}, t("light.ignore.sub")))));
  }
  // source: one radio per source with its meaning
  const name = "lf-src-" + d.id.replace(/[^\w-]/g, "_");
  out.push(el("fieldset", { class: "lf-radios" }, el("legend", { class: "light-sub" }, t("lf.d.source")),
    ...sourcesOf(d).map((s) => el("label", { class: "lf-radio" + (s === src ? " on" : "") },
      el("input", { type: "radio", name, value: s, checked: s === src, disabled: !!d.unsupported, onchange: () => setSource(d, s) }),
      el("span", {}, el("b", {}, t("lf.src." + s)), el("small", {}, s === "off" && d.backend === "lianlitl" ? t("light.devoff.sub.tl") : t("lf.src." + s + ".sub")))))));
  if (d.off && hasFlashGate(d) && !flashAllowed(d)) out.push(el("p", { class: "dev-note warn", role: "alert" }, t("light.devoff.blocked")));
  if (src === "board" && d.sync) {
    const sy = d.sync, sid = "lf-sync-" + d.id.replace(/[^\w-]/g, "_");
    if (!sy.choices.length) out.push(el("p", { class: "sync-note" }, t("light.syncsrc.nohdr")));
    else {
      out.push(el("div", { class: "cool-field" }, el("label", { for: sid }, t("light.syncsrc")),
        el("select", { id: sid, onchange: (e) => setSyncSource(d, e.target.value) },
          el("option", { value: "", selected: !sy.source }, t("light.syncsrc.none")),
          ...sy.choices.map((c) => el("option", { value: c.id, selected: c.id === sy.source }, c.label)))),
        el("p", { class: "sync-note" }, sy.fans > 0 ? t("light.syncsrc.note", { n: sy.fans }) : t("light.syncsrc.nofans")),
        el("p", { class: "sync-note" }, t("light.syncsrc.ports")));
    }
  }
  if (src === "board") out.push(el("p", { class: "sync-note" }, t("light.sync.note")));
  if (src === "own") out.push(...tlPortRows(d));
  if (d.zones.length) out.push(el("div", { class: "lf-d-sec" }, el("h4", { class: "light-sub" }, t("lf.d.zones")), el("div", { class: "zones" }, d.zones.map((z) => lightZoneRow(d, z)))));
  if (d.zones.some((z) => z.points && z.points.length)) {
    out.push(el("div", { class: "lf-d-sec" }, el("h4", { class: "light-sub" }, t("lf.d.layout")), el("small", { class: "cool-hint" }, t("lf.d.layout.sub")),
      el("div", { class: "light-canvas-box" }, lightCanvas(d))));
  }
  const perPort = src === "own" && tlPortsShown(d); // drawn above as rows per port
  const shownSettings = (d.settings || []).filter((p) => !(d.sync && p.key === "syncSource") // the helper sets it
    && !(perPort && (p.key === "follow" || tlPortKey.test(p.key))));
  if (shownSettings.length) {
    const slow = hasFlashGate(d); // writes to the controller's flash
    const warn = slow && d.settingValues && flashModes.includes(d.settingValues.mode);
    out.push(el("details", { class: "dev-settings", "data-k": "settings" }, el("summary", {}, t("lf.d.settings")),
      slow ? el("p", { class: "dev-note" }, t("light.flash")) : null,
      el("div", { class: "lp-grid" }, shownSettings.map((p) => lightParam(p, d.settingValues[p.key],
        p.key === "mode" && slow ? (v) => setTlMode(d, v)
          : p.key === "allowFlashWrites" ? (v) => setAllowFlash(d, v)
          : slow ? (v) => queueDeviceSetting(d.id, { [p.key]: v }) : (v) => lightPatch({ devices: { [d.id]: { settings: { [p.key]: v } } } }),
        slow ? (v) => queueDeviceSetting(d.id, v) : (v) => lightPatchSoon({ devices: { [d.id]: { settings: v } } })))),
      warn ? el("p", { class: "dev-note error" }, t("light.tl.modeWarn")) : null));
  }
  return out;
}
function renderDrawer() {
  const p = $("#light-drawer"), d = flow.open && lv.devices.find((x) => x.id === flow.open);
  if (!d) { if (flow.open) closeDrawer(true); return; }
  p.hidden = false;
  $("#light-flow-wrap").classList.add("open");
  const ae = focused();
  const busy = !forceRender && (devQueued() || (p.contains(ae) && ["number", "text", "range", "color", "select-one"].includes(ae.type)));
  if (busy) return;
  const sig = JSON.stringify([LANG, d, lv.calibrating || null, assist, chain, editLabel, partsList ? partsList.length : -1, (pv.sketch && pv.sketch.parts || []).map((q) => q.bind || null)]);
  if (sig === p.dataset.sig && !forceRender) return;
  p.dataset.sig = sig;
  const open = new Set($$("details[open]", p).map((x) => x.dataset.k));
  const keep = p.scrollTop;
  p.replaceChildren(...drawerContent(d));
  for (const x of $$("details", p)) if (open.has(x.dataset.k)) x.open = true;
  p.scrollTop = keep;
  queueMicrotask(() => { const i = editLabel && $(".label-input", p); if (i && document.activeElement !== i) { i.focus(); i.select(); } });
}

function renderLight() {
  if (!lv) return;
  if (assist && assist.live && !(lv.calibrating && lv.calibrating.range && lv.calibrating.zone === assist.zone)) {
    assist = null; toast(t("setup.assist.ended"), true);
  }
  if (!lv.active) { assist = null; chain = null; }
  ensureTemplates();
  const on = lv.enabled;
  $("#light-enabled").checked = on;
  let status = on ? (lv.devices.length ? (lv.devices.length === 1 ? t("light.master.on1") : t("light.master.on", { n: lv.devices.length })) : t("light.master.on0")) : t("light.master.off");
  if (on && lv.paused) status = t("light.paused");
  $("#light-status").textContent = status;
  $("#light-status").className = "sub" + (on && lv.error ? " warn" : "");
  const rst = $("#light-reset");
  rst.hidden = !lv.configReset;
  rst.textContent = lv.configReset ? t("light.reset", { file: lv.configBackup }) : "";
  const warn = $("#light-warn");
  warn.hidden = !(on && !lv.active);
  warn.textContent = t("light.nobackend");

  $("#light-look").classList.toggle("dim", !on);
  // brightness + effect
  const br = $("#light-brightness");
  if (document.activeElement !== br) { br.value = lv.brightness; setRangeFill(br); }
  $("#light-brightness-val").textContent = lv.brightness + " %";
  const ex = $("#light-exit");
  if (document.activeElement !== ex) ex.value = lv.shutdown === "keep" || lv.shutdown === "device" ? lv.shutdown : "off";
  const sel = $("#light-effect");
  // Only the featured effects are offered; one in use that is not featured stays selectable as "More: name".
  const shown = lv.effects.filter((e) => e.featured || e.name === lv.effect);
  if (document.activeElement !== sel || forceRender) {
    sel.replaceChildren(...shown.map((e) => el("option", { value: e.name, selected: e.name === lv.effect },
      e.featured ? lightLabel("fx", e.name, e.label) : t("light.more", { n: lightLabel("fx", e.name, e.label) }))));
  }
  const info = lv.effects.find((e) => e.name === lv.effect);
  const box = $("#light-params");
  const psig = JSON.stringify([LANG, lv.effect, lv.params, info && info.params]);
  if ((!(document.activeElement && box.contains(document.activeElement) && ["range", "color", "text", "select-one", "checkbox"].includes(focused().type)) && box.dataset.sig !== psig) || forceRender) {
    box.dataset.sig = psig;
    box.replaceChildren(...((info && info.params) || []).map((p) => lightParam(p, (lv.params || {})[p.key],
      (v) => lightPatch({ params: { [p.key]: v } }), (v) => lightPatchSoon({ params: v }))));
    if (info && info.capture) box.prepend(el("p", { class: "dev-note capture-note" }, lightLabel("capture", "note." + info.name, "")));
  }
  $("#light-params-head").hidden = !(info && info.params.length);

  // set-up hints, signal flow, side panel
  const setup = $("#light-setup"), cards = setupCards();
  setup.hidden = !cards.length;
  const ssig = JSON.stringify([LANG, noCounts && lv.active ? noCounts() : false, (tplList || []).map((x) => x.id)]);
  if (setup.dataset.sig !== ssig) { setup.dataset.sig = ssig; setup.replaceChildren(...cards); }
  renderFlow();
  const ram = lv.ram || {}, rh = $("#light-ram");
  rh.hidden = !(on && lv.active && (ram.hint || ram.note));
  if (!rh.hidden) rh.replaceChildren(el("b", {}, t("light.ram.title")), el("p", { class: "dev-note" + (ram.hint ? "" : " warn") }, ram.hint ? t("light.ram.hint") : ram.note));

  // the case net with the effect
  renderPreview();

  // experimental TL switch
  $("#light-tl-part").hidden = !lv.tlAvailable;
  $("#light-tl").checked = lv.tl;
  refreshDiagTrace();
  renderLibrary();
}

function refreshLightChip() {
  const chip = $("#light-chip");
  if (!chip || !lv) return;
  const n = lv.conflicts, e = lv.errors;
  const text = !lv.enabled ? "" : n ? t("light.chip.conflict", { n }) : e ? t("light.chip.error", { n: e }) : "";
  chip.hidden = !text || !state.panels.length;
  chip.textContent = text;
}


// ---------- Presets from the library ----------
// Opening the presets shows the saved copy; only "Update library" (or the
// first opening, when there is no copy yet) goes online.
let libState = null, libBusy = false, presetBusy = false;
const libText = (m) => (m && (m[LANG] || m.en || m.de)) || "";

const tagLabel = (g) => (Object.hasOwn(STRINGS[LANG], "light.tag." + g) ? t("light.tag." + g) : g);
function renderLibrary() {
  if (!libState) return;
  const st = libState.status || {}, ps = libState.presets || [];
  $("#light-presets-count").textContent = ps.length ? t("light.presets.count", { n: ps.length }) : "";
  $("#light-lib-status").textContent = libBusy ? t("light.lib.loading")
    : st.cached ? t("light.lib.status", { when: new Date(st.updated).toLocaleString(LANG === "de" ? "de-DE" : "en-GB"), c: st.components, p: st.presets }) : t("light.lib.never");
  $("#light-lib-refresh").disabled = libBusy;
  const err = $("#light-lib-error");
  const warns = (st.warnings || []).length;
  err.hidden = !(libState.error || warns);
  err.textContent = libState.error ? t("light.lib.error", { e: libState.error }) : warns ? t("light.lib.warn", { n: warns }) : "";
  const on = lv && lv.enabled && lv.active;
  const active = lv ? lv.preset || "" : "";
  $("#light-presets-list").replaceChildren(...(ps.length ? ps.map((p) => {
    const name = libText(p.name);
    const fx = lightLabel("fx", p.effect, p.effect);
    const src = t("light.preset.src." + (p.source || "library"));
    const card = el("button", { type: "button", class: "preset", title: libText(p.description), "aria-label": t("light.preset.apply", { n: name }),
      "aria-pressed": String(p.id === active), onclick: () => applyPreset(p) },
      el("span", { class: "preset-sw" }, ...(p.palette || []).slice(0, 6).map((c) => el("i", { style: { background: c } }))),
      el("b", {}, name), el("small", {}, [src, fx].join(" · ")),
      (p.tags || []).length ? el("span", { class: "preset-tags" }, (p.tags || []).map((g) => el("i", {}, tagLabel(g)))) : null);
    if (!on) card.classList.add("dim");
    if (p.id === active) card.classList.add("active");
    const wrap = el("div", { class: "preset-wrap" }, card);
    if (p.source === "user") {
      wrap.append(el("button", { type: "button", class: "ghost icon-btn sm preset-del", title: t("light.preset.delete", { n: name }),
        "aria-label": t("light.preset.delete", { n: name }), onclick: () => deletePreset(p) }, "✕"));
    }
    return wrap;
  }) : [el("div", { class: "empty-note" }, t("light.presets.none"))]));
  $("#light-preset-save-btn").disabled = presetBusy || !lv;
}

async function loadLibrary() {
  try { libState = await api("GET", "/api/library"); renderLibrary(); }
  catch (e) { /* the tab works without presets */ }
}

async function refreshLibrary() {
  if (libBusy) return;
  libBusy = true; renderLibrary();
  try {
    try { libState = await api("POST", "/api/library/refresh"); }
    catch (e) {
      // A failed download still answers with the saved copy.
      libState = { ...(libState || {}), error: e.message };
      try { const c = await api("GET", "/api/library"); libState = { ...c, error: e.message }; } catch { /* keep the old */ }
    }
  } finally { libBusy = false; }
  renderLibrary();
}

async function applyPreset(p) {
  try {
    const r = await api("POST", `/api/library/presets/${encodeURIComponent(p.id)}/apply`);
    lv = r.lighting; lightSig = JSON.stringify(lv);
    renderLight(); refreshLightChip();
    const ign = (r.ignored || []).length;
    toast(t("light.preset.applied", { n: libText(p.name) }) + (ign ? " " + t("light.preset.ignored", { k: ign }) : ""));
  } catch (e) { toast(e.message, true); }
}

async function savePreset(ev) {
  ev.preventDefault();
  const input = $("#light-preset-name");
  const name = input.value.trim();
  if (!name) { input.focus(); return; }
  if (presetBusy) return;
  presetBusy = true; renderLibrary();
  try {
    const p = await api("POST", "/api/lighting/presets", { name });
    input.value = "";
    toast(t("light.preset.saved", { n: libText(p.name) }));
  } catch (e) { toast(e.message, true); }
  finally { presetBusy = false; }
  await reloadPresets();
}

async function deletePreset(p) {
  const name = libText(p.name);
  if (presetBusy) return;
  if (!(await notice({ title: "light.preset.delete.title", paras: [["light.preset.confirmDelete", { n: name }]], ok: "light.preset.delete.ok", cancel: "ack.cancel" }))) return;
  presetBusy = true;
  try { await api("DELETE", `/api/lighting/presets/${encodeURIComponent(p.id)}`); toast(t("light.preset.deleted", { n: name })); }
  catch (e) { toast(e.message, true); }
  finally { presetBusy = false; }
  await reloadPresets();
}

// reloadPresets refreshes the list and the lighting state (its active preset).
async function reloadPresets() {
  try { lv = await api("GET", "/api/lighting"); lightSig = JSON.stringify(lv); renderLight(); } catch { /* keep the old */ }
  await loadLibrary();
}

$("#light-preset-save").addEventListener("submit", savePreset);

$("#light-lib-refresh").addEventListener("click", refreshLibrary);
$("#light-presets").addEventListener("toggle", async (e) => {
  if (!e.target.open) return;
  await loadLibrary();
  if (libState && !(libState.status || {}).cached && !libState.error) refreshLibrary();
});
$('[data-tab="light"]').addEventListener("click", loadLibrary);

// ---------- Diagnostics export (Settings -> Info) ----------
let diagBusy = false;
function diagSave(blob, name) {
  const a = el("a", { href: URL.createObjectURL(blob), download: name });
  document.body.append(a);
  a.click();
  setTimeout(() => { URL.revokeObjectURL(a.href); a.remove(); }, 1000);
}
function refreshDiagTrace() {
  const box = $("#diag-trace");
  if (!box) return;
  const can = !!(lv && lv.enabled && lv.active);
  box.disabled = diagBusy || !can;
  if (!can) box.checked = false;
  box.title = can ? "" : t("diag.trace.off");
}
async function exportDiagnostics() {
  if (diagBusy) return;
  const status = $("#diag-status");
  const trace = $("#diag-trace").checked;
  diagBusy = true; $("#btn-diag").disabled = true; refreshDiagTrace();
  status.className = "diag-status";
  try {
    if (trace) {
      let r = await api("POST", "/api/diagnostics/trace");
      while (r.active) {
        status.textContent = t("diag.running", { n: r.seconds });
        await new Promise((ok) => setTimeout(ok, 1000));
        r = await api("GET", "/api/diagnostics/trace");
      }
    }
    status.textContent = "";
    const res = await api("POST", "/api/diagnostics/export" + (trace ? "?packets=1" : ""), undefined, true);
    const d = new Date(), p2 = (n) => String(n).padStart(2, "0");
    const name = `casehud-diagnostics-${d.getFullYear()}${p2(d.getMonth() + 1)}${p2(d.getDate())}-${p2(d.getHours())}${p2(d.getMinutes())}.zip`;
    diagSave(await res.blob(), name);
    status.textContent = t("diag.done", { f: name });
  } catch (e) {
    status.textContent = t("diag.err", { e: e.message });
    status.className = "diag-status warn";
  }
  diagBusy = false; $("#btn-diag").disabled = false; refreshDiagTrace();
}
$("#btn-diag").addEventListener("click", exportDiagnostics);
$("#light-enabled").addEventListener("change", async (e) => {
  const on = e.target.checked;
  if (on && !(await ensureAck("lighting"))) { e.target.checked = false; return; }
  lightPatch({ enabled: on });
});
$("#light-tl").addEventListener("change", async (e) => { await lightPatch({ tl: e.target.checked }); refresh().catch(() => {}); });
$("#light-brightness").addEventListener("input", (e) => {
  setRangeFill(e.target);
  $("#light-brightness-val").textContent = e.target.value + " %";
  lightPatchSoon({ brightness: Number(e.target.value) });
});
$("#light-exit").addEventListener("change", (e) => lightPatch({ shutdown: e.target.value }));
$("#light-effect").addEventListener("change", async (e) => {
  const name = e.target.value;
  const info = lv && lv.effects.find((x) => x.name === name);
  // Screen and audio need a one-time confirmation; declined: back to the running effect.
  if (info && info.capture && !(await ensureAck("capture"))) { e.target.value = lv.effect; return; }
  const body = { effect: name };
  // Temperature starts with the CPU temperature, 40 to 85 degrees, blue to red.
  if (name === "sensor" && info && !((lv.params || {}).sensor) && lv.effect !== "sensor") {
    const sp = info.params.find((p) => p.key === "sensor");
    if (sp && sp.default) body.params = { sensor: sp.default, min: 40, max: 85, low: "#0060ff", high: "#ff2000" };
  }
  lightPatch(body);
});
$("#light-rescan").addEventListener("click", async () => {
  try { lv = await api("POST", "/api/lighting/rescan"); renderLight(); refreshLightChip(); }
  catch (e) { toast(e.message, true); }
});
$("#light-chip").addEventListener("click", () => showTab("light"));
$("#light-lib-refresh").addEventListener("click", () => { pv.cat = null; });
$('[data-tab="light"]').addEventListener("click", loadLight);
for (const b of $$("[data-lang]")) b.addEventListener("click", () => { pv.key = ""; flow.sig = ""; renderLight(); refreshLightChip(); renderLibrary(); refreshDiagTrace(); });
$("#light-drawer").addEventListener("keydown", (e) => { if (e.key === "Escape" && !e.defaultPrevented) { e.preventDefault(); closeDrawer(); } });

// ---------- The case net with the effect ----------
// The stored sketch is drawn as the unfolded case by sketch.js (window.SketchPreview.net).
// While the tab is visible, GET /api/lighting/live?canvas=1 brings the effect picture
// (what the devices get, or a preview while lighting is off) and the LED colours, about
// 30 times a second, so motion looks smooth (a request takes a few ms). One request at a
// time, no timer otherwise.
const pv = {
  sketch: /** @type {any} */ (undefined), // undefined: not loaded, null: none stored
  net: /** @type {any} */ (null), cat: /** @type {any} */ (null), loading: false, failedAt: 0, rev: 0,
  key: "", out: /** @type {any} */ (null),
  frame: /** @type {any} */ (null), liveOk: false, fails: 0, busy: false, timer: 0, stop: false,
  png: "", pix: /** @type {any} */ (null), canvas: /** @type {any} */ (null),
  decoded: new Map(), sug: /** @type {any} */ (null), bind: /** @type {string|null} */ (null),
};
const sketching = () => document.body.classList.contains("sketching");

async function loadPreview(force) {
  if (pv.loading || (!force && pv.sketch !== undefined && pv.cat)) return;
  if (!force && Date.now() - pv.failedAt < 5000) return;
  pv.loading = true;
  try {
    const [cat, sk] = await Promise.all([force || !pv.cat ? api("GET", "/api/lighting/catalog") : pv.cat, api("GET", "/api/lighting/sketch")]);
    let doc = sk && sk.sketch ? sk.sketch : null, net = (sk && sk.net) || null;
    if (doc && doc.version !== 2) { // an older stored sketch the server did not convert
      try { const r = await api("POST", "/api/lighting/sketch/upgrade", doc); doc = (r && r.sketch) || null; net = null; } catch { doc = null; }
    }
    pv.cat = cat; pv.sketch = doc; pv.net = net; pv.rev++; pv.failedAt = 0;
  } catch (e) { pv.failedAt = Date.now(); }
  finally { pv.loading = false; }
  flow.sig = "";
  renderPreview();
  if (lv) renderFlow();
}

const zoneOf = (dev, zone) => { const d = lv && lv.devices.find((x) => x.id === dev); return { d, z: d && d.zones.find((q) => q.id === zone) }; };
const devHue = (id) => { const i = lv ? lv.devices.findIndex((d) => d.id === id) : -1; return i < 0 ? "" : LIGHT_COLORS[i % LIGHT_COLORS.length]; };

/** Colour of every LED dot: what the device gets while lighting runs, otherwise the effect at the LED's place. */
function applyColors() {
  if (!pv.out) return;
  const live = pv.liveOk && pv.frame && pv.frame.active && lv && lv.enabled;
  const px = pv.pix;
  for (const l of pv.out.leds) {
    let fill = "", op = "";
    const zones = live && l.dev && pv.frame.devices && pv.frame.devices[l.dev];
    const raw = zones && zones[l.zone];
    if (typeof raw === "string") {
      let c = pv.decoded.get(l.dev + "/" + l.zone);
      if (!c || c.raw !== raw) {
        try { const bin = atob(raw), b = new Uint8Array(bin.length); for (let k = 0; k < bin.length; k++) b[k] = bin.charCodeAt(k); c = { raw, b }; } catch { c = { raw, b: new Uint8Array(0) }; }
        pv.decoded.set(l.dev + "/" + l.zone, c);
      }
      const o = l.i * 3;
      if (o + 2 < c.b.length) { fill = `rgb(${c.b[o]},${c.b[o + 1]},${c.b[o + 2]})`; op = "1"; }
    }
    // the orientation check: the zone shows the pattern, the rest of it is dark
    const op_ = orient.cur, pat = op_ && !op_.expired && op_.pat && !op_.pat.parallel ? op_.pat : null;
    if (pat && l.dev === pat.device && l.zone === pat.zone) {
      const k = l.i - pat.start;
      fill = k >= 0 && k < pat.colors.length ? pat.colors[k] : "#000000"; op = "1";
    }
    // a part in parallel shows the colour of the LED it shares with the part before it (su, sv)
    const u = l.su ?? l.u, v = l.sv ?? l.v;
    if (!fill && px && u >= 0 && u <= 1 && v >= 0 && v <= 1) {
      const x = Math.min(px.w - 1, Math.floor(u * px.w)), y = Math.min(px.h - 1, Math.floor(v * px.h)), o = (y * px.w + x) * 4;
      fill = `rgb(${px.d[o]},${px.d[o + 1]},${px.d[o + 2]})`;
      op = l.dev ? "1" : ".7";
    }
    if (!fill) { fill = (l.dev && devHue(l.dev)) || "var(--faint-deco)"; op = l.dev ? ".38" : ".3"; }
    if (l.node._f !== fill) { l.node.style.fill = fill; l.node._f = fill; }
    if (l.node._o !== op) { l.node.style.opacity = op; l.node._o = op; }
  }
}

/** Shows a new effect picture: under the net, in the source node and as the sampling grid of the dots. */
function showPicture(c) {
  const png = (c && c.png) || "";
  const note = c ? c.note || "" : "";
  const nb = $("#light-note");
  nb.hidden = !(note === "capture" || note === "error");
  if (!nb.hidden) nb.textContent = t(note === "capture" ? "lh.note.capture" : "lh.note.error");
  if (png === pv.png) return;
  pv.png = png;
  const url = png ? "data:image/png;base64," + png : "";
  if (pv.out && pv.out.img) { if (url) pv.out.img.setAttribute("href", url); else pv.out.img.removeAttribute("href"); }
  const sw = $("#light-flow .lf-swatch");
  if (sw) sw.src = url;
  if (!url) { pv.pix = null; applyColors(); return; }
  const im = new Image();
  im.onload = () => {
    if (pv.png !== png) return;
    const cv = pv.canvas || (pv.canvas = document.createElement("canvas"));
    cv.width = im.naturalWidth; cv.height = im.naturalHeight;
    const g = cv.getContext("2d", { willReadFrequently: true });
    g.drawImage(im, 0, 0);
    pv.pix = { w: cv.width, h: cv.height, d: g.getImageData(0, 0, cv.width, cv.height).data };
    applyColors();
  };
  im.src = url;
}

function liveBadge() {
  const b = $("#light-live");
  if (!b || !lv) return;
  const c = pv.frame && pv.frame.canvas;
  let key = "", cls = "lh-live", tip = "";
  if (!pv.out) key = "";
  else if (pv.stop || (pv.fails && !pv.liveOk)) key = "lost";
  else if (lv.enabled && lv.paused) key = "paused";
  else if (c && c.note === "capture") key = "capture";
  else if (c && c.source === "engine") { key = "live"; cls += " on"; tip = t("lh.live.tip"); }
  else if (c) { key = "preview"; cls += " pre"; tip = t("lh.live.preview.tip"); }
  b.hidden = !key;
  b.className = cls;
  b.title = tip;
  b.textContent = key ? t("lh.live" + (key === "live" ? "" : "." + key)) : "";
}

const liveWanted = () => !!(lv && pv.out && lightActive() && !document.hidden && !sketching() && !pv.stop);
function liveKick() {
  if (pv.timer || pv.busy) return;
  if (liveWanted()) pv.timer = setTimeout(liveTick, 0);
}
function liveHalt() { clearTimeout(pv.timer); pv.timer = 0; }
async function liveTick() {
  pv.timer = 0;
  if (!liveWanted()) { liveBadge(); return; }
  pv.busy = true;
  const t0 = performance.now();
  let wait = 33;
  try {
    pv.frame = await api("GET", "/api/lighting/live?canvas=1");
    pv.liveOk = true; pv.fails = 0;
  } catch (e) {
    pv.liveOk = false; pv.frame = null;
    // 409 area_off: the area is switched off; stay quiet until the tab is opened again
    if (e && (e.status === 409 || /^area_off/.test(e.raw || ""))) pv.stop = true;
    else if (++pv.fails >= 3) pv.stop = true;
    else wait = 1000 * pv.fails;
  }
  showPicture(pv.frame && pv.frame.canvas);
  applyColors();
  pv.busy = false;
  liveBadge();
  if (liveWanted()) pv.timer = setTimeout(liveTick, Math.max(0, wait - (performance.now() - t0)));
}
function liveReset() { pv.stop = false; pv.fails = 0; liveKick(); }

function previewDoc() {
  if (pv.sketch) return pv.sketch;
  if (!window.SketchPreview || !pv.cat) return null;
  const d = window.SketchPreview.emptyDoc(), k = pv.cat.cases && pv.cat.cases["custom-box"];
  if (k && k.dims) d.case.dims = { w: k.dims.w, h: k.dims.h, d: k.dims.d };
  return d;
}

function previewIcon(d) {
  const ns = "http://www.w3.org/2000/svg", s = document.createElementNS(ns, "svg"), p = document.createElementNS(ns, "path");
  for (const [k, v] of Object.entries({ viewBox: "0 0 24 24", width: 26, height: 26, fill: "none", stroke: "currentColor", "stroke-width": 1.6, "stroke-linecap": "round", "stroke-linejoin": "round" })) s.setAttribute(k, String(v));
  p.setAttribute("d", d); s.append(p);
  return s;
}
function renderEmpty() {
  const box = $("#light-empty");
  box.hidden = !!pv.sketch || pv.sketch === undefined;
  if (box.hidden) return;
  const on = !!(lv.enabled && lv.active), can = on && lv.devices.length > 0;
  const sig = [LANG, on, can].join();
  if (box.dataset.sig === sig) return;
  box.dataset.sig = sig;
  box.replaceChildren(
    el("div", { class: "lh-empty-icon", "aria-hidden": "true" }, previewIcon("M3 7h18v10H3zM7 7v10M12 12h5")),
    el("b", {}, t("lh.empty.title")), el("p", {}, t("lh.empty.text")),
    el("div", { class: "lh-empty-btns" },
      el("button", { class: "primary", type: "button", disabled: !can, onclick: openSuggest }, t("lh.empty.adopt")),
      el("button", { class: "ghost", type: "button", onclick: () => window.SketchEditor && window.SketchEditor.open() }, t("lh.empty.draw"))),
    ...(can ? [] : [el("small", {}, on ? t("light.devices.none") : t("lh.empty.need"))]));
}

function renderEntry() {
  const sk = pv.sketch, n = sk ? sk.parts.length : 0, b = sk ? sk.parts.filter((p) => p.bind).length : 0;
  $("#sketch-entry-info").textContent = !sk ? t("sk.entry.none") : n === 1 ? t(b ? "sk.entry.some1" : "sk.entry.some0") : t("sk.entry.some", { n, b });
  $("#sketch-open").textContent = sk ? t("sk.open.edit") : t("sk.open");
  $(".lh-legend").hidden = !sk;
  const loose = sk ? sk.parts.filter((p) => !p.bind && !p.hidden).length : 0, chip = $("#light-unbound");
  chip.hidden = !loose;
  chip.textContent = loose === 1 ? t("lh.unbound.chip1") : t("lh.unbound.chip", { n: loose });
  chip.title = t("lh.unbound.title");
  const ab = $("#light-auto");
  ab.hidden = !loose || !(lv && lv.enabled && lv.active && lv.devices.length);
  ab.disabled = autoBind.busy || !!autoBind.find;
  maybeAutoBind();
}

function caseTitle(doc) {
  const c = doc.case, k = c.source === "library" && pv.cat && pv.cat.cases && pv.cat.cases[c.id];
  return k ? (k.vendor && !String(k.name || "").toLowerCase().startsWith(String(k.vendor).toLowerCase()) ? k.vendor + " " : "") + (k.name || k.id) : t("lh.case.custom");
}

/** Single-column layouts: the stage is as tall as the net needs at the full width, so it fills the card. */
function fitStage(w) {
  const st = $("#light-stage"), narrow = window.matchMedia("(max-width: 1100px)").matches, net = pv.out && pv.out.net;
  let h = "";
  if (narrow && net && net.w > 0 && net.h > 0) h = Math.round(Math.max(260, Math.min(680, (w - 52) * net.h / net.w + 52))) + "px";
  if (st.style.height === h) return false;
  st.style.height = h;
  return true;
}
function renderPreview(force) {
  if (!lv || !$("#light-hero")) return;
  if (!lightActive() || sketching()) { liveHalt(); return; }
  if (pv.sketch === undefined || !pv.cat) { loadPreview(); return; }
  const doc = previewDoc(), r = $("#light-stage").getBoundingClientRect();
  if (!doc || !window.SketchPreview || r.width < 40) return;
  if (fitStage(r.width) && !pv.fit) { pv.fit = true; try { return renderPreview(true); } finally { pv.fit = false; } }
  const w = Math.floor(r.width), h = Math.floor(r.height);
  const key = [pv.rev, w, h, LANG, !!pv.sketch].join("|");
  if (force || key !== pv.key) {
    pv.key = key;
    pv.out = window.SketchPreview.net(doc, pv.cat, pv.sketch ? pv.net : null, { w, h, label: t("lh.stage"), interactive: !!pv.sketch });
    pv.out.svg.classList.toggle("ghost", !pv.sketch);
    if (pv.png) pv.out.img.setAttribute("href", "data:image/png;base64," + pv.png);
    $("#light-pv").replaceChildren(pv.out.svg);
    $("#light-tip").hidden = true;
    if (pv.bind && !pv.out.info.has(pv.bind)) closeBind();
    const dm = doc.case.dims;
    $("#light-caption").textContent = t("lh.case", { c: caseTitle(doc), w: Math.round(dm.w), h: Math.round(dm.h), d: Math.round(dm.d) });
  }
  renderEntry();
  renderEmpty();
  if (pv.bind) renderBind();
  if (autoBind.find) renderFind();
  if (orient.cur) renderOrient();
  applyColors();
  liveBadge();
  liveKick();
}

// Hover: part name, face, binding and LED count.
function previewTip(ev) {
  const tip = $("#light-tip"), g = ev.target.closest && ev.target.closest(".pv-part"), info = g && pv.out && pv.out.info.get(g.dataset.id);
  if (!info || pv.bind) { tip.hidden = true; return; }
  const { d, z } = info.device ? zoneOf(info.device, info.zone) : {};
  const bind = info.device ? [d ? d.name : info.device, z ? z.label || z.name : info.zone].join(" · ") : t("lh.tip.unbound");
  const sig = [info.title, bind, info.n, info.face, LANG].join("|");
  if (tip.dataset.sig !== sig) {
    tip.dataset.sig = sig;
    tip.replaceChildren(el("b", {}, info.title), el("span", { class: info.device ? "" : "warn" }, bind),
      el("small", {}, t("sk.face." + info.face) + " · " + (info.n === 1 ? t("lh.tip.led1") : t("lh.tip.leds", { n: info.n }))), el("small", { class: "lh-tip-hint" }, t("lh.tip.hint")));
  }
  tip.hidden = false;
  const box = $("#light-stage").getBoundingClientRect();
  const x = ev.clientX - box.left + 14, y = ev.clientY - box.top + 14;
  tip.style.left = Math.max(8, Math.min(x, box.width - tip.offsetWidth - 8)) + "px";
  tip.style.top = Math.max(8, Math.min(y, box.height - tip.offsetHeight - 8)) + "px";
}

// ---- connecting a part to a zone, right on the net
function openBind(id) {
  if (!pv.sketch || !pv.out || !pv.out.info.has(id)) return;
  pv.bind = id;
  $("#light-tip").hidden = true;
  for (const g of $$("#light-pv .pv-part")) g.classList.toggle("picked", g.dataset.id === id);
  renderBind(true);
}
function closeBind(refocus) {
  const id = pv.bind;
  pv.bind = null;
  $("#light-bind").hidden = true;
  for (const g of $$("#light-pv .pv-part.picked")) g.classList.remove("picked");
  if (refocus && id) { const g = $(`#light-pv .pv-part[data-id="${CSS.escape(id)}"]`); if (g) g.focus({ preventScroll: true }); }
}
function renderBind(focusIt) {
  const box = $("#light-bind"), id = pv.bind, info = id && pv.out && pv.out.info.get(id), pl = id && pv.sketch && pv.sketch.parts.find((p) => p.id === id);
  if (!info || !pl) { closeBind(); return; }
  const sel = $("select", box);
  if (!focusIt && sel && document.activeElement === sel) return;
  const cur = pl.bind ? pl.bind.device + "\u0000" + pl.bind.zone : "";
  const devs = (lv && lv.devices) || [], zoneCount = devs.reduce((n, d) => n + d.zones.length, 0);
  const opts = [el("option", { value: "", selected: !cur }, t("lh.bind.none"))];
  let known = !cur;
  // LEDs the other parts already take on each zone: a zone too short for this part is listed but not offered
  const used = new Map();
  for (const [pid, q] of pv.out.info) if (pid !== id && q.device) { const k = q.device + "\u0000" + q.zone; used.set(k, (used.get(k) || 0) + (q.take ?? q.n)); }
  // a port of the fan controller takes one LED per fan ring
  const need = (d, z) => (window.SketchPreview.ringZone(d.id, z.id) ? info.r ?? info.n : info.n);
  for (const d of devs) {
    if (!d.zones.length) continue;
    opts.push(el("optgroup", { label: d.name }, d.zones.map((z) => {
      const v = d.id + "\u0000" + z.id, full = v !== cur && z.max > 0 && (used.get(v) || 0) + need(d, z) > z.max;
      if (v === cur) known = true;
      return el("option", { value: v, selected: v === cur, disabled: full },
        (z.label || z.name) + " · " + (full ? t("lh.bind.full", { m: z.max }) : z.leds === 1 ? t("light.zone.led1") : t("light.zone.leds", { n: z.leds })));
    })));
  }
  if (!known) opts.push(el("option", { value: cur, selected: true }, pl.bind.device + " · " + pl.bind.zone));
  const sid = "light-bind-sel";
  box.replaceChildren(
    el("div", { class: "lh-bind-head" }, el("b", {}, info.title), el("small", {}, t("sk.face." + info.face) + " · " + (info.n === 1 ? t("lh.tip.led1") : t("lh.tip.leds", { n: info.n })))),
    el("label", { class: "lh-bind-field", for: sid }, t("lh.bind")),
    el("select", { id: sid, disabled: !zoneCount && !cur, onchange: (e) => bindPart(id, e.target.value) }, opts),
    ...(!zoneCount ? [el("small", { class: "sub" }, lv && lv.enabled && lv.active ? t("lh.bind.nozones") : t("lh.bind.need"))] : []),
    ...parallelNote(pl),
    el("button", { class: "ghost lh-bind-orient", type: "button", disabled: !pl.bind, title: pl.bind ? "" : t("lh.or.need"), onclick: () => startOrient(id) }, t("lh.or")),
    el("div", { class: "lh-bind-actions" },
      el("button", { class: "ghost", type: "button", onclick: () => { closeBind(); window.SketchEditor && window.SketchEditor.open(id); } }, t("lh.bind.edit")),
      el("button", { class: "ghost", type: "button", onclick: () => closeBind(true) }, t("lh.bind.close"))));
  box.hidden = false;
  // next to the part, inside the stage
  const st = $("#light-stage").getBoundingClientRect(), g = $(`#light-pv .pv-part[data-id="${CSS.escape(id)}"]`);
  const r = g ? g.getBoundingClientRect() : st;
  const bw = box.offsetWidth, bh = box.offsetHeight;
  let x = r.right - st.left + 10, y = r.top - st.top;
  if (x + bw > st.width - 8) x = r.left - st.left - bw - 10;
  if (x < 8) x = Math.max(8, Math.min(st.width - bw - 8, r.left - st.left));
  box.style.left = x + "px";
  box.style.top = Math.max(8, Math.min(y, st.height - bh - 8)) + "px";
  if (focusIt) $("select", box).focus({ preventScroll: true });
}
/** True while a fan controller follows a mainboard header (sync mode): every port shows that header's chain. */
const syncOn = () => !!(lv && lv.devices.some((d) => d.sync && d.settingValues && d.settingValues.mode === "motherboard"));
/** The chain of a part's zone in wire order. */
function chainOf(b) {
  return pv.sketch.parts.map((p, idx) => ({ p, idx })).filter((q) => q.p.bind && q.p.bind.device === b.device && q.p.bind.zone === b.zone)
    .sort((x, y) => x.p.bind.order - y.p.bind.order || x.idx - y.idx).map((q) => q.p);
}
/** A part wired in parallel shows the colours of the part it follows; in sync mode that is how every port works. */
function parallelNote(pl) {
  const b = pl.bind;
  if (!b || !pv.sketch) return [];
  const chain = chainOf(b), at = chain.findIndex((p) => p.id === pl.id), out = [];
  if (b.parallel && at > 0) {
    let k = at - 1;
    while (k > 0 && chain[k].bind.parallel) k--;
    out.push(el("small", { class: "sub lh-bind-par" }, t("lh.bind.parallel", { p: partLabel(chain[k].id) })));
  }
  if (syncOn() && chain.some((p) => p.bind.parallel)) out.push(el("small", { class: "sub lh-bind-sync" }, t("lh.bind.sync")));
  return out;
}
// ---- "Check orientation": a still pattern on one bound part (POST /api/lighting/sketch/pattern, at
// most 60 s, nothing stored on the device); what the user reports corrects the part's spin, mirror or
// direction in the sketch (POST /api/lighting/sketch/orient) and the pattern is shown again.
const ORIENT_COLORS = ["#ff0000", "#00c800", "#0050ff", "#ffc800"]; // top, right, bottom, left (as internal/sketch)
const orient = { cur: /** @type {any} */ (null), timer: 0 };
const orientFixes = (kind) => (kind === "strip" ? ["reverse"] : kind === "rings" ? ["rot180"] : ["rot90", "rot180", "rot270", "mirror"]);
const orientFixLabel = (kind, f) => t(kind === "rings" ? "lh.or.flip" : "lh.or." + f);
function orientErr(e) { return /nomatch/.test(e.message) ? t("lh.or.nomatch") : t("lh.or.err", { e: e.message }); }
/** The parts a check can correct: bound and not in parallel, in chain order. */
function orientQueue(binds) {
  const ids = [...binds].sort((a, b) => a.order - b.order).filter((b) => !b.parallel).map((b) => b.part);
  return ids.filter((id) => pv.sketch && pv.sketch.parts.some((p) => p.id === id && p.bind));
}
async function startOrient(id, queue) {
  // coming from the editor the preview reloads the sketch: wait for it
  if (!pv.sketch && !pv.loading) await loadPreview(true);
  for (let k = 0; k < 50 && pv.loading; k++) await new Promise((r) => setTimeout(r, 100));
  const pl = pv.sketch && pv.sketch.parts.find((p) => p.id === id);
  if (!pl || !pl.bind) { toast(t("lh.or.need"), true); return; }
  closeBind();
  if (autoBind.find) endFind();
  const q = queue && queue.includes(id) ? queue : [id];
  orient.cur = { id, queue: q, i: q.indexOf(id), pat: null, err: "", note: "", busy: true, expired: false };
  renderOrient(true);
  await showOrient();
}
async function showOrient() {
  const o = orient.cur;
  if (!o) return;
  o.busy = true; o.err = ""; o.expired = false;
  renderOrient();
  try {
    const r = await api("POST", "/api/lighting/sketch/pattern", { part: o.id });
    if (orient.cur === o) { o.pat = r.pattern; armOrient(); }
    if (r.lighting) { lv = r.lighting; lightSig = JSON.stringify(lv); }
  } catch (e) { if (orient.cur === o) o.err = orientErr(e); }
  if (orient.cur !== o) return;
  o.busy = false;
  renderOrient(true); applyColors();
}
function armOrient() {
  clearTimeout(orient.timer);
  orient.timer = setTimeout(() => {
    const o = orient.cur;
    if (o && o.pat && !o.pat.parallel) { o.expired = true; renderOrient(); applyColors(); }
  }, 60000);
}
async function orientFix(fix) {
  const o = orient.cur;
  if (!o || o.busy) return;
  o.busy = true; o.err = "";
  renderOrient();
  try {
    const r = await api("POST", "/api/lighting/sketch/orient", { part: o.id, fix });
    if (r.sketch) { pv.sketch = r.sketch; pv.net = r.net || null; pv.rev++; }
    if (r.lighting) { lv = r.lighting; lightSig = JSON.stringify(lv); }
    if (orient.cur === o) { o.pat = r.pattern; o.note = t("lh.or.fixed", { f: orientFixLabel(o.pat.kind, fix) }); o.expired = false; armOrient(); }
  } catch (e) { if (orient.cur === o) o.err = orientErr(e); }
  if (orient.cur === o) o.busy = false;
  flow.sig = "";
  renderLight();
  renderOrient(true);
}
async function orientOk() {
  const o = orient.cur;
  if (!o || o.busy) return;
  if (o.pat && !o.pat.parallel) toast(t("lh.or.done", { p: partLabel(o.id) }));
  if (o.i + 1 < o.queue.length) {
    o.i++; o.id = o.queue[o.i]; o.pat = null; o.note = "";
    renderOrient(true); applyColors();
    await showOrient();
    return;
  }
  endOrient();
}
function endOrient() {
  const was = orient.cur;
  orient.cur = null;
  clearTimeout(orient.timer);
  renderOrient(); applyColors();
  if (was && was.pat && !was.pat.parallel) api("DELETE", "/api/lighting/calibrate").then(() => loadLight()).catch(() => { /* ends by itself */ });
}
/** The colours of the pattern next to what they mean. */
function orientLegend(kind) {
  const sw = (c, txt) => el("span", { class: "lh-or-sw" }, el("i", { style: "background:" + c }), txt);
  if (kind === "strip") return el("div", { class: "lh-or-legend" }, sw("#00ff00", t("lh.or.first")), el("i", { class: "lh-or-grad" }), sw("#ff0000", t("lh.or.last")));
  if (kind === "rings") return el("div", { class: "lh-or-legend" }, sw(ORIENT_COLORS[0], t("lh.or.top")), sw(ORIENT_COLORS[2], t("lh.or.bottom")));
  return el("div", { class: "lh-or-legend" }, ...["top", "right", "bottom", "left"].map((k, i) => sw(ORIENT_COLORS[i], t("lh.or." + k))), sw("#ffffff", t("lh.or.first")));
}
function renderOrient(focusIt) {
  const box = $("#light-orient"), o = orient.cur;
  $("#light-stage").classList.toggle("orienting", !!o);
  for (const g of $$("#light-pv .pv-part")) g.classList.toggle("orient", !!o && g.dataset.id === o.id);
  if (!o) { box.hidden = true; box.replaceChildren(); return; }
  const p = o.pat, more = o.i + 1 < o.queue.length;
  const btn = (txt, fn, primary) => el("button", { class: primary ? "primary" : "ghost", type: "button", disabled: o.busy, onclick: fn }, txt);
  const kids = [el("div", { class: "lh-bind-head" },
    el("small", {}, o.queue.length > 1 ? t("lh.or") + " · " + t("lh.or.step", { i: o.i + 1, n: o.queue.length }) : t("lh.or")),
    el("b", {}, partLabel(o.id)))];
  if (o.err) kids.push(el("p", { class: "sub lh-or-err", role: "alert" }, o.err));
  if (p && p.parallel) {
    kids.push(el("p", { class: "sub" }, t("lh.or.parallel", { p: partLabel(p.model) })));
    if (syncOn()) kids.push(el("p", { class: "sub" }, t("lh.or.alt")));
    kids.push(el("div", { class: "lh-bind-actions" }, more ? btn(t("lh.or.skip"), orientOk, true) : el("span"), btn(t("lh.bind.close"), endOrient)));
  } else if (p && o.expired) {
    kids.push(el("p", { class: "sub" }, t("lh.or.expired")),
      el("div", { class: "lh-bind-actions" }, btn(t("lh.or.again"), showOrient, true), btn(t("lh.or.cancel"), endOrient)));
  } else if (p) {
    kids.push(el("p", { class: "sub" }, t("lh.or." + p.kind)), orientLegend(p.kind));
    if (o.note) kids.push(el("p", { class: "sub lh-or-note", role: "status" }, o.note));
    kids.push(el("b", { class: "lh-or-ask" }, t("lh.or.ask")),
      el("div", { class: "lh-or-fixes" }, btn(more ? t("lh.or.next") : t("lh.or.ok"), orientOk, true),
        ...orientFixes(p.kind).map((f) => el("button", { class: "ghost", type: "button", "data-fix": f, disabled: o.busy, onclick: () => orientFix(f) }, orientFixLabel(p.kind, f)))));
    if (p.kind === "fan") kids.push(el("small", { class: "sub" }, t("lh.or.turn")));
    kids.push(el("small", { class: "sub" }, t("lh.or.limit")),
      el("div", { class: "lh-bind-actions" }, el("span"), btn(t("lh.or.cancel"), endOrient)));
  } else if (o.err) {
    kids.push(el("div", { class: "lh-bind-actions" }, btn(t("lh.or.again"), showOrient, true), btn(t("lh.bind.close"), endOrient)));
  } else kids.push(el("p", { class: "sub" }, "…"));
  box.replaceChildren(...kids);
  box.hidden = false;
  if (focusIt) { const b = $("button.primary:not([disabled])", box) || $("button:not([disabled])", box); if (b) b.focus({ preventScroll: true }); }
}
window.LightOrient = (id) => startOrient(id);
async function bindPart(id, value) {
  const doc = pv.sketch && JSON.parse(JSON.stringify(pv.sketch)), pl = doc && doc.parts.find((p) => p.id === id);
  if (!pl) return;
  const title = window.SketchPreview.title(pv.cat, pl);
  let label = "";
  if (!value) delete pl.bind;
  else {
    const [device, zone] = value.split("\u0000");
    const order = Math.max(-1, ...doc.parts.filter((p) => p.bind && p.bind.device === device && p.bind.zone === zone && p.id !== id).map((p) => p.bind.order || 0)) + 1;
    pl.bind = { device, zone, order, parallel: false };
    const { d, z } = zoneOf(device, zone);
    label = [d ? d.name : device, z ? z.label || z.name : zone].join(" · ");
  }
  try {
    await putSketch(doc);
    toast(value ? t("lh.bind.done", { p: title, z: label }) : t("lh.bind.cleared", { p: title }));
  } catch (e) { toast(t("lh.bind.err", { e: e.message }), true); }
  flow.sig = "";
  renderLight(); refreshLightChip();
  if (pv.bind === id) openBind(id);
}

async function putSketch(next) {
  const r = await api("PUT", "/api/lighting/sketch", next);
  pv.sketch = r.sketch || next; pv.net = r.net || null; pv.rev++;
  if (r.lighting) { lv = r.lighting; lightSig = JSON.stringify(lv); }
  return r;
}

// ---- "Assign automatically": parts whose zone is certain are bound by the server (POST
// /api/lighting/sketch/autobind); the summary can undo it. The rest is found by lighting one free
// header at a time (POST /api/lighting/identify, the TL blink for fan ports) and clicking what lights up.
const autoBind = {
  busy: false,
  // parts the automatic run has already looked at with this set of devices: a part the user
  // unbinds later is not bound again on its own; new parts and new devices are looked at
  known: { dev: "", parts: /** @type {string[]} */ ([]) },
  sum: /** @type {any} */ (null), // {binds, notes, open, zones, prev, kind}
  find: /** @type {any} */ (null), // {zones, i, picked: [id], binds, prev, setup}
};
try {
  const k = JSON.parse(localStorage.getItem("casehud.autobind") || "null");
  if (k && typeof k.dev === "string" && Array.isArray(k.parts)) autoBind.known = { dev: k.dev, parts: k.parts.filter((x) => typeof x === "string").slice(0, 600) };
} catch { /* storage blocked */ }
const devName = (id) => { const d = lv && lv.devices.find((x) => x.id === id); return d ? d.name : id; };
const zoneLabel = (dev, zone) => { const { d, z } = zoneOf(dev, zone); return [d ? d.name : dev, z ? z.label || z.name : zone].join(" · "); };
const partLabel = (id) => { const p = pv.sketch && pv.sketch.parts.find((q) => q.id === id); return p ? window.SketchPreview.title(pv.cat, p) : id; };
/** Version of the assignment rules (sketch.AutoBind on the server): raise it when they learn new
 * cases, so parts that were looked at under older rules are looked at once more. */
const AUTO_RULES = 2;
/** The found devices and their zones: the automatic run looks at every part again when they or the rules change. */
const autoDevs = () => (lv ? "r" + AUTO_RULES + "|" + lv.devices.map((d) => d.id + ":" + d.zones.map((z) => z.id).join(",")).sort().join("|") : "");
function autoKnow(ids) {
  autoBind.known.parts = [...new Set([...autoBind.known.parts, ...ids])].slice(-600);
  try { localStorage.setItem("casehud.autobind", JSON.stringify(autoBind.known)); } catch { /* storage blocked */ }
}
/** On opening the page: bind what is certain among the parts not looked at yet with these devices. */
function maybeAutoBind() {
  if (autoBind.busy || autoBind.find || sketching() || !lightActive() || !pv.sketch || pv.bind) return;
  if (!lv || !lv.enabled || !lv.active || !lv.devices.length) return;
  const dev = autoDevs();
  if (autoBind.known.dev !== dev) autoBind.known = { dev, parts: [] };
  const fresh = pv.sketch.parts.filter((p) => !p.bind && !autoBind.known.parts.includes(p.id)).map((p) => p.id);
  if (!fresh.length) return;
  autoKnow(pv.sketch.parts.map((p) => p.id));
  runAutoBind(true, fresh);
}
async function runAutoBind(quiet, parts, sync) {
  if (autoBind.busy || !pv.sketch) return;
  autoBind.busy = true;
  const prev = JSON.parse(JSON.stringify(pv.sketch));
  try {
    const body = parts || sync ? { ...(parts ? { parts } : {}), ...(sync ? { sync } : {}) } : undefined;
    const r = await api("POST", "/api/lighting/sketch/autobind", body);
    if (pv.sketch) autoKnow(pv.sketch.parts.map((p) => p.id));
    if (r.sketch) { pv.sketch = r.sketch; pv.net = r.net || null; pv.rev++; }
    if (r.lighting) { lv = r.lighting; lightSig = JSON.stringify(lv); }
    // a sync header that only fits: asked, never taken on its own
    const ask = r.sync && r.sync.derived && (r.sync.proposed || []).length > 0;
    if (!(r.binds || []).length && !ask && quiet) return; // nothing certain: no summary on its own
    autoBind.sum = { binds: r.binds || [], notes: r.notes || [], open: r.open || [], zones: r.zones || [], sync: r.sync || null, prev: (r.binds || []).length ? prev : null, kind: "auto" };
    flow.sig = "";
    renderLight(); refreshLightChip();
    renderAutoSum(true);
    const n = autoBind.sum.binds.length;
    // on opening the page the summary may sit below the stage: a short note points to it
    if (quiet && n) toast(n === 1 ? t("lh.auto.done1") : t("lh.auto.done", { n }));
    else $("#light-auto-sum").scrollIntoView({ block: "nearest" });
  } catch (e) { if (!quiet) toast(t("lh.auto.err", { e: e.message }), true); }
  finally { autoBind.busy = false; }
}
async function undoAutoBind() {
  const s = autoBind.sum;
  if (!s || !s.prev) return;
  try {
    await putSketch(s.prev); // the parts stay looked at: not bound again on their own
    autoBind.sum = null;
    renderAutoSum();
    toast(t("lh.auto.undone"));
    flow.sig = "";
    renderLight(); refreshLightChip();
  } catch (e) { toast(t("lh.bind.err", { e: e.message }), true); }
}
const AUTO_NOTES = {
  "tl-same": () => t("lh.auto.n.tlsame"), "tl-count": () => t("lh.auto.n.tlcount"), "tl-devices": () => t("lh.auto.n.tldevs"),
  "ram-count": (n) => t("lh.auto.n.ram", { n: n.n, m: n.m }), "board-count": (n) => t("lh.auto.n.board", { n: n.n, m: n.m }),
  leds: (n) => t("lh.auto.n.leds", { p: partLabel((n.parts || [])[0]), n: n.n, z: zoneLabel(n.device, n.zone), m: n.m }),
  "leds-max": (n) => t("lh.auto.n.ledsmax", { p: partLabel((n.parts || [])[0]), n: n.n, z: zoneLabel(n.device, n.zone), m: n.m }),
  "sync-source": (n) => t("lh.auto.n.syncsrc", { n: n.n }), "sync-used": (n) => t("lh.auto.n.syncused", { z: zoneLabel(n.device, n.zone) }),
};
/** Devices with bound parts whose colours do not come from the CaseHUD effect. */
function autoSources(binds) {
  const out = [];
  for (const id of new Set(binds.map((b) => b.device))) {
    const d = lv && lv.devices.find((x) => x.id === id);
    if (d && sourceOf(d) !== "effect" && sourcesOf(d).includes("effect")) out.push(d);
  }
  return out;
}
function renderAutoSum(focusIt) {
  const box = $("#light-auto-sum"), s = autoBind.sum;
  if (!s) { box.hidden = true; box.replaceChildren(); return; }
  const groups = new Map();
  for (const b of [...s.binds].sort((a, c) => a.order - c.order)) { const k = b.device + "\u0000" + b.zone; (groups.get(k) || groups.set(k, []).get(k)).push(b); }
  const n = s.binds.length, open = s.open.filter((id) => pv.sketch && pv.sketch.parts.some((p) => p.id === id && !p.bind));
  const kids = [
    el("div", { class: "lh-auto-head" },
      el("b", {}, n ? (n === 1 ? t(s.kind === "find" ? "lh.find.done1" : "lh.auto.done1") : t(s.kind === "find" ? "lh.find.done" : "lh.auto.done", { n })) : t("lh.auto.none")),
      el("button", { class: "icon-btn sm lh-auto-x", type: "button", "aria-label": t("lh.bind.close"), title: t("lh.bind.close"), onclick: () => { autoBind.sum = null; renderAutoSum(); } }, "✕")),
  ];
  // a chain with parts in parallel (sync header): the chain in order, the parallel parts after it
  const chainText = (bs) => {
    const par = bs.filter((b) => b.parallel).map((b) => partLabel(b.part));
    return bs.filter((b) => !b.parallel).map((b) => partLabel(b.part)).join(" → ") + (par.length ? " · " + t("lh.auto.parallel", { p: par.join(", ") }) : "");
  };
  if (n) kids.push(el("ul", { class: "lh-auto-list" }, [...groups].map(([k, bs]) => {
    const [dev, zone] = k.split("\u0000");
    return el("li", {}, el("span", { class: "lh-auto-zone" }, zoneLabel(dev, zone)), el("span", {}, chainText(bs)));
  })));
  if (s.binds.some((b) => /^port\d+$/.test(b.zone) || b.kind === "sync")) kids.push(el("small", { class: "sub" }, t("lh.auto.order")));
  const sy = s.sync;
  if (sy && !sy.derived && s.binds.some((b) => b.kind === "sync")) kids.push(el("small", { class: "sub lh-auto-sync" }, t("lh.auto.sync")));
  if (sy && sy.derived && (sy.proposed || []).length && pv.sketch && sy.proposed.some((b) => pv.sketch.parts.some((p) => p.id === b.part && !p.bind))) {
    kids.push(el("div", { class: "lh-auto-ask" },
      el("p", {}, t("lh.auto.sync.ask", { z: zoneLabel(sy.board, sy.zone), n: sy.fans * 26 })),
      el("small", { class: "sub" }, chainText(sy.proposed)),
      el("small", { class: "sub" }, t("lh.auto.sync")),
      el("button", { class: "primary", type: "button", onclick: () => runAutoBind(false, null, sy.board + "/" + sy.zone) }, t("lh.auto.sync.take"))));
  }
  for (const nt of s.notes) { const f = AUTO_NOTES[nt.code]; if (f) kids.push(el("p", { class: "lh-auto-note" }, f(nt))); }
  for (const d of autoSources(s.binds)) {
    kids.push(el("div", { class: "lh-auto-src" },
      el("p", {}, t("lh.auto.src", { d: d.name, s: t("lf.src." + sourceOf(d)) })),
      el("button", { class: "ghost", type: "button", onclick: async () => { await setSource(d, "effect"); renderAutoSum(); } }, t("lh.auto.src.switch"))));
  }
  const acts = [];
  if (open.length) {
    kids.push(el("p", { class: "sub" }, open.length === 1 ? t("lh.auto.open1") : t("lh.auto.open", { n: open.length })));
    if ((s.zones || []).length) acts.push(el("button", { class: "primary", type: "button", onclick: () => startFind(s.zones) }, t("lh.find.start")));
  }
  const oq = orientQueue(s.binds);
  if (oq.length) acts.push(el("button", { class: open.length ? "ghost" : "primary", type: "button", "data-step": "orient", onclick: () => startOrient(oq[0], oq) }, t("lh.auto.orient")));
  if (s.prev) acts.push(el("button", { class: "ghost", type: "button", onclick: undoAutoBind }, t("lh.auto.undo")));
  if (acts.length) kids.push(el("div", { class: "lh-auto-acts" }, acts));
  box.replaceChildren(...kids);
  box.hidden = false;
  if (focusIt) box.focus({ preventScroll: true });
}

// ---- the guided search: one free zone lights up at a time, the user clicks what lights up
async function findLight() {
  const f = autoBind.find, z = f && f.zones[f.i];
  if (!z) return;
  const d = lv && lv.devices.find((x) => x.id === z.device);
  f.err = "";
  try {
    // fan ports while the controller follows the mainboard: only its own blink reaches the fans
    if (z.port && d && sourceOf(d) !== "effect") {
      try { await api("POST", "/api/lighting/blink?port=" + z.port); return; } catch { /* fall back to the zone */ }
    }
    await api("POST", "/api/lighting/identify", { device: z.device, zone: z.zone });
  } catch (e) { f.err = e.message; }
  renderFind();
}
function findStop() { api("DELETE", "/api/lighting/calibrate").catch(() => {}); }
function startFind(zones) {
  closeBind();
  autoBind.sum = null; renderAutoSum();
  autoBind.find = { zones: zones.slice(), i: 0, picked: [], binds: [], prev: JSON.parse(JSON.stringify(pv.sketch)), setup: null, err: "" };
  renderFind(true);
  findLight();
}
function endFind() {
  const f = autoBind.find;
  if (!f) return;
  autoBind.find = null;
  findStop();
  renderFind();
  if (f.binds.length) {
    autoBind.sum = { binds: f.binds, notes: [], open: (pv.sketch ? pv.sketch.parts.filter((p) => !p.bind).map((p) => p.id) : []), zones: [], prev: f.prev, kind: "find" };
    renderAutoSum(true);
  }
}
function findNext() {
  const f = autoBind.find;
  if (!f) return;
  f.picked = []; f.setup = null; f.i++;
  if (f.i >= f.zones.length || !pv.sketch.parts.some((p) => !p.bind)) { endFind(); return; }
  renderFind(true);
  findLight();
}
function findPick(id) {
  const f = autoBind.find, pl = pv.sketch && pv.sketch.parts.find((p) => p.id === id);
  if (!f || !pl || pl.bind || f.setup) return;
  const i = f.picked.indexOf(id);
  if (i >= 0) f.picked.splice(i, 1); else f.picked.push(id);
  renderFind();
}
async function findBind() {
  const f = autoBind.find, z = f && f.zones[f.i];
  if (!z || !f.picked.length) return;
  const doc = JSON.parse(JSON.stringify(pv.sketch));
  const base = Math.max(-1, ...doc.parts.filter((p) => p.bind && p.bind.device === z.device && p.bind.zone === z.zone).map((p) => p.bind.order || 0)) + 1;
  const now = [];
  f.picked.forEach((id, k) => { const pl = doc.parts.find((p) => p.id === id); if (pl && !pl.bind) { pl.bind = { device: z.device, zone: z.zone, order: base + k, parallel: false }; now.push({ part: id, device: z.device, zone: z.zone, order: base + k }); } });
  try {
    await putSketch(doc);
    f.binds.push(...now);
    flow.sig = "";
    renderLight(); refreshLightChip();
  } catch (e) { toast(t("lh.bind.err", { e: e.message }), true); return; }
  // a header cannot report its LED count: take it from the parts, or find it with the assistant
  const { d, z: zz } = zoneOf(z.device, z.zone);
  const leds = now.reduce((s, b) => s + ((pv.out && pv.out.info.get(b.part)) || { n: 0 }).n, 0);
  if (d && zz && zz.resizable && leds > 0 && zz.count !== leds) { f.setup = { d, z: zz, leds }; f.picked = []; renderFind(true); return; }
  findNext();
}
async function findSetCount() {
  const f = autoBind.find, s = f && f.setup;
  if (!s) return;
  try {
    lv = await api("POST", "/api/lighting/zone-setup", { device: s.d.id, zone: s.z.id, count: s.leds });
    lightSig = JSON.stringify(lv);
    toast(t("lh.find.counted", { z: zoneLabel(s.d.id, s.z.id), n: s.leds }));
  } catch (e) { toast(e.message, true); return; }
  findNext();
}
function findAssist() {
  const f = autoBind.find, s = f && f.setup;
  if (!s) return;
  endFind();
  if (flow.open !== s.d.id) openDrawer(s.d.id);
  startAssist(s.d, s.z);
}
function renderFind(focusIt) {
  const box = $("#light-find"), f = autoBind.find;
  for (const g of $$("#light-pv .pv-part")) {
    const k = f ? f.picked.indexOf(g.dataset.id) : -1;
    g.classList.toggle("found", k >= 0);
    g.classList.toggle("findable", !!f && g.classList.contains("unbound"));
  }
  $("#light-stage").classList.toggle("finding", !!f);
  if (!f) { box.hidden = true; box.replaceChildren(); return; }
  const z = f.zones[f.i], name = zoneLabel(z.device, z.zone);
  const kids = [
    el("div", { class: "lh-bind-head" },
      el("small", {}, t("lh.find.step", { i: f.i + 1, n: f.zones.length })),
      el("b", {}, name)),
  ];
  if (f.setup) {
    kids.push(el("p", { class: "sub" }, t("lh.find.count", { z: name, n: f.setup.leds })),
      el("div", { class: "lh-bind-actions" },
        el("button", { class: "primary", type: "button", onclick: findSetCount }, t("lh.find.count.take", { n: f.setup.leds })),
        el("button", { class: "ghost", type: "button", onclick: findAssist }, t("lh.find.count.assist")),
        el("button", { class: "ghost", type: "button", onclick: findNext }, t("lh.find.count.keep"))));
  } else {
    kids.push(el("p", { class: "sub" }, f.err ? t("lh.find.err", { e: f.err }) : z.port ? t("lh.find.blinkfan") : t("lh.find.blink")));
    kids.push(f.picked.length
      ? el("ol", { class: "lh-find-list" }, f.picked.map((id) => el("li", {}, partLabel(id))))
      : el("p", { class: "lh-find-hint" }, t("lh.find.hint")));
    kids.push(el("div", { class: "lh-bind-actions" },
      el("button", { class: "primary", type: "button", disabled: !f.picked.length, onclick: findBind }, f.picked.length > 1 ? t("lh.find.bindn", { n: f.picked.length }) : t("lh.find.bind")),
      el("button", { class: "ghost", type: "button", onclick: findNext }, t("lh.find.skip"))),
    el("div", { class: "lh-bind-actions" },
      el("button", { class: "ghost", type: "button", onclick: findLight }, t("lh.find.again")),
      el("button", { class: "ghost", type: "button", onclick: endFind }, t("lh.find.end"))));
  }
  box.replaceChildren(...kids);
  box.hidden = false;
  if (focusIt) ($("button.primary:not([disabled])", box) || $("button", box)).focus({ preventScroll: true });
}

// ---- "Use detected devices": the server proposes parts, nothing is stored before "Apply"
const SUG_NOTES = [
  [/^no mainboard with (\d+) LEDs/, "lh.note.board"], [/^no memory module with (\d+) LEDs/, "lh.note.ram"],
  [/^no display with a ring of (\d+) LEDs/, "lh.note.ring"], [/^no fan part in the catalog/, "lh.note.nofan"],
  [/^fans stay unbound/, "lh.note.unbound"], [/^part limit reached/, "lh.note.limit"],
];
function sugNote(n) {
  for (const [re, key] of SUG_NOTES) { const m = re.exec(n.msg || ""); if (m) return t(key, { n: m[1] }); }
  return n.msg || "";
}
function renderSuggest() {
  const sug = pv.sug, body = $("#light-suggest-body");
  if (!sug) { body.replaceChildren(el("p", { class: "sub" }, "…")); $("#light-suggest-ok").disabled = true; return; }
  const parts = sug.parts || [], notes = sug.notes || [];
  const rows = parts.map((p) => {
    const part = pv.cat && pv.cat.parts[p.part];
    const { d, z } = p.bind ? zoneOf(p.bind.device, p.bind.zone) : {};
    const g = part && part.leds && part.leds.generator;
    const n = g ? p.leds || g.default : part && part.leds && part.leds.positions_mm ? part.leds.positions_mm.length : p.leds;
    return el("li", {},
      el("span", { class: "lh-sug-name" }, el("b", {}, window.SketchPreview.name(pv.cat, p)),
        el("small", {}, [p.label && p.label !== window.SketchPreview.name(pv.cat, p) ? p.label : "", p.bind ? [d ? d.name : p.bind.device, z ? z.label || z.name : p.bind.zone].join(" · ") : t("lh.tip.unbound")].filter(Boolean).join(" · "))),
      el("small", { class: "lh-sug-n" }, n ? (n === 1 ? t("lh.tip.led1") : t("lh.tip.leds", { n })) : ""));
  });
  body.replaceChildren(
    ...(parts.length ? [el("ul", { class: "lh-sug-list" }, rows), el("p", { class: "sub" }, parts.length === 1 ? t("lh.sug.add1") : t("lh.sug.add", { n: parts.length }))] : [el("p", { class: "empty-note" }, t("lh.sug.none"))]),
    ...(notes.length ? [el("div", { class: "lh-sug-notes" }, el("b", {}, t("lh.sug.notes")), el("ul", {}, notes.map((n) => el("li", {}, sugNote(n)))))] : []));
  $("#light-suggest-ok").disabled = !parts.length;
}
async function openSuggest() {
  const dlg = $("#light-suggest");
  pv.sug = null; renderSuggest();
  if (!dlg.open) dlg.showModal();
  try {
    if (!pv.cat) await loadPreview(true);
    pv.sug = await api("GET", "/api/lighting/sketch/suggest");
    renderSuggest();
  } catch (e) {
    if (dlg.open) dlg.close();
    toast(t("lh.sug.err", { e: e.message }), true);
  }
}
async function applySuggest() {
  const sug = pv.sug;
  if (!sug || !(sug.parts || []).length) return;
  const ok = $("#light-suggest-ok");
  ok.disabled = true;
  let base = pv.sketch;
  if (!base) base = { ...(previewDoc() || window.SketchPreview.emptyDoc()), parts: [] };
  try {
    await putSketch({ ...base, parts: [...base.parts, ...sug.parts] });
    $("#light-suggest").close();
    toast(t("lh.sug.done", { n: sug.parts.length }));
    flow.sig = "";
    renderLight(); refreshLightChip();
  } catch (e) { toast(t("lh.sug.save", { e: e.message }), true); ok.disabled = false; }
}
$("#light-suggest-ok").addEventListener("click", applySuggest);
$("#light-suggest-no").addEventListener("click", () => $("#light-suggest").close());
$("#light-suggest-close").addEventListener("click", () => $("#light-suggest").close());
$("#light-adopt").addEventListener("click", openSuggest);
$("#light-pv").addEventListener("pointermove", previewTip);
$("#light-pv").addEventListener("pointerleave", () => { $("#light-tip").hidden = true; });
$("#light-pv").addEventListener("click", (e) => {
  const g = e.target.closest && e.target.closest(".pv-part");
  if (autoBind.find) { if (g) findPick(g.dataset.id); return; }
  if (g && pv.sketch) openBind(g.dataset.id); else if (pv.bind) closeBind();
});
$("#light-pv").addEventListener("keydown", (e) => {
  const g = e.target.closest && e.target.closest(".pv-part");
  if (g && (e.key === "Enter" || e.key === " ")) { e.preventDefault(); if (autoBind.find) findPick(g.dataset.id); else openBind(g.dataset.id); }
});
$("#light-auto").addEventListener("click", () => runAutoBind(false));
$("#light-find").addEventListener("keydown", (e) => { if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); endFind(); } });
$("#light-auto-sum").addEventListener("keydown", (e) => { if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); autoBind.sum = null; renderAutoSum(); } });
$("#light-bind").addEventListener("keydown", (e) => { if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); closeBind(true); } });
$("#light-unbound").addEventListener("click", () => {
  const p = pv.sketch && pv.sketch.parts.find((q) => !q.bind && !q.hidden && pv.out && pv.out.info.has(q.id));
  if (p) openBind(p.id);
});

// Tab shown or hidden, editor closed, window hidden, size changed.
{
  const tabEl = $("#tab-light");
  let wasActive = lightActive(), wasSketching = tabEl.classList.contains("sketching");
  new MutationObserver(() => {
    const a = lightActive(), sk = tabEl.classList.contains("sketching");
    if (a && !wasActive) { pv.stop = false; pv.fails = 0; renderPreview(); }
    if (!a && wasActive) { liveHalt(); endFind(); endOrient(); }
    if (sk && !wasSketching) { endFind(); endOrient(); autoBind.sum = null; renderAutoSum(); }
    if (wasSketching && !sk) { pv.sketch = undefined; loadPreview(true); }
    wasActive = a; wasSketching = sk;
  }).observe(tabEl, { attributes: true, attributeFilter: ["class"] });
  document.addEventListener("visibilitychange", () => { if (document.hidden) liveHalt(); else liveReset(); });
  let rz = 0;
  new ResizeObserver(() => { cancelAnimationFrame(rz); rz = requestAnimationFrame(() => { renderPreview(); if (pv.bind) renderBind(); }); }).observe($("#light-stage"));
  let rf = 0;
  new ResizeObserver(() => { cancelAnimationFrame(rf); rf = requestAnimationFrame(drawFlowCables); }).observe($("#light-flow"));
}

// Device states change by themselves (another program starts): keep them fresh.
setInterval(() => {
  if (document.hidden) return;
  if (lightActive() || (lv && lv.enabled)) loadLight();
}, 3000);
loadLight();
