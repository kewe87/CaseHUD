"use strict";

// ---------- Lighting tab ("Beleuchtung") ----------
// Texts live here so the tab stays one self-contained file.
Object.assign(STRINGS.de, {
  "tab.light": "Beleuchtung",
  "light.sub": "Gehäuse-, RAM- und Lüfterbeleuchtung mit einem gemeinsamen Effekt.",
  "light.master": "Beleuchtung steuern", "light.master.on": "Aktiv – {n} Geräte gefunden", "light.master.on1": "Aktiv – 1 Gerät gefunden",
  "light.master.on0": "Aktiv – kein Gerät gefunden", "light.master.off": "Aus – CaseHUD spricht kein Gerät an",
  "light.paused": "Pausiert (Displays aus)",
  "light.nobackend": "Keine Beleuchtungs-Hardware freigegeben. Aktiviere unten die TL-Lüfter-Beleuchtung.",
  "light.look": "Effekt", "light.brightness": "Helligkeit", "light.exit": "Beim Beenden", "light.exit.off": "Aus", "light.exit.keep": "Letzte Farben halten", "light.exit.device": "Eigener Effekt der Geräte", "light.exit.hint": "Gilt beim Beenden von CaseHUD und beim Ausschalten der Beleuchtung. Bei „Eigener Effekt der Geräte“ gehen Geräte ohne eigenen Effekt aus.",
  "light.effect": "Effekt", "light.effect.params": "Einstellungen des Effekts",
  "light.devices": "Geräte", "light.devices.sub": "Alles, was CaseHUD beleuchten kann. Geräte, die ein anderes Programm steuert, bleiben unberührt.",
  "light.devices.none": "Noch keine Geräte. Schalte die Beleuchtung ein oder suche erneut.", "light.devices.off": "Schalte die Beleuchtung ein, um Geräte zu suchen.",
  "light.rescan": "Neu suchen", "light.device.enabled": "Gerät ansteuern",
  "light.state.ok": "Aktiv", "light.state.conflict": "Belegt", "light.state.disabled": "Aus", "light.state.error": "Fehler",
  "light.conflict.by": "Wird von {names} gesteuert", "light.ignore": "Trotzdem ansteuern",
  "light.ignore.sub": "Beide Programme steuern dann dasselbe Gerät. Es kann flackern.",
  "light.dev.settings": "Geräteeinstellungen", "light.zones": "Zonen",
  "light.zone.leds": "{n} LEDs", "light.zone.count": "LED-Anzahl", "light.zone.calib": "Kalibrieren", "light.zone.calib.end": "Fertig",
  "light.zone.calib.hint": "Das Testlicht schreibt nur kurz an den Controller und ändert nichts dauerhaft. Die rote LED wandert mit. Verschwindet sie, ist das Ende erreicht – dann stimmt die Anzahl.",
  "light.zone.prev": "Eine LED zurück", "light.zone.next": "Eine LED weiter",
  "light.layout": "Layout", "light.layout.sub": "Jeder Punkt ist eine LED, so wie der Effekt sie auf der Fläche sieht.",
  "light.layout.none": "Keine aktiven Zonen.",
  "light.tl": "TL-Lüfter-Beleuchtung (experimentell)", "light.tl.sub": "Spricht den TL-Lüfter-Controller an. Im Standardmodus „Mit Mainboard synchronisieren“ sendet CaseHUD beim Start einen einzigen Befehl; danach folgen die Lüfter dem Mainboard. Die Modi „CaseHUD-Effekte“ und „Effekte des Controllers“ speichern jede Änderung im Flash-Speicher des Controllers und brauchen deine Bestätigung. „Blinken“ und das Testlicht schreiben nur kurz an den Controller. Noch nicht auf allen Systemen getestet.",
  "light.tl.modeWarn": "Dieser Modus schreibt Beleuchtungseinstellungen an den Controller; sie bleiben dort eventuell gespeichert.",
  "light.chip.conflict": "Beleuchtung: {n} Gerät(e) von einem anderen Programm belegt", "light.chip.error": "Beleuchtung: {n} Gerät(e) mit Fehler",
  "light.err.load": "Beleuchtung konnte nicht geladen werden: {e}", "light.on": "an", "light.off": "aus",
  "light.fx.rainbow": "Regenbogen", "light.fx.solid": "Einfarbig", "light.fx.breathing": "Atmen", "light.fx.wave": "Farbwelle",
  "light.fx.sensor": "Temperatur", "light.fx.cycle": "Farbwechsel", "light.fx.meteor": "Meteor", "light.fx.fill": "Füllen",
  "light.fx.burst": "Füllen und verblassen", "light.fx.shuttle": "Pendel", "light.fx.bounce": "Springen", "light.fx.stack": "Stapeln",
  "light.fx.sparkle": "Funkeln", "light.fx.stripes": "Streifen", "light.fx.sand": "Sanduhr", "light.fx.spark": "Funke",
  "light.fx.meet": "Treffen und mischen", "light.fx.rainbow-gaps": "Regenbogen mit Lücken",
  "light.p.speed": "Geschwindigkeit", "light.p.scale": "Breite", "light.p.vertical": "Senkrecht", "light.p.color": "Farbe",
  "light.p.useAccent": "Akzentfarbe des Designs", "light.p.period": "Dauer (s)", "light.p.background": "Hintergrund", "light.p.width": "Breite",
  "light.p.sensor": "Sensor", "light.p.min": "Wert für niedrige Farbe", "light.p.max": "Wert für hohe Farbe", "light.p.low": "Niedrige Farbe",
  "light.p.high": "Hohe Farbe", "light.p.smoothing": "Glättung", "light.p.colors": "Genutzte Farben", "light.p.path": "Verlauf",
  "light.p.mirror": "In der Mitte spiegeln", "light.p.reverse": "Richtung umkehren", "light.p.tail": "Schweiflänge", "light.p.alternate": "Richtung wechseln",
  "light.p.size": "Blockgröße", "light.p.balls": "Bälle", "light.p.density": "Dichte", "light.p.count": "Streifen", "light.p.gaps": "Lücken",
  "light.p.color1": "Farbe 1", "light.p.color2": "Farbe 2", "light.p.color3": "Farbe 3", "light.p.color4": "Farbe 4", "light.p.color5": "Farbe 5", "light.p.color6": "Farbe 6",
  "light.flash": "Der Controller speichert jede Änderung dauerhaft – Änderungen werden gebündelt gesendet.", "light.more": "Weitere: {n}",
  "light.p.mode": "Modus", "light.p.effect": "Hardware-Effekt", "light.p.direction": "Richtung", "light.p.interval": "Canvas: Sekunden zwischen Updates",
  "light.p.minDelta": "Canvas: kleinste Farbänderung", "light.p.merge": "Hardware-Effekt über alle Lüfter", "light.p.portOrder": "Port-Reihenfolge, z. B. 3,1,2",
  "light.p.brightness": "Helligkeit", "light.p.color1x": "Farbe 1",
  "arr.blink.fw": "Lüfter dieses Ports blinken lassen",
  "arr.scan": "Lüfter ohne Display suchen", "arr.scan.again": "Lüfterliste neu lesen",
  "arr.scan.confirm": "CaseHUD liest jetzt die Lüfterliste vom Controller (nur lesen, es wird nichts geändert). Andere Programme, die den Controller gerade nutzen, können dabei kurz gestört werden. Fortfahren?", "arr.blink.note": "„Identifizieren“ lässt die Lüfter kurz leuchten oder blinken. Dabei wird kurz an den Controller geschrieben, aber nichts dauerhaft geändert.", "arr.scan.hint": "Liest einmal die Lüfterliste vom Controller (nur lesen), ändert nichts. Andere Programme, die den Controller gerade nutzen, können dabei kurz gestört werden.",
  "arr.scan.live": "Die Beleuchtung hält die Liste über ihre eigene Verbindung aktuell.",
  "arr.scan.at": "Zuletzt gelesen: {when}", "arr.scan.never": "Noch nicht gelesen – gezeigt werden nur Lüfter mit Display.",
  "light.zone.led1": "1 LED", "light.unplaced": "Noch nicht auf der Fläche platziert",
  "light.unplaced.sub": "Der Effekt behandelt diese Zonen als eine gemeinsame Linie in der Mitte.",
  "light.reset": "Beleuchtungs-Einstellungen waren ungültig und wurden zurückgesetzt (Sicherung: {file}).",
  "light.presets": "Vorlagen", "light.presets.sub": "Fertige Effekte aus der Community-Bibliothek. Ein Klick stellt Effekt und Einstellungen ein; Geräte und Layouts bleiben, wie sie sind.",
  "light.presets.none": "Keine Vorlagen vorhanden. Mit „Library aktualisieren“ lädt CaseHUD sie von GitHub.", "light.presets.count": "{n} Stück",
  "light.lib.refresh": "Library aktualisieren", "light.lib.loading": "Lade …",
  "light.lib.status": "Stand: {when} · {c} Bauteile · {p} Vorlagen",
  "light.lib.never": "Noch nicht geladen. CaseHUD holt dafür öffentliche Dateien von GitHub und sendet nichts dorthin. Ohne Verbindung läuft alles mit den eingebauten Bauteilen.",
  "light.lib.error": "Bibliothek nicht erreichbar: {e}. Es gilt die zuletzt geladene Kopie.",
  "light.lib.warn": "{n} Eintrag/Einträge der Bibliothek wurden übersprungen.",
  "light.preset.applied": "Vorlage „{n}“ eingestellt.", "light.preset.ignored": "{k} Einstellung(en) der Vorlage kennt der Effekt nicht und wurde(n) ausgelassen.",
  "light.preset.apply": "Vorlage {n} anwenden",
  "light.sync": "Mit Mainboard synchronisieren", "light.sync.note": "Die Einstellung bleibt nach dem Beenden von CaseHUD bestehen.",
  "light.sync.else": "Ohne Synchronisation", "light.sync.passive": "Nicht ansteuern (nur lesen)", "light.sync.canvas": "CaseHUD-Effekte", "light.sync.hardware": "Effekte des Controllers",
  "light.flash.confirm": "Der Lüfter-Controller speichert jede Beleuchtungs-Änderung dauerhaft in seinem Flash-Speicher. Häufige Änderungen verschleißen ihn. Empfohlen: „Mit Mainboard synchronisieren“.",
  "light.ram.hint": "RAM-Beleuchtung braucht die SMBus-Brücke im Sensor-Dienst (Einstellungen → Datenquellen; Datei C:\\ProgramData\\CaseHUD\\sensors.json mit \"smbusBridge\": true, siehe docs/SMBUS-BRIDGE.md).",
  "light.mirror": "gespiegelt von {z}", "light.mirror.none": "gespiegelt vom Mainboard-Sync",
  "light.syncsrc": "Sync-Anschluss am Mainboard", "light.syncsrc.none": "– nicht gewählt –",
  "light.syncsrc.note": "Der Lüfter-Controller gibt das Signal des ARGB-Anschlusses parallel an alle Ports weiter. Mit der Auswahl legt CaseHUD diesen Anschluss im Layout mit {n} Lüftern (je 26 LEDs, in Kettenreihenfolge) an.",
  "light.syncsrc.nofans": "Die Lüfteranzahl ist noch nicht bekannt – lies zuerst die Lüfter ein (Displays → Anordnung).",
  "light.syncsrc.nohdr": "Kein Mainboard mit ARGB-Anschluss gefunden.",
  "light.unsupported": "Nicht unterstützt", "light.ram.title": "RAM-Beleuchtung",
  "light.sync.disabled": "Das Gerät ist ausgeschaltet – CaseHUD verändert nichts daran.",
  "light.sync.passiveNote": "CaseHUD schreibt nichts an dieses Gerät (passiv).",
  "diag.title": "Diagnose exportieren", "diag.sub": "Speichert eine ZIP-Datei: Ende des Protokolls, bereinigte Einstellungen, Versionen, Display-Liste und Beleuchtungsbericht. Namen, Pfade, Adressen und Tokens sind entfernt. Es wird nichts gesendet – sieh die Datei durch, bevor du sie weitergibst.",
  "diag.trace": "Mitschnitt (30 s)", "diag.trace.sub": "Zeichnet 30 Sekunden lang die Datenpakete zwischen CaseHUD und den Beleuchtungsgeräten auf und legt sie in die ZIP-Datei. Sie können Hardware-Kennungen des Controllers enthalten. Geht nur bei aktiver Beleuchtung; ändere währenddessen zum Beispiel den Effekt.",
  "diag.trace.off": "Für den Mitschnitt muss die Beleuchtung aktiv sein.",
  "diag.export": "Exportieren", "diag.running": "Mitschnitt läuft – noch {n} s. Ändere jetzt zum Beispiel Effekt oder Helligkeit.",
  "diag.done": "Gespeichert als {f}.", "diag.err": "Export fehlgeschlagen: {e}",
});
Object.assign(STRINGS.en, {
  "tab.light": "Lighting",
  "light.sub": "Case, RAM and fan lighting with one shared effect.",
  "light.master": "Control lighting", "light.master.on": "Active – {n} devices found", "light.master.on1": "Active – 1 device found",
  "light.master.on0": "Active – no device found", "light.master.off": "Off – CaseHUD does not talk to any device",
  "light.paused": "Paused (displays off)",
  "light.nobackend": "No lighting hardware is enabled. Turn on TL fan lighting below.",
  "light.look": "Effect", "light.brightness": "Brightness", "light.exit": "On exit", "light.exit.off": "Off", "light.exit.keep": "Keep last colours", "light.exit.device": "Devices' own effect", "light.exit.hint": "Applies when CaseHUD quits and when lighting is switched off. With “Devices' own effect”, devices without an effect of their own turn off.",
  "light.effect": "Effect", "light.effect.params": "Effect settings",
  "light.devices": "Devices", "light.devices.sub": "Everything CaseHUD can light. Devices that another program controls are left alone.",
  "light.devices.none": "No devices yet. Turn lighting on or search again.", "light.devices.off": "Turn lighting on to search for devices.",
  "light.rescan": "Search again", "light.device.enabled": "Drive this device",
  "light.state.ok": "Active", "light.state.conflict": "In use", "light.state.disabled": "Off", "light.state.error": "Error",
  "light.conflict.by": "Controlled by {names}", "light.ignore": "Drive it anyway",
  "light.ignore.sub": "Both programs then control the same device. It may flicker.",
  "light.dev.settings": "Device settings", "light.zones": "Zones",
  "light.zone.leds": "{n} LEDs", "light.zone.count": "LED count", "light.zone.calib": "Calibrate", "light.zone.calib.end": "Done",
  "light.zone.calib.hint": "The test light only writes briefly to the controller and changes nothing permanently. The red LED moves along. When it disappears you are past the end, which tells you the real count.",
  "light.zone.prev": "One LED back", "light.zone.next": "One LED forward",
  "light.layout": "Layout", "light.layout.sub": "Every dot is an LED, placed where the effect sees it.",
  "light.layout.none": "No active zones.",
  "light.tl": "TL fan lighting (experimental)", "light.tl.sub": "Talks to the TL fan controller. In the default mode \"Sync with motherboard\" CaseHUD sends a single command at start; after that the fans follow the motherboard. The modes \"CaseHUD effects\" and \"Controller effects\" store every change in the controller's flash memory and need your confirmation. \"Blink\" and the test light only write briefly to the controller. Not tested on all systems yet.",
  "light.tl.modeWarn": "This mode writes light settings to the controller; it may keep them stored.",
  "light.chip.conflict": "Lighting: {n} device(s) in use by another program", "light.chip.error": "Lighting: {n} device(s) with an error",
  "light.err.load": "Could not load lighting: {e}", "light.on": "on", "light.off": "off",
  "light.fx.rainbow": "Rainbow", "light.fx.solid": "Solid color", "light.fx.breathing": "Breathing", "light.fx.wave": "Color wave",
  "light.fx.sensor": "Temperature", "light.fx.cycle": "Color cycle", "light.fx.meteor": "Meteor", "light.fx.fill": "Color fill",
  "light.fx.burst": "Fill and fade", "light.fx.shuttle": "Shuttle", "light.fx.bounce": "Bounce", "light.fx.stack": "Stack",
  "light.fx.sparkle": "Sparkle", "light.fx.stripes": "Stripes", "light.fx.sand": "Sand glass", "light.fx.spark": "Spark",
  "light.fx.meet": "Meet and mix", "light.fx.rainbow-gaps": "Rainbow with gaps",
  "light.p.speed": "Speed", "light.p.scale": "Width", "light.p.vertical": "Vertical", "light.p.color": "Color",
  "light.p.useAccent": "Design accent color", "light.p.period": "Period (s)", "light.p.background": "Background", "light.p.width": "Width",
  "light.p.sensor": "Sensor", "light.p.min": "Value for low color", "light.p.max": "Value for high color", "light.p.low": "Low color",
  "light.p.high": "High color", "light.p.smoothing": "Smoothing", "light.p.colors": "Colors used", "light.p.path": "Path",
  "light.p.mirror": "Mirror at the middle", "light.p.reverse": "Reverse direction", "light.p.tail": "Tail length", "light.p.alternate": "Alternate direction",
  "light.p.size": "Block size", "light.p.balls": "Balls", "light.p.density": "Density", "light.p.count": "Stripes", "light.p.gaps": "Gaps",
  "light.p.color1": "Color 1", "light.p.color2": "Color 2", "light.p.color3": "Color 3", "light.p.color4": "Color 4", "light.p.color5": "Color 5", "light.p.color6": "Color 6",
  "light.flash": "The controller stores every change permanently – changes are sent in batches.", "light.more": "More: {n}",
  "light.p.mode": "Mode", "light.p.effect": "Hardware effect", "light.p.direction": "Direction", "light.p.interval": "Canvas: seconds between updates",
  "light.p.minDelta": "Canvas: smallest color change", "light.p.merge": "Hardware effect across all fans", "light.p.portOrder": "Port order, e.g. 3,1,2",
  "light.p.brightness": "Brightness", "light.p.color1x": "Color 1",
  "arr.blink.fw": "Blink the fans of this port",
  "arr.scan": "Find fans without display", "arr.scan.again": "Read the fan list again",
  "arr.scan.confirm": "CaseHUD will now read the fan list from the controller (read only, nothing is changed). Other programs using the controller at that moment may be disturbed briefly. Continue?", "arr.blink.note": "\"Identify\" makes the fans light up or blink briefly. This writes briefly to the controller but changes nothing permanently.", "arr.scan.hint": "Reads the fan list from the controller once (read only) and changes nothing. Other programs using the controller at that moment may be disturbed briefly.",
  "arr.scan.live": "Lighting keeps the list current through its own connection.",
  "arr.scan.at": "Last read: {when}", "arr.scan.never": "Not read yet – only fans with a display are shown.",
  "light.zone.led1": "1 LED", "light.unplaced": "Not placed on the canvas yet",
  "light.unplaced.sub": "The effect treats these zones as one shared line in the middle.",
  "light.reset": "The lighting settings were invalid and have been reset (backup: {file}).",
  "light.presets": "Presets", "light.presets.sub": "Ready-made effects from the community library. One click sets the effect and its settings; devices and layouts stay as they are.",
  "light.presets.none": "No presets yet. \"Update library\" fetches them from GitHub.", "light.presets.count": "{n} items",
  "light.lib.refresh": "Update library", "light.lib.loading": "Loading …",
  "light.lib.status": "As of: {when} · {c} parts · {p} presets",
  "light.lib.never": "Not loaded yet. CaseHUD fetches public files from GitHub and sends nothing there. Offline, everything runs with the built-in parts.",
  "light.lib.error": "Library not reachable: {e}. The last loaded copy stays in use.",
  "light.lib.warn": "{n} library entries were skipped.",
  "light.preset.applied": "Preset \"{n}\" applied.", "light.preset.ignored": "{k} preset setting(s) are unknown to the effect and were left out.",
  "light.preset.apply": "Apply preset {n}",
  "light.sync": "Sync with motherboard", "light.sync.note": "The setting stays in place after you quit CaseHUD.",
  "light.sync.else": "Without sync", "light.sync.passive": "Do not drive (read only)", "light.sync.canvas": "CaseHUD effects", "light.sync.hardware": "Controller effects",
  "light.flash.confirm": "The fan controller stores every lighting change permanently in its flash memory. Frequent changes wear it out. Recommended: \"Sync with motherboard\".",
  "light.ram.hint": "RAM lighting needs the SMBus bridge in the sensor service (Settings > Data sources; file C:\\ProgramData\\CaseHUD\\sensors.json with \"smbusBridge\": true, see docs/SMBUS-BRIDGE.md).",
  "light.mirror": "mirrored from {z}", "light.mirror.none": "mirrored from the motherboard sync",
  "light.syncsrc": "Sync header on the motherboard", "light.syncsrc.none": "- not chosen -",
  "light.syncsrc.note": "The fan controller passes the signal of the ARGB header on to all ports in parallel. Choosing a header lays it out with {n} fans (26 LEDs each, in chain order).",
  "light.syncsrc.nofans": "The number of fans is not known yet - read the fans first (Displays > Arrangement).",
  "light.syncsrc.nohdr": "No mainboard with an ARGB header found.",
  "light.unsupported": "Not supported", "light.ram.title": "RAM lighting",
  "light.sync.disabled": "The device is switched off – CaseHUD changes nothing on it.",
  "light.sync.passiveNote": "CaseHUD writes nothing to this device (passive).",
  "diag.title": "Export diagnostics", "diag.sub": "Saves a ZIP file: end of the log, cleaned settings, versions, display list and lighting report. Names, paths, addresses and tokens are removed. Nothing is sent – look through the file before you share it.",
  "diag.trace": "Recording (30 s)", "diag.trace.sub": "Records the data packets between CaseHUD and the lighting devices for 30 seconds and adds them to the ZIP file. They can contain hardware identifiers of the controller. Only works while lighting is active; change the effect, for example, during the recording.",
  "diag.trace.off": "Lighting has to be active for a recording.",
  "diag.export": "Export", "diag.running": "Recording – {n} s left. Change the effect or brightness now, for example.",
  "diag.done": "Saved as {f}.", "diag.err": "Export failed: {e}",
});
Object.assign(STRINGS.de, {
  "setup.assist": "LED-Anzahl ermitteln", "setup.parts": "Bauteile wählen",
  "setup.assist.hint": "Erhöhe die Zahl, bis die letzte LED am Ende deiner Kette gerade aufleuchtet. Die rote LED ist die gewählte, davor leuchten zwei blaue. Sie erscheint nur, wenn die LED wirklich existiert.",
  "setup.assist.live": "Nur Vorschau – es wird nichts im Gerät gespeichert.",
  "setup.apply": "Übernehmen", "setup.cancel": "Abbrechen",
  "setup.led": "LED", "setup.of": "{n} von {m}",
  "setup.minus1": "Eine weniger", "setup.plus1": "Eine mehr", "setup.minus10": "Zehn weniger", "setup.plus10": "Zehn mehr",
  "setup.assist.ended": "Der Assistent ist abgelaufen oder wurde beendet.",
  "setup.parts.hint": "Wähle die Bauteile in der Reihenfolge, in der sie am Anschluss hängen. Die Summe ihrer LEDs ist die LED-Anzahl.",
  "setup.parts.add": "Bauteil hinzufügen", "setup.parts.pick": "- Bauteil wählen -", "setup.parts.addbtn": "Hinzufügen",
  "setup.parts.none": "Noch keine Bauteile gewählt.", "setup.parts.up": "Nach oben", "setup.parts.down": "Nach unten", "setup.parts.remove": "Entfernen",
  "setup.parts.parallel": "parallel geschaltet (Splitter)",
  "setup.parts.parallel.sub": "Alle Bauteile hängen am selben Signal und zeigen es ab LED 1. Zählt das längste Bauteil.",
  "setup.parts.sum": "Summe: {n} LEDs", "setup.parts.longest": "Längstes Bauteil: {n} LEDs",
  "setup.parts.over": "Der Anschluss nimmt höchstens {m} LEDs.",
  "setup.parts.lib": "Fehlt ein Bauteil? Unter „Vorlagen“ die Library aktualisieren.",
  "setup.seg.parallel": "parallel (Splitter)", "setup.seg.leds": "{n} LEDs",
  "setup.seg.note": "Parallel geschaltete Bauteile bekommen dasselbe Signal ab LED 1; die kürzeren zeigen nur die ersten LEDs davon.",
  "setup.hint.title": "Keine LED-Anzahl gesetzt",
  "setup.hint.text": "Die ARGB-Anschlüsse deines Mainboards können nicht melden, wie viele LEDs dranhängen – bis du es einstellst, leuchtet dort nichts. Wende eine Vorlage an oder ermittle die LED-Anzahl pro Anschluss.",
  "setup.tpl.title": "Vorlage anwenden", "setup.tpl.sub": "Fertige Einstellungen für bekannte Hardware. Setzt LED-Anzahl und Layout der gelisteten Zonen; Effekt und Helligkeit bleiben.",
  "setup.tpl.btn": "Vorlage anwenden", "setup.tpl.sync": "Der Lüfter-Controller folgt danach dem Anschluss ARGB 1.",
  "setup.tpl.confirm": "Die Vorlage „{n}“ setzt LED-Anzahl und Layout dieser Zonen neu. Bestehende Layouts dieser Zonen werden ersetzt. Fortfahren?",
  "setup.tpl.done": "Vorlage angewendet: {a} Einstellung(en).", "setup.tpl.skipped": "Nicht angewendet: {s}",
  "setup.saved": "LED-Anzahl übernommen.",
  "setup.rename": "Namen ändern", "setup.rename.ph": "Eigener Name",
});
Object.assign(STRINGS.en, {
  "setup.assist": "Find LED count", "setup.parts": "Pick parts",
  "setup.assist.hint": "Raise the number until the last LED at the end of your chain just lights up. The red LED is the chosen one, two blue ones come before it. It only shows when that LED really exists.",
  "setup.assist.live": "Preview only - nothing is stored in the device.",
  "setup.apply": "Apply", "setup.cancel": "Cancel",
  "setup.led": "LED", "setup.of": "{n} of {m}",
  "setup.minus1": "One less", "setup.plus1": "One more", "setup.minus10": "Ten less", "setup.plus10": "Ten more",
  "setup.assist.ended": "The assistant timed out or was ended.",
  "setup.parts.hint": "Pick the parts in the order they hang on the header. The sum of their LEDs is the LED count.",
  "setup.parts.add": "Add part", "setup.parts.pick": "- pick a part -", "setup.parts.addbtn": "Add",
  "setup.parts.none": "No parts picked yet.", "setup.parts.up": "Move up", "setup.parts.down": "Move down", "setup.parts.remove": "Remove",
  "setup.parts.parallel": "wired in parallel (splitter)",
  "setup.parts.parallel.sub": "All parts share the same signal and show it from LED 1. The longest part counts.",
  "setup.parts.sum": "Sum: {n} LEDs", "setup.parts.longest": "Longest part: {n} LEDs",
  "setup.parts.over": "The header takes at most {m} LEDs.",
  "setup.parts.lib": "A part is missing? Update the library under \"Presets\".",
  "setup.seg.parallel": "parallel (splitter)", "setup.seg.leds": "{n} LEDs",
  "setup.seg.note": "Parts wired in parallel get the same signal from LED 1; the shorter ones show only the first LEDs of it.",
  "setup.hint.title": "No LED count set",
  "setup.hint.text": "The ARGB headers of your mainboard cannot report how many LEDs are attached - until you set it, nothing lights there. Apply a template or find the LED count per header.",
  "setup.tpl.title": "Apply a template", "setup.tpl.sub": "Ready-made settings for known hardware. Sets LED count and layout of the listed zones; effect and brightness stay.",
  "setup.tpl.btn": "Apply template", "setup.tpl.sync": "The fan controller then follows header ARGB 1.",
  "setup.tpl.confirm": "The template \"{n}\" sets LED count and layout of these zones anew. Existing layouts of these zones are replaced. Continue?",
  "setup.tpl.done": "Template applied: {a} setting(s).", "setup.tpl.skipped": "Not applied: {s}",
  "setup.saved": "LED count applied.",
  "setup.rename": "Rename", "setup.rename.ph": "Your own name",
});
applyI18n();

let lv = null; // last state from /api/lighting
let lightCalibPos = {}; // "device/zone" -> marker position
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
  }
}
const lightPatchSoon = debounce(lightPatch, 160);

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
    const step = hi - lo <= 20 ? 0.1 : 1;
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
      ...p.values.map((v) => el("option", { value: v, selected: v === value }, v)));
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
async function setTlMode(d, mode) {
  if (document.activeElement && document.activeElement.blur) document.activeElement.blur();
  if (hasFlashGate(d)) {
    const flash = flashModes.includes(mode);
    if (flash && !d.settingValues.allowFlashWrites) {
      if (!confirm(t("light.flash.confirm"))) { renderLight(); return; }
      return lightPatch({ devices: { [d.id]: { settings: { mode, allowFlashWrites: true } } } });
    }
    if (!flash) return lightPatch({ devices: { [d.id]: { settings: { mode, allowFlashWrites: false } } } });
  }
  return lightPatch({ devices: { [d.id]: { settings: { mode } } } });
}

async function setAllowFlash(d, on) {
  if (document.activeElement && document.activeElement.blur) document.activeElement.blur();
  if (on && !confirm(t("light.flash.confirm"))) { renderLight(); return; }
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

// Per controller: follow the motherboard's lighting, or let CaseHUD drive it.
// Maps to the device setting "mode" (on: motherboard, off: canvas = CaseHUD
// effects; hardware = the controller's own effects, if it offers them).
function lightSyncRow(d) {
  const set = (d.settings || []).find((p) => p.key === "mode");
  const vals = (set && set.values) || [];
  if (!vals.includes("motherboard")) return null;
  const mode = (d.settingValues && d.settingValues.mode) || "passive";
  const send = (v) => setTlMode(d, v);
  const off = d.disabled;
  const id = "sync-" + Math.random().toString(36).slice(2, 8);
  const rows = [el("label", { class: "check-row", for: id },
    el("input", { type: "checkbox", class: "switch", id, checked: mode === "motherboard", disabled: off,
      onchange: (e) => send(e.target.checked ? "motherboard" : "passive") }),
    el("span", {}, el("b", {}, t("light.sync")), el("small", {}, t("light.sync.note"))))];
  if (off) rows.push(el("p", { class: "sync-note" }, t("light.sync.disabled")));
  else if (mode === "motherboard" && d.sync) {
    const sid = id + "-src", sy = d.sync;
    if (!sy.choices.length) rows.push(el("p", { class: "sync-note" }, t("light.syncsrc.nohdr")));
    else {
      rows.push(el("div", { class: "sync-else" },
        el("label", { for: sid }, t("light.syncsrc")),
        el("select", { id: sid, onchange: (e) => setSyncSource(d, e.target.value) },
          el("option", { value: "", selected: !sy.source }, t("light.syncsrc.none")),
          ...sy.choices.map((c) => el("option", { value: c.id, selected: c.id === sy.source }, c.label)))));
      rows.push(el("p", { class: "sync-note" }, sy.fans > 0 ? t("light.syncsrc.note", { n: sy.fans }) : t("light.syncsrc.nofans")));
    }
  } else if (mode !== "motherboard") {
    const opts = ["passive", "canvas", "hardware"].filter((v) => vals.includes(v));
    const sid = id + "-else";
    rows.push(el("div", { class: "sync-else" },
      el("label", { for: sid }, t("light.sync.else")),
      el("select", { id: sid, onchange: (e) => send(e.target.value) },
        ...opts.map((v) => el("option", { value: v, selected: v === mode }, t("light.sync." + v))))));
    if (mode === "passive") rows.push(el("p", { class: "sync-note" }, t("light.sync.passiveNote")));
  }
  return el("div", { class: "sync" + (off ? " dim" : "") }, ...rows);
}

function lightDeviceCard(d, idx) {
  const col = LIGHT_COLORS[idx % LIGHT_COLORS.length];
  const dim = d.state === "disabled";
  let note = "";
  if (d.state === "conflict") note = t("light.conflict.by", { names: (d.message || "").replace(/^in use by /, "") });
  else if (d.state === "error" || d.unsupported) note = d.message || "";
  const head = el("div", { class: "dev-head" },
    el("span", { class: "dev-dot", style: { background: col } }),
    el("div", { class: "dev-title" }, el("b", {}, d.name), el("small", { title: d.firmware ? t("field.firmware.sub") : "" }, d.backend + " · " + d.id + (d.firmware ? " · " + t("field.firmware") + " " + d.firmware : "")), ...fwNoteNodes(d.firmwareLatest).map((n) => el("small", {}, n))),
    el("span", { class: "badge " + (d.unsupported ? "disabled" : d.state) }, d.unsupported ? t("light.unsupported") : t("light.state." + d.state)),
    d.unsupported ? null : el("input", { type: "checkbox", class: "switch", checked: !d.disabled, title: t("light.device.enabled"), "aria-label": t("light.device.enabled"),
      onchange: (e) => lightPatch({ devices: { [d.id]: { disabled: !e.target.checked } } }) }));
  const body = [];
  if (d.warning) body.push(el("p", { class: "dev-note warn", role: "alert" }, d.warning));
  if (note) body.push(el("p", { class: "dev-note " + d.state }, note));
  if (d.state === "conflict" || d.ignoreConflicts) {
    body.push(el("label", { class: "check-row" },
      el("input", { type: "checkbox", class: "switch", checked: d.ignoreConflicts, onchange: (e) => lightPatch({ devices: { [d.id]: { ignoreConflicts: e.target.checked } } }) }),
      el("span", {}, el("b", {}, t("light.ignore")), el("small", {}, t("light.ignore.sub")))));
  }
  const sync = lightSyncRow(d);
  if (sync) body.push(sync);
  const zones = el("div", { class: "zones" }, d.zones.map((z) => lightZoneRow(d, z)));
  body.push(zones);
  const shownSettings = (d.settings || []).filter((p) => !(d.sync && p.key === "syncSource")); // the helper sets it
  if (shownSettings.length) {
    const slow = hasFlashGate(d); // writes to the controller's flash
    const warn = slow && d.settingValues && flashModes.includes(d.settingValues.mode);
    body.push(el("details", { class: "dev-settings" }, el("summary", {}, t("light.dev.settings")),
      slow ? el("p", { class: "dev-note" }, t("light.flash")) : null,
      el("div", { class: "lp-grid" }, shownSettings.map((p) => lightParam(p, d.settingValues[p.key],
        p.key === "mode" && slow ? (v) => setTlMode(d, v)
          : p.key === "allowFlashWrites" ? (v) => setAllowFlash(d, v)
          : slow ? (v) => queueDeviceSetting(d.id, { [p.key]: v }) : (v) => lightPatch({ devices: { [d.id]: { settings: { [p.key]: v } } } }),
        slow ? (v) => queueDeviceSetting(d.id, v) : (v) => lightPatchSoon({ devices: { [d.id]: { settings: v } } })))),
      warn ? el("p", { class: "dev-note error" }, t("light.tl.modeWarn")) : null));
  }
  return el("div", { class: "card dev" + (dim ? " dim" : "") }, head, ...body);
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
  const btn = (label, delta, txt) => el("button", { class: "ghost sm", type: "button", title: label, "aria-label": label,
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
      el("button", { class: "ghost sm", type: "button", title: t("setup.parts.up"), "aria-label": t("setup.parts.up"), disabled: i === 0, onclick: () => move(i, -1) }, "↑"),
      el("button", { class: "ghost sm", type: "button", title: t("setup.parts.down"), "aria-label": t("setup.parts.down"), disabled: i === c.parts.length - 1, onclick: () => move(i, 1) }, "↓"),
      el("button", { class: "ghost sm", type: "button", title: t("setup.parts.remove"), "aria-label": t("setup.parts.remove"), onclick: () => { c.parts.splice(i, 1); renderLight(); } }, "✕"))))
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
    el("ul", {}, rows.map(({ s, par }) => el("li", {},
      el("span", {}, s.name || s.component || "-"),
      s.count ? el("small", {}, t("setup.seg.leds", { n: s.count })) : null,
      par ? el("em", { class: "par-badge" }, t("setup.seg.parallel")) : null))),
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
  if (!confirm(t("setup.tpl.confirm", { n: name }))) return;
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
      tp.tlSync ? el("p", { class: "sync-note" }, t("setup.tpl.sync")) : null,
      el("div", { class: "setup-actions" }, el("button", { class: "primary", type: "button", onclick: () => applyTemplate(tp) }, t("setup.tpl.btn")))));
  }
  return out;
}

function lightZoneRow(d, z) {
  const key = d.id + "/" + z.id;
  const calibrating = lv.calibrating && lv.calibrating.device === d.id && lv.calibrating.zone === z.id && !lv.calibrating.range;
  const limit = z.max > 0 ? Math.min(z.max, 512) : z.resizable ? 512 : z.count;
  const pos = Math.min(lightCalibPos[key] ?? 0, Math.max(limit - 1, 0));
  const send = (p) => {
    lightCalibPos[key] = p;
    api("POST", "/api/lighting/calibrate", { device: d.id, zone: z.id, pos: p }).then(loadLight).catch((e) => toast(e.message, true));
  };
  const active = d.state === "ok";
  const lkey = d.id + "/" + z.id;
  const commitLabel = (input) => {
    if (editLabel !== lkey) return;
    editLabel = null;
    const v = input.value.trim();
    input.blur(); // so the list can be rebuilt
    if (v === (z.label || "")) { setTimeout(renderLight, 0); return; }
    lightPatch({ devices: { [d.id]: { zones: { [z.id]: { label: v } } } } });
  };
  const nameTop = editLabel === lkey
    ? el("input", { class: "label-input", type: "text", maxlength: 40, value: z.label || "", placeholder: z.name, "aria-label": t("setup.rename.ph"),
        onkeydown: (e) => { if (e.key === "Enter") e.target.blur(); else if (e.key === "Escape") { editLabel = null; e.target.blur(); setTimeout(renderLight, 0); } },
        onchange: (e) => commitLabel(e.target), onblur: (e) => commitLabel(e.target) })
    : el("span", { class: "zone-title" }, el("b", {}, z.label || z.name),
        el("button", { class: "icon-btn sm pencil", type: "button", title: t("setup.rename"), "aria-label": t("setup.rename"),
          onclick: () => { editLabel = lkey; renderLight(); } }, "✎"));
  const row = el("div", { class: "zone" },
    el("div", { class: "zone-name" }, nameTop, el("small", {}, (z.label ? z.name + " · " : "") + (z.leds === 1 ? t("light.zone.led1") : t("light.zone.leds", { n: z.leds })) + " · " + z.order),
      z.mirrored ? el("small", { class: "mirror" }, z.mirroredFrom ? t("light.mirror", { z: z.mirroredFrom }) : t("light.mirror.none")) : null),
    z.resizable ? el("label", { class: "zone-count" }, el("span", {}, t("light.zone.count")),
      el("input", { type: "number", min: 0, max: Math.min(z.max || 1024, 1024), value: z.leds, onchange: (e) => {
        const n = Math.max(0, Math.min(Number(e.target.value) || 0, Math.min(z.max || 1024, 1024)));
        lightPatch({ devices: { [d.id]: { zones: { [z.id]: { count: n } } } } });
      } })) : el("span"),
    calibrating
      ? el("button", { class: "ghost", onclick: () => api("DELETE", "/api/lighting/calibrate").then(loadLight) }, t("light.zone.calib.end"))
      : el("button", { class: "ghost", disabled: !active || limit < 1, onclick: () => send(pos) }, t("light.zone.calib")));
  const extras = [zoneParts(z)];
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
      el("button", { class: "ghost sm", title: t("light.zone.prev"), "aria-label": t("light.zone.prev"), disabled: pos <= 0, onclick: () => send(pos - 1) }, "‹"),
      el("span", { class: "calib-pos" }, `${pos + 1} / ${limit}`),
      el("button", { class: "ghost sm", title: t("light.zone.next"), "aria-label": t("light.zone.next"), disabled: pos >= limit - 1, onclick: () => send(pos + 1) }, "›"),
      el("small", {}, t("light.zone.calib.hint"))));
}

function lightCanvas() {
  const [cw, ch] = lv.canvas;
  const W = 640, H = Math.round((640 * ch) / cw);
  const c = el("canvas", { width: W, height: H, class: "light-canvas", role: "img", "aria-label": t("light.layout") });
  const g = c.getContext("2d");
  g.fillStyle = "#0a0d13"; g.fillRect(0, 0, W, H);
  g.strokeStyle = "#232c3c"; g.lineWidth = 1;
  for (let i = 1; i < 8; i++) { g.beginPath(); g.moveTo((W * i) / 8 + .5, 0); g.lineTo((W * i) / 8 + .5, H); g.stroke(); }
  for (let i = 1; i < 5; i++) { g.beginPath(); g.moveTo(0, (H * i) / 5 + .5); g.lineTo(W, (H * i) / 5 + .5); g.stroke(); }
  g.strokeStyle = "#334155"; g.strokeRect(.5, .5, W - 1, H - 1);
  lv.devices.forEach((d, di) => {
    if (d.state === "disabled") return;
    g.fillStyle = LIGHT_COLORS[di % LIGHT_COLORS.length];
    g.globalAlpha = d.state === "ok" ? 1 : .35;
    for (const z of d.zones) if (z.placed) {
      for (const [x, y] of z.points) { g.beginPath(); g.arc(x * (W - 12) + 6, y * (H - 12) + 6, 4, 0, Math.PI * 2); g.fill(); }
      if (z.label && z.points.length) {
        const xs = z.points.map((q) => q[0]), ys = z.points.map((q) => q[1]);
        g.font = "11px system-ui, sans-serif"; g.textBaseline = "bottom";
        g.fillText(z.label, Math.min(Math.min(...xs) * (W - 12) + 6, W - g.measureText(z.label).width - 4), Math.max(12, Math.min(...ys) * (H - 12) - 2));
      }
      // parts in parallel (splitter) repeat LEDs: drawn hollow
      g.strokeStyle = g.fillStyle; g.lineWidth = 1.5;
      for (const [x, y] of z.echo || []) { g.beginPath(); g.arc(x * (W - 12) + 6, y * (H - 12) + 6, 3.5, 0, Math.PI * 2); g.stroke(); }
    }
  });
  g.globalAlpha = 1;
  return c;
}

function renderLight() {
  if (!lv) return;
  queueMicrotask(() => { const i = editLabel && $(".label-input"); if (i && document.activeElement !== i) { i.focus(); i.select(); } });
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
  sel.replaceChildren(...shown.map((e) => el("option", { value: e.name, selected: e.name === lv.effect },
    e.featured ? lightLabel("fx", e.name, e.label) : t("light.more", { n: lightLabel("fx", e.name, e.label) }))));
  const info = lv.effects.find((e) => e.name === lv.effect);
  const box = $("#light-params");
  if (!(document.activeElement && box.contains(document.activeElement) && ["range", "color", "text", "select-one"].includes(document.activeElement.type))) {
    box.replaceChildren(...((info && info.params) || []).map((p) => lightParam(p, (lv.params || {})[p.key],
      (v) => lightPatch({ params: { [p.key]: v } }), (v) => lightPatchSoon({ params: v }))));
  }
  $("#light-params-head").hidden = !(info && info.params.length);

  // devices
  const list = $("#light-devices");
  const busy = devQueued() || document.activeElement && list.contains(document.activeElement) && ["number", "text", "range", "color", "select-one"].includes(document.activeElement.type);
  if (!busy) {
    const open = new Set($$("#light-devices details[open]").map((d) => d.closest(".dev").dataset.id));
    list.replaceChildren(...setupCards(), ...(lv.devices.length ? lv.devices.map((d, i) => {
      const card = lightDeviceCard(d, i);
      card.dataset.id = d.id;
      if (open.has(d.id)) $("details", card)?.setAttribute("open", "");
      return card;
    }) : [el("div", { class: "empty-note" }, on ? t("light.devices.none") : t("light.devices.off"))]));
    const ram = lv.ram || {};
    if (on && lv.active && (ram.hint || ram.note)) {
      list.append(el("div", { class: "card ram-hint" }, el("b", {}, t("light.ram.title")),
        el("p", { class: "dev-note" + (ram.hint ? "" : " warn") }, ram.hint ? t("light.ram.hint") : ram.note)));
    }
  }
  $("#light-rescan").disabled = !on || !lv.active;

  // layout
  const lay = $("#light-layout");
  const zonesShown = lv.devices.filter((d) => d.state !== "disabled" && d.zones.length);
  lay.hidden = !zonesShown.length;
  if (zonesShown.length) {
    $("#light-canvas-box").replaceChildren(lightCanvas());
    // Zones without a layout of their own would all sit on one line: list them instead.
    const loose = zonesShown.flatMap((d) => d.zones.filter((z) => !z.placed && z.leds > 0).map((z) => ({ d, z })));
    const tray = $("#light-unplaced");
    tray.hidden = !loose.length;
    tray.replaceChildren(...(loose.length ? [el("b", {}, t("light.unplaced")), el("small", {}, t("light.unplaced.sub")),
      el("ul", {}, loose.map(({ d, z }) => el("li", {}, `${d.name} · ${z.label || z.name} · ` + (z.leds === 1 ? t("light.zone.led1") : t("light.zone.leds", { n: z.leds })))))] : []));
    $("#light-legend").replaceChildren(...lv.devices.map((d, i) => d.state === "disabled" ? null :
      el("span", { class: "leg" }, el("i", { style: { background: LIGHT_COLORS[i % LIGHT_COLORS.length] } }), d.name)));
  }

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
let libState = null, libBusy = false;
const libText = (m) => (m && (m[LANG] || m.en || m.de)) || "";

function renderLibrary() {
  if (!libState) return;
  const st = libState.status || {}, ps = libState.presets || [];
  $("#light-presets-count").textContent = ps.length ? t("light.presets.count", { n: ps.length }) : "";
  $("#light-lib-status").textContent = libBusy ? t("light.lib.loading")
    : st.cached ? t("light.lib.status", { when: new Date(st.updated).toLocaleString(LANG), c: st.components, p: st.presets }) : t("light.lib.never");
  $("#light-lib-refresh").disabled = libBusy;
  const err = $("#light-lib-error");
  const warns = (st.warnings || []).length;
  err.hidden = !(libState.error || warns);
  err.textContent = libState.error ? t("light.lib.error", { e: libState.error }) : warns ? t("light.lib.warn", { n: warns }) : "";
  const on = lv && lv.enabled && lv.active;
  $("#light-presets-list").replaceChildren(...(ps.length ? ps.map((p) => {
    const name = libText(p.name);
    const fx = lightLabel("fx", p.effect, p.effect);
    const card = el("button", { type: "button", class: "preset", title: libText(p.description), "aria-label": t("light.preset.apply", { n: name }),
      onclick: () => applyPreset(p) },
      el("span", { class: "preset-sw" }, ...(p.palette || []).slice(0, 6).map((c) => el("i", { style: { background: c } }))),
      el("b", {}, name), el("small", {}, fx + ((p.tags || []).length ? " · " + p.tags.join(", ") : "")));
    if (!on) card.classList.add("dim");
    return card;
  }) : [el("div", { class: "empty-note" }, t("light.presets.none"))]));
}

async function loadLibrary() {
  try { libState = await api("GET", "/api/library"); renderLibrary(); }
  catch (e) { /* the tab works without presets */ }
}

async function refreshLibrary() {
  if (libBusy) return;
  libBusy = true; renderLibrary();
  try { libState = await api("POST", "/api/library/refresh"); }
  catch (e) {
    // A failed download still answers with the saved copy.
    libState = { ...(libState || {}), error: e.message };
    try { const c = await api("GET", "/api/library"); libState = { ...c, error: e.message }; } catch { /* keep the old */ }
  }
  libBusy = false; renderLibrary();
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
$("#light-effect").addEventListener("change", (e) => {
  const name = e.target.value;
  const info = lv && lv.effects.find((x) => x.name === name);
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
$('[data-tab="light"]').addEventListener("click", loadLight);
for (const b of $$("[data-lang]")) b.addEventListener("click", () => { renderLight(); refreshLightChip(); renderLibrary(); refreshDiagTrace(); });

// Device states change by themselves (another program starts): keep them fresh.
setInterval(() => {
  if (document.hidden) return;
  if (lightActive() || (lv && lv.enabled)) loadLight();
}, 3000);
loadLight();
