"use strict";

// ---------- Case sketch editor ("Gehäuse-Skizze") ----------
// A full-width view in the lighting tab: the case is a box in millimetres,
// parts from the catalog are placed in it (position, facing, spin) and bound to
// LED zones. Five views (front, rear, side, top, bottom) show the same model.
// The server derives LED positions and zone segments from the saved sketch
// (PUT /api/lighting/sketch). Texts live here, like lighting.js.
Object.assign(STRINGS.de, {
  "sk.entry.title": "Gehäuse-Skizze",
  "sk.entry.sub": "Gehäuse mit Lüftern, RAM und LED-Streifen zeichnen und Teile den Zonen zuweisen. Der Effekt folgt der tatsächlichen LED-Position.",
  "sk.entry.none": "Noch keine Skizze gespeichert.",
  "sk.entry.some": "Gespeichert: {n} Teile, davon {b} an Zonen gebunden.",
  "sk.entry.some1": "Gespeichert: 1 Teil, an eine Zone gebunden.",
  "sk.entry.some0": "Gespeichert: 1 Teil, noch nicht gebunden.",
  "sk.open": "Skizze öffnen", "sk.open.edit": "Skizze bearbeiten",
  "sk.title": "Gehäuse-Skizze", "sk.sub": "Teile auf das Gehäuse ziehen, ausrichten und einer LED-Zone zuweisen. Gespeichert wird erst mit „Speichern“.",
  "sk.back": "Zurück", "sk.dirty": "Nicht gespeichert",
  "sk.save": "Speichern", "sk.discard": "Verwerfen", "sk.delete": "Skizze löschen",
  "sk.import": "Importieren", "sk.export": "Exportieren",
  "sk.undo": "Rückgängig (Strg+Z)", "sk.redo": "Wiederholen (Strg+Y)",
  "sk.saved": "Skizze gespeichert.", "sk.deleted": "Skizze gelöscht.", "sk.discarded": "Änderungen verworfen.",
  "sk.err.load": "Die Skizze konnte nicht geladen werden: {e}",
  "sk.err.save": "Speichern fehlgeschlagen: {e}", "sk.err.delete": "Löschen fehlgeschlagen: {e}",
  "sk.err.close": "Meldung schließen",
  "sk.leave": "Die Skizze hat ungespeicherte Änderungen. Sie gehen beim Verlassen verloren.",
  "sk.leave.title": "Ungespeicherte Änderungen", "sk.leave.ok": "Verlassen", "sk.import.replace": "Ersetzen",
  "sk.discard.confirm": "Alle ungespeicherten Änderungen verwerfen?",
  "sk.delete.confirm": "Die gespeicherte Skizze löschen? Gebundene Zonen fallen auf ihr Standard-Layout zurück.",
  "sk.delete.dirty": "Die ungespeicherten Änderungen gehen dabei ebenfalls verloren.",
  "sk.retry": "Erneut laden",
  "sk.zone.upto": "bis {n} LEDs", "sk.zone.any": "LED-Zahl frei",
  "sk.dims.moved": "{n} Teil(e) wurden zurück in den erlaubten Bereich geschoben.",
  "sk.palette": "Bauteile", "sk.search": "Hersteller, Modell, Kategorie …", "sk.stage": "Zeichenfläche der Gehäuse-Skizze",
  "sk.inspector": "Eigenschaften", "sk.views": "Ansicht",
  "sk.pal.none": "Kein Bauteil gefunden.", "sk.pal.add": "Bauteil „{n}“ hinzufügen",
  "sk.pal.count": "{n} LEDs", "sk.pal.var": "{a}–{b} LEDs",
  "sk.pal.generic": "Allgemein", "sk.approx": "ungefähr", "sk.approx.tip": "Die Maße und LED-Positionen dieses Bauteils sind nur geschätzt.",
  "sk.cat.fan": "Lüfter", "sk.cat.fan-lcd": "Lüfter mit Display", "sk.cat.ram": "Arbeitsspeicher", "sk.cat.strip": "LED-Streifen",
  "sk.cat.gpu-block": "Grafikkarte", "sk.cat.pump": "Pumpe / Kühler", "sk.cat.distro-plate": "Verteilerplatte (Distroplate)",
  "sk.cat.case-accessory": "Gehäuse-Zubehör", "sk.cat.cpu-block": "CPU-Kühler", "sk.cat.display": "Display", "sk.cat.mainboard": "Mainboard",
  "sk.cat.case-light": "Gehäusebeleuchtung", "sk.cat.cooler": "Kühler", "sk.cat.controller": "Controller", "sk.cat.other": "Sonstiges",
  "sk.part.generic-fan-120": "Lüfter 120 mm, LED-Ring", "sk.part.generic-fan-140": "Lüfter 140 mm, LED-Ring",
  "sk.part.generic-ram-dimm": "RAM-Riegel, LED-Leiste", "sk.part.generic-strip": "LED-Streifen",
  "sk.part.generic-gpu-box": "Grafikkarte, LED-Rahmen", "sk.part.generic-pump-cyl": "Pumpe, runde LED-Fläche",
  "sk.part.generic-distro-rect": "Verteilerplatte (Distroplate), LED-Rahmen",
  "sk.face.side": "Seitenansicht", "sk.face.top": "Deckel", "sk.face.bottom": "Boden", "sk.face.front": "Front", "sk.face.rear": "Rückseite", "sk.face.tray": "Trägerseite",
  "sk.face.side.long": "Seitenansicht: Innenraum von der Seite gesehen", "sk.face.top.long": "Deckel von oben", "sk.face.bottom.long": "Boden von unten",
  "sk.face.front.long": "Front von vorne", "sk.face.rear.long": "Rückseite von hinten", "sk.face.tray.long": "Trägerseite (Rückseite des Mainboard-Trägers) von außen",
  "sk.f.face": "Fläche", "sk.f.face.sub": "Auf dieser Fläche des aufgeklappten Gehäuses liegt das Teil; dort nimmt es die Farbe des Effekts auf.",
  "sk.live.face": "„{n}“ liegt jetzt auf „{f}“",
  "sk.facing.front": "Front", "sk.facing.rear": "Rückseite", "sk.facing.side": "Seitenansicht", "sk.facing.tray": "Trägerseite", "sk.facing.top": "Deckel", "sk.facing.bottom": "Boden",
  "sk.edge.rear": "Rückseite", "sk.edge.front": "Front", "sk.edge.top": "Deckel", "sk.edge.bottom": "Boden", "sk.edge.glass": "Seitenansicht", "sk.edge.tray": "Trägerseite",
  "sk.unbound.n": "{n} Teile ohne Zone", "sk.unbound.n1": "1 Teil ohne Zone",
  "sk.zoomIn": "Vergrößern", "sk.zoomOut": "Verkleinern", "sk.fit": "Einpassen",
  "sk.snap": "Raster fangen", "sk.grid": "Raster",
  "sk.readout.size": "{w} × {h} mm", "sk.readout.pos": "x {x} · y {y} mm",
  "sk.empty.title": "Keine Teile",
  "sk.empty.text": "Links ein Bauteil auf das Gehäuse ziehen oder anklicken. Teile erscheinen in allen Ansichten. Rechts Gehäuse und Maße einstellen, danach Teile einer Zone zuweisen.",
  "sk.led": "LED {n}", "sk.led.zone": "LED {n} · Zonen-LED {z}", "sk.led.first": "Erste LED",
  "sk.case": "Gehäuse", "sk.case.type": "Gehäuse wählen", "sk.case.custom": "Eigene Box",
  "sk.case.library": "Aus der Bibliothek", "sk.case.dims": "Außenmaße (mm)", "sk.case.w": "Breite", "sk.case.h": "Höhe", "sk.case.d": "Tiefe",
  "sk.case.fixed": "Die Maße kommen aus der Bibliothek.",
  "sk.case.divider": "Kammertrennung", "sk.case.divider.x": "Trennung bei (mm von links)",
  "sk.case.divider.hint": "Teilt die Ansichten in zwei Kammern.",
  "sk.case.glass": "Seitenansicht (von vorne gesehen)", "sk.case.glass.left": "Links", "sk.case.glass.right": "Rechts",
  "sk.case.chamber": "Kammer {n}",
  "sk.area.motherboard": "Mainboard-Träger", "sk.area.psu": "Netzteil", "sk.area.psu-shroud": "Netzteil-Abdeckung", "sk.area.drive": "Laufwerke", "sk.area.gpu-bay": "Grafikkarte", "sk.area.cable-area": "Kabel", "sk.area.chamber": "Kammer",
  "sk.parts": "Teile in der Skizze", "sk.parts.none": "Keine Teile.",
  "sk.parts.item": "{n} auswählen",
  "sk.bound": "gebunden", "sk.unbound": "nicht gebunden",
  "sk.warn.title": "Hinweise", "sk.warn.none": "Keine Hinweise.",
  "sk.w.segments": "{z}: {n} Teile in einer Zone, erlaubt sind {max}.",
  "sk.w.total": "Insgesamt {n} LEDs, erlaubt sind {max}.",
  "sk.w.zoneLeds": "Zone {z}: Die Kette braucht {n} LEDs, die Zone fasst höchstens {m}.",
  "sk.w.outside": "{p} liegt ganz oder teilweise außerhalb des Gehäuses.",
  "sk.w.unbound": "{p} ist an keine Zone gebunden und leuchtet nicht.",
  "sk.w.noZone": "{p}: Gerät oder Zone „{z}“ nicht vorhanden.",
  "sk.w.unknown": "{p}: Das Bauteil „{id}“ fehlt im Katalog.",
  "sk.w.leds": "{p}: LED-Zahl außerhalb von {a}–{b}.",
  "sk.part": "Teil", "sk.part.default": "Teil {n}",
  "sk.part.dup": "Duplizieren", "sk.part.del": "Löschen", "sk.part.size": "Größe: {w} × {h} mm, {n} LEDs", "sk.part.size3": "{w} × {h} × {d} mm · {n} LEDs",
  "sk.f.label": "Name", "sk.f.label.ph": "z. B. Front oben",
  "sk.f.pos": "Position (Mitte, mm)", "sk.f.x": "x", "sk.f.y": "y", "sk.f.z": "z",
  "sk.f.pos.hint": "Rechts: Abstand der Teilkanten zu den Wänden in mm. x ab Seitenansicht, y ab Deckel, z ab Rückseite. Bild ↑/↓ verschiebt in der Tiefe.",
  "sk.wall.x": "Seitenansicht {a}\nTrägerseite {b}", "sk.wall.y": "Deckel {a}\nBoden {b}", "sk.wall.z": "Rückseite {a}\nFront {b}",
  "sk.quick": "Schnellausrichtung", "sk.quick.vert": "Senkrecht", "sk.quick.horiz": "Waagerecht", "sk.quick.floor": "Am Boden",
  "sk.f.rot": "Drehung", "sk.f.rot.free": "Grad", "sk.f.rot.btn": "90° drehen", "sk.f.rot.sub": "Im Uhrzeigersinn, von der LED-Seite aus gesehen.",
  "sk.f.mirror": "Spiegeln", "sk.f.mirror.sub": "Dreht die Richtung der LED-Reihenfolge um.",
  "sk.cl.default": "Gehäuse-Beleuchtung", "sk.cl.top": "Leiste oben", "sk.cl.bottom": "Leiste unten", "sk.cl.front-leg": "Leiste vorne", "sk.cl.side-leg": "Leiste Seite",
  "sk.cl.short": "Gehäuse", "sk.cl.sub": "{f} · {n} LEDs", "sk.cl.fixed": "Teil des Gehäuses: Lage und Länge gibt die Gehäusevorlage vor. Zuordnen, Reihenfolge und LED-Richtung sind einstellbar.",
  "sk.cl.reverse": "LED-Richtung umkehren", "sk.cl.reverse.sub": "LED 1 liegt dann am anderen Ende der Leiste.",
  "sk.cl.nodelete": "Gehäuse-Beleuchtung gehört zum Gehäuse und kann nicht gelöscht werden.", "sk.cl.aria": "Gehäuse-Beleuchtung, {n} LEDs",
  "sk.f.facing": "LED-Seite zeigt nach", "sk.f.facing.sub": "Die Seite des Gehäuses, auf die die LEDs blicken.",
  "sk.f.flow": "Luftstrom", "sk.f.flow.intake": "Ansaugen", "sk.f.flow.exhaust": "Ausblasen",
  "sk.f.flow.hint": "Ausblasen dreht die LED-Reihenfolge des Rings.",
  "sk.f.leds": "LED-Anzahl", "sk.f.leds.range": "{a} bis {b}", "sk.f.leds.fixed": "{n} LEDs (fest)",
  "sk.f.len": "Länge (mm)", "sk.f.len.range": "{a} bis {b}",
  "sk.bind": "Anschluss", "sk.bind.device": "Gerät", "sk.bind.zone": "Zone", "sk.bind.none": "– nicht gebunden –",
  "sk.bind.gone": "{n} (nicht vorhanden)", "sk.bind.zone.pick": "– Zone wählen –",
  "sk.bind.nodev": "Keine Geräte vorhanden. Beleuchtung einschalten, um Teile zuzuweisen. Die Skizze lässt sich auch ohne Geräte zeichnen.",
  "sk.bind.order": "Reihenfolge in der Kette", "sk.bind.parallel": "Parallel zum Vorgänger",
  "sk.bind.parallel.sub": "Gleiche LEDs wie das Teil davor, zum Beispiel über einen Splitter.",
  "sk.bind.range": "Belegt in der Zone die LEDs {a} bis {b}.", "sk.bind.range1": "Belegt in der Zone die LED {a}.",
  "sk.test": "LED testen", "sk.test.go": "Testlicht", "sk.test.end": "Testlicht aus",
  "sk.test.prev": "Vorherige LED", "sk.test.next": "Nächste LED", "sk.test.pos": "LED {n} von {m}",
  "sk.test.hint": "Lässt die LED {n} dieses Teils kurz leuchten (Zonen-LED {z}). Es wird nichts dauerhaft geändert.",
  "sk.test.need": "Bauteil zuerst an eine Zone binden und die Skizze speichern, danach ist ein LED-Test möglich.",
  "sk.test.dirty": "Zuerst speichern, damit die Zone zur Skizze passt.",
  "sk.test.inactive": "Gerät nicht aktiv.",
  "sk.test.err": "Testlicht: {e}",
  "sk.orient": "Ausrichtung prüfen", "sk.orient.sub": "Zeigt ein Prüfmuster am Teil und korrigiert Drehung, Spiegelung oder Laufrichtung.",
  "sk.nothing": "Teil auf der Zeichenfläche auswählen, um es zu bearbeiten.",
  "sk.max.parts": "Mehr als {n} Teile sind nicht möglich.",
  "sk.live.added": "„{n}“ hinzugefügt", "sk.live.deleted": "„{n}“ gelöscht", "sk.live.sel": "„{n}“ ausgewählt",
  "sk.import.err": "Die Datei ist keine gültige Skizze: {e}", "sk.import.big": "Die Datei ist größer als 1 MiB.",
  "sk.import.ok": "Skizze importiert. Mit „Speichern“ übernehmen.", "sk.import.skipped": "{n} Teil(e) mit unbekanntem Bauteil oder ungültigen Angaben wurden übersprungen.",
  "sk.import.fixed": "{n} Angabe(n) wurden an die erlaubten Werte angepasst.",
  "sk.import.confirm": "Die aktuelle Skizze durch die Datei ersetzen?",
  "sk.import.v1": "Skizzen im alten Format (Version 1) lassen sich mit diesem Stand der Anwendung nicht importieren.",
  "sk.import.notes": "{n} Hinweis(e) zur Umstellung auf das neue Format, siehe „Hinweise“.",
  "sk.export.name": "casehud-skizze.json",
  "sk.hint": "Rad: zoomen · Leertaste + ziehen: verschieben · Umschalt: mehrere", "sk.hint.full": "Rad: zoomen · Leertaste + ziehen: verschieben · Umschalt + Klick oder Rahmen: mehrere wählen · Strg+G: verschweißen · Alt + Klick: einzelnes Teil · Alt beim Ziehen: Kollision übergehen · Pfeile: 1 mm (Umschalt: 10 mm) · Bild ↑/↓: Tiefe · R: drehen · Entf: löschen",
  "sk.ptab.parts": "Bauteile", "sk.ptab.layers": "Ebenen",
  "sk.layers.none": "Keine Teile.", "sk.layers.hint": "Obere Ebenen verdecken untere. Innerhalb einer Ebene liegt das nähere Teil vorn.",
  "sk.layer.show": "Einblenden", "sk.layer.hide": "Ausblenden", "sk.layer.up": "Eine Ebene nach oben", "sk.layer.down": "Eine Ebene nach unten", "sk.layer.n": "Ebene {n}",
  "sk.group": "Gruppe", "sk.group.n": "Gruppe {n}", "sk.group.count": "{n} Teile", "sk.group.select": "Gruppe wählen",
  "sk.group.leave": "Aus Gruppe nehmen", "sk.group.sub": "Teil von {g} mit {n} Teilen. Die Gruppe bewegt sich gemeinsam.",
  "sk.multi.title": "{n} Teile ausgewählt", "sk.multi.sub": "Die Auswahl bewegt sich gemeinsam.",
  "sk.weld": "Verschweißen", "sk.weld.tip": "Verschweißen (Strg+G)", "sk.unweld": "Gruppe lösen", "sk.unweld.tip": "Gruppe lösen (Strg+Umschalt+G)",
  "sk.rot.cw": "90° rechts", "sk.rot.ccw": "90° links", "sk.rot.tip": "Um 90° drehen (R, Umschalt+R)",
  "sk.arrange": "Anordnung", "sk.f.layer": "Ebene", "sk.f.visible": "Sichtbar", "sk.f.visible.sub": "Ausgeblendete Teile bleiben in der Skizze und leuchten weiter.",
  "sk.on": "Montiert auf", "sk.on.none": "– frei –", "sk.on.sub": "Das Teil bewegt sich mit seinem Träger und darf ihn überlappen.",
  "sk.w.overlap": "{a} und {b} überlappen sich.",
  "sk.collide.n": "{n} Überlappungen", "sk.collide.n1": "1 Überlappung",
  "sk.blocked": "Kein Platz: Das Teil bleibt stehen. Alt beim Ziehen übergeht die Prüfung.",
  "sk.live.sels": "{n} Teile ausgewählt", "sk.live.weld": "{n} Teile verschweißt", "sk.live.unweld": "Gruppe gelöst",
  "sk.live.rot": "Um 90° gedreht", "sk.live.hidden": "„{n}“ ausgeblendet", "sk.live.shown": "„{n}“ eingeblendet"
});
Object.assign(STRINGS.en, {
  "sk.entry.title": "Case sketch",
  "sk.entry.sub": "Draw the case with fans, RAM and LED strips and assign the parts to zones. The effect follows the actual LED positions.",
  "sk.entry.none": "No sketch saved yet.",
  "sk.entry.some": "Saved: {n} parts, {b} of them bound to zones.",
  "sk.entry.some1": "Saved: 1 part, bound to a zone.",
  "sk.entry.some0": "Saved: 1 part, not bound yet.",
  "sk.open": "Open sketch", "sk.open.edit": "Edit sketch",
  "sk.title": "Case sketch", "sk.sub": "Drag parts onto the case, rotate them and assign them to an LED zone. Nothing is stored until you press “Save”.",
  "sk.back": "Back", "sk.dirty": "Unsaved",
  "sk.save": "Save", "sk.discard": "Discard", "sk.delete": "Delete sketch",
  "sk.import": "Import", "sk.export": "Export",
  "sk.undo": "Undo (Ctrl+Z)", "sk.redo": "Redo (Ctrl+Y)",
  "sk.saved": "Sketch saved.", "sk.deleted": "Sketch deleted.", "sk.discarded": "Changes discarded.",
  "sk.err.load": "The sketch could not be loaded: {e}",
  "sk.err.save": "Saving failed: {e}", "sk.err.delete": "Deleting failed: {e}",
  "sk.err.close": "Close message",
  "sk.leave": "The sketch has unsaved changes. They are lost when you leave.",
  "sk.leave.title": "Unsaved changes", "sk.leave.ok": "Leave", "sk.import.replace": "Replace",
  "sk.discard.confirm": "Discard all unsaved changes?",
  "sk.delete.confirm": "Delete the saved sketch? Bound zones return to their default layout.",
  "sk.delete.dirty": "The unsaved changes will be lost as well.",
  "sk.retry": "Reload",
  "sk.zone.upto": "up to {n} LEDs", "sk.zone.any": "any LED count",
  "sk.dims.moved": "{n} part(s) were moved back into the allowed range.",
  "sk.palette": "Parts", "sk.search": "Vendor, model, category …", "sk.stage": "Case sketch drawing area",
  "sk.inspector": "Properties", "sk.views": "View",
  "sk.pal.none": "No part found.", "sk.pal.add": "Add part “{n}”",
  "sk.pal.count": "{n} LEDs", "sk.pal.var": "{a}–{b} LEDs",
  "sk.pal.generic": "Generic", "sk.approx": "approximate", "sk.approx.tip": "Size and LED positions of this part are estimates only.",
  "sk.cat.fan": "Fans", "sk.cat.fan-lcd": "Fans with display", "sk.cat.ram": "Memory", "sk.cat.strip": "LED strips",
  "sk.cat.gpu-block": "Graphics card", "sk.cat.pump": "Pump / cooler", "sk.cat.distro-plate": "Distribution plate (distro plate)",
  "sk.cat.case-accessory": "Case accessories", "sk.cat.cpu-block": "CPU cooler", "sk.cat.display": "Display", "sk.cat.mainboard": "Motherboard",
  "sk.cat.case-light": "Case lighting", "sk.cat.cooler": "Cooler", "sk.cat.controller": "Controller", "sk.cat.other": "Other",
  "sk.part.generic-fan-120": "Fan 120 mm, LED ring", "sk.part.generic-fan-140": "Fan 140 mm, LED ring",
  "sk.part.generic-ram-dimm": "RAM module, LED bar", "sk.part.generic-strip": "LED strip",
  "sk.part.generic-gpu-box": "Graphics card, LED frame", "sk.part.generic-pump-cyl": "Pump, round LED face",
  "sk.part.generic-distro-rect": "Distribution plate (distro plate), LED frame",
  "sk.face.side": "Side view", "sk.face.top": "Top", "sk.face.bottom": "Bottom", "sk.face.front": "Front", "sk.face.rear": "Rear", "sk.face.tray": "Back panel",
  "sk.face.side.long": "Side view: the interior seen from the side", "sk.face.top.long": "Top from above", "sk.face.bottom.long": "Bottom from below",
  "sk.face.front.long": "Front from the front", "sk.face.rear.long": "Rear from behind", "sk.face.tray.long": "Back panel (motherboard tray side) from outside",
  "sk.f.face": "Face", "sk.f.face.sub": "The face of the unfolded case the part lies on; it takes the effect colour there.",
  "sk.live.face": "“{n}” is now on “{f}”",
  "sk.facing.front": "Front", "sk.facing.rear": "Rear", "sk.facing.side": "Side view", "sk.facing.tray": "Back panel", "sk.facing.top": "Top", "sk.facing.bottom": "Bottom",
  "sk.edge.rear": "Rear", "sk.edge.front": "Front", "sk.edge.top": "Top", "sk.edge.bottom": "Bottom", "sk.edge.glass": "Side view", "sk.edge.tray": "Back panel",
  "sk.unbound.n": "{n} parts without a zone", "sk.unbound.n1": "1 part without a zone",
  "sk.zoomIn": "Zoom in", "sk.zoomOut": "Zoom out", "sk.fit": "Fit",
  "sk.snap": "Snap to grid", "sk.grid": "Grid",
  "sk.readout.size": "{w} × {h} mm", "sk.readout.pos": "x {x} · y {y} mm",
  "sk.empty.title": "No parts",
  "sk.empty.text": "Drag a part from the left onto the case or click it. Parts appear in every view. Set the case and its size on the right, then bind the parts to a zone.",
  "sk.led": "LED {n}", "sk.led.zone": "LED {n} · zone LED {z}", "sk.led.first": "First LED",
  "sk.case": "Case", "sk.case.type": "Choose case", "sk.case.custom": "Custom box",
  "sk.case.library": "From the library", "sk.case.dims": "Outer size (mm)", "sk.case.w": "Width", "sk.case.h": "Height", "sk.case.d": "Depth",
  "sk.case.fixed": "The size comes from the library.",
  "sk.case.divider": "Chamber divider", "sk.case.divider.x": "Divider at (mm from the left)",
  "sk.case.divider.hint": "Splits the views into two chambers.",
  "sk.case.glass": "Side view (seen from the front)", "sk.case.glass.left": "Left", "sk.case.glass.right": "Right",
  "sk.case.chamber": "Chamber {n}",
  "sk.area.motherboard": "Motherboard tray", "sk.area.psu": "Power supply", "sk.area.psu-shroud": "PSU shroud", "sk.area.drive": "Drives", "sk.area.gpu-bay": "Graphics card", "sk.area.cable-area": "Cables", "sk.area.chamber": "Chamber",
  "sk.parts": "Parts in the sketch", "sk.parts.none": "No parts.",
  "sk.parts.item": "Select {n}",
  "sk.bound": "bound", "sk.unbound": "not bound",
  "sk.warn.title": "Notes", "sk.warn.none": "No notes.",
  "sk.w.segments": "{z}: {n} parts in one zone, {max} are allowed.",
  "sk.w.total": "{n} LEDs in total, {max} are allowed.",
  "sk.w.zoneLeds": "Zone {z}: the chain needs {n} LEDs, the zone holds at most {m}.",
  "sk.w.outside": "{p} is partly or fully outside the case.",
  "sk.w.unbound": "{p} is not bound to a zone and stays dark.",
  "sk.w.noZone": "{p}: device or zone “{z}” not available.",
  "sk.w.unknown": "{p}: the part “{id}” is missing from the catalog.",
  "sk.w.leds": "{p}: LED count outside {a}–{b}.",
  "sk.part": "Part", "sk.part.default": "Part {n}",
  "sk.part.dup": "Duplicate", "sk.part.del": "Delete", "sk.part.size": "Size: {w} × {h} mm, {n} LEDs", "sk.part.size3": "{w} × {h} × {d} mm · {n} LEDs",
  "sk.f.label": "Name", "sk.f.label.ph": "e.g. front top",
  "sk.f.pos": "Position (centre, mm)", "sk.f.x": "x", "sk.f.y": "y", "sk.f.z": "z",
  "sk.f.pos.hint": "On the right: distance of the part edges to the walls in mm. x from the side view, y from the top, z from the rear. Page Up/Down moves along the depth.",
  "sk.wall.x": "Side view {a}\nBack panel {b}", "sk.wall.y": "Top {a}\nBottom {b}", "sk.wall.z": "Rear {a}\nFront {b}",
  "sk.quick": "Quick orientation", "sk.quick.vert": "Upright", "sk.quick.horiz": "Flat", "sk.quick.floor": "On the floor",
  "sk.f.rot": "Rotation", "sk.f.rot.free": "degrees", "sk.f.rot.btn": "Rotate 90°", "sk.f.rot.sub": "Clockwise, seen from the LED side.",
  "sk.f.mirror": "Mirror", "sk.f.mirror.sub": "Reverses the direction of the LED order.",
  "sk.cl.default": "Case lighting", "sk.cl.top": "Top strip", "sk.cl.bottom": "Bottom strip", "sk.cl.front-leg": "Front strip", "sk.cl.side-leg": "Side strip",
  "sk.cl.short": "Case", "sk.cl.sub": "{f} · {n} LEDs", "sk.cl.fixed": "Part of the case: position and length come from the case template. Binding, order and LED direction can be set.",
  "sk.cl.reverse": "Reverse LED direction", "sk.cl.reverse.sub": "LED 1 is then at the other end of the strip.",
  "sk.cl.nodelete": "Case lighting belongs to the case and cannot be deleted.", "sk.cl.aria": "Case lighting, {n} LEDs",
  "sk.f.facing": "LED side faces", "sk.f.facing.sub": "The side of the case the LEDs look at.",
  "sk.f.flow": "Air flow", "sk.f.flow.intake": "Intake", "sk.f.flow.exhaust": "Exhaust",
  "sk.f.flow.hint": "Exhaust reverses the LED order of the ring.",
  "sk.f.leds": "LED count", "sk.f.leds.range": "{a} to {b}", "sk.f.leds.fixed": "{n} LEDs (fixed)",
  "sk.f.len": "Length (mm)", "sk.f.len.range": "{a} to {b}",
  "sk.bind": "Connection", "sk.bind.device": "Device", "sk.bind.zone": "Zone", "sk.bind.none": "– not bound –",
  "sk.bind.gone": "{n} (not available)", "sk.bind.zone.pick": "– choose a zone –",
  "sk.bind.nodev": "No devices found. Turn lighting on to bind parts. The sketch can also be drawn without devices.",
  "sk.bind.order": "Order in the chain", "sk.bind.parallel": "In parallel with the previous part",
  "sk.bind.parallel.sub": "Same LEDs as the part before it, for example through a splitter.",
  "sk.bind.range": "Uses LEDs {a} to {b} of the zone.", "sk.bind.range1": "Uses LED {a} of the zone.",
  "sk.test": "Test LED", "sk.test.go": "Test light", "sk.test.end": "Test light off",
  "sk.test.prev": "Previous LED", "sk.test.next": "Next LED", "sk.test.pos": "LED {n} of {m}",
  "sk.test.hint": "Briefly lights LED {n} of this part (zone LED {z}). Nothing is changed permanently.",
  "sk.test.need": "Bind the part to a zone and save the sketch first; then an LED test is possible.",
  "sk.test.dirty": "Save first so the zone matches the sketch.",
  "sk.test.inactive": "Device not active.",
  "sk.test.err": "Test light: {e}",
  "sk.orient": "Check orientation", "sk.orient.sub": "Shows a test pattern on the part and corrects its rotation, mirroring or direction.",
  "sk.nothing": "Select a part on the drawing area to edit it.",
  "sk.max.parts": "More than {n} parts are not possible.",
  "sk.live.added": "“{n}” added", "sk.live.deleted": "“{n}” deleted", "sk.live.sel": "“{n}” selected",
  "sk.import.err": "The file is not a valid sketch: {e}", "sk.import.big": "The file is larger than 1 MiB.",
  "sk.import.ok": "Sketch imported. Press “Save” to keep it.", "sk.import.skipped": "{n} part(s) with an unknown part or invalid data were skipped.",
  "sk.import.fixed": "{n} value(s) were adjusted to the allowed range.",
  "sk.import.confirm": "Replace the current sketch with the file?",
  "sk.import.v1": "Sketches in the old format (version 1) cannot be imported with this build of the application.",
  "sk.import.notes": "{n} note(s) on the conversion to the new format, see “Notes”.",
  "sk.export.name": "casehud-sketch.json",
  "sk.hint": "Wheel: zoom · Space + drag: pan · Shift: select several", "sk.hint.full": "Wheel: zoom · Space + drag: pan · Shift + click or drag a frame: select several · Ctrl+G: weld · Alt + click: single part · Alt while dragging: ignore collisions · Arrows: 1 mm (Shift: 10 mm) · Page Up/Down: depth · R: rotate · Del: delete",
  "sk.ptab.parts": "Parts", "sk.ptab.layers": "Layers",
  "sk.layers.none": "No parts.", "sk.layers.hint": "Upper layers cover lower ones. Within a layer the nearer part is in front.",
  "sk.layer.show": "Show", "sk.layer.hide": "Hide", "sk.layer.up": "One layer up", "sk.layer.down": "One layer down", "sk.layer.n": "Layer {n}",
  "sk.group": "Group", "sk.group.n": "Group {n}", "sk.group.count": "{n} parts", "sk.group.select": "Select group",
  "sk.group.leave": "Remove from group", "sk.group.sub": "Part of {g} with {n} parts. The group moves as one.",
  "sk.multi.title": "{n} parts selected", "sk.multi.sub": "The selection moves as one.",
  "sk.weld": "Weld", "sk.weld.tip": "Weld (Ctrl+G)", "sk.unweld": "Ungroup", "sk.unweld.tip": "Ungroup (Ctrl+Shift+G)",
  "sk.rot.cw": "90° right", "sk.rot.ccw": "90° left", "sk.rot.tip": "Rotate by 90° (R, Shift+R)",
  "sk.arrange": "Arrangement", "sk.f.layer": "Layer", "sk.f.visible": "Visible", "sk.f.visible.sub": "Hidden parts stay in the sketch and keep lighting.",
  "sk.on": "Mounted on", "sk.on.none": "– free –", "sk.on.sub": "The part moves with its carrier and may overlap it.",
  "sk.w.overlap": "{a} and {b} overlap.",
  "sk.collide.n": "{n} overlaps", "sk.collide.n1": "1 overlap",
  "sk.blocked": "No room: the part stays put. Hold Alt while dragging to skip the check.",
  "sk.live.sels": "{n} parts selected", "sk.live.weld": "{n} parts welded", "sk.live.unweld": "Group released",
  "sk.live.rot": "Rotated by 90°", "sk.live.hidden": "“{n}” hidden", "sk.live.shown": "“{n}” shown"
});

applyI18n();

(() => {
  const SVGNS = "http://www.w3.org/2000/svg";
  const FACINGS = ["front", "rear", "side", "tray", "top", "bottom"];
  // The faces of the unfolded case: every part belongs to exactly one, the editor shows one at a time.
  const FACES = ["side", "top", "bottom", "front", "rear", "tray"];
  const MIN_SCALE = 0.15, MAX_SCALE = 40; // pixels per millimetre
  const GRID = 5; // mm
  const CAT_ORDER = ["fan", "fan-lcd", "ram", "strip", "gpu-block", "pump", "distro-plate", "case-light", "cooler"];

  /** @type {(tag: string, attrs?: any, ...kids: any[]) => any} */
  const E = el;
  /** @type {(tag: string, attrs?: any, ...kids: any[]) => any} */
  const sv = (tag, attrs = {}, ...kids) => {
    const n = document.createElementNS(SVGNS, tag);
    for (const [k, v] of Object.entries(attrs)) if (v !== false && v != null) n.setAttribute(k, String(v));
    for (const c of kids.flat()) if (c != null && c !== false) n.append(c);
    return n;
  };
  const r2 = (v) => Math.round(v * 100) / 100;
  const round6 = (v) => Math.round(v * 1e6) / 1e6 + 0;
  const BIG = 1e6; // far beyond any case; keeps imported numbers from overflowing
  const num = (v) => (Number.isFinite(+v) && v !== null && v !== "" && typeof v !== "boolean" ? Math.max(-BIG, Math.min(BIG, +v)) : 0);
  const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));
  const fmt = (v) => String(r2(v)).replace(".", LANG === "de" ? "," : ".");
  const AXES = ["x", "y", "z"];

  const S = {
    open: false, busy: false,
    cat: /** @type {any} */ (null), lim: { maxParts: 64, maxLabel: 40, maxLeds: 512, maxTotalLeds: 4096, maxDimMm: 2000, maxChambers: 4, marginMm: 50, maxSegments: 16, maxLayer: 100, maxSpin: 360, overlapTolMm: 0.5 },
    doc: /** @type {any} */ (null), saved: "", stored: false, notes: /** @type {{id: string, text: string}[]} */ ([]),
    sel: /** @type {string|null} */ (null), sels: /** @type {Set<string>} */ (new Set()), ptab: "parts", band: /** @type {any} */ (null), blocked: 0, view: "side",
    vp: /** @type {Record<string, any>} */ ({}), size: { w: 0, h: 0 },
    snap: true, grid: true, search: "", err: "",
    undo: /** @type {string[]} */ ([]), redo: /** @type {string[]} */ ([]), lastKey: "", lastAt: 0,
    space: false, drag: /** @type {any} */ (null), cursor: /** @type {any} */ (null),
    test: 0, testLive: false, insSel: /** @type {string|null} */ (null), raf: 0, insSig: "", insSync: /** @type {(() => void)[]} */ ([]),
    rev: 0, full: false, loadFailed: false, focus: /** @type {string|null} */ (null), wheel: /** @type {any} */ (null), drawn: /** @type {Map<string, {x: number, y: number}>} */ (new Map()), dimsKey: "",
  };
  const maxSegments = () => (S.lim.maxSegments > 0 ? S.lim.maxSegments : 16);
  const maxLayer = () => (S.lim.maxLayer > 0 ? S.lim.maxLayer : 100);
  const maxSpin = () => (S.lim.maxSpin > 0 ? S.lim.maxSpin : 360);
  const overlapTol = () => (S.lim.overlapTolMm > 0 ? S.lim.overlapTolMm : 0.5);
  const svgEl = () => $("#sk-svg");

  // ---------- document (version 2: parts in the case box, mm) ----------
  const newDoc = () => ({ version: 2, case: { source: "custom", id: "custom-box", dims: { w: 230, h: 480, d: 450 }, chambers: [] }, canvasView: "net", parts: [] });
  const SIDE_LEFT = ["rear", "front"], GLASS = ["left", "right"];
  const GROUP_RE = /^[a-z0-9-]{1,16}$/;
  const ID_RE = /^[a-z0-9][a-z0-9._-]{0,56}$/;

  /** Only the fields the server knows, numbers rounded: what is saved, exported and compared. */
  function cleanDoc(d) {
    const c = d.case, ori = c.orientation || {}, orient = {};
    if (ori.side_view_left === "front") orient.side_view_left = "front"; // the defaults (rear, left) are left out
    if (ori.glass_side === "right") orient.glass_side = "right";
    const kase = {
      source: c.source === "library" ? "library" : "custom", id: c.id || "",
      dims: { w: r2(num(c.dims.w)), h: r2(num(c.dims.h)), d: r2(num(c.dims.d)) },
      chambers: (c.chambers || []).map((q) => ({ x: r2(num(q.x)), w: r2(num(q.w)) })),
    };
    if (Object.keys(orient).length) kase.orientation = orient;
    return {
      version: 2,
      case: kase,
      canvasView: "net", // older sketches named a view; every sketch is mapped through the case net now
      parts: d.parts.map((p) => {
        const pos = p.pos || {};
        const o = {
          id: String(p.id), part: String(p.part), pos: { x: r2(num(pos.x)), y: r2(num(pos.y)), z: r2(num(pos.z)) },
          facing: FACINGS.includes(p.facing) ? p.facing : "side", ...(FACINGS.includes(p.face) ? { face: p.face } : {}), spin: r2(num(p.spin)), mirror: !!p.mirror, flow: p.flow || "",
          leds: Math.round(num(p.leds)), label: p.label || "",
        };
        if (num(p.len) > 0) o.len = r2(num(p.len));
        if (p.bind && p.bind.device && p.bind.zone) {
          o.bind = { device: p.bind.device, zone: p.bind.zone, order: Math.round(num(p.bind.order)), parallel: !!p.bind.parallel };
        }
        if (p.on) o.on = String(p.on);
        if (p.group) o.group = String(p.group);
        if (Math.round(num(p.layer)) !== 0) o.layer = Math.round(num(p.layer));
        if (p.hidden) o.hidden = true;
        if (isCL(p)) { // geometry from the case; the server takes it from the template again
          o.caseLight = String(p.caseLight);
          o.path = p.path.map((q) => ({ x: r2(num(q.x)), y: r2(num(q.y)), z: r2(num(q.z)) }));
          delete o.face; delete o.len; delete o.on; delete o.group;
        }
        return o;
      }),
    };
  }
  const docJSON = () => JSON.stringify(cleanDoc(S.doc));
  const isDirty = () => !!S.doc && docJSON() !== S.saved;
  const parts = () => S.doc.parts;
  /** The parts the editor draws: those on the face it shows. */
  const shown = () => parts().filter((p) => !p.hidden && facesOf(p).includes(S.view));
  const partById = (id) => parts().find((p) => p.id === id);
  const selPart = () => (S.sel ? partById(S.sel) : null);
  const selIds = () => [...S.sels];

  /** Records the state before a change. Changes with the same key in quick succession share one undo step. */
  function mutate(fn, key = "") {
    const now = Date.now();
    if (!(key && key === S.lastKey && now - S.lastAt < 900)) {
      S.undo.push(JSON.stringify(S.doc));
      if (S.undo.length > 100) S.undo.shift();
      S.redo = [];
    }
    S.lastKey = key; S.lastAt = now;
    fn();
    afterChange();
  }
  function restore(json) {
    S.doc = JSON.parse(json);
    S.insSig = ""; // the inspector fields point into the old document
    pruneSel();
    endTest();
    S.lastKey = "";
    afterChange();
  }
  function undo() { if (S.undo.length) { S.redo.push(JSON.stringify(S.doc)); restore(S.undo.pop()); } }
  function redo() { if (S.redo.length) { S.undo.push(JSON.stringify(S.doc)); restore(S.redo.pop()); } }

  // ---------- catalog geometry (mirrors internal/sketch) ----------
  const partOf = (id) => (S.cat && typeof id === "string" && Object.hasOwn(S.cat.parts, id) ? S.cat.parts[id] : null);
  const caseOf = (id) => (S.cat && typeof id === "string" && Object.hasOwn(S.cat.cases, id) ? S.cat.cases[id] : null);
  const isFan = (p) => !!p && (p.category === "fan" || p.category === "fan-lcd");
  const gen = (p) => (p && p.leds && p.leds.generator) || null;
  const isApprox = (p) => !!(p && ((p.dimensions_mm && p.dimensions_mm.approximate) || (p.outline && p.outline.approximate) || (p.leds && p.leds.approximate)));
  const isGeneric = (p) => !!p && (p.vendor === "Generic" || String(p.id).startsWith("generic-"));
  function partName(p) {
    const k = "sk.part." + p.id;
    const own = (Object.hasOwn(STRINGS[LANG], k) && STRINGS[LANG][k]) || (Object.hasOwn(STRINGS.de, k) && STRINGS.de[k]);
    if (own) return own;
    return isGeneric(p) ? p.model : [p.vendor, p.model].filter(Boolean).join(" ");
  }
  const catName = (c) => (Object.hasOwn(STRINGS[LANG], "sk.cat." + c) && STRINGS[LANG]["sk.cat." + c]) || (Object.hasOwn(STRINGS.de, "sk.cat." + c) && STRINGS.de["sk.cat." + c]) || (c.charAt(0).toUpperCase() + c.slice(1).replace(/-/g, " "));
  const caseName = (c) => (c.vendor && !String(c.name || "").toLowerCase().startsWith(c.vendor.toLowerCase()) ? c.vendor + " " : "") + (c.name || c.id);

  /** The concrete part for an LED count and length: positions, size and outline. */
  function resolve(part, leds, len) {
    const g = gen(part);
    const d = part.dimensions_mm;
    let w = d.w, h = d.h, front = part.outline.front || [];
    let pos = part.leds.positions_mm || [];
    if (g) {
      const n = clamp(Math.round(num(leds)) || g.default, g.min, g.max);
      if (g.lengthMax > 0 && num(len) > 0) {
        w = clamp(num(len), g.lengthMin, g.lengthMax);
        front = [{ type: "rect", w, h, r: Math.min(h / 2, 4), role: "body" }];
      }
      pos = [];
      const inset = g.inset || 0;
      for (let i = 0; i < n; i++) {
        if (g.kind === "ring") {
          const a = (2 * Math.PI * i) / n;
          pos.push([w / 2 + g.radius * Math.sin(a), h / 2 - g.radius * Math.cos(a)]);
        } else if (g.kind === "line") {
          const f = n > 1 ? i / (n - 1) : 0.5;
          if (g.axis === "y") pos.push([g.offset > 0 ? g.offset : w / 2, inset + (h - 2 * inset) * f]);
          else pos.push([inset + (w - 2 * inset) * f, g.offset > 0 ? g.offset : h / 2]);
        } else if (g.kind === "rect") {
          const x0 = inset, y0 = inset, x1 = w - inset, y1 = h - inset, rw = x1 - x0, rh = y1 - y0;
          const per = 2 * (rw + rh), dd = (per * i) / n;
          if (dd <= rw) pos.push([x0 + dd, y0]);
          else if (dd <= rw + rh) pos.push([x1, y0 + dd - rw]);
          else if (dd <= 2 * rw + rh) pos.push([x1 - (dd - rw - rh), y1]);
          else pos.push([x0, y1 - (dd - 2 * rw - rh)]);
        }
      }
      pos = pos.map(([x, y]) => [clamp(x, 0, w), clamp(y, 0, h)]);
    }
    return { w, h, d: d.d, front, pos };
  }

  // ---------- 3D placement (JS copy of internal/sketch/orient.go and project.go) ----------
  // Frame of a facing: U (right) and V (down) seen from the facing side, N the outward normal of the LED side.
  const FRAMES = {
    front: { U: [1, 0, 0], V: [0, 1, 0], N: [0, 0, 1] },
    rear: { U: [-1, 0, 0], V: [0, 1, 0], N: [0, 0, -1] },
    side: { U: [0, 0, 1], V: [0, 1, 0], N: [-1, 0, 0] },
    tray: { U: [0, 0, -1], V: [0, 1, 0], N: [1, 0, 0] },
    top: { U: [1, 0, 0], V: [0, 0, 1], N: [0, -1, 0] },
    bottom: { U: [1, 0, 0], V: [0, 0, -1], N: [0, 1, 0] },
  };
  /** Frame of a facing; with the glass on the right "right" is the other way round (see FrameIn in internal/sketch). */
  const frameIn = (facing, glass) => {
    const fr = FRAMES[facing];
    return fr && glass ? { U: fr.U.map((v) => -v + 0), V: fr.V, N: fr.N } : fr;
  };
  /** Case as the projection needs it: dims {w, h, d} and which side the glass is on. */
  const curCase = () => ({ dims: S.doc.case.dims, glassRight: !!(S.doc.case.orientation && S.doc.case.orientation.glass_side === "right") });
  const glassRight = () => !!S.doc && curCase().glassRight;

  /** A point of the case box in a view: u right, v down, depth away from the viewer. */
  function toViewC(c, view, p) {
    const d = c.dims, fx = c.glassRight ? d.w - p.x : p.x;
    switch (view) {
      case "side": return { u: p.z, v: p.y, depth: p.x };
      case "front": return { u: fx, v: p.y, depth: d.d - p.z };
      case "rear": return { u: d.w - fx, v: p.y, depth: p.z };
      case "top": return { u: fx, v: p.z, depth: p.y };
      case "bottom": return { u: fx, v: d.d - p.z, depth: d.h - p.y };
    }
    return { u: 0, v: 0, depth: 0 };
  }
  /** Inverse of toViewC for the two coordinates a view shows; free is the axis it does not determine (set to 0). */
  function fromViewC(c, view, u, v) {
    const d = c.dims, fromFx = (fx) => (c.glassRight ? d.w - fx : fx);
    switch (view) {
      case "side": return { p: { x: 0, y: v, z: u }, free: "x" };
      case "front": return { p: { x: fromFx(u), y: v, z: 0 }, free: "z" };
      case "rear": return { p: { x: fromFx(d.w - u), y: v, z: 0 }, free: "z" };
      case "top": return { p: { x: fromFx(u), y: 0, z: v }, free: "y" };
      case "bottom": return { p: { x: fromFx(u), y: 0, z: d.d - v }, free: "y" };
    }
    return { p: { x: 0, y: 0, z: 0 }, free: "z" };
  }

  /** Size of a face of the case (internal/sketch FaceSize): side and tray D x H, front and rear W x H, top and bottom D x W. */
  function faceSize(d, face) {
    return face === "side" || face === "tray" ? { w: d.d, h: d.h } : face === "top" || face === "bottom" ? { w: d.d, h: d.w } : { w: d.w, h: d.h };
  }
  /**
   * A point of the case box on a face, as the face lies in the case net (internal/sketch ToFace): u right, v down,
   * depth away from someone looking at the face from outside. With the glass on the right the net is mirrored.
   */
  function toFaceC(c, face, p) {
    const d = c.dims;
    let q;
    switch (face) {
      case "side": q = { u: p.z, v: p.y, depth: p.x }; break;
      case "front": q = { u: p.x, v: p.y, depth: d.d - p.z }; break;
      case "rear": q = { u: d.w - p.x, v: p.y, depth: p.z }; break;
      case "tray": q = { u: d.d - p.z, v: p.y, depth: d.w - p.x }; break;
      case "top": q = { u: p.z, v: d.w - p.x, depth: p.y }; break;
      case "bottom": q = { u: p.z, v: p.x, depth: d.h - p.y }; break;
      default: return { u: 0, v: 0, depth: 0 };
    }
    if (c.glassRight) q.u = faceSize(d, face).w - q.u;
    return q;
  }
  /** Inverse of toFaceC for the two coordinates a face shows; free is the axis it does not determine (set to 0). */
  function fromFaceC(c, face, u, v) {
    const d = c.dims;
    if (c.glassRight) u = faceSize(d, face).w - u;
    switch (face) {
      case "side": return { p: { x: 0, y: v, z: u }, free: "x" };
      case "front": return { p: { x: u, y: v, z: 0 }, free: "z" };
      case "rear": return { p: { x: d.w - u, y: v, z: 0 }, free: "z" };
      case "tray": return { p: { x: 0, y: v, z: d.d - u }, free: "x" };
      case "top": return { p: { x: d.w - v, y: 0, z: u }, free: "y" };
      case "bottom": return { p: { x: v, y: 0, z: u }, free: "y" };
    }
    return { p: { x: 0, y: 0, z: 0 }, free: "x" };
  }
  const OPPOSITE = { side: "tray", tray: "side", top: "bottom", bottom: "top", rear: "front", front: "rear" };
  /** Distance from a box to the wall on the facing side f (0 when it touches or crosses it). */
  function wallDist(d, p3, f) {
    const v = { side: p3.min.x, tray: d.w - p3.max.x, top: p3.min.y, bottom: d.h - p3.max.y, rear: p3.min.z, front: d.d - p3.max.z }[f];
    return Math.max(v ?? 0, 0);
  }
  /** The face a placement belongs to without an explicit one (the rule of internal/sketch DefaultFace). */
  function defaultFace(part, pl, p3, dims) {
    if (!p3) return "side";
    if (wallDist(dims, p3, pl.facing) <= 40) return pl.facing;
    const w = OPPOSITE[pl.facing];
    if (isFan(part) && w && w !== "tray" && wallDist(dims, p3, w) <= 40) return w;
    return "side";
  }

  // ---------- zones with one colour per fan ring (internal/sketch rings.go) ----------
  /** The ports of the TL fan controller take two LEDs per fan: the top ring, then the bottom ring. */
  function ringZone(dev, zone) {
    if (!/^port\d+$/.test(zone || "")) return false;
    const d = typeof lv !== "undefined" && lv && lv.devices ? lv.devices.find((x) => x.id === dev) : null;
    return d ? d.backend === "lianlitl" : String(dev).startsWith("lianlitl:") || dev === "demo:fans";
  }
  /** The ring of every LED (0 top, 1 bottom: above or below the middle of the part's front view) and the ring count. */
  function ringsOf(part, pl, n) {
    const groups = new Array(n).fill(0), pos = (part && part.leds && part.leds.positions_mm) || [];
    if (!n) return { groups, rings: 0 };
    if (gen(part) || pos.length !== n) {
      if (n === 1) return { groups, rings: 1 };
      for (let i = Math.floor(n / 2); i < n; i++) groups[i] = 1;
      return { groups, rings: 2 };
    }
    const mid = num(part.dimensions_mm && part.dimensions_mm.h) / 2;
    let top = false, bottom = false;
    pos.forEach((q, i) => { if (q[1] < mid) top = true; else { groups[i] = 1; bottom = true; } });
    if (top && bottom) return { groups, rings: 2 };
    return { groups: groups.fill(0), rings: 1 };
  }

  // ---------- lighting built into the case (internal/sketch caselight.go) ----------
  /** A placement that belongs to the case: it runs along a path and cannot be moved or deleted. */
  const isCL = (p) => !!(p && typeof p.caseLight === "string" && p.caseLight && Array.isArray(p.path) && p.path.length >= 2);
  const WALL_ORDER = ["side", "front", "rear", "top", "bottom", "tray"];
  const pointWall = (d, p, f) => Math.abs({ side: p.x, tray: d.w - p.x, top: p.y, bottom: d.h - p.y, rear: p.z, front: d.d - p.z }[f]);
  /** The face a path segment runs on: the wall both ends are nearest to (the glass side wins a tie). */
  function segFace(d, a, b) {
    let best = "side", bd = Infinity;
    for (const f of WALL_ORDER) { const v = Math.max(pointWall(d, a, f), pointWall(d, b, f)); if (v < bd - 1e-9) { best = f; bd = v; } }
    return best;
  }
  const clCount = (part, pl) => (num(pl.leds) > 0 ? Math.round(num(pl.leds)) : gen(part) ? gen(part).default : (part.leds.positions_mm || []).length);
  /** LEDs spread evenly along the path in wire order (reversed by mirror), each on the face of its segment; the segments. */
  function pathInfo(part, pl, d) {
    const path = pl.path.map((q) => ({ x: num(q.x), y: num(q.y), z: num(q.z) })), n = clCount(part, pl);
    const segs = [], len = [];
    let total = 0;
    for (let i = 0; i + 1 < path.length; i++) {
      const a = path[i], b = path[i + 1], l = Math.hypot(b.x - a.x, b.y - a.y, b.z - a.z);
      segs.push({ a, b, face: segFace(d, a, b) }); len.push(l); total += l;
    }
    const leds = [];
    let s = 0, start = 0;
    for (let i = 0; i < n; i++) {
      if (!total) { leds.push({ p: path[0], face: segs[0].face }); continue; }
      const tt = ((i + 0.5) * total) / n;
      while (s < segs.length - 1 && (len[s] === 0 || tt > start + len[s])) { start += len[s]; s++; }
      const { a, b } = segs[s], f = len[s] > 0 ? clamp((tt - start) / len[s], 0, 1) : 0;
      leds.push({ p: { x: round6(a.x + (b.x - a.x) * f), y: round6(a.y + (b.y - a.y) * f), z: round6(a.z + (b.z - a.z) * f) }, face: segs[s].face });
    }
    if (pl.mirror) leds.reverse();
    const faces = [...new Set(leds.map((l) => l.face))];
    return { leds, segs, faces: faces.length ? faces : ["side"], width: Math.max(num(part.dimensions_mm && part.dimensions_mm.h), 2) };
  }
  /** The name of a case light: own translation, the case template's name, a generic one. */
  function clName(cat, pl) {
    const k = "sk.cl." + pl.caseLight;
    if (Object.hasOwn(STRINGS[LANG], k)) return STRINGS[LANG][k];
    for (const c of Object.values((cat && cat.cases) || {})) {
      const l = (c.lighting || []).find((q) => q.caseLight === pl.caseLight && q.part === pl.part);
      if (l && l.name) return l.name;
    }
    return t("sk.cl.default");
  }
  /** The case lights of a template as the server places them (GET /api/lighting/catalog). */
  const tplLights = (tpl) => (tpl && Array.isArray(tpl.lighting) ? tpl.lighting : []);
  /**
   * Brings the case lights of a document in line with its case (internal/sketch SyncCaseLights): lights of another case
   * go, missing ones are added unbound, the geometry comes from the template; binding, label, direction, layer and
   * visibility stay. A library case this catalog does not know keeps its lights.
   */
  function syncCaseLights(doc, catCases) {
    const lib = doc.case.source === "library";
    const tpl = lib && catCases && Object.hasOwn(catCases, doc.case.id) ? catCases[doc.case.id] : null;
    if (lib && !tpl) return;
    const want = tplLights(tpl), old = new Map(doc.parts.filter(isCL).map((p) => [p.caseLight, p]));
    const rest = doc.parts.filter((p) => !isCL(p)), ids = new Set(rest.map((p) => p.id));
    for (const p of rest) if (p.on && !ids.has(p.on)) delete p.on;
    const add = [];
    for (const w of want) {
      const o = old.get(w.caseLight);
      const p = { id: w.id, part: w.part, pos: { ...w.pos }, facing: w.facing, spin: 0, mirror: false, flow: "", leds: w.leds, label: "", caseLight: w.caseLight, path: w.path.map((q) => ({ ...q })) };
      if (o) { p.id = o.id; p.label = o.label || ""; p.mirror = !!o.mirror; if (o.bind) p.bind = o.bind; if (o.layer) p.layer = o.layer; if (o.hidden) p.hidden = true; }
      else for (let n = 2; ids.has(p.id); n++) p.id = w.id + "-" + n;
      if (ids.has(p.id)) continue;
      ids.add(p.id); add.push(p);
    }
    doc.parts = [...rest, ...add];
  }

  /** Steps shared by the projections: resolved part, mirror and spin, and the mapping of a part point to the case box. */
  function layout3(part, pl, glass = glassRight()) {
    const fr = frameIn(pl.facing, glass);
    const pos = pl.pos || {};
    if (!fr || ![pos.x, pos.y, pos.z, pl.spin].every((v) => Number.isFinite(v))) return null;
    const res = resolve(part, pl.leds, pl.len);
    const { w, h, d } = res;
    const mirror = !!pl.mirror !== (pl.flow === "exhaust" && isFan(part));
    const rad = (num(pl.spin) * Math.PI) / 180, sin = Math.sin(rad), cos = Math.cos(rad);
    const tf = (a, b) => {
      if (mirror) a = w - a;
      const dx = a - w / 2, dy = b - h / 2;
      return [round6(dx * cos - dy * sin), round6(dx * sin + dy * cos)];
    };
    const at = (du, dv, dn) => ({
      x: pos.x + du * fr.U[0] + dv * fr.V[0] + dn * fr.N[0],
      y: pos.y + du * fr.U[1] + dv * fr.V[1] + dn * fr.N[1],
      z: pos.z + du * fr.U[2] + dv * fr.V[2] + dn * fr.N[2],
    });
    return { fr, res, w, h, d, tf, at };
  }
  const roundV = (v) => ({ x: round6(v.x), y: round6(v.y), z: round6(v.z) });
  /** Places a part in the case box: LEDs in wire order, the box of the eight turned corners, the normal of the LED side. */
  function project3d(part, pl, lay, glass) {
    lay = lay || layout3(part, pl, glass);
    if (!lay) return null;
    const { fr, res, w, h, d, tf, at } = lay;
    let min = null, max = null;
    for (const c of [[0, 0], [w, 0], [w, h], [0, h]]) {
      const [du, dv] = tf(c[0], c[1]);
      for (const dn of [-d / 2, d / 2]) {
        const q = at(du, dv, dn);
        if (!min) { min = { ...q }; max = { ...q }; continue; }
        min = { x: Math.min(min.x, q.x), y: Math.min(min.y, q.y), z: Math.min(min.z, q.z) };
        max = { x: Math.max(max.x, q.x), y: Math.max(max.y, q.y), z: Math.max(max.z, q.z) };
      }
    }
    min = roundV(min); max = roundV(max);
    const leds = res.pos.map((q) => {
      const [du, dv] = tf(q[0], q[1]);
      const p = roundV(at(du, dv, d / 2));
      return { x: clamp(p.x, min.x, max.x), y: clamp(p.y, min.y, max.y), z: clamp(p.z, min.z, max.z) };
    });
    return { leds, min, max, normal: { x: fr.N[0], y: fr.N[1], z: fr.N[2] } };
  }
  /** A projected 3D placement seen in a view: flat (LED side towards the viewer), away (seen from behind) or edge. */
  function viewOf(p3, c, view, toC = toViewC) {
    const vn = FRAMES[view].N, n = p3.normal;
    const dot = n.x * vn[0] + n.y * vn[1] + n.z * vn[2];
    let minU = Infinity, minV = Infinity, maxU = -Infinity, maxV = -Infinity;
    for (const x of [p3.min.x, p3.max.x]) for (const y of [p3.min.y, p3.max.y]) for (const z of [p3.min.z, p3.max.z]) {
      const q = toC(c, view, { x, y, z });
      minU = Math.min(minU, q.u); maxU = Math.max(maxU, q.u); minV = Math.min(minV, q.v); maxV = Math.max(maxV, q.v);
    }
    const depth = [];
    const leds = p3.leds.map((l) => {
      const q = toC(c, view, l);
      depth.push(round6(q.depth));
      return { x: round6(clamp(q.u, minU, maxU)), y: round6(clamp(q.v, minV, maxV)) };
    });
    return { leds, depth, box: { x: round6(minU), y: round6(minV), w: round6(maxU - minU), h: round6(maxV - minV) }, kind: dot === 0 ? "edge" : dot < 0 ? "away" : "flat" };
  }
  function projectViewC(part, pl, c, view) {
    const p3 = project3d(part, pl, undefined, c.glassRight);
    return p3 ? viewOf(p3, c, view) : null;
  }
  // The pure maths, for the headless check against internal/sketch/testdata/project3d_golden.json.
  window.SketchGeo = { project3d, projectViewC, toViewC, fromViewC, toFaceC, fromFaceC, faceSize, resolve, overlapPairs: (pls, partFn, tol) => overlapPairs(pls, partFn, tol), rotatePlacement: (...a) => rotatePlacement(...a), turner };

  const MISSING = () => ({ missing: true, kind: "edge", leds: [], depth: [], box: { x: 0, y: 0, w: 40, h: 40 }, n: 0, xf: "", shapes: null, det: 1, centre: 0, d3: null });
  /** Projections are cached per placement object and invalidated when one of its inputs changes. */
  let projCache = new WeakMap();
  const caseSig = () => { const c = curCase(); return [c.dims.w, c.dims.h, c.dims.d, c.glassRight].join("|"); };
  function entryOf(pl) {
    const cl = isCL(pl);
    const sig = [pl.part, pl.facing, pl.pos && pl.pos.x, pl.pos && pl.pos.y, pl.pos && pl.pos.z, pl.spin, pl.mirror, pl.flow, pl.leds, pl.len, glassRight(), cl ? caseSig() + JSON.stringify(pl.path) : ""].join("|");
    let c = projCache.get(pl);
    if (!c || c.sig !== sig) {
      const part = partOf(pl.part);
      if (cl) c = { sig, part, lay: null, p3: null, path: part ? pathInfo(part, pl, S.doc.case.dims) : null, views: {}, cs: "" };
      else {
        const lay = part && layout3(part, pl), p3 = lay && project3d(part, pl, lay);
        c = { sig, part, lay, p3, views: {}, cs: "" };
      }
      projCache.set(pl, c);
    }
    return c;
  }
  /** The part in the box: LEDs in 3D, min/max and normal (null when the part is unknown or the numbers are unusable). */
  const p3of = (pl) => entryOf(pl).p3;
  /** The part in a view: LEDs in view millimetres, box, kind, the matrix for the outline and its shapes. */
  function proj(pl, view = S.view) {
    const e = entryOf(pl), cs = caseSig();
    if (e.cs !== cs) { e.views = {}; e.cs = cs; }
    if (e.views[view]) return e.views[view];
    let out;
    if (e.path) out = projPath(e.path, view);
    else if (!e.p3) out = MISSING();
    else {
      const c = curCase(), v = viewOf(e.p3, c, view, toFaceC), { lay } = e;
      const map = (a, b) => { const [du, dv] = lay.tf(a, b); const q = toFaceC(c, view, lay.at(du, dv, lay.d / 2)); return [q.u, q.v]; };
      const m0 = map(0, 0), mx = map(lay.w, 0), my = map(0, lay.h);
      const A = (mx[0] - m0[0]) / lay.w, B = (mx[1] - m0[1]) / lay.w, C = (my[0] - m0[0]) / lay.h, D = (my[1] - m0[1]) / lay.h;
      const mid = toFaceC(c, view, { x: (e.p3.min.x + e.p3.max.x) / 2, y: (e.p3.min.y + e.p3.max.y) / 2, z: (e.p3.min.z + e.p3.max.z) / 2 });
      out = { ...v, n: v.leds.length, xf: `matrix(${A} ${B} ${C} ${D} ${m0[0]} ${m0[1]})`, shapes: v.kind === "edge" ? null : lay.res.front, det: A * D - B * C, centre: mid.depth, d3: lay };
    }
    e.views[view] = out;
    return out;
  }
  /** A case light on one face: its LEDs there (idx: their place in the wire order) and the strip's runs on that face. */
  function projPath(info, view) {
    const c = curCase(), at = (p) => { const q = toFaceC(c, view, p); return { x: round6(q.u), y: round6(q.v) }; };
    const leds = [], idx = [], lines = [];
    info.leds.forEach((l, i) => { if (l.face === view) { leds.push(at(l.p)); idx.push(i); } });
    for (const s of info.segs) {
      if (s.face !== view) continue;
      const a = at(s.a), b = at(s.b), last = lines.at(-1), end = last && last.at(-1);
      if (end && Math.abs(end.x - a.x) < 1e-6 && Math.abs(end.y - a.y) < 1e-6) last.push(b); else lines.push([a, b]);
    }
    const pts = [...lines.flat(), ...leds], h = info.width / 2;
    if (!pts.length) return { ...MISSING(), missing: false, kind: "path", lines: [], idx: [], width: info.width, n: info.leds.length, box: { x: 0, y: 0, w: 0, h: 0 } };
    const x0 = Math.min(...pts.map((q) => q.x)) - h, y0 = Math.min(...pts.map((q) => q.y)) - h;
    const x1 = Math.max(...pts.map((q) => q.x)) + h, y1 = Math.max(...pts.map((q) => q.y)) + h;
    return { missing: false, kind: "path", leds, idx, lines, width: info.width, depth: [], box: { x: round6(x0), y: round6(y0), w: round6(x1 - x0), h: round6(y1 - y0) }, n: info.leds.length, xf: "", shapes: null, det: 1, centre: 0, d3: null };
  }
  const ledCount = (pl) => { const e = entryOf(pl); return e.path ? e.path.leds.length : e.p3 ? e.p3.leds.length : 0; };
  /** Every face a placement is drawn on: one, or all those a case light runs over. */
  const facesOf = (pl) => { const e = isCL(pl) && entryOf(pl); return e && e.path ? e.path.faces : [faceOf(pl)]; };

  const viewSize = (face) => faceSize(S.doc.case.dims, face);
  /** The face of a placement: its own or the default one. */
  function faceOf(pl) {
    if (isCL(pl)) { const e = entryOf(pl); return e.path ? e.path.faces[0] : "side"; }
    if (FACINGS.includes(pl.face)) return pl.face;
    const e = entryOf(pl), dm = S.doc.case.dims, sig = e.sig + "|" + [dm.w, dm.h, dm.d].join("|");
    if (e.fsig !== sig) { e.fsig = sig; e.face = e.part && FACINGS.includes(pl.facing) ? defaultFace(e.part, pl, e.p3, dm) : "side"; }
    return e.face;
  }
  /** Axis a face looks along and the direction in which that coordinate grows away from the viewer. */
  function depthAxis(view) {
    const c = curCase(), o = toFaceC(c, view, { x: 0, y: 0, z: 0 }).depth;
    for (const a of AXES) {
      const dd = toFaceC(c, view, { x: a === "x" ? 1 : 0, y: a === "y" ? 1 : 0, z: a === "z" ? 1 : 0 }).depth - o;
      if (dd !== 0) return { axis: a, sign: dd > 0 ? 1 : -1 };
    }
    return { axis: "x", sign: 1 };
  }

  // ---------- placement helpers ----------
  const dimOf = (axis) => S.doc.case.dims[axis === "x" ? "w" : axis === "y" ? "h" : "d"];
  /** Keeps the box of a part within the case plus the allowed margin; the box shape does not depend on the position. */
  function clampInto(p) {
    const q = p3of(p);
    if (!q) return;
    const m = S.lim.marginMm;
    for (const a of AXES) {
      const half = (q.max[a] - q.min[a]) / 2, lo = -m + half, hi = dimOf(a) + m - half;
      p.pos[a] = r2(lo <= hi ? clamp(p.pos[a], lo, hi) : dimOf(a) / 2);
    }
  }
  function setPos(p, pos) { p.pos = { x: pos.x, y: pos.y, z: pos.z }; clampInto(p); }
  /** The position that puts the part's centre at (u, v) of a view; the coordinate the view does not show stays. */
  function posFromView(view, pos0, u, v) {
    const r = fromFaceC(curCase(), view, u, v), np = { ...pos0 };
    for (const a of AXES) if (a !== r.free) np[a] = r.p[a];
    return np;
  }
  function setFacing(p, f) { p.facing = f; clampInto(p); }
  /** Puts a new part where its face is: interior parts (glass side) against the tray wall, others flat on their wall. */
  function restOnFace(p, face) {
    if (face === "side") { restOnBackWall(p); return; }
    const q = p3of(p), fr = FRAMES[face];
    if (!q || !fr) return;
    for (let i = 0; i < 3; i++) {
      if (!fr.N[i]) continue;
      const a = AXES[i], half = (q.max[a] - q.min[a]) / 2;
      p.pos[a] = r2(fr.N[i] > 0 ? dimOf(a) - half : half);
    }
  }
  /** Puts the part against the wall behind its LED side (the wall its back would rest on). */
  function restOnBackWall(p) {
    const q = p3of(p), fr = FRAMES[p.facing];
    if (!q || !fr) return;
    for (let i = 0; i < 3; i++) {
      if (!fr.N[i]) continue;
      const a = AXES[i], half = (q.max[a] - q.min[a]) / 2;
      p.pos[a] = r2(fr.N[i] > 0 ? half : dimOf(a) - half);
    }
  }
  /** Quick orientation of graphics cards and distro plates. */
  function quickOrient(p, mode) {
    const cat = (partOf(p.part) || {}).category;
    const facing = mode === "vert" ? "side" : mode === "floor" ? "top" : cat === "distro-plate" ? "top" : "bottom";
    p.facing = facing; clampInto(p);
    if (mode === "floor") {
      const q = p3of(p);
      if (q) p.pos.y = r2(dimOf("y") - (q.max.y - q.min.y) / 2);
      clampInto(p);
    }
  }
  /** Keeps every part within the allowed range (after the case changed). Returns how many moved. */
  function fitParts() {
    let n = 0;
    for (const q of parts()) {
      const before = JSON.stringify(q.pos);
      clampInto(q);
      if (JSON.stringify(q.pos) !== before) n++;
    }
    return n;
  }

  // ---------- collision (the rule of internal/sketch/collide.go) ----------
  /** Strips and case accessories may overlap anything. */
  const exemptPart = (p) => p.category === "strip" || p.category === "case-accessory";
  /** Two boxes overlap when they interpenetrate by more than tol on every axis. */
  function boxesOverlap(a, b, tol) {
    return a.min.x < b.max.x - tol && b.min.x < a.max.x - tol && a.min.y < b.max.y - tol && b.min.y < a.max.y - tol && a.min.z < b.max.z - tol && b.min.z < a.max.z - tol;
  }
  /** Pairs of placements (ids, in list order) whose boxes overlap. A part and the part it is mounted on never do. partFn: catalog id to part. */
  function overlapPairs(pls, partFn, tol = 0.5) {
    const box = pls.map((p) => { const part = partFn(p.part); return part && !exemptPart(part) && !isCL(p) ? project3d(part, p, undefined, false) : null; });
    const out = [];
    for (let i = 0; i < pls.length; i++) {
      for (let j = i + 1; j < pls.length; j++) {
        if (!box[i] || !box[j] || pls[i].on === pls[j].id || pls[j].on === pls[i].id) continue;
        if (boxesOverlap(box[i], box[j], tol)) out.push([pls[i].id, pls[j].id]);
      }
    }
    return out;
  }
  let ovCache = { rev: -1, pairs: /** @type {string[][]} */ ([]), ids: /** @type {Set<string>} */ (new Set()) };
  /** The overlaps of the document as it was after the last change (imports and old sketches are only marked, never refused). */
  function overlaps() {
    if (ovCache.rev !== S.rev) {
      const pairs = overlapPairs(parts(), (id) => partOf(id), overlapTol());
      ovCache = { rev: S.rev, pairs, ids: new Set(pairs.flat()) };
    }
    return ovCache;
  }
  const pairKey = (a, b) => (a < b ? a + "\u0000" + b : b + "\u0000" + a);
  /** Keys of the overlaps between the given parts and all others (the given parts may overlap each other). */
  function hitsOthers(ids) {
    const set = new Set(ids), out = new Set(), tol = overlapTol();
    for (const id of ids) {
      const m = partById(id), mp = m && partOf(m.part), mq = mp && !exemptPart(mp) && p3of(m);
      if (!mq) continue;
      for (const q of parts()) {
        if (set.has(q.id) || q.on === m.id || m.on === q.id) continue;
        const qp = partOf(q.part), qq = qp && !exemptPart(qp) && p3of(q);
        if (qq && boxesOverlap(mq, qq, tol)) out.add(pairKey(id, q.id));
      }
    }
    return out;
  }

  // ---------- selection, groups ----------
  function pruneSel() {
    for (const id of [...S.sels]) if (!partById(id)) S.sels.delete(id);
    if (!S.sel || !S.sels.has(S.sel)) S.sel = [...S.sels].at(-1) ?? null;
  }
  /** Sets the selection without redrawing (inside mutate). */
  function setSelRaw(ids, primary) {
    S.sels = new Set(ids);
    S.sel = primary && S.sels.has(primary) ? primary : ids.at(-1) ?? null;
  }
  const groupMembers = (id) => { const p = partById(id); return p && p.group ? parts().filter((q) => q.group === p.group).map((q) => q.id) : p ? [id] : []; };
  const groupLabel = (g) => { const m = /^g(\d+)$/.exec(g); return m ? t("sk.group.n", { n: +m[1] }) : t("sk.group") + " " + g; };
  function newGroupId() {
    const used = new Set(parts().map((p) => p.group).filter(Boolean));
    for (let i = 1; ; i++) if (!used.has("g" + i)) return "g" + i;
  }
  /** A group of one part is no group. */
  function tidyGroups() {
    const n = new Map();
    for (const p of parts()) if (p.group) n.set(p.group, (n.get(p.group) || 0) + 1);
    for (const p of parts()) if (p.group && n.get(p.group) < 2) delete p.group;
  }
  /** The ids plus the parts mounted on them (a carrier takes its mounted parts along). */
  function moveSet(ids) {
    const set = new Set(ids);
    for (let k = 0; k < 4; k++) {
      let grew = false;
      for (const q of parts()) if (q.on && set.has(q.on) && !set.has(q.id)) { set.add(q.id); grew = true; }
      if (!grew) break;
    }
    return [...set].filter((id) => !isCL(partById(id))); // the case's own lighting stays where the case has it
  }
  function setSel(ids, primary) {
    ids = [...new Set(ids.filter((id) => partById(id)))];
    const prim = primary && ids.includes(primary) ? primary : ids.at(-1) ?? null;
    if (ids.length === S.sels.size && ids.every((i) => S.sels.has(i)) && prim === S.sel) return;
    endTest();
    setSelRaw(ids, prim);
    S.test = 0;
    if (ids.length === 1) live(t("sk.live.sel", { n: partTitle(partById(ids[0])) }));
    else if (ids.length > 1) live(t("sk.live.sels", { n: ids.length }));
    afterChange(true);
  }
  function weld() {
    const ids = selIds().filter((id) => !isCL(partById(id)));
    if (ids.length < 2) return;
    const gid = newGroupId();
    mutate(() => { for (const id of ids) partById(id).group = gid; tidyGroups(); });
    live(t("sk.live.weld", { n: ids.length }));
  }
  function unweld() {
    const gs = new Set(selIds().map((id) => partById(id).group).filter(Boolean));
    if (!gs.size) return;
    mutate(() => { for (const p of parts()) if (gs.has(p.group)) delete p.group; });
    live(t("sk.live.unweld"));
  }
  function leaveGroup(id) { mutate(() => { delete partById(id).group; tidyGroups(); }); }

  // ---------- moving a set of parts as one ----------
  /** The shift (x, y, z) closest to s0 that keeps every part within the allowed range, so a group is clamped as one body. */
  function rigidShift(ids, s0, posOf = (p) => p.pos) {
    const m = S.lim.marginMm, s = { ...s0 };
    for (const a of AXES) {
      let lo = -Infinity, hi = Infinity;
      for (const id of ids) {
        const p = partById(id), q = p && p3of(p);
        if (!q) continue;
        const half = (q.max[a] - q.min[a]) / 2, at = posOf(p)[a];
        lo = Math.max(lo, -m + half - at); hi = Math.min(hi, dimOf(a) + m - half - at);
      }
      s[a] = lo <= hi ? clamp(s0[a], lo, hi) : 0;
    }
    return s;
  }
  /** Rotation of a vector by 90 degrees as a view shows it: right (eu) turns to down (ev), down to left. dir < 0 turns the other way. */
  function turner(eu, ev, dir) {
    const dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
    return (w) => {
      const al = dot(w, eu), be = dot(w, ev), out = [0, 0, 0];
      for (let i = 0; i < 3; i++) out[i] = w[i] - al * eu[i] - be * ev[i] + (dir > 0 ? al * ev[i] - be * eu[i] : -al * ev[i] + be * eu[i]) + 0;
      return out;
    };
  }
  /** The facing, spin and mirror a placement gets when the part is turned by `turn` (a rotation of the case box). Null if no orientation fits. */
  function rotatePlacement(pl, fanLike, turn, glass) {
    const fr = frameIn(pl.facing, glass);
    if (!fr) return null;
    const flip = pl.flow === "exhaust" && fanLike, mEff = !!pl.mirror !== flip;
    const rad = (num(pl.spin) * Math.PI) / 180, sin = Math.sin(rad), cos = Math.cos(rad);
    const comb = (c1, c2) => [0, 1, 2].map((i) => c1 * fr.U[i] + c2 * fr.V[i]);
    const A = turn(mEff ? comb(-cos, -sin) : comb(cos, sin)), B = turn(comb(-sin, cos)), N = turn(fr.N);
    const f = FACINGS.find((k) => FRAMES[k].N.every((v, i) => Math.abs(v - N[i]) < 1e-6));
    if (!f) return null;
    const f2 = frameIn(f, glass), dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
    const ca = dot(A, f2.U), sa = dot(A, f2.V), cb = dot(B, f2.U), sb = dot(B, f2.V);
    let mirror, deg;
    if (Math.abs(cb + sa) < 1e-6 && Math.abs(sb - ca) < 1e-6) { mirror = false; deg = Math.atan2(sa, ca); }
    else if (Math.abs(cb - sa) < 1e-6 && Math.abs(sb + ca) < 1e-6) { mirror = true; deg = Math.atan2(-sa, -ca); }
    else return null;
    const spin = ((Math.round((deg * 180) / Math.PI * 100) / 100) % 360 + 360) % 360;
    return { facing: f, spin: spin === 360 ? 0 : spin, mirror: mirror !== flip };
  }
  /** World directions of the current view's right and down, for turning in the picture. */
  function viewTurner(dir) {
    const c = curCase(), o = toFaceC(c, S.view, { x: 0, y: 0, z: 0 }), eu = [0, 0, 0], ev = [0, 0, 0];
    AXES.forEach((a, i) => {
      const q = toFaceC(c, S.view, { x: a === "x" ? 1 : 0, y: a === "y" ? 1 : 0, z: a === "z" ? 1 : 0 });
      eu[i] = Math.round(q.u - o.u); ev[i] = Math.round(q.v - o.v);
    });
    return turner(eu, ev, dir);
  }
  /** Turns the selection (and what is mounted on it) by 90 degrees about the viewing axis, around the centre of the whole. */
  function rotateSelection(dir) {
    const ids = moveSet(selIds());
    const boxes = ids.map((id) => p3of(partById(id))).filter(Boolean);
    if (!boxes.length) return;
    const lo = {}, hi = {};
    for (const a of AXES) { lo[a] = Math.min(...boxes.map((q) => q.min[a])); hi[a] = Math.max(...boxes.map((q) => q.max[a])); }
    const pivot = [(lo.x + hi.x) / 2, (lo.y + hi.y) / 2, (lo.z + hi.z) / 2], turn = viewTurner(dir), glass = glassRight();
    mutate(() => {
      for (const id of ids) {
        const p = partById(id), part = partOf(p.part);
        const r = rotatePlacement(p, isFan(part), turn, glass);
        const w = turn([p.pos.x - pivot[0], p.pos.y - pivot[1], p.pos.z - pivot[2]]);
        p.pos = { x: r2(pivot[0] + w[0]), y: r2(pivot[1] + w[1]), z: r2(pivot[2] + w[2]) };
        if (r) { p.facing = r.facing; p.spin = r.spin; p.mirror = r.mirror; }
      }
      const s = rigidShift(ids, { x: 0, y: 0, z: 0 });
      for (const id of ids) { const p = partById(id); p.pos = { x: r2(p.pos.x + s.x), y: r2(p.pos.y + s.y), z: r2(p.pos.z + s.z) }; }
    });
    live(t("sk.live.rot"));
  }

  // ---------- lighting devices and chains ----------
  const devices = () => (typeof lv !== "undefined" && lv && lv.devices) || [];
  const devIndex = (id) => devices().findIndex((d) => d.id === id);
  const devColor = (id) => { const i = devIndex(id); return i < 0 ? null : LIGHT_COLORS[i % LIGHT_COLORS.length]; };
  const findZone = (dev, zone) => { const d = devices().find((x) => x.id === dev); return d ? { d, z: d.zones.find((q) => q.id === zone) } : { d: null, z: null }; };
  /** How many LEDs a zone can take: resizable zones up to their maximum, fixed ones exactly their count (Infinity = unknown). */
  function zoneCap(z) {
    const cap = z.resizable ? z.max : z.max || z.count;
    return cap > 0 ? cap : Infinity;
  }
  const zoneName = (z) => z.label || z.name;
  function zoneText(z) {
    const cap = zoneCap(z);
    if (z.resizable) return Number.isFinite(cap) ? t("sk.zone.upto", { n: cap }) : t("sk.zone.any");
    return cap === 1 ? t("light.zone.led1") : Number.isFinite(cap) ? t("light.zone.leds", { n: cap }) : "";
  }
  const stateText = (s) => (Object.hasOwn(STRINGS[LANG], "light.state." + s) ? t("light.state." + s) : s);

  /** Per (device, zone): the chain as the server builds it: sorted by order, parallel parts share the start. */
  let chainCache = { rev: -1, v: /** @type {any} */ (null) };
  function chains() {
    if (chainCache.v && chainCache.rev === S.rev) return chainCache.v;
    const map = new Map();
    parts().forEach((p, idx) => { if (p.bind && p.bind.device) { const k = p.bind.device + "\u0000" + p.bind.zone; (map.get(k) || map.set(k, []).get(k)).push({ p, idx }); } });
    const info = new Map(); // part id -> {start, n}
    const zones = new Map(); // key -> {leds, parts}
    for (const [k, list] of map) {
      list.sort((a, b) => a.p.bind.order - b.p.bind.order || a.idx - b.idx);
      let next = 0, prev = 0, seen = 0;
      const [dev, zone] = k.split("\u0000"), ring = ringZone(dev, zone);
      for (const { p } of list) {
        const n = ring && !isCL(p) ? ringsOf(partOf(p.part), p, ledCount(p)).rings : ledCount(p);
        const start = p.bind.parallel && seen > 0 ? prev : next;
        info.set(p.id, { start, n });
        prev = start; next = Math.max(next, start + n); seen++;
      }
      zones.set(k, { leds: next, parts: list.length, list: list.map((q) => q.p) });
    }
    chainCache = { rev: S.rev, v: { info, zones } };
    return chainCache.v;
  }

  // ---------- warnings ----------
  const partTitle = (p) => p.label || (isCL(p) ? clName(S.cat, p) : partOf(p.part) ? partName(partOf(p.part)) : p.part);
  const unboundParts = () => parts().filter((p) => !(p.bind && p.bind.device));
  function warnings() {
    const out = [], flagged = new Set();
    const ch = chains();
    let total = 0;
    for (const p of parts()) {
      const part = partOf(p.part), name = partTitle(p), n = ledCount(p);
      total += n;
      if (!part) { out.push({ id: p.id, kind: "warn", text: t("sk.w.unknown", { p: name, id: p.part }) }); continue; }
      const g = gen(part);
      if (g && p.leds && (p.leds < g.min || p.leds > g.max)) out.push({ id: p.id, kind: "warn", text: t("sk.w.leds", { p: name, a: g.min, b: g.max }) });
      const q = p3of(p), dm = S.doc.case.dims;
      if (q && (q.min.x < -0.01 || q.min.y < -0.01 || q.min.z < -0.01 || q.max.x > dm.w + 0.01 || q.max.y > dm.h + 0.01 || q.max.z > dm.d + 0.01)) out.push({ id: p.id, kind: "warn", text: t("sk.w.outside", { p: name }) });
      if (!p.bind || !p.bind.device) { out.push({ id: p.id, kind: "info", text: t("sk.w.unbound", { p: name }) }); continue; }
      const { d, z } = findZone(p.bind.device, p.bind.zone);
      if (devices().length && (!d || !z)) out.push({ id: p.id, kind: "info", text: t("sk.w.noZone", { p: name, z: p.bind.zone }) });
    }
    for (const [k, c] of ch.zones) {
      const [dev, zone] = k.split("\u0000"), { z } = findZone(dev, zone);
      const cap = z ? zoneCap(z) : Infinity;
      if (c.leds > cap && !flagged.has(k)) { // the part that crosses the limit is the one to look at
        flagged.add(k);
        const over = c.list.find((q) => { const i = ch.info.get(q.id); return i && i.start + i.n > cap; }) || c.list[0];
        out.push({ id: over.id, kind: "warn", text: t("sk.w.zoneLeds", { n: c.leds, z: zoneName(z), m: cap }) });
      }
      if (c.parts > maxSegments()) out.push({ id: null, kind: "warn", text: t("sk.w.segments", { z: (z && zoneName(z)) || zone, n: c.parts, max: maxSegments() }) });
    }
    if (total > S.lim.maxTotalLeds) out.push({ id: null, kind: "warn", text: t("sk.w.total", { n: total, max: S.lim.maxTotalLeds }) });
    for (const [a, b] of overlaps().pairs) out.push({ id: a, kind: "warn", text: t("sk.w.overlap", { a: partTitle(partById(a)), b: partTitle(partById(b)) }) });
    for (const n of S.notes) out.push({ id: n.id && partById(n.id) ? n.id : null, kind: "info", text: n.text });
    return out;
  }

  // ---------- viewport ----------
  function fitView(view = S.view) {
    const vs = viewSize(view), m = 28;
    const { w, h } = S.size;
    if (w < 40 || h < 40) return;
    const scale = clamp(Math.min((w - 2 * m) / vs.w, (h - 2 * m) / vs.h), MIN_SCALE, MAX_SCALE);
    S.vp[view] = { cx: vs.w / 2, cy: vs.h / 2, scale, fit: true };
  }
  const vp = () => S.vp[S.view] || (fitView(), S.vp[S.view]) || { cx: 0, cy: 0, scale: 1, fit: true };
  function vbox() {
    const v = vp(), { w, h } = S.size;
    return { x: v.cx - w / 2 / v.scale, y: v.cy - h / 2 / v.scale, w: w / v.scale, h: h / v.scale, scale: v.scale };
  }
  function zoomAt(px, py, factor) { // px, py: stage pixels
    const v = vp(), b = vbox();
    const ns = clamp(v.scale * factor, MIN_SCALE, MAX_SCALE);
    const mx = b.x + px / v.scale, my = b.y + py / v.scale;
    v.scale = ns; v.fit = false;
    v.cx = mx - (px - S.size.w / 2) / ns; v.cy = my - (py - S.size.h / 2) / ns;
    schedule();
  }
  const zoomCentre = (f) => zoomAt(S.size.w / 2, S.size.h / 2, f);
  /** Pointer position in view millimetres. */
  function toMM(ev) {
    const r = $("#sk-stage").getBoundingClientRect(), b = vbox();
    return { x: b.x + (ev.clientX - r.left) / b.scale, y: b.y + (ev.clientY - r.top) / b.scale };
  }
  const snapV = (v, on = S.snap) => (on ? Math.round(v / GRID) * GRID : v);

  // ---------- drawing ----------
  function shapeEl(sh, extra = "") {
    const cls = "sk-sh sk-r-" + (sh.role || "body") + extra;
    const base = { class: cls };
    switch (sh.type) {
      case "rect": return sv("rect", { ...base, x: sh.x ?? 0, y: sh.y ?? 0, width: sh.w ?? 0, height: sh.h ?? 0, rx: sh.r ?? 0 });
      case "circle": return sv("circle", { ...base, cx: sh.cx ?? 0, cy: sh.cy ?? 0, r: sh.r ?? 0 });
      case "ring": {
        const { cx = 0, cy = 0, r_outer: ro = 0, r_inner: ri = 0 } = sh;
        const c = (r) => `M${cx + r} ${cy}a${r} ${r} 0 1 0 ${-2 * r} 0a${r} ${r} 0 1 0 ${2 * r} 0Z`;
        return sv("path", { ...base, d: c(ro) + c(ri), "fill-rule": "evenodd" });
      }
      case "polygon": return sv("polygon", { ...base, points: (sh.points || []).map((q) => q.join(",")).join(" ") });
      default: return null;
    }
  }

  /** Rectangle on a face covered by the x range [x0, x1] of the case (a chamber), or null on faces that do not show the width. */
  function chamberRect(face, x0, x1) {
    if (face === "side" || face === "tray") return null;
    const c = curCase(), d = c.dims, a = toFaceC(c, face, { x: x0, y: 0, z: 0 }), b = toFaceC(c, face, { x: x1, y: d.h, z: d.d });
    return { x: Math.min(a.u, b.u), y: Math.min(a.v, b.v), w: Math.abs(b.u - a.u), h: Math.abs(b.v - a.v), across: face === "top" || face === "bottom" };
  }
  /**
   * Library templates draw the front and the glass side as the old views did. The front view is the front face; the side
   * view is the glass face, mirrored when the glass is on the right.
   */
  function templateXf(face) {
    if (face === "front") return "";
    if (face === "side") return glassRight() ? `matrix(-1 0 0 1 ${S.doc.case.dims.d} 0)` : "";
    return null;
  }
  function caseLayer(view) {
    const vs = viewSize(view), g = sv("g", { class: "sk-case" });
    const tpl = S.doc.case.source === "library" && caseOf(S.doc.case.id), xf = templateXf(view);
    const cv = tpl && xf != null && tpl.views && tpl.views[view];
    g.append(sv("rect", { class: "sk-case-body", x: 0, y: 0, width: vs.w, height: vs.h, rx: cv ? 0 : 8 }));
    if (tpl && xf != null) {
      const tg = sv("g", { transform: xf || null }), labels = [];
      if (cv) for (const sh of cv.outline || []) tg.append(shapeEl(sh, " sk-case-sh"));
      for (const a of (tpl.areas || []).filter((q) => q.view === view)) {
        tg.append(shapeEl({ ...a.shape, role: "area" }));
        const b = shapeBox(a.shape);
        if (b && (a.name || a.kind)) labels.push([b.x + b.w / 2, b.y + b.h / 2, areaName(a)]);
      }
      for (const m of (tpl.mounts || []).filter((q) => q.view === view)) {
        tg.append(shapeEl({ ...m.region, role: "mount" }));
        const lay = (m.layouts || [])[0];
        if (lay) for (const [cx, cy] of lay.snap_mm || []) tg.append(sv("circle", { class: "sk-mount-dot", cx, cy, r: lay.fan_mm / 2 }));
      }
      g.append(tg);
      const flip = xf ? (x) => S.doc.case.dims.d - x : (x) => x; // text stays readable
      for (const [x, y, txt] of labels) g.append(sv("text", { class: "sk-area-label", x: flip(x), y, "font-size": 11 / vp().scale, "text-anchor": "middle" }, txt));
    }
    const ch = S.doc.case.chambers || [];
    ch.forEach((c, i) => { // chamber dividers on the faces that show the width
      const r = chamberRect(view, c.x, c.x + c.w);
      if (!r) return;
      g.append(sv("rect", { class: "sk-chamber c" + (i % 2), x: r.x, y: r.y, width: r.w, height: r.h }));
      if (i > 0) {
        const e = chamberRect(view, c.x, c.x);
        g.append(r.across ? sv("line", { class: "sk-divider", x1: 0, y1: e.y, x2: vs.w, y2: e.y }) : sv("line", { class: "sk-divider", x1: e.x, y1: 0, x2: e.x, y2: vs.h }));
      }
      g.append(sv("text", { class: "sk-area-label", x: r.x + r.w / 2, y: r.across ? r.y + r.h / 2 : 14 / vp().scale + 4, "font-size": 11 / vp().scale, "text-anchor": "middle" }, t("sk.case.chamber", { n: i + 1 })));
    });
    return g;
  }
  const AREA_KINDS = ["motherboard", "psu", "psu-shroud", "drive", "gpu-bay", "cable-area", "chamber"];
  const areaName = (a) => (AREA_KINDS.includes(a.kind) ? t("sk.area." + a.kind) : short(a.name || a.kind));
  const short = (v, n = 32) => (v.length > n ? v.slice(0, n - 1) + "…" : v);
  function shapeBox(sh) {
    if (sh.type === "rect") return { x: sh.x ?? 0, y: sh.y ?? 0, w: sh.w ?? 0, h: sh.h ?? 0 };
    if (sh.type === "circle") { const r = sh.r ?? 0; return { x: (sh.cx ?? 0) - r, y: (sh.cy ?? 0) - r, w: 2 * r, h: 2 * r }; }
    if (sh.type === "ring") { const r = sh.r_outer ?? 0; return { x: (sh.cx ?? 0) - r, y: (sh.cy ?? 0) - r, w: 2 * r, h: 2 * r }; }
    if (sh.type === "polygon" && sh.points && sh.points.length) {
      const xs = sh.points.map((q) => q[0]), ys = sh.points.map((q) => q[1]);
      return { x: Math.min(...xs), y: Math.min(...ys), w: Math.max(...xs) - Math.min(...xs), h: Math.max(...ys) - Math.min(...ys) };
    }
    return null;
  }

  /** What each edge of a face is on the real case: the orientation markers (the neighbours in the case net). */
  function edgeNames(face) {
    const e = {
      side: { l: "rear", r: "front", t: "top", b: "bottom" },
      front: { l: "glass", r: "tray", t: "top", b: "bottom" },
      rear: { l: "tray", r: "glass", t: "top", b: "bottom" },
      tray: { l: "front", r: "rear", t: "top", b: "bottom" },
      top: { l: "rear", r: "front", t: "tray", b: "glass" },
      bottom: { l: "rear", r: "front", t: "glass", b: "tray" },
    }[face] || { l: "rear", r: "front", t: "top", b: "bottom" };
    return glassRight() ? { ...e, l: e.r, r: e.l } : e;
  }
  function edgeMarkers(view, sc) {
    const vs = viewSize(view), n = edgeNames(view), f = 11 / sc, o = 9 / sc;
    const lab = (k) => t("sk.edge." + k);
    const cls = (k) => "sk-front-label" + (k === "front" ? " front" : "");
    return [
      sv("text", { class: cls(n.t), x: vs.w / 2, y: -o, "font-size": f, "text-anchor": "middle" }, lab(n.t)),
      sv("text", { class: cls(n.b), x: vs.w / 2, y: vs.h + o + f * 0.7, "font-size": f, "text-anchor": "middle" }, lab(n.b)),
      sv("text", { class: cls(n.l), x: -o, y: vs.h / 2, "font-size": f, "text-anchor": "middle", transform: `rotate(-90 ${-o} ${vs.h / 2})` }, lab(n.l)),
      sv("text", { class: cls(n.r), x: vs.w + o, y: vs.h / 2, "font-size": f, "text-anchor": "middle", transform: `rotate(90 ${vs.w + o} ${vs.h / 2})` }, lab(n.r)),
    ];
  }

  function partGroup(p) {
    const pr = proj(p), sc = vp().scale, sel = S.sels.has(p.id);
    const b = pr.box, col = p.bind ? devColor(p.bind.device) : null;
    const name = partTitle(p);
    const g = sv("g", { class: "sk-part" + (sel ? " sel" : "") + (overlaps().ids.has(p.id) ? " collide" : "") + (p.group ? " grouped" : "") + (pr.missing ? " missing" : "") + (pr.kind === "away" ? " away" : "") + (pr.kind === "edge" ? " edge" : "") + (pr.kind === "path" ? " caselight" : ""), "data-id": p.id, role: "img", "aria-label": name + ", " + (pr.kind === "path" ? t("sk.cl.aria", { n: pr.n }) : t("sk.part.size", { w: fmt(b.w), h: fmt(b.h), n: pr.n })) });
    g.append(sv("rect", { class: "sk-hit", x: b.x, y: b.y, width: b.w, height: b.h }));
    if (pr.kind === "path") {
      for (const ln of pr.lines) g.append(sv("polyline", { class: "sk-cl-strip", points: ln.map((q) => q.x + "," + q.y).join(" "), "stroke-width": pr.width }));
    } else if (pr.kind === "edge" || pr.missing || !pr.shapes) {
      g.append(sv("rect", { class: "sk-sh sk-r-body sk-edge", x: b.x, y: b.y, width: b.w, height: b.h, rx: Math.min(2, Math.min(b.w, b.h) / 3) }));
    } else {
      const inner = sv("g", { transform: pr.xf });
      for (const sh of pr.shapes) { const n = shapeEl(sh); if (n) inner.append(n); }
      g.append(inner);
    }
    if (pr.leds.length) {
      let minD = Infinity;
      for (let i = 1; i < pr.leds.length; i++) minD = Math.min(minD, Math.hypot(pr.leds[i].x - pr.leds[i - 1].x, pr.leds[i].y - pr.leds[i - 1].y));
      const rad = clamp(Number.isFinite(minD) ? minD * 0.3 : 2, 0.8, 2.4);
      const dots = sv("g", { class: "sk-leds" + (col ? "" : " plain"), style: col ? `--led:${col}` : "" });
      if (pr.leds.length > 1) dots.append(sv("polyline", { class: "sk-wire", points: pr.leds.map((q) => q.x + "," + q.y).join(" ") }));
      pr.leds.forEach((q, j) => { const i = pr.idx ? pr.idx[j] : j; dots.append(sv("circle", { class: "sk-led" + (i === 0 ? " first" : "") + (sel && S.testLive && i === S.test ? " test" : ""), cx: q.x, cy: q.y, r: rad, "data-led": i })); });
      g.append(dots);
      if (pr.leds.length > 1) { // wire order arrow on the first step
        const a = pr.leds[0], c = pr.leds[1], ang = (Math.atan2(c.y - a.y, c.x - a.x) * 180) / Math.PI;
        const len = Math.hypot(c.x - a.x, c.y - a.y), sz = clamp(8 / sc, 2.5, Math.max(len * 0.9, 2.5));
        if (len > 0) g.append(sv("path", { class: "sk-arrow", d: `M${-sz * 0.6} ${-sz * 0.5}L${sz * 0.6} 0L${-sz * 0.6} ${sz * 0.5}Z`, transform: `translate(${(a.x + c.x) / 2} ${(a.y + c.y) / 2}) rotate(${ang})` }));
      }
    }
    const fs = 11 / sc;
    g.append(sv("text", { class: "sk-part-label", x: b.x + b.w / 2, y: b.y + b.h + 14 / sc, "font-size": fs, "text-anchor": "middle" }, name));
    return g;
  }

  function selectionLayer(p) {
    const pr = proj(p), sc = vp().scale, b = pr.box, pad = 3 / sc;
    const g = sv("g", { class: "sk-selection", "data-id": p.id });
    g.append(sv("rect", { class: "sk-selbox", x: b.x - pad, y: b.y - pad, width: b.w + 2 * pad, height: b.h + 2 * pad, rx: 3 / sc }));
    if (!pr.missing && pr.kind === "flat" && S.sels.size === 1) { // only a single part seen flat-on is turned on the drawing
      const hx = b.x + b.w / 2, hy = b.y - pad - 22 / sc;
      g.append(sv("line", { class: "sk-rotline", x1: hx, y1: b.y - pad, x2: hx, y2: hy + 6 / sc }));
      g.append(sv("circle", { class: "sk-rot", cx: hx, cy: hy, r: 7 / sc }));
      g.append(sv("path", { class: "sk-rot-icon", transform: `translate(${hx} ${hy}) scale(${1 / sc})`, d: "M-3 -1.5a3.4 3.4 0 1 1 1 3.4M-3 -4v2.5h2.5" }));
    }
    return g;
  }

  function gridDefs() {
    const sc = vp().scale;
    const steps = [5, 10, 25, 50, 100, 250, 500];
    const minor = steps.find((s) => s * sc >= 10) || 500;
    const major = steps.find((s) => s > minor && s * sc >= 48) || minor * 5;
    const pat = (id, step, cls) => sv("pattern", { id, width: step, height: step, patternUnits: "userSpaceOnUse" },
      sv("path", { class: cls, d: `M${step} 0H0V${step}`, fill: "none", "vector-effect": "non-scaling-stroke" }));
    return sv("defs", {}, pat("sk-g1", minor, "sk-gl"), pat("sk-g2", major, "sk-gl major"));
  }

  /** A changed case (type, size or glass side) invalidates the automatic fit of every view. */
  function syncCase() {
    const c = S.doc.case, key = [c.source, c.id, c.dims.w, c.dims.h, c.dims.d].join("|");
    if (key === S.dimsKey) return;
    S.dimsKey = key;
    for (const [k, v] of Object.entries(S.vp)) if (v.fit) delete S.vp[k];
  }
  /** Lower layers first, within a layer far parts before near ones; a part mounted on another is drawn after it. */
  function drawOrder(all = false) {
    const list = (all ? parts() : shown()).map((p) => ({ p, pr: proj(p), layer: p.layer || 0 }));
    list.sort((a, b) => a.layer - b.layer || b.pr.centre - a.pr.centre);
    const out = [], placed = new Set(), byId = new Map(list.map((x) => [x.p.id, x]));
    const put = (x) => {
      if (placed.has(x.p.id)) return;
      placed.add(x.p.id);
      const parent = x.p.on && byId.get(x.p.on);
      if (parent && parent !== x) put(parent);
      out.push(x.p);
    };
    list.forEach(put);
    return out;
  }
  function draw() {
    S.raf = 0; S.full = false;
    const svg = svgEl();
    if (!S.open || !svg || S.size.w < 40 || !S.doc) return;
    syncCase();
    const b = vbox();
    svg.setAttribute("viewBox", `${b.x} ${b.y} ${b.w} ${b.h}`);
    svg.setAttribute("width", String(S.size.w)); svg.setAttribute("height", String(S.size.h));
    const kids = [gridDefs()];
    if (S.grid) {
      kids.push(sv("rect", { class: "sk-grid", x: b.x, y: b.y, width: b.w, height: b.h, fill: "url(#sk-g1)" }));
      kids.push(sv("rect", { class: "sk-grid", x: b.x, y: b.y, width: b.w, height: b.h, fill: "url(#sk-g2)" }));
    }
    kids.push(caseLayer(S.view));
    // the case edge on top of the grid again
    const vs = viewSize(S.view);
    kids.push(sv("rect", { class: "sk-case-edge", x: 0, y: 0, width: vs.w, height: vs.h, rx: S.doc.case.source === "library" ? 0 : 8, fill: "none" }));
    kids.push(...edgeMarkers(S.view, b.scale));
    const list = drawOrder();
    S.drawn = new Map(list.map((p) => [p.id, { x: proj(p).box.x, y: proj(p).box.y }]));
    const layer = sv("g", { class: "sk-parts" });
    for (const p of list) layer.append(partGroup(p));
    kids.push(layer);
    const sel = list.filter((p) => S.sels.has(p.id));
    if (sel.length) kids.push(sv("g", { class: "sk-selections" }, sel.map(selectionLayer)));
    if (S.band) {
      const b = S.band;
      kids.push(sv("rect", { class: "sk-band", x: Math.min(b.x0, b.x1), y: Math.min(b.y0, b.y1), width: Math.abs(b.x1 - b.x0), height: Math.abs(b.y1 - b.y0) }));
    }
    svg.replaceChildren(...kids);
    drawRulers();
    updateStatus();
    updateEmpty();
  }
  /** One frame per animation tick. fast: only the dragged part and the rulers move, nothing is rebuilt. */
  function schedule(fast = false) {
    if (!fast) S.full = true;
    if (!S.raf) S.raf = requestAnimationFrame(frame);
  }
  function frame() {
    S.raf = 0;
    if (S.full || !S.open || !S.doc) { draw(); return; }
    const d = S.drag;
    if (d && d.kind === "move") {
      const svg = svgEl();
      for (const id of d.ids) {
        const p = partById(id), o = p && S.drawn.get(id);
        if (!p || !o) continue;
        const b = proj(p).box, tr = `translate(${r2(b.x - o.x)} ${r2(b.y - o.y)})`, q = CSS.escape(id);
        const g = svg.querySelector(`.sk-part[data-id="${q}"]`), sel = svg.querySelector(`.sk-selection[data-id="${q}"]`);
        if (g) g.setAttribute("transform", tr);
        if (sel) sel.setAttribute("transform", tr);
      }
      S.insSync.forEach((f) => f());
    }
    drawRulers(); updateStatus();
  }

  function niceStep(minMM) {
    const p = Math.pow(10, Math.floor(Math.log10(minMM)));
    for (const m of [1, 2, 5, 10]) if (p * m >= minMM) return p * m;
    return p * 10;
  }
  function drawRulers() {
    const b = vbox(), { w, h } = S.size;
    const rx = $("#sk-ruler-x"), ry = $("#sk-ruler-y");
    rx.setAttribute("width", String(w)); rx.setAttribute("height", "20"); rx.setAttribute("viewBox", `0 0 ${w} 20`);
    ry.setAttribute("width", "28"); ry.setAttribute("height", String(h)); ry.setAttribute("viewBox", `0 0 28 ${h}`);
    const step = niceStep(56 / b.scale), sub = step / (step >= 10 ? 5 : 2);
    const kx = [], ky = [];
    for (let v = Math.ceil(b.x / sub) * sub; v <= b.x + b.w; v += sub) {
      const px = (v - b.x) * b.scale, major = Math.abs(v / step - Math.round(v / step)) < 1e-6;
      kx.push(sv("line", { x1: px, x2: px, y1: major ? 8 : 14, y2: 20, class: "sk-tick" }));
      if (major) kx.push(sv("text", { x: px + 3, y: 11, class: "sk-tick-label" }, fmt(v)));
    }
    for (let v = Math.ceil(b.y / sub) * sub; v <= b.y + b.h; v += sub) {
      const py = (v - b.y) * b.scale, major = Math.abs(v / step - Math.round(v / step)) < 1e-6;
      ky.push(sv("line", { y1: py, y2: py, x1: major ? 16 : 22, x2: 28, class: "sk-tick" }));
      if (major) ky.push(sv("text", { x: 14, y: py - 3, class: "sk-tick-label", transform: `rotate(-90 14 ${py - 3})`, "text-anchor": "end" }, fmt(v)));
    }
    if (S.cursor) {
      kx.push(sv("line", { x1: (S.cursor.x - b.x) * b.scale, x2: (S.cursor.x - b.x) * b.scale, y1: 0, y2: 20, class: "sk-tick cur" }));
      ky.push(sv("line", { y1: (S.cursor.y - b.y) * b.scale, y2: (S.cursor.y - b.y) * b.scale, x1: 0, x2: 28, class: "sk-tick cur" }));
    }
    rx.replaceChildren(...kx); ry.replaceChildren(...ky);
  }

  function updateStatus() {
    const vs = viewSize(S.view), c = S.cursor;
    $("#sk-zoom").textContent = fmt(vp().scale) + " px/mm";
    $("#sk-readout").textContent = t("sk.readout.size", { w: fmt(vs.w), h: fmt(vs.h) }) + (c ? "  ·  " + t("sk.readout.pos", { x: fmt(c.x), y: fmt(c.y) }) : "");
    renderUnbound();
    renderCollide();
  }
  /** Chip in the status bar: parts that are not bound to a zone (they stay dark). */
  function renderUnbound() {
    let b = $("#sk-unbound");
    if (!b) {
      b = el("button", { id: "sk-unbound", class: "sk-unbound", type: "button", hidden: true, onclick: () => { const u = unboundParts()[0]; if (u) focusPart(u.id); } });
      $("#sk-readout").after(b);
    }
    const n = S.doc ? unboundParts().length : 0;
    b.hidden = !n;
    const txt = n === 1 ? t("sk.unbound.n1") : t("sk.unbound.n", { n });
    if (b.textContent !== txt) b.textContent = txt;
  }
  /** Chip in the status bar: parts that overlap each other (only marked, never refused). */
  function renderCollide() {
    let b = $("#sk-collide");
    if (!b) {
      b = el("button", { id: "sk-collide", class: "sk-unbound sk-collide", type: "button", hidden: true, onclick: () => { const o = overlaps().pairs[0]; if (o) setSel(o, o[0]); } });
      $("#sk-readout").after(b);
    }
    const n = S.doc ? overlaps().pairs.length : 0;
    b.hidden = !n;
    const txt = n === 1 ? t("sk.collide.n1") : t("sk.collide.n", { n });
    if (b.textContent !== txt) b.textContent = txt;
  }
  function updateEmpty() {
    const box = $("#sk-empty");
    box.hidden = !!parts().length;
    if (box.hidden) return;
    if (box.dataset.key === "empty" + LANG) return;
    box.dataset.key = "empty" + LANG;
    box.replaceChildren(E("div", { class: "sk-empty-icon", "aria-hidden": "true" }, svgIcon("M3 7h18v10H3zM7 7v10M12 12h5")), E("b", {}, t("sk.empty.title")), E("p", {}, t("sk.empty.text")));
  }
  const svgIcon = (d) => { const n = sv("svg", { viewBox: "0 0 24 24", width: 28, height: 28, fill: "none", stroke: "currentColor", "stroke-width": 1.6, "stroke-linecap": "round", "stroke-linejoin": "round" }, sv("path", { d })); return n; };

  // ---------- palette ----------
  function thumb(part) {
    const d = part.dimensions_mm, w = d.w, h = d.h;
    const s = sv("svg", { class: "sk-thumb", viewBox: `${-2} ${-2} ${w + 4} ${h + 4}`, "aria-hidden": "true" });
    for (const sh of part.outline.front || []) { const n = shapeEl(sh); if (n) s.append(n); }
    for (const [x, y] of part.leds.positions_mm || []) s.append(sv("circle", { class: "sk-thumb-led", cx: x, cy: y, r: Math.max(w, h) / 36 }));
    return s;
  }
  /** Search over vendor, model, shown name, category (shown and raw) and id; every word must match. */
  function partMatches(p, words) {
    if (!words.length) return true;
    const hay = [partName(p), p.vendor, p.model, p.id, catName(p.category), p.category].join(" ").toLowerCase();
    return words.every((w) => hay.includes(w));
  }
  function renderPalette() {
    const box = $("#sk-pal-list");
    if (!S.cat) { box.replaceChildren(E("p", { class: "empty-note" }, "…")); return; }
    const q = S.search.trim().toLowerCase(), words = q.split(/\s+/).filter(Boolean);
    const byCat = new Map();
    for (const p of Object.values(S.cat.parts)) {
      if (!partMatches(p, words)) continue;
      (byCat.get(p.category) || byCat.set(p.category, []).get(p.category)).push(p);
    }
    const cats = [...byCat.keys()].sort((a, b) => {
      const ia = CAT_ORDER.indexOf(a), ib = CAT_ORDER.indexOf(b);
      return (ia < 0 ? 99 : ia) - (ib < 0 ? 99 : ib) || a.localeCompare(b);
    });
    if (!cats.length) { box.replaceChildren(E("p", { class: "empty-note" }, t("sk.pal.none"))); return; }
    const open = new Set($$("#sk-pal-list details[open]").map((d) => d.dataset.cat));
    const first = !box.children.length || !!box.querySelector(".empty-note");
    box.replaceChildren(...cats.map((c) => {
      const list = byCat.get(c).sort((a, b) => Number(isGeneric(b)) - Number(isGeneric(a)) || partName(a).localeCompare(partName(b), LANG));
      const det = E("details", { class: "sk-cat", "data-cat": c }, E("summary", {}, E("span", {}, catName(c)), E("small", {}, String(list.length))),
        E("div", { class: "sk-items" }, list.map(paletteItem)));
      if (q || first || open.has(c)) det.setAttribute("open", "");
      return det;
    }));
  }
  function paletteItem(p) {
    const g = gen(p), d = p.dimensions_mm;
    const leds = g ? (g.min === g.max ? t("sk.pal.count", { n: g.min }) : t("sk.pal.var", { a: g.min, b: g.max })) : t("sk.pal.count", { n: (p.leds.positions_mm || []).length });
    const b = E("button", { class: "sk-item", type: "button", "data-part": p.id, "aria-label": t("sk.pal.add", { n: partName(p) }), title: t("sk.pal.add", { n: partName(p) }) },
      E("span", { class: "sk-thumb-box" }, thumb(p)),
      E("span", { class: "sk-item-text" },
        E("b", {}, partName(p)),
        E("small", {}, `${fmt(d.w)} × ${fmt(d.h)} × ${fmt(d.d)} mm`),
        E("small", {}, leds),
        isGeneric(p) || isApprox(p) ? E("span", { class: "sk-badges" },
          isGeneric(p) ? E("span", { class: "badge sk-b" }, t("sk.pal.generic")) : null,
          !isGeneric(p) && isApprox(p) ? E("span", { class: "badge sk-b warn", title: t("sk.approx.tip") }, t("sk.approx")) : null) : null));
    b.addEventListener("click", () => addPart(p.id));
    // the wrapper is what gets dragged: Firefox does not start a drag on a button
    const wrap = E("div", { class: "sk-drag", draggable: "true" }, b);
    wrap.addEventListener("dragstart", (e) => { e.dataTransfer.setData("text/x-casehud-part", p.id); e.dataTransfer.setData("text/plain", p.id); e.dataTransfer.effectAllowed = "copy"; });
    return wrap;
  }

  // ---------- part operations ----------
  const live = (msg) => { $("#sk-live").textContent = msg; };
  function newId() {
    const used = new Set(parts().map((p) => p.id));
    for (let i = 1; ; i++) if (!used.has("p" + i)) return "p" + i;
  }
  /** Adds a part seen flat-on in the current view, centred at `at` (view millimetres) and resting on the wall behind it. */
  function addPart(partId, at) {
    const part = partOf(partId);
    if (!part) return;
    if (parts().length >= S.lim.maxParts) { toast(t("sk.max.parts", { n: S.lim.maxParts }), true); return; }
    const g = gen(part);
    const pl = { id: newId(), part: partId, pos: { x: 0, y: 0, z: 0 }, facing: S.view, face: S.view, spin: 0, mirror: false, flow: isFan(part) ? "intake" : "", leds: g ? g.default : 0, label: "" };
    const vs = viewSize(S.view);
    let c = at;
    if (!c) { // centre of the visible part of the case, nudged off parts already there
      const b = vbox();
      c = { x: clamp(b.x + b.w / 2, 0, vs.w), y: clamp(b.y + b.h / 2, 0, vs.h) };
    } else c = { x: snapV(c.x), y: snapV(c.y) };
    const rest = (u, v) => { pl.pos = posFromView(S.view, pl.pos, u, v); restOnFace(pl, S.view); clampInto(pl); };
    rest(c.x, c.y);
    for (let i = 0; i < 20 && parts().some((q) => AXES.every((a) => Math.abs(q.pos[a] - pl.pos[a]) < 1)); i++) { c = { x: c.x + 10, y: c.y + 10 }; rest(c.x, c.y); }
    mutate(() => { parts().push(pl); setSelRaw([pl.id], pl.id); S.test = 0; });
    live(t("sk.live.added", { n: partName(part) }));
    svgEl().focus({ preventScroll: true });
  }
  const deletePart = (id) => deleteIds([id]);
  function duplicatePart(id) {
    const p = partById(id);
    if (!p || isCL(p)) return;
    if (parts().length >= S.lim.maxParts) { toast(t("sk.max.parts", { n: S.lim.maxParts }), true); return; }
    const c = JSON.parse(JSON.stringify(p));
    c.id = newId(); delete c.group;
    setPos(c, { x: p.pos.x, y: p.pos.y, z: p.pos.z });
    const q = toFaceC(curCase(), S.view, c.pos);
    setPos(c, posFromView(S.view, c.pos, q.u + 10, q.v + 10));
    if (c.bind) c.bind.order = Math.min(1000, (c.bind.order || 0) + 1);
    mutate(() => { parts().push(c); setSelRaw([c.id], c.id); });
  }
  /** Copies the selection (groups stay groups, mounted parts stay mounted), a little to the side. */
  function duplicateSelected() {
    const ids = selIds().filter((id) => !isCL(partById(id)));
    if (ids.length < 2) { if (S.sel) duplicatePart(S.sel); return; }
    if (parts().length + ids.length > S.lim.maxParts) { toast(t("sk.max.parts", { n: S.lim.maxParts }), true); return; }
    const used = new Set(parts().map((p) => p.id)), map = new Map(), gmap = new Map(), made = [];
    for (const p of parts()) {
      if (!ids.includes(p.id)) continue;
      let id; for (let i = 1; ; i++) if (!used.has("p" + i)) { id = "p" + i; used.add(id); break; }
      map.set(p.id, id);
    }
    mutate(() => {
      for (const p of [...parts()]) {
        if (!map.has(p.id)) continue;
        const c = JSON.parse(JSON.stringify(p));
        c.id = map.get(p.id);
        if (p.group) { if (!gmap.has(p.group)) gmap.set(p.group, newGroupId()); c.group = gmap.get(p.group); }
        if (p.on && map.has(p.on)) c.on = map.get(p.on);
        if (c.bind) c.bind.order = Math.min(1000, (c.bind.order || 0) + 1);
        made.push(c);
        parts().push(c);
      }
      const ids2 = made.map((c) => c.id), s = rigidShift(ids2, viewShift(10, 10));
      for (const c of made) c.pos = { x: r2(c.pos.x + s.x), y: r2(c.pos.y + s.y), z: r2(c.pos.z + s.z) };
      setSelRaw(ids2, ids2.at(-1));
    });
  }
  /** Selects a part (every part is visible in every view). */
  function focusPart(id) {
    const p = partById(id);
    if (!p) return;
    if (!facesOf(p).includes(S.view)) setView(faceOf(p));
    select(id);
  }
  /** Parts under a point (view millimetres), the smallest first so small parts on large ones stay reachable. */
  function hitParts(mm) {
    const tol = 3 / vp().scale;
    return shown().filter((q) => {
      const b = proj(q).box;
      return mm.x >= b.x - tol && mm.x <= b.x + b.w + tol && mm.y >= b.y - tol && mm.y <= b.y + b.h + tol;
    }).map((q) => { const b = proj(q).box; return { id: q.id, area: b.w * b.h }; }).sort((a, b) => a.area - b.area).map((q) => q.id);
  }
  function select(id) { setSel(id ? [id] : [], id); }

  // ---------- pointer interaction ----------
  /** Parts whose box meets a rectangle in view millimetres. */
  function partsInRect(r) {
    const x0 = Math.min(r.x0, r.x1), x1 = Math.max(r.x0, r.x1), y0 = Math.min(r.y0, r.y1), y1 = Math.max(r.y0, r.y1);
    return shown().filter((q) => { const b = proj(q).box; return b.x <= x1 && b.x + b.w >= x0 && b.y <= y1 && b.y + b.h >= y0; }).map((q) => q.id);
  }
  /** Selects a part, or its whole group; alt picks the single part. */
  function pick(id, single) { setSel(single ? [id] : groupMembers(id), id); }
  function onPointerDown(ev) {
    if (ev.button !== 0 && ev.button !== 1) return;
    const svg = svgEl();
    svg.focus({ preventScroll: true });
    const target = ev.target;
    const pan = ev.button === 1 || S.space;
    const rot = !pan && target.closest && target.closest(".sk-rot");
    const g = !pan && target.closest && target.closest(".sk-part");
    const mm = toMM(ev);
    if (rot && selPart() && S.sels.size === 1) {
      const p = selPart(), pr = proj(p), b = pr.box;
      S.drag = { kind: "rot", pid: ev.pointerId, id: p.id, cx: b.x + b.w / 2, cy: b.y + b.h / 2, spin0: p.spin, det: pr.det, moved: false };
    } else if (g && partById(g.dataset.id)) {
      const hits = hitParts(mm);
      const wasSel = S.sel;
      const id = hits.length ? (wasSel && hits.includes(wasSel) ? wasSel : hits[0]) : g.dataset.id;
      const p = partById(id), single = ev.altKey;
      let collapse = null;
      if (ev.shiftKey) { // add to or take out of the selection
        const members = single ? [id] : groupMembers(id), all = members.every((m) => S.sels.has(m));
        const rest = selIds().filter((m) => !members.includes(m));
        setSel(all ? rest : [...rest, ...members], all ? undefined : id);
        if (all) { ev.preventDefault(); return; }
      } else if (S.sels.has(id) && S.sels.size > 1) collapse = id; // dragging moves all; a plain click narrows the selection
      else pick(id, single);
      if (isCL(p)) { // built into the case: it is selected, never dragged
        S.drag = { kind: "fixed", pid: ev.pointerId };
        ev.preventDefault();
        return;
      }
      const ids = moveSet(selIds()), b = proj(p).box;
      const starts = new Map(ids.map((i) => { const q = partById(i); return [i, { pos: { ...q.pos }, facing: q.facing }]; }));
      // a click without moving on a spot with several parts passes the selection on
      S.drag = {
        kind: "move", pid: ev.pointerId, id: p.id, ids, starts, sx: mm.x, sy: mm.y, pos0: { ...p.pos }, view: S.view,
        q0: toFaceC(curCase(), S.view, p.pos), box0: { x: b.x, y: b.y }, moved: false, alt0: ev.altKey, solo: ids.length === 1, collapse,
        cycle: hits.length > 1 && wasSel === p.id && S.sels.size === 1 && !ev.shiftKey ? hits : null,
        base: hitsOthers(ids), last: { du: 0, dv: 0 }, stuck: false,
      };
    } else if (pan) {
      const v = vp();
      S.drag = { kind: "pan", pid: ev.pointerId, px: ev.clientX, py: ev.clientY, cx: v.cx, cy: v.cy };
      svg.classList.add("panning");
    } else { // empty space: a frame selects, Shift adds to the selection
      if (!ev.shiftKey) setSel([], null);
      S.drag = { kind: "band", pid: ev.pointerId, x0: mm.x, y0: mm.y, keep: ev.shiftKey ? selIds() : [], single: ev.altKey, moved: false, sx: ev.clientX, sy: ev.clientY };
    }
    try { svg.setPointerCapture(ev.pointerId); } catch { /* the pointer is already gone */ }
    ev.preventDefault();
  }
  function startStep(d) {
    d.moved = true; S.undo.push(JSON.stringify(S.doc)); if (S.undo.length > 100) S.undo.shift(); S.redo = []; S.lastKey = ""; renderHead();
  }
  /**
   * Moves the parts of a drag by (du, dv) view millimetres from where they started. Without `force` the move is refused
   * when it makes a part overlap another one that it did not overlap before. Returns whether it was applied.
   */
  function tryMove(d, du, dv, force) {
    const p = partById(d.id);
    const np = posFromView(d.view, d.pos0, d.q0.u + du, d.q0.v + dv);
    const prev = d.ids.map((id) => { const q = partById(id); return [q, q.pos, q.facing]; });
    if (d.solo) {
      p.facing = d.starts.get(d.id).facing;
      const m = mountSnap(p, np);
      if (m) { p.facing = m.facing; setPos(p, m.pos); } else setPos(p, np);
    } else {
      const s = rigidShift(d.ids, { x: np.x - d.pos0.x, y: np.y - d.pos0.y, z: np.z - d.pos0.z }, (q) => d.starts.get(q.id).pos);
      for (const id of d.ids) { const q = partById(id), s0 = d.starts.get(id).pos; q.pos = { x: r2(s0.x + s.x), y: r2(s0.y + s.y), z: r2(s0.z + s.z) }; }
    }
    if (!force) {
      for (const k of hitsOthers(d.ids)) {
        if (d.base.has(k)) continue;
        for (const [q, pos, facing] of prev) { q.pos = pos; q.facing = facing; }
        return false;
      }
    }
    d.last = { du, dv };
    return true;
  }
  function onPointerMove(ev) {
    const mm = toMM(ev);
    S.cursor = mm;
    const d = S.drag;
    if (!d) { hoverLed(ev); schedule(true); return; }
    if (d.kind === "pan") {
      const v = vp();
      v.cx = d.cx - (ev.clientX - d.px) / v.scale; v.cy = d.cy - (ev.clientY - d.py) / v.scale; v.fit = false;
    } else if (d.kind === "band") {
      if (!d.moved && Math.hypot(ev.clientX - d.sx, ev.clientY - d.sy) < 4) return;
      d.moved = true;
      S.band = { x0: d.x0, y0: d.y0, x1: mm.x, y1: mm.y };
    } else if (d.kind === "move") {
      const p = partById(d.id);
      if (!p) return;
      let du = mm.x - d.sx, dv = mm.y - d.sy;
      if (!d.moved && Math.hypot(du, dv) * vp().scale < 3) return; // a click, not a drag
      if (!d.moved) startStep(d);
      if (S.snap) { // the box corner of the part under the pointer snaps to the grid
        du = snapV(d.box0.x + du, true) - d.box0.x; dv = snapV(d.box0.y + dv, true) - d.box0.y;
      }
      // Alt held during the drag skips the collision check. Otherwise the part runs up to the obstacle and may slide along it.
      // The way is walked in small steps so a quick pointer cannot jump over a part.
      const reach = (a, b) => {
        const fu = d.last.du, fv = d.last.dv, n = clamp(Math.ceil(Math.hypot(a - fu, b - fv) / 4), 1, 250);
        for (let i = 1; i <= n; i++) {
          if (tryMove(d, fu + ((a - fu) * i) / n, fv + ((b - fv) * i) / n, false)) continue;
          let lo = (i - 1) / n, hi = i / n;
          for (let k = 0; k < 6; k++) { const m = (lo + hi) / 2; if (tryMove(d, fu + (a - fu) * m, fv + (b - fv) * m, false)) lo = m; else hi = m; }
          return false;
        }
        return true;
      };
      let ok = true;
      if (ev.altKey) tryMove(d, du, dv, true);
      else if (!reach(du, dv)) { ok = false; reach(du, d.last.dv); reach(d.last.du, dv); }
      if (!ok && !d.stuck) live(t("sk.blocked"));
      d.stuck = !ok;
      svgEl().classList.toggle("blocked", !ok);
      d.dirty = true;
    } else if (d.kind === "rot") {
      const p = partById(d.id);
      if (!p) return;
      let a = (Math.atan2(mm.y - d.cy, mm.x - d.cx) * 180) / Math.PI + 90; // handle is above the box
      a = ((a % 360) + 360) % 360;
      a = ev.shiftKey ? Math.round(a) : Math.round(a / 15) * 15;
      if (d.det < 0) a = (360 - a) % 360; // the view shows this part mirrored: the sense of turning flips
      if (a >= 360) a = 0;
      if (a !== p.spin) {
        if (!d.moved) startStep(d);
        p.spin = a; clampInto(p); d.dirty = true;
        S.insSync.forEach((f) => f());
      }
    }
    if (d.kind === "move") schedule(true); else schedule();
  }
  function onPointerUp(ev) {
    const d = S.drag;
    if (!d) return;
    S.drag = null;
    const svg = svgEl();
    svg.classList.remove("panning", "blocked");
    try { svg.releasePointerCapture(ev.pointerId); } catch { /* already released */ }
    if (d.kind === "pan") return;
    if (d.kind === "band") {
      S.band = null;
      if (d.moved) {
        const hit = partsInRect({ x0: d.x0, y0: d.y0, x1: toMM(ev).x, y1: toMM(ev).y });
        const ids = d.single ? hit : [...new Set(hit.flatMap(groupMembers))];
        setSel([...d.keep, ...ids], ids.at(-1));
      }
      schedule();
      return;
    }
    if (d.dirty) afterChange();
    else if (d.kind === "move" && d.collapse) pick(d.collapse, d.alt0); // plain click on a member of a selection
    else if (d.kind === "move" && d.cycle) { // plain click on a stack of parts: the next one
      const i = d.cycle.indexOf(d.id);
      pick(d.cycle[(i + 1) % d.cycle.length], d.alt0);
    }
  }
  /** Escape during a drag: the parts go back to where they were and the undo step disappears. */
  function cancelDrag() {
    const d = S.drag;
    if (!d || d.kind === "pan") return;
    S.drag = null;
    const svg = svgEl();
    svg.classList.remove("blocked");
    try { svg.releasePointerCapture(d.pid); } catch { /* already released */ }
    S.band = null;
    if (d.kind === "move" && d.moved) {
      for (const id of d.ids) { const q = partById(id), s0 = d.starts.get(id); if (q) { q.pos = { ...s0.pos }; q.facing = s0.facing; } }
      S.undo.pop();
    } else if (d.kind === "rot" && d.moved) {
      const p = partById(d.id);
      if (p) p.spin = d.spin0;
      S.undo.pop();
    }
    afterChange();
  }

  /** The case face a library mount sits on: the axis, which end of it, and the facing that looks into the case. */
  const WALLS = {
    top: { axis: "y", end: 0, facing: "bottom" }, bottom: { axis: "y", end: 1, facing: "top" },
    front: { axis: "z", end: 1, facing: "rear" }, rear: { axis: "z", end: 0, facing: "front" },
    side: { axis: "x", end: 0, facing: "tray" },
  };
  /**
   * Fans snap onto a mounting position of a library case when their centre is near one: the position along the wall
   * is taken from the mount, the fan lies flat on the wall and its LED side faces into the case.
   * Without the mount's face (older catalog) nothing snaps.
   */
  function mountSnap(p, np) {
    const part = partOf(p.part), tpl = S.doc.case.source === "library" && caseOf(S.doc.case.id);
    if (!tpl || !isFan(part) || !p3of(p)) return null;
    const c = curCase();
    let best = null, bd = 12;
    for (const m of tpl.mounts || []) {
      const wall = WALLS[m.face];
      if (!wall) continue;
      const wallAt = wall.end ? dimOf(wall.axis) : 0;
      if (Math.abs(np[wall.axis] - wallAt) > 60) continue; // only near the wall the mount belongs to
      for (const lay of m.layouts || []) {
        if (Math.abs(lay.fan_mm - part.dimensions_mm.w) > 2) continue;
        for (const [su, sw] of lay.snap_mm || []) {
          const r = fromViewC(c, m.view, su, sw);
          const used = AXES.filter((a) => a !== r.free && a !== wall.axis);
          if (!used.length) continue;
          const dist = Math.hypot(...used.map((a) => r.p[a] - np[a]));
          if (dist < bd) { bd = dist; best = { r, used, wall }; }
        }
      }
    }
    if (!best) return null;
    const pos = { ...np }, half = part.dimensions_mm.d / 2;
    for (const a of best.used) pos[a] = best.r.p[a];
    pos[best.wall.axis] = best.wall.end ? dimOf(best.wall.axis) - half : half;
    return { pos, facing: best.wall.facing };
  }
  function hoverLed(ev) {
    const tip = $("#sk-tip"), n = ev.target;
    const led = n && n.classList && n.classList.contains("sk-led") ? n : null;
    if (!led) { if (!tip.hidden) tip.hidden = true; return; }
    const g = led.closest(".sk-part"), p = partById(g.dataset.id), i = +led.dataset.led;
    const info = p && chains().info.get(p.id);
    const r = $("#sk-stage").getBoundingClientRect();
    tip.textContent = info ? t("sk.led.zone", { n: i + 1, z: info.start + i + 1 }) : t("sk.led", { n: i + 1 });
    tip.hidden = false;
    tip.style.left = Math.min(ev.clientX - r.left + 12, r.width - tip.offsetWidth - 6) + "px";
    tip.style.top = Math.max(ev.clientY - r.top - 30, 4) + "px";
  }
  function onWheel(ev) {
    ev.preventDefault();
    const r = $("#sk-stage").getBoundingClientRect();
    zoomAt(ev.clientX - r.left, ev.clientY - r.top, Math.pow(1.0018, -ev.deltaY * (ev.deltaMode === 1 ? 16 : 1)));
  }
  function onDrop(ev) {
    const id = ev.dataTransfer && ev.dataTransfer.getData("text/x-casehud-part");
    if (!id) return;
    ev.preventDefault();
    addPart(id, toMM(ev));
  }

  // ---------- keyboard ----------
  const typing = (n) => !!n && (n.tagName === "INPUT" || n.tagName === "SELECT" || n.tagName === "TEXTAREA" || n.isContentEditable);
  /** Moves the selection (and what is mounted on it) by a shift in x, y, z. Refused when a part would newly overlap another one. */
  function shiftSelection(s0, key) {
    const ids = moveSet(selIds());
    const s = rigidShift(ids, s0);
    if (!ids.length || (!s.x && !s.y && !s.z)) return;
    const moved = (p) => ({ x: r2(p.pos.x + s.x), y: r2(p.pos.y + s.y), z: r2(p.pos.z + s.z) });
    const old = new Map(ids.map((id) => [id, partById(id).pos])), base = hitsOthers(ids);
    for (const id of ids) { const p = partById(id); p.pos = moved(p); }
    let blocked = false;
    for (const k of hitsOthers(ids)) if (!base.has(k)) { blocked = true; break; }
    for (const id of ids) partById(id).pos = old.get(id);
    if (blocked) { live(t("sk.blocked")); return; }
    mutate(() => { for (const id of ids) { const p = partById(id); p.pos = moved(p); } }, key);
  }
  /** A shift of the primary part by du, dv in the current view. */
  function viewShift(du, dv) {
    const p = selPart(), q = toFaceC(curCase(), S.view, p.pos), np = posFromView(S.view, p.pos, q.u + du, q.v + dv);
    return { x: np.x - p.pos.x, y: np.y - p.pos.y, z: np.z - p.pos.z };
  }
  function deleteIds(ids) {
    const fixed = ids.some((id) => isCL(partById(id)));
    ids = ids.filter((id) => partById(id) && !isCL(partById(id)));
    if (fixed && !ids.length) toast(t("sk.cl.nodelete"));
    if (!ids.length) return;
    endTest();
    const name = ids.length === 1 ? partTitle(partById(ids[0])) : t("sk.group.count", { n: ids.length });
    const gone = new Set(ids);
    mutate(() => { S.doc.parts = parts().filter((q) => !gone.has(q.id)).map((q) => (gone.has(q.on) ? { ...q, on: "" } : q)); tidyGroups(); pruneSel(); });
    live(t("sk.live.deleted", { n: name }));
  }
  function onKey(ev) {
    if (!S.open || !$("#tab-light").classList.contains("active")) return;
    const k = ev.key;
    if (k === " " && !typing(ev.target) && ev.target.tagName !== "BUTTON" && ev.target.tagName !== "SUMMARY") { S.space = true; svgEl().classList.add("pan-ready"); ev.preventDefault(); return; }
    const mod = ev.ctrlKey || ev.metaKey;
    if (mod && !typing(ev.target)) {
      const lk = k.toLowerCase();
      if (lk === "z") { ev.preventDefault(); ev.shiftKey ? redo() : undo(); return; }
      if (lk === "y") { ev.preventDefault(); redo(); return; }
      if (lk === "g") { ev.preventDefault(); ev.shiftKey ? unweld() : weld(); return; }
      if (lk === "a") { ev.preventDefault(); const all = shown().map((p) => p.id); setSel(all, all.at(-1)); return; }
    }
    if (mod && k.toLowerCase() === "s") { ev.preventDefault(); save(); return; }
    if (typing(ev.target) || (ev.target.tagName === "BUTTON" && k === "Enter")) return;
    if (mod && k.toLowerCase() === "d" && S.sel) { ev.preventDefault(); duplicateSelected(); return; }
    if (mod || ev.altKey) return; // Alt+Arrow and friends belong to the browser
    const p = selPart();
    if (k === "Escape") {
      if (S.drag && S.drag.kind !== "pan") { cancelDrag(); ev.preventDefault(); } else if (S.sels.size) { select(null); ev.preventDefault(); }
      return;
    }
    if ((k === "Delete" || k === "Backspace") && p) { ev.preventDefault(); deleteIds(selIds()); return; }
    if ((k === "r" || k === "R") && p) { ev.preventDefault(); rotateSelection(ev.shiftKey ? -1 : 1); return; }
    if (k.startsWith("Arrow") && p) {
      ev.preventDefault();
      const st = ev.shiftKey ? 10 : 1;
      shiftSelection(viewShift(k === "ArrowRight" ? st : k === "ArrowLeft" ? -st : 0, k === "ArrowDown" ? st : k === "ArrowUp" ? -st : 0), "nudge" + S.sel);
      return;
    }
    if ((k === "PageUp" || k === "PageDown") && p) { // up: towards the viewer
      ev.preventDefault();
      const st = ev.shiftKey ? 10 : 1, { axis, sign } = depthAxis(S.view);
      shiftSelection({ x: 0, y: 0, z: 0, [axis]: sign * (k === "PageUp" ? -st : st) }, "depth" + S.sel);
      return;
    }
    if (k === "+" || k === "=") { zoomCentre(1.25); ev.preventDefault(); } else if (k === "-") { zoomCentre(0.8); ev.preventDefault(); } else if (k === "0") { fitView(); draw(); ev.preventDefault(); }
  }
  function onKeyUp(ev) { if (ev.key === " ") { S.space = false; svgEl().classList.remove("pan-ready"); } }

  // ---------- inspector ----------
  const field = (label, ...kids) => E("div", { class: "field" }, E("span", {}, label), ...kids);
  /** Number input committing on input (undo steps merged). */
  function numInput(key, get, set, o = {}) {
    const commit = (e) => { const v = e.target.value; if (v === "" || !Number.isFinite(+v)) return; mutate(() => set(+v), key); };
    // o.late: the value is taken when the field is left (a size typed digit by digit must not rearrange the sketch)
    const i = E("input", { type: "number", "data-f": key, step: o.step ?? 1, min: o.min, max: o.max, value: r2(get()), "aria-label": o.aria || key,
      oninput: o.late ? undefined : commit,
      onchange: (e) => { if (o.late) commit(e); e.target.value = r2(get()); } });
    S.insSync.push(() => { if (document.activeElement !== i) i.value = r2(get()); });
    return i;
  }
  function seg(key, items, get, set, aria, cls = "") {
    const box = E("div", { class: "seg small " + cls, role: "group", "aria-label": aria });
    const btns = items.map(([v, label, dis]) => E("button", { type: "button", "data-f": key + ":" + v, disabled: !!dis, "aria-pressed": "false", onclick: () => set(v) }, label));
    box.append(...btns);
    const sync = () => btns.forEach((b, i) => { const on = items[i][0] === get(); b.classList.toggle("active", on); b.setAttribute("aria-pressed", String(on)); });
    sync(); S.insSync.push(sync);
    return box;
  }
  function switchRow(key, label, sub, get, set) {
    const i = E("input", { type: "checkbox", class: "switch", "data-f": key, "aria-label": label, checked: !!get(), onchange: (e) => mutate(() => set(e.target.checked)) });
    S.insSync.push(() => { i.checked = !!get(); });
    return E("label", { class: "sk-switch" }, i, E("span", { class: "row-text" }, E("b", {}, label), sub ? E("small", {}, sub) : null));
  }

  function renderInspector(force) {
    const box = $("#sk-inspector");
    const p = selPart();
    const sig = [selIds().join(","), p && p.bind && p.bind.device, !!p, p && (p.group || "") + "/" + (p.on || ""), LANG, devices().map((d) => d.id + d.zones.length).join(), S.doc.case.source, S.doc.case.id].join("|");
    if (!force && sig === S.insSig) { S.insSync.forEach((f) => f()); fillDynamic(); return; }
    S.insSig = sig;
    const keepScroll = S.insSel === S.sel;
    S.insSel = S.sel;
    const focusKey = document.activeElement && box.contains(document.activeElement) ? /** @type {any} */ (document.activeElement).dataset.f : "";
    S.insSync = [];
    box.replaceChildren(...(S.sels.size > 1 ? multiInspector() : p ? (isCL(p) ? caseLightInspector(p) : partInspector(p)) : caseInspector()), warnBox());
    S.insSync.forEach((f) => f());
    fillDynamic();
    if (!keepScroll) box.scrollTop = 0;
    if (focusKey) { const n = box.querySelector(`[data-f="${focusKey}"]`); if (n) n.focus({ preventScroll: true }); }
  }
  // warnings and the range hint change with every edit but must not rebuild the inputs
  function warnBox() { return E("section", { class: "sk-sec sk-warns" }); }
  function fillDynamic() {
    const w = $(".sk-warns");
    if (w) {
      const list = warnings();
      w.replaceChildren(E("h3", {}, t("sk.warn.title"), list.length ? E("span", { class: "badge sk-b warn" }, String(list.length)) : null),
        list.length ? E("ul", {}, list.map((x) => E("li", { class: x.kind }, x.id ? E("button", { class: "link", type: "button", onclick: () => focusPart(x.id) }, x.text) : x.text)))
          : E("p", { class: "sub" }, t("sk.warn.none")));
    }
    const rg = $(".sk-range");
    if (rg) rg.textContent = bindRangeText(selPart());
    const p = selPart(), q = p && p3of(p);
    const sz = $(".sk-part-size");
    if (sz && p) { const part = partOf(p.part); sz.textContent = q ? t("sk.part.size3", { w: fmt(q.max.x - q.min.x), h: fmt(q.max.y - q.min.y), d: fmt(q.max.z - q.min.z), n: q.leds.length }) : ""; }
    for (const n of $$(".sk-wall")) {
      const a = n.dataset.axis;
      n.textContent = q ? t("sk.wall." + a, { a: fmt(q.min[a]), b: fmt(dimOf(a) - q.max[a]) }) : "";
    }
    renderTest();
  }
  function bindRangeText(p) {
    if (!p || !p.bind) return "";
    const i = chains().info.get(p.id);
    if (!i || !i.n) return "";
    return i.n === 1 ? t("sk.bind.range1", { a: i.start + 1 }) : t("sk.bind.range", { a: i.start + 1, b: i.start + i.n });
  }

  function caseInspector() {
    const c = S.doc.case, lib = c.source === "library";
    const tpls = Object.values((S.cat && S.cat.cases) || {}).filter((x) => x.id !== "custom-box").sort((a, b) => caseName(a).localeCompare(caseName(b)));
    const sel = E("select", { "data-f": "case", "aria-label": t("sk.case.type"), onchange: (e) => setCase(e.target.value) },
      E("option", { value: "custom-box" }, t("sk.case.custom")),
      tpls.length ? E("optgroup", { label: t("sk.case.library") }, tpls.map((x) => E("option", { value: x.id }, caseName(x)))) : null);
    sel.value = lib ? c.id : "custom-box";
    S.insSync.push(() => { if (document.activeElement !== sel) sel.value = S.doc.case.source === "library" ? S.doc.case.id : "custom-box"; });
    const dim = (k, label) => E("label", { class: "field" }, E("span", {}, label),
      numInput("dim" + k, () => S.doc.case.dims[k], (v) => setDim(k, v), { min: 1, max: S.lim.maxDimMm, aria: label, late: true }));
    const dimsBox = E("div", { class: "sk-dims" }, dim("w", t("sk.case.w")), dim("h", t("sk.case.h")), dim("d", t("sk.case.d")));
    if (lib) for (const i of $$("input", dimsBox)) i.disabled = true;
    const hasDiv = (c.chambers || []).length > 1;
    const divRow = lib ? null : E("div", { class: "sk-divider-row" },
      switchRow("divider", t("sk.case.divider"), t("sk.case.divider.hint"), () => (S.doc.case.chambers || []).length > 1,
        (on) => { const cc = S.doc.case; cc.chambers = on ? [{ x: 0, w: r2(cc.dims.w * 0.65) }, { x: r2(cc.dims.w * 0.65), w: r2(cc.dims.w * 0.35) }] : []; renderInspector(true); }),
      hasDiv ? field(t("sk.case.divider.x"), numInput("divx", () => (S.doc.case.chambers[0] || { w: 0 }).w, (v) => setDivider(v), { min: 1, max: c.dims.w - 1 })) : null);
    const glass = lib ? null : field(t("sk.case.glass"), seg("glass", [["left", t("sk.case.glass.left")], ["right", t("sk.case.glass.right")]],
      () => (glassRight() ? "right" : "left"), (v) => mutate(() => { setGlass(v); }), t("sk.case.glass")));
    const list = parts();
    return [
      E("section", { class: "sk-sec" }, E("h3", {}, t("sk.case")),
        field(t("sk.case.type"), sel), lib ? E("p", { class: "sub" }, t("sk.case.fixed")) : null,
        field(t("sk.case.dims"), dimsBox), divRow, glass),
      E("section", { class: "sk-sec" }, E("h3", {}, t("sk.parts"), list.length ? E("span", { class: "badge sk-b" }, String(list.length)) : null),
        list.length ? E("ul", { class: "sk-plist" }, list.map((q) => {
          const col = q.bind ? devColor(q.bind.device) : null;
          return E("li", {}, E("button", { type: "button", class: "sk-prow", "aria-label": t("sk.parts.item", { n: partTitle(q) }), onclick: () => focusPart(q.id) },
            E("i", { class: "sk-dot" + (col ? "" : " off"), style: col ? { background: col } : {} }),
            E("span", { class: "sk-prow-text" }, E("b", {}, partTitle(q)), E("small", {}, (isCL(q) ? t("sk.cl.short") : t("sk.facing." + q.facing)) + " · " + (q.bind ? t("sk.bound") : t("sk.unbound")))),
            E("span", { class: "sk-prow-n" }, String(ledCount(q)))));
        })) : E("p", { class: "sub" }, t("sk.parts.none"))),
    ];
  }
  function setGlass(v) {
    const c = S.doc.case, o = { ...(c.orientation || {}) };
    if (v === "right") o.glass_side = "right"; else delete o.glass_side;
    if (Object.keys(o).length) c.orientation = o; else delete c.orientation;
  }
  /** A new case size: the divider keeps its relative place and parts stay within the allowed range. */
  function setDim(k, v) {
    const c = S.doc.case, w0 = c.dims.w;
    c.dims[k] = clamp(v, 1, S.lim.maxDimMm);
    if (k === "w" && (c.chambers || []).length > 1 && w0 > 0) setDivider((c.chambers[0].w / w0) * c.dims.w);
    const n = fitParts();
    if (n) toast(t("sk.dims.moved", { n }));
  }
  function setDivider(v) {
    const c = S.doc.case;
    const x = clamp(v, Math.min(1, c.dims.w / 2), Math.max(c.dims.w - 1, c.dims.w / 2));
    c.chambers = [{ x: 0, w: r2(x) }, { x: r2(x), w: r2(c.dims.w - x) }];
  }
  function setCase(id) {
    mutate(() => {
      const c = S.doc.case;
      if (id === "custom-box") { c.source = "custom"; c.id = "custom-box"; syncCaseLights(S.doc, S.cat && S.cat.cases); pruneSel(); return; }
      const tpl = caseOf(id);
      if (!tpl) return;
      c.source = "library"; c.id = id; c.dims = { w: tpl.dims.w, h: tpl.dims.h, d: tpl.dims.d }; c.chambers = (tpl.chambers || []).map((q) => ({ x: q.x, w: q.w }));
      const o = {};
      if (tpl.orientation && tpl.orientation.glass_side === "right") o.glass_side = "right";
      if (Object.keys(o).length) c.orientation = o; else delete c.orientation;
      syncCaseLights(S.doc, S.cat && S.cat.cases); pruneSel();
      const n = fitParts();
      if (n) toast(t("sk.dims.moved", { n }));
    });
    fitView(); renderInspector(true); schedule();
  }

  /** A light built into the case: name, LED count and direction, arrangement and binding; it stays where the case has it. */
  function caseLightInspector(p) {
    const part = partOf(p.part);
    if (!part) return partInspector(p);
    const tpl = S.doc.case.source === "library" ? caseOf(S.doc.case.id) : null, n = ledCount(p);
    const faces = facesOf(p).map((f) => t("sk.face." + f)).join(" → ");
    const out = [E("section", { class: "sk-sec" },
      E("button", { class: "link sk-up", type: "button", onclick: () => select(null) }, "‹ " + t("sk.case")),
      E("div", { class: "sk-part-head" },
        E("div", { class: "sk-part-title" }, E("h3", {}, partTitle(p)), E("small", {}, t("sk.cl.sub", { f: faces, n })))),
      E("p", { class: "sub sk-cl-note" }, t("sk.cl.fixed")),
      (tpl && tpl.approximate) || isApprox(part) ? E("span", { class: "badge sk-b warn sk-approx", title: t("sk.approx.tip") }, t("sk.approx")) : null,
      field(t("sk.f.label"), E("input", { type: "text", "data-f": "label", maxlength: S.lim.maxLabel, placeholder: clName(S.cat, p), value: p.label || "", "aria-label": t("sk.f.label"),
        oninput: (e) => mutate(() => { p.label = e.target.value; }, "label" + p.id) })))];
    out.push(E("section", { class: "sk-sec" },
      E("p", { class: "sub" }, t("sk.f.leds.fixed", { n })),
      switchRow("mirror", t("sk.cl.reverse"), t("sk.cl.reverse.sub"), () => p.mirror, (on) => { p.mirror = on; })));
    out.push(E("section", { class: "sk-sec" }, E("h3", {}, t("sk.arrange")),
      field(t("sk.f.layer"), numInput("layer", () => p.layer || 0, (v) => { p.layer = clamp(Math.round(v), -maxLayer(), maxLayer()); }, { min: -maxLayer(), max: maxLayer(), aria: t("sk.f.layer") })),
      switchRow("visible", t("sk.f.visible"), t("sk.f.visible.sub"), () => !p.hidden, (on) => { if (on) delete p.hidden; else p.hidden = true; })));
    out.push(bindSection(p));
    return out;
  }

  function partInspector(p) {
    const part = partOf(p.part);
    if (!part) return [E("section", { class: "sk-sec" }, E("h3", {}, p.part), E("p", { class: "sub warn" }, t("sk.w.unknown", { p: partTitle(p), id: p.part })),
      E("button", { class: "danger", type: "button", onclick: () => deletePart(p.id) }, t("sk.part.del")))];
    const g = gen(part);
    const out = [];
    out.push(E("section", { class: "sk-sec" },
      E("button", { class: "link sk-up", type: "button", onclick: () => select(null) }, "‹ " + t("sk.case")),
      E("div", { class: "sk-part-head" },
        E("div", { class: "sk-part-title" }, E("h3", {}, partName(part)), E("small", { class: "sk-part-size" }, "")),
        E("button", { class: "ghost icon-btn sm", type: "button", title: t("sk.part.dup") + " (Ctrl+D)", "aria-label": t("sk.part.dup"), onclick: () => duplicatePart(p.id) }, svgIcon("M9 9h10v10H9zM5 15V5h10")),
        E("button", { class: "ghost icon-btn sm danger-text", type: "button", title: t("sk.part.del") + " (Del)", "aria-label": t("sk.part.del"), onclick: () => deletePart(p.id) }, svgIcon("M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"))),
      !isGeneric(part) && isApprox(part) ? E("span", { class: "badge sk-b warn sk-approx", title: t("sk.approx.tip") }, t("sk.approx")) : null,
      field(t("sk.f.label"), E("input", { type: "text", "data-f": "label", maxlength: S.lim.maxLabel, placeholder: t("sk.f.label.ph"), value: p.label || "", "aria-label": t("sk.f.label"),
        oninput: (e) => mutate(() => { p.label = e.target.value; }, "label" + p.id) }))));

    // position: centre in the case box, with the distance to the walls
    const ax = (a) => E("div", { class: "sk-axis" },
      E("label", {}, E("small", {}, t("sk.f." + a)), numInput("pos" + a, () => p.pos[a], (v) => setPos(p, { ...p.pos, [a]: v }), { step: 1, aria: a })),
      E("small", { class: "sk-wall", "data-axis": a }, ""));
    out.push(E("section", { class: "sk-sec" },
      field(t("sk.f.pos"), E("div", { class: "sk-xyz" }, ax("x"), ax("y"), ax("z")), E("small", {}, t("sk.f.pos.hint")))));

    // orientation
    const quick = ["gpu-block", "distro-plate"].includes(part.category)
      ? field(t("sk.quick"), E("div", { class: "sk-quick" },
        E("button", { class: "ghost sk-btn", type: "button", "data-f": "q-vert", onclick: () => mutate(() => quickOrient(p, "vert")) }, t("sk.quick.vert")),
        E("button", { class: "ghost sk-btn", type: "button", "data-f": "q-horiz", onclick: () => mutate(() => quickOrient(p, "horiz")) }, t("sk.quick.horiz")),
        part.category === "distro-plate" ? E("button", { class: "ghost sk-btn", type: "button", "data-f": "q-floor", onclick: () => mutate(() => quickOrient(p, "floor")) }, t("sk.quick.floor")) : null))
      : null;
    const rotRow = E("div", { class: "sk-rotrow" },
      seg("rot", [0, 90, 180, 270].map((d) => [d, d + "°"]), () => ((Math.round(p.spin * 100) / 100) % 360 + 360) % 360, (d) => mutate(() => { p.spin = d; clampInto(p); }), t("sk.f.rot")),
      E("label", { class: "sk-rot-free" }, numInput("rotfree", () => p.spin, (v) => { p.spin = clamp(v, -maxSpin(), maxSpin()); clampInto(p); }, { min: -maxSpin(), max: maxSpin(), aria: t("sk.f.rot.free") }), E("small", {}, "°")));
    out.push(E("section", { class: "sk-sec" },
      field(t("sk.f.face"), seg("face", FACES.map((f) => [f, t("sk.face." + f)]), () => faceOf(p), (f) => moveToFace(p, f), t("sk.f.face"), "wrap"), E("small", {}, t("sk.f.face.sub")))));
    out.push(E("section", { class: "sk-sec" },
      field(t("sk.f.facing"), seg("facing", FACINGS.map((f) => [f, t("sk.facing." + f)]), () => p.facing, (f) => mutate(() => setFacing(p, f)), t("sk.f.facing"), "wrap"), E("small", {}, t("sk.f.facing.sub"))),
      quick,
      field(t("sk.f.rot"), rotRow, E("small", {}, t("sk.f.rot.sub"))),
      switchRow("mirror", t("sk.f.mirror"), t("sk.f.mirror.sub"), () => p.mirror, (on) => { p.mirror = on; })));
    if (isFan(part)) {
      out.push(E("section", { class: "sk-sec" }, field(t("sk.f.flow"),
        seg("flow", [["intake", t("sk.f.flow.intake")], ["exhaust", t("sk.f.flow.exhaust")]], () => p.flow || "intake", (v) => mutate(() => { p.flow = v; }), t("sk.f.flow")),
        E("small", {}, t("sk.f.flow.hint")))));
    }
    // LEDs
    const ledSec = E("section", { class: "sk-sec" });
    if (g) {
      ledSec.append(field(t("sk.f.leds"), numInput("leds", () => p.leds || g.default, (v) => { p.leds = clamp(Math.round(v), g.min, g.max); S.test = Math.min(S.test, p.leds - 1); clampInto(p); }, { min: g.min, max: g.max, aria: t("sk.f.leds") }),
        E("small", {}, t("sk.f.leds.range", { a: g.min, b: g.max }))));
      if (g.lengthMax > 0) {
        ledSec.append(field(t("sk.f.len"), numInput("len", () => p.len || part.dimensions_mm.w, (v) => { p.len = clamp(v, g.lengthMin, g.lengthMax); clampInto(p); },
          { min: g.lengthMin, max: g.lengthMax, aria: t("sk.f.len") }), E("small", {}, t("sk.f.len.range", { a: g.lengthMin, b: g.lengthMax }))));
      }
    } else ledSec.append(E("p", { class: "sub" }, t("sk.f.leds.fixed", { n: (part.leds.positions_mm || []).length })));
    out.push(ledSec);
    out.push(arrangeSection(p));
    out.push(bindSection(p));
    return out;
  }

  /** Arrangement of one part: group, carrier, layer, visibility. */
  function arrangeSection(p) {
    const sec = E("section", { class: "sk-sec" }, E("h3", {}, t("sk.arrange")));
    if (p.group) {
      const n = groupMembers(p.id).length;
      sec.append(E("div", { class: "sk-groupbox" },
        E("p", { class: "sub" }, t("sk.group.sub", { g: groupLabel(p.group), n })),
        E("div", { class: "sk-quick" },
          E("button", { class: "ghost sk-btn", type: "button", "data-f": "g-select", onclick: () => setSel(groupMembers(p.id), p.id) }, t("sk.group.select")),
          E("button", { class: "ghost sk-btn", type: "button", "data-f": "g-leave", onclick: () => leaveGroup(p.id) }, t("sk.group.leave")),
          E("button", { class: "ghost sk-btn", type: "button", "data-f": "g-unweld", onclick: () => { setSel(groupMembers(p.id), p.id); unweld(); } }, t("sk.unweld")))));
    }
    // one level only: a carrier is not mounted itself, and a part with mounted parts cannot be mounted
    const hasKids = parts().some((q) => q.on === p.id);
    const carriers = parts().filter((q) => q.id !== p.id && !q.on);
    const sel = E("select", { "data-f": "on", "aria-label": t("sk.on"), disabled: hasKids, onchange: (e) => {
      const v = e.target.value;
      mutate(() => { if (v) p.on = v; else delete p.on; });
      renderInspector(true);
    } },
    E("option", { value: "" }, t("sk.on.none")),
    carriers.map((q) => E("option", { value: q.id }, partTitle(q))),
    p.on && !carriers.some((q) => q.id === p.on) ? E("option", { value: p.on }, p.on) : null);
    sel.value = p.on || "";
    sec.append(field(t("sk.on"), sel, E("small", {}, t("sk.on.sub"))),
      field(t("sk.f.layer"), numInput("layer", () => p.layer || 0, (v) => { p.layer = clamp(Math.round(v), -maxLayer(), maxLayer()); }, { min: -maxLayer(), max: maxLayer(), aria: t("sk.f.layer") })),
      switchRow("visible", t("sk.f.visible"), t("sk.f.visible.sub"), () => !p.hidden, (on) => { if (on) delete p.hidden; else p.hidden = true; }));
    return sec;
  }
  /** Inspector of a selection of several parts. */
  function multiInspector() {
    const ps = selIds().map(partById).filter(Boolean);
    const grouped = ps.some((p) => p.group), whole = ps.every((p) => p.group && p.group === ps[0].group) && groupMembers(ps[0].id).length === ps.length;
    const btn = (f, label, tip, fn, cls = "ghost sk-btn", dis = false) => E("button", { class: cls, type: "button", "data-f": f, title: tip, disabled: dis, onclick: fn }, label);
    return [
      E("section", { class: "sk-sec" },
        E("button", { class: "link sk-up", type: "button", onclick: () => select(null) }, "‹ " + t("sk.case")),
        E("div", { class: "sk-part-head" }, E("div", { class: "sk-part-title" }, E("h3", {}, t("sk.multi.title", { n: ps.length })), E("small", {}, t("sk.multi.sub")))),
        E("div", { class: "sk-quick" },
          btn("weld", t("sk.weld"), t("sk.weld.tip"), weld, "ghost sk-btn", whole),
          btn("unweld", t("sk.unweld"), t("sk.unweld.tip"), unweld, "ghost sk-btn", !grouped),
          btn("rot-cw", t("sk.rot.cw"), t("sk.rot.tip"), () => rotateSelection(1)),
          btn("rot-ccw", t("sk.rot.ccw"), t("sk.rot.tip"), () => rotateSelection(-1)),
          btn("dup", t("sk.part.dup"), t("sk.part.dup") + " (Ctrl+D)", duplicateSelected),
          btn("del", t("sk.part.del"), t("sk.part.del") + " (Del)", () => deleteIds(selIds()), "ghost sk-btn danger-text"))),
      E("section", { class: "sk-sec" }, E("h3", {}, t("sk.parts"), E("span", { class: "badge sk-b" }, String(ps.length))),
        E("ul", { class: "sk-plist" }, ps.map((q) => E("li", {}, E("button", { type: "button", class: "sk-prow", onclick: () => select(q.id) },
          E("span", { class: "sk-prow-text" }, E("b", {}, partTitle(q)), E("small", {}, (q.group ? groupLabel(q.group) + " · " : "") + t("sk.facing." + q.facing))),
          E("span", { class: "sk-prow-n" }, String(ledCount(q)))))))),
    ];
  }

  // ---------- layers panel ----------
  const eyeIcon = (off) => svgIcon(off ? "M3 3l18 18M10.6 6.2A9.6 9.6 0 0 1 12 6c5 0 8.5 4 9.5 6a13 13 0 0 1-2.2 3M6.6 7.6A13 13 0 0 0 2.5 12c1 2 4.5 6 9.5 6 1.5 0 2.8-.3 4-.9M9.9 9.9a3 3 0 0 0 4.2 4.2" : "M2.5 12c1-2 4.5-6 9.5-6s8.5 4 9.5 6c-1 2-4.5 6-9.5 6s-8.5-4-9.5-6zM12 9.5a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5z");
  const chevIcon = (up) => svgIcon(up ? "M6 15l6-6 6 6" : "M6 9l6 6 6-6");
  function setHidden(ids, hide) {
    mutate(() => { for (const id of ids) { const p = partById(id); if (hide) p.hidden = true; else delete p.hidden; } });
    const p = partById(ids[0]);
    if (ids.length === 1 && p) live(t(hide ? "sk.live.hidden" : "sk.live.shown", { n: partTitle(p) }));
  }
  function shiftLayer(ids, delta) {
    mutate(() => { for (const id of ids) { const p = partById(id); p.layer = clamp((p.layer || 0) + delta, -maxLayer(), maxLayer()); } });
  }
  /** The parts from the top layer down, a group as one block at the place of its topmost part. */
  function layerRows() {
    const order = drawOrder(true).reverse(), out = [], seen = new Set();
    for (const p of order) {
      if (seen.has(p.id)) continue;
      if (!p.group) { seen.add(p.id); out.push({ parts: [p] }); continue; }
      const m = order.filter((q) => q.group === p.group);
      m.forEach((q) => seen.add(q.id));
      out.push({ group: p.group, parts: m });
    }
    return out;
  }
  function layerRow(p, indent) {
    const col = p.bind ? devColor(p.bind.device) : null, lay = p.layer || 0;
    const row = E("div", { class: "sk-lrow" + (S.sels.has(p.id) ? " sel" : "") + (p.hidden ? " off" : "") + (indent ? " in" : ""), "data-id": p.id },
      E("button", { class: "ghost icon-btn sm sk-eye", type: "button", "data-f": "eye", title: t(p.hidden ? "sk.layer.show" : "sk.layer.hide"), "aria-label": t(p.hidden ? "sk.layer.show" : "sk.layer.hide") + ": " + partTitle(p), "aria-pressed": String(!p.hidden), onclick: () => setHidden([p.id], !p.hidden) }, eyeIcon(!!p.hidden)),
      E("button", { class: "sk-lname", type: "button", "aria-label": t("sk.parts.item", { n: partTitle(p) }), onclick: (e) => (e.shiftKey ? setSel(S.sels.has(p.id) ? selIds().filter((i) => i !== p.id) : [...selIds(), p.id], p.id) : select(p.id)) },
        E("i", { class: "sk-dot" + (col ? "" : " off"), style: col ? { background: col } : {} }), E("span", {}, partTitle(p))),
      E("span", { class: "sk-lnum", title: t("sk.layer.n", { n: lay }) }, String(lay)),
      E("button", { class: "ghost icon-btn sm", type: "button", "data-f": "up", title: t("sk.layer.up"), "aria-label": t("sk.layer.up") + ": " + partTitle(p), disabled: lay >= maxLayer(), onclick: () => shiftLayer([p.id], 1) }, chevIcon(true)),
      E("button", { class: "ghost icon-btn sm", type: "button", "data-f": "down", title: t("sk.layer.down"), "aria-label": t("sk.layer.down") + ": " + partTitle(p), disabled: lay <= -maxLayer(), onclick: () => shiftLayer([p.id], -1) }, chevIcon(false)));
    return row;
  }
  function renderLayers() {
    const box = $("#sk-layers");
    if (!box || box.hidden || !S.doc) return;
    const rows = layerRows();
    if (!rows.length) { box.replaceChildren(E("p", { class: "empty-note" }, t("sk.layers.none"))); return; }
    box.replaceChildren(E("p", { class: "sk-lhint" }, t("sk.layers.hint")), ...rows.map((r) => {
      if (!r.group) return layerRow(r.parts[0], false);
      const ids = r.parts.map((q) => q.id), allHidden = r.parts.every((q) => q.hidden), topLayer = Math.max(...r.parts.map((q) => q.layer || 0)), lowLayer = Math.min(...r.parts.map((q) => q.layer || 0));
      const on = ids.every((i) => S.sels.has(i));
      return E("div", { class: "sk-lgroup" + (on ? " sel" : "") + (allHidden ? " off" : ""), "data-group": r.group },
        E("div", { class: "sk-lhead" },
          E("button", { class: "ghost icon-btn sm sk-eye", type: "button", "data-f": "geye", title: t(allHidden ? "sk.layer.show" : "sk.layer.hide"), "aria-label": t(allHidden ? "sk.layer.show" : "sk.layer.hide") + ": " + groupLabel(r.group), onclick: () => setHidden(ids, !allHidden) }, eyeIcon(allHidden)),
          E("button", { class: "sk-lname", type: "button", "data-f": "gsel", title: t("sk.group.select"), onclick: () => setSel(ids, ids[0]) },
            E("b", {}, groupLabel(r.group)), E("small", {}, t("sk.group.count", { n: ids.length }))),
          E("button", { class: "ghost icon-btn sm", type: "button", "data-f": "gup", title: t("sk.layer.up"), "aria-label": t("sk.layer.up") + ": " + groupLabel(r.group), disabled: topLayer >= maxLayer(), onclick: () => shiftLayer(ids, 1) }, chevIcon(true)),
          E("button", { class: "ghost icon-btn sm", type: "button", "data-f": "gdown", title: t("sk.layer.down"), "aria-label": t("sk.layer.down") + ": " + groupLabel(r.group), disabled: lowLayer <= -maxLayer(), onclick: () => shiftLayer(ids, -1) }, chevIcon(false))),
        ...r.parts.map((q) => layerRow(q, true)));
    }));
  }
  /** Tabs of the left panel: parts or layers. */
  function renderPanelTabs() {
    const box = $("#sk-ptabs");
    if (box.children.length !== 2) {
      box.replaceChildren(...["parts", "layers"].map((k) => E("button", { type: "button", role: "tab", "data-k": k, onclick: () => { S.ptab = k; renderPanelTabs(); } })));
    }
    ["parts", "layers"].forEach((k, i) => {
      const b = box.children[i], on = S.ptab === k;
      b.className = on ? "active" : ""; b.setAttribute("aria-selected", String(on)); b.tabIndex = on ? 0 : -1;
      b.textContent = t("sk.ptab." + k);
    });
    $("#sk-pal-list").hidden = S.ptab !== "parts";
    $("#sk-layers").hidden = S.ptab !== "layers";
    $("#sk-search").hidden = S.ptab !== "parts";
    renderLayers();
  }

  function bindSection(p) {
    const devs = devices();
    const b = p.bind || null;
    const sec = E("section", { class: "sk-sec" }, E("h3", {}, t("sk.bind")));
    if (!devs.length && !b) { sec.append(E("p", { class: "sub" }, t("sk.bind.nodev"))); return sec; }
    const curDev = b ? devs.find((d) => d.id === b.device) : null;
    const dsel = E("select", { "data-f": "dev", "aria-label": t("sk.bind.device"), onchange: (e) => {
      const v = e.target.value;
      mutate(() => {
        if (!v) { delete p.bind; return; }
        const first = (devices().find((d) => d.id === v) || { zones: [] }).zones[0]; // a binding needs a zone: take the first
        p.bind = { device: v, zone: first ? first.id : "", order: p.bind ? p.bind.order : 0, parallel: p.bind ? p.bind.parallel : false };
      });
      renderInspector(true);
    } },
    E("option", { value: "" }, t("sk.bind.none")),
    devs.map((d) => E("option", { value: d.id, disabled: !d.zones.length && d.id !== (b && b.device) }, d.name + (d.state && d.state !== "ok" ? " (" + stateText(d.state) + ")" : ""))),
    b && !curDev ? E("option", { value: b.device }, t("sk.bind.gone", { n: b.device })) : null);
    dsel.value = b ? b.device : "";
    sec.append(field(t("sk.bind.device"), dsel));
    if (b) {
      const zones = curDev ? curDev.zones : [];
      const zsel = E("select", { "data-f": "zone", "aria-label": t("sk.bind.zone"), onchange: (e) => { mutate(() => { p.bind.zone = e.target.value; }); renderInspector(true); } },
        !b.zone ? E("option", { value: "" }, t("sk.bind.zone.pick")) : null,
        zones.map((z) => E("option", { value: z.id }, zoneName(z) + (zoneText(z) ? " · " + zoneText(z) : ""))),
        b.zone && !zones.some((z) => z.id === b.zone) ? E("option", { value: b.zone }, t("sk.bind.gone", { n: b.zone })) : null);
      zsel.value = b.zone || "";
      sec.append(field(t("sk.bind.zone"), zsel),
        field(t("sk.bind.order"), numInput("order", () => p.bind.order, (v) => { p.bind.order = clamp(Math.round(v), 0, 1000); }, { min: 0, max: 1000, aria: t("sk.bind.order") })),
        switchRow("parallel", t("sk.bind.parallel"), t("sk.bind.parallel.sub"), () => p.bind.parallel, (on) => { p.bind.parallel = on; }),
        E("p", { class: "sub sk-range" }, ""));
      sec.append(testBox());
    }
    return sec;
  }

  // ---------- test LED ----------
  function testBox() {
    return E("div", { class: "sk-test" });
  }
  function renderTest() {
    const box = $(".sk-test"), p = selPart();
    if (!box || !p || !p.bind) return;
    const info = chains().info.get(p.id), n = ledCount(p);
    S.test = clamp(S.test, 0, Math.max(n - 1, 0));
    const { d, z } = findZone(p.bind.device, p.bind.zone);
    const saved = S.stored && savedHas(p);
    let why = "";
    if (!p.bind.zone || !n) why = t("sk.test.need");
    else if (!saved || isDirty()) why = t("sk.test.dirty");
    else if (!d || !z || d.state !== "ok") why = t("sk.test.inactive");
    const zi = info ? info.start + S.test : S.test;
    const key = [p.id, S.test, n, why, zi, LANG].join("|");
    if (box.dataset.key === key) return;
    box.dataset.key = key;
    box.replaceChildren(
      E("div", { class: "sk-test-head" }, E("b", {}, t("sk.test"))),
      E("div", { class: "sk-test-row" },
        E("button", { class: "ghost icon-btn sm", type: "button", title: t("sk.test.prev"), "aria-label": t("sk.test.prev"), disabled: S.test <= 0, onclick: () => stepTest(-1) }, "‹"),
        E("span", { class: "sk-test-pos" }, t("sk.test.pos", { n: S.test + 1, m: n })),
        E("button", { class: "ghost icon-btn sm", type: "button", title: t("sk.test.next"), "aria-label": t("sk.test.next"), disabled: S.test >= n - 1, onclick: () => stepTest(1) }, "›"),
        E("button", { class: "ghost sk-btn", type: "button", disabled: !!why, onclick: () => sendTest() }, t("sk.test.go")),
        E("button", { class: "ghost sk-btn", type: "button", onclick: endTest }, t("sk.test.end"))),
      E("small", {}, why || t("sk.test.hint", { n: S.test + 1, z: zi + 1 })),
      // the check runs on the lighting page, next to the preview: the editor closes (it is saved)
      E("div", { class: "sk-test-row" },
        E("button", { class: "ghost sk-btn", type: "button", disabled: !!why, title: t("sk.orient.sub"),
          onclick: async () => { const id = p.id; if (await closeEditor()) window.LightOrient && window.LightOrient(id); } }, t("sk.orient"))));
  }
  /** The saved copy has this part bound the same way (the server's chain must match what is shown). */
  function savedHas(p) {
    try {
      const sp = JSON.parse(S.saved).parts.find((q) => q.id === p.id);
      return !!sp && !!sp.bind && sp.bind.device === p.bind.device && sp.bind.zone === p.bind.zone;
    } catch { return false; }
  }
  function stepTest(dx) {
    S.test += dx; renderTest(); draw();
    if (S.testLive) sendTest();
  }
  async function sendTest() {
    const p = selPart(), info = p && chains().info.get(p.id);
    if (!p || !p.bind || !info) return;
    try { await api("POST", "/api/lighting/calibrate", { device: p.bind.device, zone: p.bind.zone, pos: info.start + S.test }); S.testLive = true; }
    catch (e) { toast(t("sk.test.err", { e: e.message }), true); }
  }
  async function endTest() {
    if (!S.testLive) return;
    S.testLive = false;
    try { await api("DELETE", "/api/lighting/calibrate"); } catch { /* the marker ends by itself */ }
  }

  // ---------- header, view tabs ----------
  /** Shows another face; the selection keeps only what is on it. */
  function setView(v) {
    if (S.view === v) return;
    endTest();
    S.view = v;
    const keep = selIds().filter((id) => facesOf(partById(id)).includes(v));
    setSelRaw(keep, S.sel);
    renderViews(); afterChange(true); fitIfNeeded();
  }
  function fitIfNeeded() { if (!S.vp[S.view]) fitView(); draw(); }
  /** Moves a part (and its group) to another face and follows it there. */
  function moveToFace(p, f) {
    const ids = groupMembers(p.id);
    mutate(() => { for (const id of ids) partById(id).face = f; setSelRaw(ids, p.id); });
    if (S.view !== f) setView(f);
    live(t("sk.live.face", { n: partTitle(p), f: t("sk.face." + f) }));
  }
  /** The face tabs are kept (focus stays on them); only labels, counts and states change. The count is the parts on that face. */
  function renderViews() {
    const box = $("#sk-views");
    if (box.children.length !== FACES.length) {
      box.setAttribute("aria-orientation", "horizontal");
      box.replaceChildren(...FACES.map((v) => E("button", { type: "button", role: "tab", "data-v": v, onclick: () => setView(v) })));
    }
    const count = new Map();
    for (const q of parts()) for (const f of facesOf(q)) count.set(f, (count.get(f) || 0) + 1);
    FACES.forEach((v, i) => {
      const b = box.children[i], n = count.get(v) || 0, on = S.view === v;
      b.className = on ? "active" : "";
      b.setAttribute("aria-selected", String(on));
      b.tabIndex = on ? 0 : -1;
      const title = t("sk.face." + v + ".long");
      b.title = title; b.setAttribute("aria-label", title + " (" + n + ")");
      b.replaceChildren(t("sk.face." + v), E("small", { class: "sk-count" + (n ? "" : " zero") }, String(n)));
    });
  }
  /** Arrow keys, Home and End move between the face tabs. */
  function onViewsKey(ev) {
    const i = FACES.indexOf(S.view), L = FACES.length;
    const next = ev.key === "ArrowRight" || ev.key === "ArrowDown" ? (i + 1) % L : ev.key === "ArrowLeft" || ev.key === "ArrowUp" ? (i + L - 1) % L : ev.key === "Home" ? 0 : ev.key === "End" ? L - 1 : -1;
    if (next < 0 || ev.altKey || ev.ctrlKey || ev.metaKey) return;
    ev.preventDefault(); ev.stopPropagation(); // the arrows must not nudge the selected part
    setView(FACES[next]);
    const b = $("#sk-views").children[next];
    if (b) b.focus();
  }
  function renderHead() {
    const dirty = isDirty();
    $("#sk-dirty").hidden = !dirty;
    // after a failed load the empty document must never replace the stored sketch
    $("#sk-save").disabled = !dirty || S.busy || S.loadFailed;
    $("#sk-discard").disabled = !dirty || S.busy;
    $("#sk-delete").disabled = !S.stored || S.busy || S.loadFailed;
    $("#sk-import").disabled = S.busy || S.loadFailed;
    $("#sk-export").disabled = S.loadFailed;
    $("#sk-undo").disabled = !S.undo.length;
    $("#sk-redo").disabled = !S.redo.length;
    $("#sk-hint").textContent = t("sk.hint"); $("#sk-hint").title = t("sk.hint.full");
    renderBanner();
  }
  function renderBanner() {
    const b = $("#sk-banner");
    b.hidden = !S.err;
    b.replaceChildren(...(S.err ? [E("span", {}, S.err),
      E("span", { class: "sk-banner-btns" },
        S.loadFailed ? E("button", { class: "ghost sk-btn", type: "button", onclick: () => load() }, t("sk.retry")) : null,
        E("button", { class: "ghost sm", type: "button", "aria-label": t("sk.err.close"), title: t("sk.err.close"), onclick: () => { S.err = ""; renderBanner(); } }, "✕"))] : []));
  }
  function afterChange(selectionOnly) {
    S.rev++;
    renderHead();
    if (!selectionOnly) renderViews();
    renderInspector(false);
    renderLayers();
    schedule();
  }

  // ---------- load, save, delete ----------
  function adopt(sketch) {
    S.doc = sketch ? cleanDoc(sketch) : newDoc();
    S.stored = !!sketch;
    S.saved = docJSON();
    S.undo = []; S.redo = []; S.lastKey = ""; S.notes = [];
    setSelRaw([]); S.test = 0; S.rev++;
  }
  /** Migration lives on the server (POST /api/lighting/sketch/upgrade); an older server does not have it. */
  async function upgradeRemote(raw) {
    try {
      const r = await api("POST", "/api/lighting/sketch/upgrade", raw);
      const sketch = r && r.sketch && typeof r.sketch === "object" ? r.sketch : r;
      const notes = r && Array.isArray(r.notes) ? [...r.notes] : [];
      if (r && typeof r.invalid === "string" && r.invalid) notes.push({ id: "", msg: r.invalid }); // the server would refuse this sketch on save
      return { sketch, notes: notes.filter((n) => n && typeof n.msg === "string").map((n) => ({ id: typeof n.id === "string" ? n.id : "", text: (n.id ? n.id + ": " : "") + n.msg })), missing: false };
    } catch (e) {
      if (e && (e.status === 404 || e.status === 405)) return { sketch: null, notes: [], missing: true };
      throw e;
    }
  }
  async function load() {
    S.busy = true; S.err = ""; S.loadFailed = false; renderHead();
    try {
      const [cat, sk, light] = await Promise.all([api("GET", "/api/lighting/catalog"), api("GET", "/api/lighting/sketch"), api("GET", "/api/lighting").catch(() => null)]);
      S.cat = cat; projCache = new WeakMap(); chainCache = { rev: -1, v: null };
      if (cat.limits) S.lim = { ...S.lim, ...cat.limits };
      if (light && typeof lv !== "undefined") { lv = light; }
      let stored = sk && sk.sketch, notes = [];
      if (stored && stored.version !== 2) { // an older stored sketch the server did not convert
        const r = await upgradeRemote(stored);
        if (r.missing || !r.sketch) throw new Error(t("sk.import.v1"));
        stored = r.sketch; notes = r.notes;
      }
      adopt(stored);
      S.notes = notes;
      S.view = "side";
      S.vp = {};
      const want = S.focus && partById(S.focus);
      if (want) { S.view = faceOf(want); setSelRaw([want.id], want.id); }
    } catch (e) {
      S.err = t("sk.err.load", { e: e.message });
      S.loadFailed = true;
      adopt(null); // an old document must not stay: it could be saved over the stored sketch
      S.vp = {};
    }
    S.focus = null;
    S.busy = false;
    renderAll();
  }
  async function save() {
    if (!S.open || S.busy || S.loadFailed) return;
    const ae = document.activeElement;
    if (ae && ae.tagName === "INPUT" && ae.closest("#sk-inspector")) /** @type {HTMLElement} */ (ae).blur(); // commits a value that is still being typed
    if (!isDirty()) return;
    const snap = cleanDoc(S.doc), snapJSON = JSON.stringify(snap);
    S.busy = true; S.err = ""; renderHead();
    try {
      const r = await api("PUT", "/api/lighting/sketch", snap);
      applyLighting(r.lighting);
      // What was sent is what is stored now. Edits made while the request ran stay in the document (and stay "unsaved").
      const stored = r.sketch ? cleanDoc(r.sketch) : snap, same = docJSON() === snapJSON;
      S.stored = true; S.saved = JSON.stringify(stored);
      if (same && S.saved !== snapJSON) { S.doc = stored; S.insSig = ""; S.rev++; }
      S.notes = [];
      toast(t("sk.saved"));
    } catch (e) { S.err = t("sk.err.save", { e: e.message }); }
    S.busy = false;
    renderAll();
    refreshEntry();
  }
  async function remove() {
    if (!S.stored || S.busy || S.loadFailed) return;
    if (!(await notice({ title: "sk.delete", paras: ["sk.delete.confirm", ...(isDirty() ? ["sk.delete.dirty"] : [])], ok: "sk.delete", cancel: "ack.cancel" }))) return;
    endTest();
    S.busy = true; S.err = ""; renderHead();
    try {
      const r = await api("DELETE", "/api/lighting/sketch");
      applyLighting(r.lighting);
      adopt(null);
      toast(t("sk.deleted"));
    } catch (e) { S.err = t("sk.err.delete", { e: e.message }); }
    S.busy = false;
    renderAll();
    refreshEntry();
  }
  async function discard() {
    if (S.busy || !isDirty()) return;
    if (!(await notice({ title: "sk.discard", paras: ["sk.discard.confirm"], ok: "sk.discard", cancel: "ack.cancel" }))) return;
    endTest();
    let saved = null;
    try { saved = S.stored ? JSON.parse(S.saved) : null; } catch { /* keep null */ }
    const view = S.view;
    adopt(saved); S.view = view;
    toast(t("sk.discarded"));
    renderAll();
  }
  /** The server's answer to a save carries the lighting view with the derived zone segments. */
  function applyLighting(next) {
    if (!next || typeof lv === "undefined") return;
    lv = next; lightSig = JSON.stringify(lv);
    renderLight(); refreshLightChip();
  }

  // ---------- import and export ----------
  function exportDoc() {
    const blob = new Blob([JSON.stringify(cleanDoc(S.doc), null, 2) + "\n"], { type: "application/json" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob); a.download = t("sk.export.name");
    document.body.append(a); a.click(); a.remove();
    setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  }
  /** Reads a sketch file (version 1 or 2). Version 1 is converted by the server; a version 2 file is checked here as well. */
  async function importFile(file) {
    if (!file) return;
    if (file.size > 1 << 20) { toast(t("sk.import.big"), true); return; }
    let raw;
    try { raw = JSON.parse(await file.text()); } catch (e) { toast(t("sk.import.err", { e: e.message }), true); return; }
    const bad = (m) => toast(t("sk.import.err", { e: m }), true);
    if (!raw || typeof raw !== "object" || (raw.version !== 1 && raw.version !== 2) || !raw.case || typeof raw.case !== "object" || !raw.case.dims || !Array.isArray(raw.parts)) { bad("version, case, parts"); return; }
    const d = raw.case.dims;
    if (![d.w, d.h, d.d].every((v) => Number.isFinite(v) && v > 0 && v <= S.lim.maxDimMm)) { bad("case.dims"); return; }
    if (raw.parts.length > S.lim.maxParts) { bad(t("sk.max.parts", { n: S.lim.maxParts })); return; }
    if ((parts().length || isDirty()) && !(await notice({ title: "sk.import", paras: ["sk.import.confirm"], ok: "sk.import.replace", cancel: "ack.cancel" }))) return;
    let sketch = raw, notes = [];
    try {
      const r = await upgradeRemote(raw);
      if (r.missing) {
        if (raw.version !== 2) { toast(t("sk.import.v1"), true); return; }
      } else { sketch = r.sketch; notes = r.notes; }
    } catch (e) { bad(e.message); return; }
    if (!sketch || sketch.version !== 2 || !Array.isArray(sketch.parts) || !sketch.case || !sketch.case.dims) { bad("version, case, parts"); return; }
    const { doc, skipped, fixed } = sanitizeSketch(sketch);
    endTest();
    mutate(() => { S.doc = doc; setSelRaw([]); S.notes = notes; });
    S.view = "side";
    S.vp = {}; fitView();
    renderAll();
    toast(t("sk.import.ok"));
    const msgs = [skipped > 0 ? t("sk.import.skipped", { n: skipped }) : "", fixed > 0 ? t("sk.import.fixed", { n: fixed }) : "", notes.length ? t("sk.import.notes", { n: notes.length }) : ""].filter(Boolean);
    if (msgs.length) setTimeout(() => toast(msgs.join(" "), true), 2400);
  }
  /** Text as the server accepts it: printable characters only, at most max characters. */
  const printable = (v, max) => Array.from(v.replace(/\p{C}/gu, "").replace(/\p{Z}/gu, (m) => (m === " " ? m : " "))).slice(0, max).join("");
  /** An imported version 2 sketch brought to the rules the server enforces: enums, ranges, ids, text. Counts what was dropped or adjusted. */
  function sanitizeSketch(raw) {
    let skipped = 0, fixed = 0;
    /** A number within [lo, hi]; anything else becomes dflt. */
    const lim = (v, lo, hi, dflt = 0) => {
      if (v == null) return dflt;
      if (typeof v !== "number" || !Number.isFinite(v)) { fixed++; return dflt; }
      const c = clamp(v, lo, hi);
      if (c !== v) fixed++;
      return c;
    };
    const rc = raw.case, tpl = rc.source === "library" ? caseOf(rc.id) : null;
    const ro = rc.orientation && typeof rc.orientation === "object" ? rc.orientation : {};
    let kase;
    if (tpl) {
      kase = { source: "library", id: rc.id, dims: { w: tpl.dims.w, h: tpl.dims.h, d: tpl.dims.d }, chambers: (tpl.chambers || []).map((q) => ({ x: q.x, w: q.w })) };
      const to = tpl.orientation || {};
      kase.orientation = { side_view_left: SIDE_LEFT.includes(to.side_view_left) ? to.side_view_left : "", glass_side: GLASS.includes(to.glass_side) ? to.glass_side : "" };
    } else {
      if (rc.source === "library") fixed++; // a case this library does not know becomes a custom box of that size
      const dims = { w: r2(num(rc.dims.w)), h: r2(num(rc.dims.h)), d: r2(num(rc.dims.d)) };
      const ch = (Array.isArray(rc.chambers) ? rc.chambers : []).filter((q) => q && Number.isFinite(q.x) && Number.isFinite(q.w) && q.x >= 0 && q.w > 0 && q.x + q.w <= dims.w + 1e-9);
      if ((Array.isArray(rc.chambers) ? rc.chambers.length : 0) !== Math.min(ch.length, S.lim.maxChambers)) fixed++;
      kase = { source: "custom", id: "custom-box", dims, chambers: ch.slice(0, S.lim.maxChambers).map((q) => ({ x: q.x, w: q.w })) };
      kase.orientation = { side_view_left: SIDE_LEFT.includes(ro.side_view_left) ? ro.side_view_left : "", glass_side: GLASS.includes(ro.glass_side) ? ro.glass_side : "" };
    }
    const dimOfC = { x: kase.dims.w, y: kase.dims.h, z: kase.dims.d };
    const m = S.lim.marginMm, ids = new Set(), out = [];
    for (const q of raw.parts) {
      if (q && typeof q === "object" && q.caseLight) { // rebuilt from the case template below, with what the user set
        if (typeof q.id === "string" && ID_RE.test(q.id) && !ids.has(q.id) && typeof q.caseLight === "string" && Array.isArray(q.path)) {
          ids.add(q.id);
          const o = { id: q.id, part: String(q.part), pos: { x: 0, y: 0, z: 0 }, facing: "side", spin: 0, mirror: q.mirror === true, flow: "", leds: 0, label: typeof q.label === "string" ? printable(q.label, S.lim.maxLabel) : "", caseLight: q.caseLight, path: [{ x: 0, y: 0, z: 0 }, { x: 0, y: 0, z: 0 }] };
          const b = q.bind;
          if (b && typeof b === "object" && typeof b.device === "string" && typeof b.zone === "string" && b.device && b.zone) o.bind = { device: printable(b.device, 128), zone: printable(b.zone, 128), order: Math.round(lim(b.order, 0, 1000)), parallel: b.parallel === true };
          if (q.layer != null) o.layer = Math.round(lim(q.layer, -maxLayer(), maxLayer()));
          if (q.hidden === true) o.hidden = true;
          out.push(o);
        } else skipped++;
        continue;
      }
      const part = q && typeof q === "object" ? partOf(q.part) : null;
      if (!part || typeof q.id !== "string" || !ID_RE.test(q.id) || ids.has(q.id) || !FACINGS.includes(q.facing)) { skipped++; continue; }
      ids.add(q.id);
      const qp = q.pos && typeof q.pos === "object" ? q.pos : {}, g = gen(part);
      const o = {
        id: q.id, part: q.part,
        pos: { x: lim(qp.x, -m, dimOfC.x + m), y: lim(qp.y, -m, dimOfC.y + m), z: lim(qp.z, -m, dimOfC.z + m) },
        facing: q.facing, spin: lim(q.spin, -maxSpin(), maxSpin()), mirror: q.mirror === true, flow: isFan(part) ? (q.flow === "exhaust" ? "exhaust" : "intake") : "",
        leds: g ? Math.round(q.leds ? lim(q.leds, g.min, g.max, g.default) : g.default) : 0,
        label: typeof q.label === "string" ? printable(q.label, S.lim.maxLabel) : "",
      };
      if (typeof q.label === "string" && o.label !== q.label) fixed++;
      if (g && g.lengthMax > 0 && q.len > 0) o.len = lim(q.len, g.lengthMin, g.lengthMax);
      const b = q.bind;
      if (b && typeof b === "object") {
        const dev = typeof b.device === "string" ? printable(b.device, 128) : "", zone = typeof b.zone === "string" ? printable(b.zone, 128) : "";
        if (dev && zone) o.bind = { device: dev, zone, order: Math.round(lim(b.order, 0, 1000)), parallel: b.parallel === true };
        else fixed++;
      }
      if (typeof q.on === "string" && q.on) o.on = q.on;
      if (typeof q.group === "string" && q.group) { if (GROUP_RE.test(q.group)) o.group = q.group; else fixed++; }
      if (q.layer != null) o.layer = Math.round(lim(q.layer, -maxLayer(), maxLayer()));
      if (q.hidden === true) o.hidden = true;
      out.push(o);
    }
    // a mounting parent must exist and must not be mounted itself (one level)
    const parentOf = new Map(out.map((p) => [p.id, p.on || ""]));
    for (const p of out) {
      if (!p.on) continue;
      if (p.on === p.id || !parentOf.has(p.on) || parentOf.get(p.on)) { delete p.on; fixed++; }
    }
    const draft = { version: 2, case: kase, canvasView: raw.canvasView, parts: out };
    syncCaseLights(draft, S.cat && S.cat.cases);
    if (kase.source === "custom") draft.parts = draft.parts.filter((p) => !p.caseLight);
    const doc = cleanDoc(draft);
    return { doc, skipped, fixed };
  }

  // ---------- open, close ----------
  function renderAll() {
    if (!S.open || !S.doc) return;
    renderHead(); renderViews(); renderPalette(); renderPanelTabs(); renderInspector(true); applyChrome();
    if (!S.vp[S.view]) fitView();
    draw();
  }
  function applyChrome() {
    $("#sk-snap").checked = S.snap; $("#sk-grid").checked = S.grid;
    $("#sk-search").value = S.search;
  }
  /** Opens the editor; focus: a part to select (and show its face). */
  function openEditor(focus) {
    if (S.open) return;
    S.focus = typeof focus === "string" ? focus : null;
    S.open = true;
    $("#tab-light").classList.add("sketching");
    $("#sketch-view").hidden = false;
    document.body.classList.add("sketching");
    window.scrollTo(0, 0);
    measure();
    load();
  }
  const askLeave = () => notice({ title: "sk.leave.title", paras: ["sk.leave"], ok: "sk.leave.ok", cancel: "ack.cancel" });
  async function closeEditor(force) {
    if (!force && isDirty() && !(await askLeave())) return false;
    if (S.testLive) endTest();
    S.open = false;
    $("#tab-light").classList.remove("sketching");
    $("#sketch-view").hidden = true;
    document.body.classList.remove("sketching");
    refreshEntry();
    return true;
  }
  /** The editor fills the window below its own top edge (banners above may come and go). */
  function fitHeight() {
    const v = $("#sketch-view");
    if (v.hidden || window.innerWidth <= 900) { v.style.height = ""; return; }
    const top = v.getBoundingClientRect().top + window.scrollY;
    v.style.height = Math.max(600, window.innerHeight - top - 24) + "px";
  }
  function measure() {
    fitHeight();
    const st = $("#sk-stage");
    const r = st.getBoundingClientRect();
    const was = S.size;
    S.size = { w: Math.floor(r.width), h: Math.floor(r.height) };
    if (S.size.w !== was.w || S.size.h !== was.h) {
      for (const [k, v] of Object.entries(S.vp)) if (v.fit) delete S.vp[k];
      if (S.doc && S.open) { if (!S.vp[S.view]) fitView(); schedule(); }
    }
  }
  /** Summary under the entry button in the lighting tab. */
  async function refreshEntry() {
    let sketch = null;
    if (S.open && S.doc) {
      if (S.stored) { try { sketch = JSON.parse(S.saved); } catch { /* none */ } }
    } else {
      try { sketch = (await api("GET", "/api/lighting/sketch")).sketch; } catch { return; }
    }
    const n = sketch ? sketch.parts.length : 0, b = sketch ? sketch.parts.filter((p) => p.bind).length : 0;
    $("#sketch-entry-info").textContent = !sketch ? t("sk.entry.none") : n === 1 ? t(b ? "sk.entry.some1" : "sk.entry.some0") : t("sk.entry.some", { n, b });
    $("#sketch-open").textContent = sketch ? t("sk.open.edit") : t("sk.open");
  }

  // ---------- read-only case net (lighting tab) ----------
  // Draws a stored sketch as the unfolded case without touching the editor state: the glass side in the middle, the
  // top above, the bottom below, rear and front beside it, the tray side next to the front when parts are on it.
  // The lighting tab lays the effect picture under it and colours the LED dots. Only the pure maths above is used.
  const pvPart = (cat, id) => (cat && typeof id === "string" && Object.hasOwn(cat.parts, id) ? cat.parts[id] : null);
  function pvTitle(cat, pl) {
    const part = pvPart(cat, pl.part);
    return pl.label || (isCL(pl) ? clName(cat, pl) : part ? partName(part) : pl.part);
  }
  /** Lays out the case net like internal/sketch NetLayout (used when the server did not send one). */
  function netLayout(doc, faces) {
    const d = doc.case.dims, count = {};
    for (const f of faces) count[f] = (count[f] || 0) + 1;
    const thick = (f, v) => (!count[f] && f !== "side" ? [round6(v * 0.2), true] : [v, false]);
    const [wr, cr] = thick("rear", d.w), [wf, cf] = thick("front", d.w), [ht, ct] = thick("top", d.w), [hb, cb] = thick("bottom", d.w);
    const out = [], add = (face, x, y, w, h, collapsed) => out.push({ face, x, y, w, h, parts: count[face] || 0, collapsed });
    add("side", wr, ht, d.d, d.h, false); add("top", wr, 0, d.d, ht, ct); add("bottom", wr, ht + d.h, d.d, hb, cb);
    add("rear", 0, ht, wr, d.h, cr); add("front", wr + d.d, ht, wf, d.h, cf);
    let W = wr + d.d + wf;
    if (count.tray) { add("tray", W, ht, d.d, d.h, false); W += d.d; }
    const H = ht + d.h + hb, gr = !!(doc.case.orientation && doc.case.orientation.glass_side === "right");
    if (gr) for (const f of out) { f.x = round6(W - f.x - f.w); f.mirrored = true; }
    return { w: round6(W), h: round6(H), faces: out };
  }
  /** The face of every placement: its own, else the default rule. */
  function pvFaces(doc, cat) {
    const gr = !!(doc.case.orientation && doc.case.orientation.glass_side === "right");
    return doc.parts.map((pl) => {
      if (isCL(pl)) { const part = pvPart(cat, pl.part); return part ? pathInfo(part, pl, doc.case.dims).faces[0] : "side"; }
      if (FACINGS.includes(pl.face)) return pl.face;
      const part = pvPart(cat, pl.part), p3 = part && FACINGS.includes(pl.facing) ? project3d(part, pl, undefined, gr) : null;
      return part ? defaultFace(part, pl, p3, doc.case.dims) : "side";
    });
  }
  /**
   * Draws doc as the case net into an SVG of w x h pixels. net: the server's layout (optional). Returns the svg, the
   * <image> for the effect picture, one entry per LED dot (net mm and chain index), info per part and the net.
   */
  function pvNet(doc, cat, net, o) {
    const d = doc.case.dims, gr = !!(doc.case.orientation && doc.case.orientation.glass_side === "right");
    const c = { dims: d, glassRight: gr }, faces = pvFaces(doc, cat);
    if (!net || !Array.isArray(net.faces) || !(net.w > 0) || !(net.h > 0)) {
      const used = [...faces]; // a case light counts on every face it runs over
      doc.parts.forEach((pl) => { const part = isCL(pl) && pvPart(cat, pl.part); if (part) used.push(...pathInfo(part, pl, d).faces.slice(1)); });
      net = netLayout(doc, used);
    }
    const byFace = new Map(net.faces.map((f) => [f.face, f]));
    const W = Math.max(40, Math.floor(o.w)), H = Math.max(40, Math.floor(o.h)), pad = o.pad ?? 26;
    const sc = Math.max(1e-3, Math.min((W - 2 * pad) / net.w, (H - 2 * pad) / net.h));
    const svg = sv("svg", { class: "pv-svg pv-net", viewBox: `${net.w / 2 - W / 2 / sc} ${net.h / 2 - H / 2 / sc} ${W / sc} ${H / sc}`, width: W, height: H, role: "img", "aria-label": o.label || "" });
    const uid = "pv" + Math.random().toString(36).slice(2, 8);
    // the effect canvas (320 x 200) holds the net letterboxed (internal/sketch Letterbox)
    const k = Math.min(320 / net.w, 200 / net.h), iw = 320 / k, ih = 200 / k, area = { x: (net.w - iw) / 2, y: (net.h - ih) / 2, w: iw, h: ih };
    const clip = sv("clipPath", { id: uid + "-c" }, net.faces.map((f) => sv("rect", { x: f.x, y: f.y, width: f.w, height: f.h })));
    svg.append(sv("defs", {}, clip, sv("filter", { id: uid + "-g", x: "-60%", y: "-60%", width: "220%", height: "220%", "color-interpolation-filters": "sRGB" },
      sv("feGaussianBlur", { stdDeviation: 2.2 / sc, result: "b" }), sv("feMerge", {}, sv("feMergeNode", { in: "b" }), sv("feMergeNode", { in: "SourceGraphic" })))));
    // faces: base, effect picture, then outlines and names on top
    for (const f of net.faces) svg.append(sv("rect", { class: "pv-face-bg" + (f.collapsed ? " collapsed" : ""), x: f.x, y: f.y, width: f.w, height: f.h }));
    const img = sv("image", { class: "pv-effect", x: area.x, y: area.y, width: area.w, height: area.h, preserveAspectRatio: "none", "clip-path": `url(#${uid}-c)` });
    svg.append(img);
    const toNet = (f) => { const fs = faceSize(d, f.face); return `translate(${f.x} ${f.y}) scale(${f.w / fs.w} ${f.h / fs.h})`; };
    const tpl = doc.case.source === "library" && cat && Object.hasOwn(cat.cases, doc.case.id) ? cat.cases[doc.case.id] : null;
    const deco = sv("g", { class: "pv-deco" });
    for (const f of net.faces) {
      if (f.collapsed) continue;
      const g = sv("g", { transform: toNet(f) });
      const cv = tpl && tpl.views && (f.face === "front" || f.face === "side") && tpl.views[f.face];
      if (cv) {
        const tg = sv("g", { transform: f.face === "side" && gr ? `matrix(-1 0 0 1 ${d.d} 0)` : null });
        for (const sh of cv.outline || []) { const n = shapeEl(sh, " pv-case-sh"); if (n) tg.append(n); }
        g.append(tg);
      }
      if (f.face !== "side" && f.face !== "tray") {
        (doc.case.chambers || []).forEach((q, i) => {
          if (i === 0) return;
          const a = toFaceC(c, f.face, { x: q.x, y: 0, z: 0 }), across = f.face === "top" || f.face === "bottom", fs = faceSize(d, f.face);
          g.append(across ? sv("line", { class: "pv-divider", x1: 0, y1: a.v, x2: fs.w, y2: a.v }) : sv("line", { class: "pv-divider", x1: a.u, y1: 0, x2: a.u, y2: fs.h }));
        });
      }
      deco.append(g);
    }
    svg.append(deco);
    const fsz = 12 / sc;
    for (const f of net.faces) {
      svg.append(sv("rect", { class: "pv-face" + (f.collapsed ? " collapsed" : "") + (f.face === "side" ? " glass" : ""), x: f.x, y: f.y, width: f.w, height: f.h, "data-face": f.face }));
      const name = t("sk.face." + f.face), need = name.length * 9.4 + 12; // px of the capitals at 12 px with their spacing
      const cls = "pv-face-label" + (f.collapsed ? " dim" : "");
      if (f.w * sc >= need || f.h * sc < need) svg.append(sv("text", { class: cls, x: f.x + 6 / sc, y: f.y + 6 / sc + fsz * 0.8, "font-size": fsz }, name));
      else { const tx = f.x + 6 / sc + fsz * 0.8, ty = f.y + f.h - 6 / sc; svg.append(sv("text", { class: cls, x: tx, y: ty, "font-size": fsz, transform: `rotate(-90 ${tx} ${ty})` }, name)); }
    }

    // parts on their faces
    const items = doc.parts.map((pl, idx) => {
      const part = pvPart(cat, pl.part), cl = part && isCL(pl) ? pathInfo(part, pl, d) : null;
      const lay = !cl && part && layout3(part, pl, gr), p3 = lay && project3d(part, pl, lay, gr);
      return { pl, idx, part, lay, p3, cl, face: faces[idx] };
    });
    // chains as the server builds them: sorted by order, a part in parallel starts where the one before started
    const lists = new Map(), start = new Map();
    for (const it of items) if (it.pl.bind && it.pl.bind.device) { const kk = it.pl.bind.device + "\u0000" + it.pl.bind.zone; (lists.get(kk) || lists.set(kk, []).get(kk)).push(it); }
    for (const list of lists.values()) {
      list.sort((a, b) => a.pl.bind.order - b.pl.bind.order || a.idx - b.idx);
      let next = 0, prev = 0, seen = 0;
      const ring = ringZone(list[0].pl.bind.device, list[0].pl.bind.zone);
      for (const it of list) {
        let n = it.p3 ? it.p3.leds.length : it.cl ? it.cl.leds.length : 0;
        if (ring && it.p3) { it.ring = ringsOf(it.part, it.pl, n); n = it.ring.rings; }
        const s = it.pl.bind.parallel && seen > 0 ? prev : next;
        start.set(it.pl.id, s); prev = s; next = Math.max(next, s + n); seen++;
      }
    }
    const shownItems = items.filter((it) => (it.p3 || it.cl) && !it.pl.hidden && byFace.has(it.face));
    /** A case point on a face of the net, in net millimetres (null: the face is not in the net). */
    const netPt = (face, p) => { const f = byFace.get(face); if (!f) return null; const fs = faceSize(d, face), q = toFaceC(c, face, p); return { x: f.x + (q.u * f.w) / fs.w, y: f.y + (q.v * f.h) / fs.h }; };
    for (const it of shownItems) {
      if (it.cl) { // one strip over all its faces: LEDs and runs each on their own face, continuous across the edges
        it.pts = it.cl.leds.map((l) => netPt(l.face, l.p));
        it.lines = it.cl.segs.map((s) => [netPt(s.face, s.a), netPt(s.face, s.b)]).filter((q) => q[0] && q[1]);
        const all = [...it.lines.flat(), ...it.pts.filter(Boolean)], h = it.cl.width / 2;
        const x0 = Math.min(...all.map((q) => q.x)) - h, y0 = Math.min(...all.map((q) => q.y)) - h;
        it.box = { x: x0, y: y0, w: Math.max(...all.map((q) => q.x)) + h - x0, h: Math.max(...all.map((q) => q.y)) + h - y0 };
        it.v = { kind: "path", leds: it.pts.filter(Boolean) };
        it.centre = -Infinity; // on the case wall: in front of everything inside
        continue;
      }
      const { lay, p3 } = it, f = byFace.get(it.face), fs = faceSize(d, it.face), sx = f.w / fs.w, sy = f.h / fs.h;
      const v = viewOf(p3, c, it.face, toFaceC);
      const map = (a, b) => { const [du, dv] = lay.tf(a, b); const q = toFaceC(c, it.face, lay.at(du, dv, lay.d / 2)); return [q.u, q.v]; };
      const m0 = map(0, 0), mx = map(lay.w, 0), my = map(0, lay.h);
      it.v = v; it.f = f; it.sx = sx; it.sy = sy;
      it.xf = `matrix(${(mx[0] - m0[0]) / lay.w} ${(mx[1] - m0[1]) / lay.w} ${(my[0] - m0[0]) / lay.h} ${(my[1] - m0[1]) / lay.h} ${m0[0]} ${m0[1]})`;
      it.centre = toFaceC(c, it.face, { x: (p3.min.x + p3.max.x) / 2, y: (p3.min.y + p3.max.y) / 2, z: (p3.min.z + p3.max.z) / 2 }).depth;
      it.box = { x: f.x + v.box.x * sx, y: f.y + v.box.y * sy, w: v.box.w * sx, h: v.box.h * sy };
    }
    shownItems.sort((a, b) => (a.pl.layer || 0) - (b.pl.layer || 0) || b.centre - a.centre || b.box.w * b.box.h - a.box.w * a.box.h);
    const ordered = [], placed = new Set(), byId = new Map(shownItems.map((it) => [it.pl.id, it]));
    const put = (it) => {
      if (placed.has(it.pl.id)) return;
      placed.add(it.pl.id);
      const parent = it.pl.on && byId.get(it.pl.on);
      if (parent && parent !== it) put(parent);
      ordered.push(it);
    };
    shownItems.forEach(put);

    const leds = [], info = new Map(), layer = sv("g", { class: "pv-parts" });
    for (const it of ordered) {
      const { pl, v, f } = it, b = it.box, bound = pl.bind && pl.bind.device ? pl.bind : null, title = pvTitle(cat, pl);
      // n: LEDs of the part; r: its rings (what a ring zone takes); take: what it takes on its zone now
      const nLeds = it.cl ? it.cl.leds.length : it.p3 ? it.p3.leds.length : v.leds.length;
      const rings = it.cl ? nLeds : ringsOf(it.part, pl, nLeds).rings;
      info.set(pl.id, { title, device: bound ? bound.device : "", zone: bound ? bound.zone : "", n: nLeds, r: rings, take: it.ring ? it.ring.rings : nLeds, face: it.face, box: b });
      const g = sv("g", { class: "pv-part" + (bound ? "" : " unbound") + (v.kind === "away" ? " away" : "") + (v.kind === "edge" ? " edge" : "") + (it.cl ? " caselight" : ""), "data-id": pl.id, tabindex: o.interactive ? 0 : null, role: o.interactive ? "button" : null, "aria-label": o.interactive ? title : null });
      if (it.cl) {
        for (const [a, bb] of it.lines) g.append(sv("line", { class: "pv-cl-strip", x1: a.x, y1: a.y, x2: bb.x, y2: bb.y, "stroke-width": it.cl.width }));
        g.append(sv("rect", { class: "pv-hit", x: b.x, y: b.y, width: b.w, height: b.h }));
        if (!bound) g.append(sv("rect", { class: "pv-unbound", x: b.x - 2 / sc, y: b.y - 2 / sc, width: b.w + 4 / sc, height: b.h + 4 / sc, rx: 3 / sc }));
        const real = it.pts.filter(Boolean);
        let minD = Infinity;
        for (let i = 1; i < real.length; i++) minD = Math.min(minD, Math.hypot(real[i].x - real[i - 1].x, real[i].y - real[i - 1].y));
        const rad = clamp(Number.isFinite(minD) ? minD * 0.36 : 2.4, 1, 3.4), dots = sv("g", { class: "pv-leds", filter: `url(#${uid}-g)` });
        const s0 = bound ? start.get(pl.id) || 0 : 0;
        it.pts.forEach((q, i) => {
          if (!q) return;
          const node = sv("circle", { class: "pv-led", cx: q.x, cy: q.y, r: rad });
          dots.append(node);
          leds.push({ node, id: pl.id, dev: bound ? bound.device : "", zone: bound ? bound.zone : "", i: s0 + i, u: (q.x - area.x) / area.w, v: (q.y - area.y) / area.h });
        });
        g.append(dots);
        layer.append(g);
        continue;
      }
      const inFace = sv("g", { transform: `translate(${f.x} ${f.y}) scale(${it.sx} ${it.sy})` });
      const vb = v.box;
      if (v.kind === "edge") inFace.append(sv("rect", { class: "sk-sh sk-r-body pv-edge", x: vb.x, y: vb.y, width: vb.w, height: vb.h, rx: Math.min(2, Math.min(vb.w, vb.h) / 3) }));
      else {
        const inner = sv("g", { transform: it.xf });
        for (const sh of it.lay.res.front) { const n = shapeEl(sh); if (n) inner.append(n); }
        inFace.append(inner);
      }
      g.append(inFace);
      g.append(sv("rect", { class: "pv-hit", x: b.x, y: b.y, width: b.w, height: b.h }));
      if (!bound) g.append(sv("rect", { class: "pv-unbound", x: b.x - 2 / sc, y: b.y - 2 / sc, width: b.w + 4 / sc, height: b.h + 4 / sc, rx: 3 / sc }));
      if (v.leds.length) {
        const pts = v.leds.map((q) => ({ x: f.x + q.x * it.sx, y: f.y + q.y * it.sy }));
        let minD = Infinity;
        for (let i = 1; i < pts.length; i++) minD = Math.min(minD, Math.hypot(pts[i].x - pts[i - 1].x, pts[i].y - pts[i - 1].y));
        const rad = clamp(Number.isFinite(minD) ? minD * 0.36 : 2.4, 1, 3.4);
        const dots = sv("g", { class: "pv-leds", filter: v.kind === "away" ? null : `url(#${uid}-g)` });
        const s0 = bound ? start.get(pl.id) || 0 : 0;
        pts.forEach((q, i) => {
          const node = sv("circle", { class: "pv-led", cx: q.x, cy: q.y, r: rad });
          dots.append(node);
          // u, v: the LED's place in the effect picture (0..1); on a ring zone every LED shows its ring's colour
          leds.push({ node, id: pl.id, dev: bound ? bound.device : "", zone: bound ? bound.zone : "", i: s0 + (it.ring ? it.ring.groups[i] || 0 : i), u: (q.x - area.x) / area.w, v: (q.y - area.y) / area.h });
        });
        g.append(dots);
      }
      layer.append(g);
    }
    svg.append(layer);
    // An LED driven twice (parts in parallel) takes its place in the effect picture from the first
    // part of the chain that has it, as the engine does: su, sv are where it is sampled. Ring zones
    // (one colour per ring) keep their own places.
    const byPart = new Map();
    for (const l of leds) if (l.dev) (byPart.get(l.id) || byPart.set(l.id, []).get(l.id)).push(l);
    for (const list of lists.values()) {
      if (list.some((it) => it.ring)) continue;
      const first = new Map();
      for (const it of list) for (const l of byPart.get(it.pl.id) || []) {
        const q = first.get(l.i);
        if (q) { l.su = q.u; l.sv = q.v; } else first.set(l.i, l);
      }
    }
    return { svg, img, leds, info, parts: ordered.length, net, area, scale: sc };
  }
  window.SketchPreview = { net: pvNet, title: pvTitle, name: (cat, pl) => { const part = pvPart(cat, pl.part); return part ? partName(part) : pl.part; }, faces: FACES, emptyDoc: newDoc, ringZone };
  window.SketchEditor = { open: (id) => openEditor(id) };

  // ---------- wiring ----------
  $("#sketch-open").addEventListener("click", () => openEditor());
  $("#sk-back").addEventListener("click", () => closeEditor());
  $("#sk-save").addEventListener("click", save);
  $("#sk-discard").addEventListener("click", discard);
  $("#sk-delete").addEventListener("click", remove);
  $("#sk-views").addEventListener("keydown", onViewsKey);
  $("#sk-undo").addEventListener("click", undo);
  $("#sk-redo").addEventListener("click", redo);
  $("#sk-export").addEventListener("click", exportDoc);
  $("#sk-import").addEventListener("click", () => $("#sk-file").click());
  $("#sk-file").addEventListener("change", (e) => { const f = e.target.files && e.target.files[0]; e.target.value = ""; importFile(f); });
  $("#sk-zoom-in").addEventListener("click", () => zoomCentre(1.25));
  $("#sk-zoom-out").addEventListener("click", () => zoomCentre(0.8));
  $("#sk-fit").addEventListener("click", () => { fitView(); draw(); });
  $("#sk-snap").addEventListener("change", (e) => { S.snap = e.target.checked; });
  $("#sk-grid").addEventListener("change", (e) => { S.grid = e.target.checked; draw(); });
  $("#sk-search").addEventListener("input", (e) => { S.search = e.target.value; renderPalette(); });
  const svgNode = svgEl();
  svgNode.addEventListener("pointerdown", onPointerDown);
  svgNode.addEventListener("pointermove", onPointerMove);
  svgNode.addEventListener("pointerup", onPointerUp);
  svgNode.addEventListener("pointercancel", onPointerUp);
  svgNode.addEventListener("pointerleave", () => { S.cursor = null; $("#sk-tip").hidden = true; if (S.open) { updateStatus(); drawRulers(); } });
  svgNode.addEventListener("wheel", onWheel, { passive: false });
  svgNode.addEventListener("dblclick", (e) => { if (!(e.target.closest && e.target.closest(".sk-part"))) { fitView(); draw(); } });
  const stageNode = $("#sk-stage");
  stageNode.addEventListener("dragover", (e) => { if (e.dataTransfer && [...e.dataTransfer.types].includes("text/x-casehud-part")) { e.preventDefault(); e.dataTransfer.dropEffect = "copy"; stageNode.classList.add("drop"); } });
  stageNode.addEventListener("dragleave", () => stageNode.classList.remove("drop"));
  stageNode.addEventListener("drop", (e) => { stageNode.classList.remove("drop"); onDrop(e); });
  document.addEventListener("keydown", onKey);
  document.addEventListener("keyup", onKeyUp);
  window.addEventListener("blur", () => { S.space = false; svgEl().classList.remove("pan-ready"); });
  new ResizeObserver(() => { if (S.open) measure(); }).observe(stageNode);
  new ResizeObserver(() => { if (S.open) measure(); }).observe(document.body);
  window.addEventListener("beforeunload", (e) => { if (S.open && isDirty()) { e.preventDefault(); e.returnValue = ""; } });
  // leaving the lighting tab with unsaved changes
  let leaveOK = false;
  $("#tabs").addEventListener("click", (e) => {
    const b = e.target.closest && e.target.closest("[data-tab]");
    if (S.open && b && b.dataset.tab !== "light") {
      if (isDirty() && !leaveOK) {
        e.stopPropagation(); e.preventDefault();
        askLeave().then((ok) => { if (!ok) return; leaveOK = true; try { b.click(); } finally { leaveOK = false; } });
      } else endTest();
    }
  }, true);
  for (const b of $$("[data-lang]")) b.addEventListener("click", () => { refreshEntry(); if (S.open && S.doc) { $("#sk-empty").dataset.key = ""; renderAll(); } });
  refreshEntry();
})();
