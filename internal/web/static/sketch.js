"use strict";

// ---------- Case sketch editor ("Gehäuse-Skizze") ----------
// A full-width view in the lighting tab: the case is drawn in millimetres
// (front, side or top view), parts from the catalog are placed on it and
// bound to LED zones. The server derives LED positions and zone segments from
// the saved sketch (PUT /api/lighting/sketch). Texts live here, like lighting.js.
Object.assign(STRINGS.de, {
  "sk.entry.title": "Gehäuse-Skizze",
  "sk.entry.sub": "Zeichne dein Gehäuse mit Lüftern, RAM und LED-Streifen und weise die Teile den Zonen zu. Der Effekt folgt dann der echten Lage der LEDs.",
  "sk.entry.none": "Noch keine Skizze gespeichert.",
  "sk.entry.some": "Gespeichert: {n} Teile, davon {b} an Zonen gebunden.",
  "sk.entry.some1": "Gespeichert: 1 Teil, an eine Zone gebunden.",
  "sk.entry.some0": "Gespeichert: 1 Teil, noch nicht gebunden.",
  "sk.open": "Skizze öffnen", "sk.open.edit": "Skizze bearbeiten",
  "sk.title": "Gehäuse-Skizze", "sk.sub": "Teile auf das Gehäuse ziehen, drehen und einer LED-Zone zuweisen. Gespeichert wird erst mit „Speichern“.",
  "sk.back": "Zurück", "sk.dirty": "Nicht gespeichert",
  "sk.save": "Speichern", "sk.discard": "Verwerfen", "sk.delete": "Skizze löschen",
  "sk.import": "Importieren", "sk.export": "Exportieren",
  "sk.undo": "Rückgängig (Strg+Z)", "sk.redo": "Wiederholen (Strg+Y)",
  "sk.saved": "Skizze gespeichert.", "sk.deleted": "Skizze gelöscht.", "sk.discarded": "Änderungen verworfen.",
  "sk.err.load": "Die Skizze konnte nicht geladen werden: {e}",
  "sk.err.save": "Speichern fehlgeschlagen: {e}", "sk.err.delete": "Löschen fehlgeschlagen: {e}",
  "sk.err.close": "Meldung schließen",
  "sk.leave": "Die Skizze hat ungespeicherte Änderungen. Trotzdem verlassen?",
  "sk.discard.confirm": "Alle ungespeicherten Änderungen verwerfen?",
  "sk.delete.confirm": "Die gespeicherte Skizze löschen? Gebundene Zonen fallen auf ihr Standard-Layout zurück.",
  "sk.delete.dirty": "Die ungespeicherten Änderungen gehen dabei ebenfalls verloren.",
  "sk.retry": "Erneut laden",
  "sk.zone.upto": "bis {n} LEDs", "sk.zone.any": "LED-Zahl frei",
  "sk.f.view": "Liegt in Ansicht",
  "sk.dims.moved": "{n} Teil(e) wurden zurück in den erlaubten Bereich geschoben.",
  "sk.palette": "Bauteile", "sk.search": "Bauteile suchen …", "sk.stage": "Zeichenfläche der Gehäuse-Skizze",
  "sk.inspector": "Eigenschaften", "sk.views": "Ansicht",
  "sk.pal.none": "Kein Bauteil gefunden.", "sk.pal.add": "Bauteil „{n}“ hinzufügen",
  "sk.pal.count": "{n} LEDs", "sk.pal.var": "{a}–{b} LEDs",
  "sk.pal.generic": "Allgemein", "sk.approx": "ungefähr", "sk.approx.tip": "Die Maße und LED-Positionen dieses Bauteils sind nur geschätzt.",
  "sk.cat.fan": "Lüfter", "sk.cat.fan-lcd": "Lüfter mit Display", "sk.cat.ram": "Arbeitsspeicher", "sk.cat.strip": "LED-Streifen",
  "sk.cat.gpu-block": "Grafikkarte", "sk.cat.pump": "Pumpe / Kühler", "sk.cat.distro-plate": "Verteilerplatte",
  "sk.cat.case-accessory": "Gehäuse-Zubehör", "sk.cat.cpu-block": "CPU-Kühler", "sk.cat.display": "Display", "sk.cat.mainboard": "Mainboard",
  "sk.cat.case-light": "Gehäusebeleuchtung", "sk.cat.cooler": "Kühler", "sk.cat.controller": "Controller", "sk.cat.other": "Sonstiges",
  "sk.part.generic-fan-120": "Lüfter 120 mm, LED-Ring", "sk.part.generic-fan-140": "Lüfter 140 mm, LED-Ring",
  "sk.part.generic-ram-dimm": "RAM-Riegel, LED-Leiste", "sk.part.generic-strip": "LED-Streifen",
  "sk.part.generic-gpu-box": "Grafikkarte, LED-Rahmen", "sk.part.generic-pump-cyl": "Pumpe, runde LED-Fläche",
  "sk.part.generic-distro-rect": "Verteilerplatte, LED-Rahmen",
  "sk.view.front": "Front", "sk.view.side": "Seite", "sk.view.top": "Oben",
  "sk.view.front.long": "Vorderansicht", "sk.view.side.long": "Seitenansicht", "sk.view.top.long": "Draufsicht",
  "sk.face.front": "Front", "sk.face.side": "Seite", "sk.face.top": "Oben",
  "sk.front": "vorn", "sk.other.views": "{n} in anderen Ansichten",
  "sk.zoomIn": "Vergrößern", "sk.zoomOut": "Verkleinern", "sk.fit": "Einpassen",
  "sk.snap": "Raster fangen", "sk.grid": "Raster",
  "sk.hint": "Rad: zoomen · Leertaste + ziehen: verschieben · Pfeile: 1 mm (Umschalt: 10 mm) · R: drehen · Entf: löschen · Esc: Ziehen abbrechen · Klick auf Überlagerung: nächstes Teil",
  "sk.readout.size": "{w} × {h} mm", "sk.readout.pos": "x {x} · y {y} mm",
  "sk.empty.title": "Noch keine Teile",
  "sk.empty.text": "Ziehe links ein Bauteil auf das Gehäuse oder klicke darauf. Stelle rechts Gehäuse und Maße ein, binde die Teile dann an eine Zone.",
  "sk.empty.view": "In der Ansicht „{v}“ liegt noch kein Teil.",
  "sk.led": "LED {n}", "sk.led.zone": "LED {n} · Zonen-LED {z}", "sk.led.first": "Erste LED",
  "sk.case": "Gehäuse", "sk.case.type": "Gehäuse wählen", "sk.case.custom": "Eigene Box",
  "sk.case.library": "Aus der Bibliothek", "sk.case.dims": "Außenmaße (mm)", "sk.case.w": "Breite", "sk.case.h": "Höhe", "sk.case.d": "Tiefe",
  "sk.case.fixed": "Die Maße kommen aus der Bibliothek.",
  "sk.case.divider": "Kammertrennung", "sk.case.divider.x": "Trennung bei (mm von links)",
  "sk.case.divider.hint": "Teilt Front- und Draufsicht in zwei Kammern.",
  "sk.case.chamber": "Kammer {n}",
  "sk.case.canvas": "Ansicht für die LED-Fläche", "sk.case.canvas.hint": "Aus dieser Richtung entsteht die LED-Fläche (die kleine Karte im Reiter „Beleuchtung“). Alle Teile werden so darauf abgebildet, wie man sie aus dieser Ansicht sieht.",
  "sk.view.canvas": "Ansicht für die LED-Fläche",
  "sk.parts": "Teile in der Skizze", "sk.parts.none": "Noch keine Teile.",
  "sk.parts.item": "{n} auswählen",
  "sk.bound": "gebunden", "sk.unbound": "nicht gebunden",
  "sk.warn.title": "Hinweise", "sk.warn.none": "Keine Hinweise.",
  "sk.w.segments": "{z}: {n} Teile in einer Zone, erlaubt sind {max}.",
  "sk.w.total": "Insgesamt {n} LEDs, erlaubt sind {max}.",
  "sk.w.zoneLeds": "Zone {z}: Die Kette braucht {n} LEDs, die Zone fasst höchstens {m}.",
  "sk.w.outside": "{p} liegt ganz oder teilweise außerhalb des Gehäuses.",
  "sk.w.unbound": "{p} ist an keine Zone gebunden und leuchtet nicht.",
  "sk.w.noZone": "{p}: Gerät oder Zone „{z}“ ist gerade nicht vorhanden.",
  "sk.w.unknown": "{p}: Das Bauteil „{id}“ fehlt im Katalog.",
  "sk.w.leds": "{p}: LED-Zahl außerhalb von {a}–{b}.",
  "sk.part": "Teil", "sk.part.default": "Teil {n}",
  "sk.part.dup": "Duplizieren", "sk.part.del": "Löschen", "sk.part.size": "Größe: {w} × {h} mm, {n} LEDs",
  "sk.f.label": "Name", "sk.f.label.ph": "z. B. Front oben",
  "sk.f.pos": "Position (mm)", "sk.f.x": "x", "sk.f.y": "y",
  "sk.f.rot": "Drehung", "sk.f.rot.free": "Grad", "sk.f.rot.btn": "90° drehen",
  "sk.f.mirror": "Spiegeln", "sk.f.mirror.sub": "Dreht die Richtung der LED-Reihenfolge um.",
  "sk.f.face": "LED-Seite zeigt nach", "sk.f.face.edge": "Das Teil wird von der Kante gesehen; die LEDs liegen auf einer Linie.",
  "sk.f.flow": "Luftstrom", "sk.f.flow.intake": "Ansaugen", "sk.f.flow.exhaust": "Ausblasen",
  "sk.f.flow.hint": "Ausblasen dreht die LED-Reihenfolge des Rings.",
  "sk.f.leds": "LED-Anzahl", "sk.f.leds.range": "{a} bis {b}", "sk.f.leds.fixed": "{n} LEDs (fest)",
  "sk.f.len": "Länge (mm)", "sk.f.len.range": "{a} bis {b}",
  "sk.bind": "Anschluss", "sk.bind.device": "Gerät", "sk.bind.zone": "Zone", "sk.bind.none": "– nicht gebunden –",
  "sk.bind.gone": "{n} (nicht vorhanden)", "sk.bind.zone.pick": "– Zone wählen –",
  "sk.bind.nodev": "Es sind keine Geräte da. Schalte die Beleuchtung ein, dann kannst du Teile binden. Die Skizze lässt sich auch ohne Geräte zeichnen.",
  "sk.bind.order": "Reihenfolge in der Kette", "sk.bind.parallel": "Parallel zum Vorgänger",
  "sk.bind.parallel.sub": "Gleiche LEDs wie das Teil davor, zum Beispiel über einen Splitter.",
  "sk.bind.range": "Belegt in der Zone die LEDs {a} bis {b}.", "sk.bind.range1": "Belegt in der Zone die LED {a}.",
  "sk.test": "LED testen", "sk.test.go": "Testlicht", "sk.test.end": "Testlicht aus",
  "sk.test.prev": "Vorherige LED", "sk.test.next": "Nächste LED", "sk.test.pos": "LED {n} von {m}",
  "sk.test.hint": "Lässt die LED {n} dieses Teils kurz leuchten (Zonen-LED {z}). Es wird nichts dauerhaft geändert.",
  "sk.test.need": "Binde das Teil erst an eine Zone und speichere die Skizze, dann lässt sich eine LED testen.",
  "sk.test.dirty": "Speichere zuerst, damit die Zone zu dieser Skizze passt.",
  "sk.test.inactive": "Das Gerät ist gerade nicht aktiv.",
  "sk.test.err": "Testlicht: {e}",
  "sk.nothing": "Wähle ein Teil auf der Zeichenfläche aus, um es zu bearbeiten.",
  "sk.max.parts": "Mehr als {n} Teile sind nicht möglich.",
  "sk.live.added": "„{n}“ hinzugefügt", "sk.live.deleted": "„{n}“ gelöscht", "sk.live.sel": "„{n}“ ausgewählt",
  "sk.import.err": "Die Datei ist keine gültige Skizze: {e}", "sk.import.big": "Die Datei ist größer als 1 MiB.",
  "sk.import.ok": "Skizze importiert. Mit „Speichern“ übernehmen.", "sk.import.skipped": "{n} Teil(e) mit unbekanntem Bauteil oder ungültigen Angaben wurden übersprungen.",
  "sk.import.fixed": "{n} Angabe(n) wurden an die erlaubten Werte angepasst.",
  "sk.import.confirm": "Die aktuelle Skizze durch die Datei ersetzen?",
  "sk.export.name": "casehud-skizze.json"
});
Object.assign(STRINGS.en, {
  "sk.entry.title": "Case sketch",
  "sk.entry.sub": "Draw your case with fans, RAM and LED strips and assign the parts to zones. The effect then follows the real position of the LEDs.",
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
  "sk.leave": "The sketch has unsaved changes. Leave anyway?",
  "sk.discard.confirm": "Discard all unsaved changes?",
  "sk.delete.confirm": "Delete the saved sketch? Bound zones return to their default layout.",
  "sk.delete.dirty": "The unsaved changes will be lost as well.",
  "sk.retry": "Reload",
  "sk.zone.upto": "up to {n} LEDs", "sk.zone.any": "any LED count",
  "sk.f.view": "Placed in view",
  "sk.dims.moved": "{n} part(s) were moved back into the allowed range.",
  "sk.palette": "Parts", "sk.search": "Search parts …", "sk.stage": "Case sketch drawing area",
  "sk.inspector": "Properties", "sk.views": "View",
  "sk.pal.none": "No part found.", "sk.pal.add": "Add part “{n}”",
  "sk.pal.count": "{n} LEDs", "sk.pal.var": "{a}–{b} LEDs",
  "sk.pal.generic": "Generic", "sk.approx": "approximate", "sk.approx.tip": "Size and LED positions of this part are estimates only.",
  "sk.cat.fan": "Fans", "sk.cat.fan-lcd": "Fans with display", "sk.cat.ram": "Memory", "sk.cat.strip": "LED strips",
  "sk.cat.gpu-block": "Graphics card", "sk.cat.pump": "Pump / cooler", "sk.cat.distro-plate": "Distribution plate",
  "sk.cat.case-accessory": "Case accessories", "sk.cat.cpu-block": "CPU cooler", "sk.cat.display": "Display", "sk.cat.mainboard": "Motherboard",
  "sk.cat.case-light": "Case lighting", "sk.cat.cooler": "Cooler", "sk.cat.controller": "Controller", "sk.cat.other": "Other",
  "sk.part.generic-fan-120": "Fan 120 mm, LED ring", "sk.part.generic-fan-140": "Fan 140 mm, LED ring",
  "sk.part.generic-ram-dimm": "RAM module, LED bar", "sk.part.generic-strip": "LED strip",
  "sk.part.generic-gpu-box": "Graphics card, LED frame", "sk.part.generic-pump-cyl": "Pump, round LED face",
  "sk.part.generic-distro-rect": "Distribution plate, LED frame",
  "sk.view.front": "Front", "sk.view.side": "Side", "sk.view.top": "Top",
  "sk.view.front.long": "Front view", "sk.view.side.long": "Side view", "sk.view.top.long": "Top view",
  "sk.face.front": "Front", "sk.face.side": "Side", "sk.face.top": "Top",
  "sk.front": "front", "sk.other.views": "{n} in other views",
  "sk.zoomIn": "Zoom in", "sk.zoomOut": "Zoom out", "sk.fit": "Fit",
  "sk.snap": "Snap to grid", "sk.grid": "Grid",
  "sk.hint": "Wheel: zoom · Space + drag: pan · Arrows: 1 mm (Shift: 10 mm) · R: rotate · Del: delete · Esc: cancel a drag · Click on overlap: next part",
  "sk.readout.size": "{w} × {h} mm", "sk.readout.pos": "x {x} · y {y} mm",
  "sk.empty.title": "No parts yet",
  "sk.empty.text": "Drag a part from the left onto the case, or click it. Set the case and its size on the right, then bind the parts to a zone.",
  "sk.empty.view": "There is no part in the “{v}” view yet.",
  "sk.led": "LED {n}", "sk.led.zone": "LED {n} · zone LED {z}", "sk.led.first": "First LED",
  "sk.case": "Case", "sk.case.type": "Choose case", "sk.case.custom": "Custom box",
  "sk.case.library": "From the library", "sk.case.dims": "Outer size (mm)", "sk.case.w": "Width", "sk.case.h": "Height", "sk.case.d": "Depth",
  "sk.case.fixed": "The size comes from the library.",
  "sk.case.divider": "Chamber divider", "sk.case.divider.x": "Divider at (mm from the left)",
  "sk.case.divider.hint": "Splits the front and top views into two chambers.",
  "sk.case.chamber": "Chamber {n}",
  "sk.case.canvas": "View for the LED canvas", "sk.case.canvas.hint": "The LED canvas (the small map in the Lighting tab) is drawn from this direction. All parts are mapped onto it as seen from this view.",
  "sk.view.canvas": "View for the LED canvas",
  "sk.parts": "Parts in the sketch", "sk.parts.none": "No parts yet.",
  "sk.parts.item": "Select {n}",
  "sk.bound": "bound", "sk.unbound": "not bound",
  "sk.warn.title": "Notes", "sk.warn.none": "No notes.",
  "sk.w.segments": "{z}: {n} parts in one zone, {max} are allowed.",
  "sk.w.total": "{n} LEDs in total, {max} are allowed.",
  "sk.w.zoneLeds": "Zone {z}: the chain needs {n} LEDs, the zone holds at most {m}.",
  "sk.w.outside": "{p} is partly or fully outside the case.",
  "sk.w.unbound": "{p} is not bound to a zone and stays dark.",
  "sk.w.noZone": "{p}: the device or zone “{z}” is not available right now.",
  "sk.w.unknown": "{p}: the part “{id}” is missing from the catalog.",
  "sk.w.leds": "{p}: LED count outside {a}–{b}.",
  "sk.part": "Part", "sk.part.default": "Part {n}",
  "sk.part.dup": "Duplicate", "sk.part.del": "Delete", "sk.part.size": "Size: {w} × {h} mm, {n} LEDs",
  "sk.f.label": "Name", "sk.f.label.ph": "e.g. front top",
  "sk.f.pos": "Position (mm)", "sk.f.x": "x", "sk.f.y": "y",
  "sk.f.rot": "Rotation", "sk.f.rot.free": "degrees", "sk.f.rot.btn": "Rotate 90°",
  "sk.f.mirror": "Mirror", "sk.f.mirror.sub": "Reverses the direction of the LED order.",
  "sk.f.face": "LED side faces", "sk.f.face.edge": "The part is seen from its edge; the LEDs lie on one line.",
  "sk.f.flow": "Air flow", "sk.f.flow.intake": "Intake", "sk.f.flow.exhaust": "Exhaust",
  "sk.f.flow.hint": "Exhaust reverses the LED order of the ring.",
  "sk.f.leds": "LED count", "sk.f.leds.range": "{a} to {b}", "sk.f.leds.fixed": "{n} LEDs (fixed)",
  "sk.f.len": "Length (mm)", "sk.f.len.range": "{a} to {b}",
  "sk.bind": "Connection", "sk.bind.device": "Device", "sk.bind.zone": "Zone", "sk.bind.none": "– not bound –",
  "sk.bind.gone": "{n} (not available)", "sk.bind.zone.pick": "– choose a zone –",
  "sk.bind.nodev": "No devices found. Turn lighting on to bind parts. You can still draw the sketch without devices.",
  "sk.bind.order": "Order in the chain", "sk.bind.parallel": "In parallel with the previous part",
  "sk.bind.parallel.sub": "Same LEDs as the part before it, for example through a splitter.",
  "sk.bind.range": "Uses LEDs {a} to {b} of the zone.", "sk.bind.range1": "Uses LED {a} of the zone.",
  "sk.test": "Test LED", "sk.test.go": "Test light", "sk.test.end": "Test light off",
  "sk.test.prev": "Previous LED", "sk.test.next": "Next LED", "sk.test.pos": "LED {n} of {m}",
  "sk.test.hint": "Briefly lights LED {n} of this part (zone LED {z}). Nothing is changed permanently.",
  "sk.test.need": "Bind the part to a zone and save the sketch first, then you can test an LED.",
  "sk.test.dirty": "Save first so the zone matches this sketch.",
  "sk.test.inactive": "The device is not active right now.",
  "sk.test.err": "Test light: {e}",
  "sk.nothing": "Select a part on the drawing area to edit it.",
  "sk.max.parts": "More than {n} parts are not possible.",
  "sk.live.added": "“{n}” added", "sk.live.deleted": "“{n}” deleted", "sk.live.sel": "“{n}” selected",
  "sk.import.err": "The file is not a valid sketch: {e}", "sk.import.big": "The file is larger than 1 MiB.",
  "sk.import.ok": "Sketch imported. Press “Save” to keep it.", "sk.import.skipped": "{n} part(s) with an unknown part or invalid data were skipped.",
  "sk.import.fixed": "{n} value(s) were adjusted to the allowed range.",
  "sk.import.confirm": "Replace the current sketch with the file?",
  "sk.export.name": "casehud-sketch.json"
});

applyI18n();

(() => {
  const SVGNS = "http://www.w3.org/2000/svg";
  const VIEWS = ["front", "side", "top"];
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
  const BIG = 1e6; // far beyond any case; keeps imported numbers from overflowing
  const num = (v) => (Number.isFinite(+v) && v !== null && v !== "" && typeof v !== "boolean" ? Math.max(-BIG, Math.min(BIG, +v)) : 0);
  const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));
  const fmt = (v) => String(r2(v)).replace(".", LANG === "de" ? "," : ".");
  const viewName = (v) => t("sk.view." + v);

  const S = {
    open: false, busy: false,
    cat: /** @type {any} */ (null), lim: { maxParts: 64, maxLabel: 40, maxLeds: 512, maxTotalLeds: 4096, maxDimMm: 2000, maxChambers: 4, marginMm: 50, maxSegments: 16 },
    doc: /** @type {any} */ (null), saved: "", stored: false,
    sel: /** @type {string|null} */ (null), view: "side",
    vp: /** @type {Record<string, any>} */ ({}), size: { w: 0, h: 0 },
    snap: true, grid: true, search: "", err: "",
    undo: /** @type {string[]} */ ([]), redo: /** @type {string[]} */ ([]), lastKey: "", lastAt: 0,
    space: false, drag: /** @type {any} */ (null), cursor: /** @type {any} */ (null),
    test: 0, testLive: false, insSel: /** @type {string|null} */ (null), raf: 0, insSig: "", insSync: /** @type {(() => void)[]} */ ([]),
    rev: 0, full: false, loadFailed: false, wheel: /** @type {any} */ (null), drawn: /** @type {Map<string, {x: number, y: number}>} */ (new Map()), dimsKey: "",
  };
  const maxSegments = () => (S.lim.maxSegments > 0 ? S.lim.maxSegments : 16);
  const svgEl = () => $("#sk-svg");

  // ---------- document ----------
  const newDoc = () => ({ version: 1, case: { source: "custom", id: "custom-box", dims: { w: 230, h: 480, d: 450 }, chambers: [] }, canvasView: "side", parts: [] });

  /** Only the fields the server knows, numbers rounded: what is saved, exported and compared. */
  function cleanDoc(d) {
    return {
      version: 1,
      case: {
        source: d.case.source === "library" ? "library" : "custom", id: d.case.id || "",
        dims: { w: r2(num(d.case.dims.w)), h: r2(num(d.case.dims.h)), d: r2(num(d.case.dims.d)) },
        chambers: (d.case.chambers || []).map((c) => ({ x: r2(num(c.x)), w: r2(num(c.w)) })),
      },
      canvasView: VIEWS.includes(d.canvasView) ? d.canvasView : "side",
      parts: d.parts.map((p) => {
        const o = {
          id: String(p.id), part: String(p.part), view: p.view || "side", x: r2(num(p.x)), y: r2(num(p.y)), rot: r2(num(p.rot)),
          face: p.face || "", mirror: !!p.mirror, flow: p.flow || "", leds: Math.round(num(p.leds)), label: p.label || "",
        };
        if (num(p.len) > 0) o.len = r2(num(p.len));
        if (p.bind && p.bind.device && p.bind.zone) {
          o.bind = { device: p.bind.device, zone: p.bind.zone, order: Math.round(num(p.bind.order)), parallel: !!p.bind.parallel };
        }
        return o;
      }),
    };
  }
  const docJSON = () => JSON.stringify(cleanDoc(S.doc));
  const isDirty = () => !!S.doc && docJSON() !== S.saved;
  const parts = () => S.doc.parts;
  const partById = (id) => parts().find((p) => p.id === id);
  const selPart = () => (S.sel ? partById(S.sel) : null);

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
    const sp = S.sel && partById(S.sel);
    if (!sp || (sp.view || "side") !== S.view) S.sel = null;
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

  /** Direction the LED side points to when a part with face is drawn in view. */
  function faceNormal(view, face) {
    if (view === face) return { flat: true, nx: 0, ny: 0 };
    const m = { front: { side: [1, 0], top: [0, -1] }, side: { front: [-1, 0], top: [0, -1] }, top: { front: [0, -1], side: [1, 0] } };
    const n = m[view] && m[view][face];
    return n ? { flat: false, nx: n[0], ny: n[1] } : { flat: true, nx: 0, ny: 0 };
  }

  /** Placement in view millimetres: LEDs in wire order, bounding box, transform for the outline. */
  function project(pl) {
    const part = partOf(pl.part);
    if (!part) return null;
    const view = pl.view || "side", face = pl.face || view;
    const fn = faceNormal(view, face);
    const res = resolve(part, pl.leds, pl.len);
    const { w, h, d } = res;
    const mirror = !!pl.mirror !== (pl.flow === "exhaust" && isFan(part));
    const rad = (num(pl.rot) * Math.PI) / 180, sin = Math.sin(rad), cos = Math.cos(rad);
    const tf = (x, y) => {
      if (mirror) x = w - x;
      const dx = x - w / 2, dy = y - h / 2;
      return [dx * cos - dy * sin, dx * sin + dy * cos];
    };
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (const c of [[0, 0], [w, 0], [w, h], [0, h]]) {
      const [u, v] = tf(c[0], c[1]);
      minX = Math.min(minX, u); maxX = Math.max(maxX, u); minY = Math.min(minY, v); maxY = Math.max(maxY, v);
    }
    const bw = maxX - minX, bh = maxY - minY;
    let box;
    if (fn.flat) box = { x: pl.x, y: pl.y, w: bw, h: bh };
    else if (fn.ny !== 0) box = { x: pl.x, y: pl.y, w: bw, h: d };
    else box = { x: pl.x, y: pl.y, w: d, h: bh };
    const leds = res.pos.map(([qx, qy]) => {
      const [u0, v0] = tf(qx, qy);
      const u = u0 - minX, v = v0 - minY;
      let px, py;
      if (fn.flat) { px = u; py = v; }
      else if (fn.ny !== 0) { px = u; py = fn.ny > 0 ? d : 0; }
      else { px = fn.nx > 0 ? d : 0; py = v; }
      return { x: pl.x + clamp(px, 0, box.w), y: pl.y + clamp(py, 0, box.h) };
    });
    const xf = `translate(${pl.x - minX} ${pl.y - minY}) rotate(${num(pl.rot)}) translate(${-w / 2} ${-h / 2})` + (mirror ? ` translate(${w} 0) scale(-1 1)` : "");
    return { leds, box, edgeOn: !fn.flat, xf, shapes: fn.flat ? res.front : null, n: leds.length, w, h };
  }
  const MISSING = (pl) => ({ leds: [], box: { x: pl.x, y: pl.y, w: 40, h: 40 }, edgeOn: false, xf: "", shapes: null, n: 0, w: 40, h: 40, missing: true });
  /** Projections are cached per placement object and invalidated when one of its inputs changes. */
  let projCache = new WeakMap();
  const proj = (pl) => {
    const sig = [pl.part, pl.view, pl.face, pl.x, pl.y, pl.rot, pl.mirror, pl.flow, pl.leds, pl.len].join("|");
    const c = projCache.get(pl);
    if (c && c.sig === sig) return c.pr;
    const pr = project(pl) || MISSING(pl);
    projCache.set(pl, { sig, pr });
    return pr;
  };

  const viewSize = (view) => {
    const d = S.doc.case.dims;
    return view === "front" ? { w: d.w, h: d.h } : view === "side" ? { w: d.d, h: d.h } : { w: d.w, h: d.d };
  };

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
      for (const { p } of list) {
        const n = proj(p).n;
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
  const partTitle = (p) => p.label || (partOf(p.part) ? partName(partOf(p.part)) : p.part);
  function warnings() {
    const out = [], flagged = new Set();
    const ch = chains();
    let total = 0;
    for (const p of parts()) {
      const part = partOf(p.part), pr = proj(p), name = partTitle(p);
      total += pr.n;
      if (!part) { out.push({ id: p.id, kind: "warn", text: t("sk.w.unknown", { p: name, id: p.part }) }); continue; }
      const g = gen(part);
      if (g && p.leds && (p.leds < g.min || p.leds > g.max)) out.push({ id: p.id, kind: "warn", text: t("sk.w.leds", { p: name, a: g.min, b: g.max }) });
      const vs = viewSize(p.view || "side"), b = pr.box;
      if (b.x < -0.01 || b.y < -0.01 || b.x + b.w > vs.w + 0.01 || b.y + b.h > vs.h + 0.01) out.push({ id: p.id, kind: "warn", text: t("sk.w.outside", { p: name }) });
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

  function caseLayer(view) {
    const vs = viewSize(view), g = sv("g", { class: "sk-case" });
    const tpl = S.doc.case.source === "library" && caseOf(S.doc.case.id);
    const cv = tpl && tpl.views && tpl.views[view];
    g.append(sv("rect", { class: "sk-case-body", x: 0, y: 0, width: vs.w, height: vs.h, rx: cv ? 0 : 8 }));
    if (cv) for (const sh of cv.outline || []) g.append(shapeEl(sh, " sk-case-sh"));
    if (tpl) {
      for (const a of (tpl.areas || []).filter((q) => q.view === view)) {
        g.append(shapeEl({ ...a.shape, role: "area" }));
        const b = shapeBox(a.shape);
        if (b && (a.name || a.kind)) g.append(sv("text", { class: "sk-area-label", x: b.x + b.w / 2, y: b.y + b.h / 2, "font-size": 11 / vp().scale, "text-anchor": "middle" }, short(a.name || a.kind)));
      }
      for (const m of (tpl.mounts || []).filter((q) => q.view === view)) {
        g.append(shapeEl({ ...m.region, role: "mount" }));
        const lay = (m.layouts || [])[0];
        if (lay) for (const [cx, cy] of lay.snap_mm || []) g.append(sv("circle", { class: "sk-mount-dot", cx, cy, r: lay.fan_mm / 2 }));
      }
    } else if (view !== "side") { // chamber divider of the custom box: front and top view
      const ch = S.doc.case.chambers || [];
      ch.forEach((c, i) => {
        g.append(sv("rect", { class: "sk-chamber c" + (i % 2), x: c.x, y: 0, width: c.w, height: vs.h }));
        if (i > 0) g.append(sv("line", { class: "sk-divider", x1: c.x, y1: 0, x2: c.x, y2: vs.h }));
        g.append(sv("text", { class: "sk-area-label", x: c.x + c.w / 2, y: 14 / vp().scale + 4, "font-size": 11 / vp().scale, "text-anchor": "middle" }, t("sk.case.chamber", { n: i + 1 })));
      });
    }
    return g;
  }
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

  function partGroup(p) {
    const part = partOf(p.part), pr = proj(p), sc = vp().scale, sel = p.id === S.sel;
    const b = pr.box, col = p.bind ? devColor(p.bind.device) : null;
    const name = partTitle(p);
    const g = sv("g", { class: "sk-part" + (sel ? " sel" : "") + (pr.missing ? " missing" : ""), "data-id": p.id, role: "img", "aria-label": name + ", " + t("sk.part.size", { w: fmt(b.w), h: fmt(b.h), n: pr.n }) });
    g.append(sv("rect", { class: "sk-hit", x: b.x, y: b.y, width: b.w, height: b.h }));
    if (pr.edgeOn || pr.missing || !pr.shapes) {
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
      pr.leds.forEach((q, i) => dots.append(sv("circle", { class: "sk-led" + (i === 0 ? " first" : "") + (sel && S.testLive && i === S.test ? " test" : ""), cx: q.x, cy: q.y, r: rad, "data-led": i })));
      g.append(dots);
      if (pr.leds.length > 1) { // wire order arrow on the first step
        const a = pr.leds[0], c = pr.leds[1], ang = (Math.atan2(c.y - a.y, c.x - a.x) * 180) / Math.PI;
        const len = Math.hypot(c.x - a.x, c.y - a.y), sz = clamp(8 / sc, 2.5, Math.max(len * 0.9, 2.5));
        g.append(sv("path", { class: "sk-arrow", d: `M${-sz * 0.6} ${-sz * 0.5}L${sz * 0.6} 0L${-sz * 0.6} ${sz * 0.5}Z`, transform: `translate(${(a.x + c.x) / 2} ${(a.y + c.y) / 2}) rotate(${ang})` }));
      }
    }
    const fs = 11 / sc;
    g.append(sv("text", { class: "sk-part-label", x: b.x + b.w / 2, y: b.y + b.h + 14 / sc, "font-size": fs, "text-anchor": "middle" }, name));
    return g;
  }

  function selectionLayer(p) {
    const pr = proj(p), sc = vp().scale, b = pr.box, pad = 3 / sc;
    const g = sv("g", { class: "sk-selection" });
    g.append(sv("rect", { class: "sk-selbox", x: b.x - pad, y: b.y - pad, width: b.w + 2 * pad, height: b.h + 2 * pad, rx: 3 / sc }));
    if (!pr.missing) {
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

  /** A changed case (type or size) invalidates the automatic fit of every view. */
  function syncCase() {
    const c = S.doc.case, key = [c.source, c.id, c.dims.w, c.dims.h, c.dims.d].join("|");
    if (key === S.dimsKey) return;
    S.dimsKey = key;
    for (const [k, v] of Object.entries(S.vp)) if (v.fit) delete S.vp[k];
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
    // "front" marker: the case front is on the left of the side view, at the top of the top view
    if (S.view !== "front") {
      const f = 11 / b.scale;
      kids.push(S.view === "side"
        ? sv("text", { class: "sk-front-label", x: -8 / b.scale, y: vs.h / 2, "font-size": f, "text-anchor": "middle", transform: `rotate(-90 ${-8 / b.scale} ${vs.h / 2})` }, "◂ " + t("sk.front"))
        : sv("text", { class: "sk-front-label", x: vs.w / 2, y: -8 / b.scale, "font-size": f, "text-anchor": "middle" }, "▴ " + t("sk.front")));
    }
    const list = parts().filter((p) => (p.view || "side") === S.view);
    S.drawn = new Map(list.map((p) => [p.id, { x: p.x, y: p.y }]));
    const layer = sv("g", { class: "sk-parts" });
    for (const p of list) if (p.id !== S.sel) layer.append(partGroup(p));
    const sp = selPart();
    if (sp && (sp.view || "side") === S.view) layer.append(partGroup(sp)); // selected part on top
    kids.push(layer);
    if (sp && (sp.view || "side") === S.view) kids.push(selectionLayer(sp));
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
    const d = S.drag, p = d && d.kind === "move" && partById(d.id), o = p && S.drawn.get(d.id);
    if (p && o) {
      const tr = `translate(${r2(p.x - o.x)} ${r2(p.y - o.y)})`, svg = svgEl();
      const g = svg.querySelector(`.sk-part[data-id="${CSS.escape(d.id)}"]`), sel = svg.querySelector(".sk-selection");
      if (g) g.setAttribute("transform", tr);
      if (sel) sel.setAttribute("transform", tr);
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
      if (major) kx.push(sv("text", { x: px + 3, y: 11, class: "sk-tick-label" }, String(r2(v))));
    }
    for (let v = Math.ceil(b.y / sub) * sub; v <= b.y + b.h; v += sub) {
      const py = (v - b.y) * b.scale, major = Math.abs(v / step - Math.round(v / step)) < 1e-6;
      ky.push(sv("line", { y1: py, y2: py, x1: major ? 16 : 22, x2: 28, class: "sk-tick" }));
      if (major) ky.push(sv("text", { x: 14, y: py - 3, class: "sk-tick-label", transform: `rotate(-90 14 ${py - 3})`, "text-anchor": "end" }, String(r2(v))));
    }
    if (S.cursor) {
      kx.push(sv("line", { x1: (S.cursor.x - b.x) * b.scale, x2: (S.cursor.x - b.x) * b.scale, y1: 0, y2: 20, class: "sk-tick cur" }));
      ky.push(sv("line", { y1: (S.cursor.y - b.y) * b.scale, y2: (S.cursor.y - b.y) * b.scale, x1: 0, x2: 28, class: "sk-tick cur" }));
    }
    rx.replaceChildren(...kx); ry.replaceChildren(...ky);
  }

  function updateStatus() {
    const vs = viewSize(S.view), c = S.cursor;
    $("#sk-zoom").textContent = Math.round(vp().scale * 100) / 100 + " px/mm";
    $("#sk-readout").textContent = t("sk.readout.size", { w: fmt(vs.w), h: fmt(vs.h) }) + (c ? "  ·  " + t("sk.readout.pos", { x: fmt(c.x), y: fmt(c.y) }) : "");
  }
  function updateEmpty() {
    const box = $("#sk-empty");
    const none = !parts().length, here = parts().filter((p) => (p.view || "side") === S.view).length;
    box.hidden = !(none || !here);
    if (box.hidden) return;
    const key = none ? "empty" : "view";
    if (box.dataset.key === key + LANG) return;
    box.dataset.key = key + LANG;
    box.replaceChildren(...(none
      ? [E("div", { class: "sk-empty-icon", "aria-hidden": "true" }, svgIcon("M3 7h18v10H3zM7 7v10M12 12h5")), E("b", {}, t("sk.empty.title")), E("p", {}, t("sk.empty.text"))]
      : [E("p", {}, t("sk.empty.view", { v: viewName(S.view) }))]));
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
  function renderPalette() {
    const box = $("#sk-pal-list");
    if (!S.cat) { box.replaceChildren(E("p", { class: "empty-note" }, "…")); return; }
    const q = S.search.trim().toLowerCase();
    const byCat = new Map();
    for (const p of Object.values(S.cat.parts)) {
      const hay = (partName(p) + " " + p.vendor + " " + p.model + " " + p.id + " " + catName(p.category)).toLowerCase();
      if (q && !hay.includes(q)) continue;
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
  /** Moves a part so that the centre of its box is at (cx, cy), snapped. */
  function addPart(partId, at) {
    const part = partOf(partId);
    if (!part) return;
    if (parts().length >= S.lim.maxParts) { toast(t("sk.max.parts", { n: S.lim.maxParts }), true); return; }
    const g = gen(part);
    const pl = { id: newId(), part: partId, view: S.view, x: 0, y: 0, rot: 0, face: "", mirror: false, flow: isFan(part) ? "intake" : "", leds: g ? g.default : 0, label: "" };
    const box = proj(pl).box, vs = viewSize(S.view);
    let c = at;
    if (!c) { // centre of the visible part of the case, nudged off parts already there
      const b = vbox();
      c = { x: clamp(b.x + b.w / 2, 0, vs.w), y: clamp(b.y + b.h / 2, 0, vs.h) };
      while (parts().some((q) => (q.view || "side") === S.view && Math.abs(q.x - (snapV(c.x - box.w / 2))) < 1 && Math.abs(q.y - snapV(c.y - box.h / 2)) < 1)) { c = { x: c.x + 10, y: c.y + 10 }; }
    }
    pl.x = clampPos(snapV(c.x - box.w / 2), "x", pl.view); pl.y = clampPos(snapV(c.y - box.h / 2), "y", pl.view);
    mutate(() => { parts().push(pl); S.sel = pl.id; S.test = 0; });
    live(t("sk.live.added", { n: partName(part) }));
    svgEl().focus({ preventScroll: true });
  }
  function clampPos(v, axis, view) {
    const vs = viewSize(view), m = S.lim.marginMm;
    return clamp(v, -m, (axis === "x" ? vs.w : vs.h) + m);
  }
  function movePart(p, x, y) { p.x = r2(clampPos(x, "x", p.view || "side")); p.y = r2(clampPos(y, "y", p.view || "side")); }
  /** Rotation about the centre of the box: the box grows or shrinks around it. */
  function rotatePart(p, deg) {
    const b0 = proj(p).box, cx = b0.x + b0.w / 2, cy = b0.y + b0.h / 2;
    p.rot = r2(deg);
    const b1 = proj(p).box;
    movePart(p, cx - b1.w / 2, cy - b1.h / 2);
  }
  function setFace(p, face) {
    const b0 = proj(p).box, cx = b0.x + b0.w / 2, cy = b0.y + b0.h / 2;
    p.face = face === (p.view || "side") ? "" : face;
    const b1 = proj(p).box;
    movePart(p, cx - b1.w / 2, cy - b1.h / 2);
  }
  /** Moves a part to another view; the position numbers stay and are limited to the new view. */
  function setPartView(p, v) {
    if ((p.view || "side") === v) return;
    p.view = v;
    if (p.face === v) p.face = "";
    movePart(p, p.x, p.y);
  }
  function deletePart(id) {
    const p = partById(id);
    if (!p) return;
    endTest();
    const n = partTitle(p);
    mutate(() => { S.doc.parts = parts().filter((q) => q.id !== id); if (S.sel === id) S.sel = null; });
    live(t("sk.live.deleted", { n }));
  }
  function duplicatePart(id) {
    const p = partById(id);
    if (!p) return;
    if (parts().length >= S.lim.maxParts) { toast(t("sk.max.parts", { n: S.lim.maxParts }), true); return; }
    const c = JSON.parse(JSON.stringify(p));
    c.id = newId(); movePart(c, p.x + 10, p.y + 10);
    if (c.bind) c.bind.order = Math.min(1000, (c.bind.order || 0) + 1);
    mutate(() => { parts().push(c); S.sel = c.id; });
  }
  /** Selects a part in any view: the editor switches to the view the part lies in. */
  function focusPart(id) {
    const p = partById(id);
    if (!p) return;
    if ((p.view || "side") !== S.view) setView(p.view || "side");
    select(id);
  }
  /** Parts under a point (view millimetres), the smallest first so small parts on large ones stay reachable. */
  function hitParts(mm) {
    const tol = 3 / vp().scale;
    return parts().filter((q) => {
      if ((q.view || "side") !== S.view) return false;
      const b = proj(q).box;
      return mm.x >= b.x - tol && mm.x <= b.x + b.w + tol && mm.y >= b.y - tol && mm.y <= b.y + b.h + tol;
    }).map((q) => { const b = proj(q).box; return { id: q.id, area: b.w * b.h }; }).sort((a, b) => a.area - b.area).map((q) => q.id);
  }
  /** Keeps every part within the allowed range of its view (after the case changed). Returns how many moved. */
  function fitParts() {
    let n = 0;
    for (const q of parts()) {
      const v = q.view || "side", x = r2(clampPos(q.x, "x", v)), y = r2(clampPos(q.y, "y", v));
      if (x !== q.x || y !== q.y) { q.x = x; q.y = y; n++; }
    }
    return n;
  }
  function select(id) {
    if (S.sel === id) return;
    endTest();
    S.sel = id; S.test = 0;
    const p = id && partById(id);
    if (p) live(t("sk.live.sel", { n: partTitle(p) }));
    afterChange(true);
  }

  // ---------- pointer interaction ----------
  function onPointerDown(ev) {
    if (ev.button !== 0 && ev.button !== 1) return;
    const svg = svgEl();
    svg.focus({ preventScroll: true });
    const target = ev.target;
    const pan = ev.button === 1 || S.space;
    const rot = !pan && target.closest && target.closest(".sk-rot");
    const g = !pan && target.closest && target.closest(".sk-part");
    const mm = toMM(ev);
    if (rot && selPart()) {
      const p = selPart(), b = proj(p).box;
      S.drag = { kind: "rot", pid: ev.pointerId, id: p.id, cx: b.x + b.w / 2, cy: b.y + b.h / 2, orig: p.rot, ox: p.x, oy: p.y, moved: false };
    } else if (g && partById(g.dataset.id)) {
      const hits = hitParts(mm);
      const wasSel = S.sel;
      const id = hits.length ? (wasSel && hits.includes(wasSel) ? wasSel : hits[0]) : g.dataset.id;
      const p = partById(id);
      select(p.id);
      // a click without moving on a spot with several parts passes the selection on
      S.drag = { kind: "move", pid: ev.pointerId, id: p.id, sx: mm.x, sy: mm.y, ox: p.x, oy: p.y, moved: false, cycle: hits.length > 1 && wasSel === p.id ? hits : null };
    } else {
      if (!pan) select(null);
      const v = vp();
      S.drag = { kind: "pan", pid: ev.pointerId, px: ev.clientX, py: ev.clientY, cx: v.cx, cy: v.cy };
      svg.classList.add("panning");
    }
    try { svg.setPointerCapture(ev.pointerId); } catch { /* the pointer is already gone */ }
    ev.preventDefault();
  }
  function onPointerMove(ev) {
    const mm = toMM(ev);
    S.cursor = mm;
    const d = S.drag;
    if (!d) { hoverLed(ev); schedule(true); return; }
    if (d.kind === "pan") {
      const v = vp();
      v.cx = d.cx - (ev.clientX - d.px) / v.scale; v.cy = d.cy - (ev.clientY - d.py) / v.scale; v.fit = false;
    } else if (d.kind === "move") {
      const p = partById(d.id);
      if (!p) return;
      let nx = d.ox + (mm.x - d.sx), ny = d.oy + (mm.y - d.sy);
      const snap = S.snap !== ev.altKey; // Alt flips the snapping for this move
      if (!d.moved && Math.hypot(nx - d.ox, ny - d.oy) * vp().scale < 3) return; // a click, not a drag
      if (!d.moved) { d.moved = true; S.undo.push(JSON.stringify(S.doc)); if (S.undo.length > 100) S.undo.shift(); S.redo = []; S.lastKey = ""; renderHead(); }
      const pr = proj(p);
      const m = mountSnap(p, pr, nx, ny);
      if (m) { nx = m.x; ny = m.y; } else if (snap) { nx = snapV(nx, true); ny = snapV(ny, true); }
      movePart(p, nx, ny);
      d.dirty = true;
    } else if (d.kind === "rot") {
      const p = partById(d.id);
      if (!p) return;
      let a = (Math.atan2(mm.y - d.cy, mm.x - d.cx) * 180) / Math.PI + 90; // handle is above the box
      a = ((a % 360) + 360) % 360;
      a = ev.shiftKey ? Math.round(a) : Math.round(a / 15) * 15;
      if (a >= 360) a = 0;
      if (a !== p.rot) {
        if (!d.moved) { d.moved = true; S.undo.push(JSON.stringify(S.doc)); S.redo = []; S.lastKey = ""; renderHead(); }
        rotatePart(p, a); d.dirty = true;
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
    svg.classList.remove("panning");
    try { svg.releasePointerCapture(ev.pointerId); } catch { /* already released */ }
    if (d.kind === "pan") return;
    if (d.dirty) afterChange();
    else if (d.kind === "move" && d.cycle) { // plain click on a stack of parts: the next one
      const i = d.cycle.indexOf(d.id);
      select(d.cycle[(i + 1) % d.cycle.length]);
    }
  }
  /** Escape during a drag: the part goes back to where it was and the undo step disappears. */
  function cancelDrag() {
    const d = S.drag;
    if (!d || d.kind === "pan") return;
    S.drag = null;
    const svg = svgEl();
    try { svg.releasePointerCapture(d.pid); } catch { /* already released */ }
    const p = partById(d.id);
    if (p && d.moved) {
      p.x = d.ox; p.y = d.oy;
      if (d.kind === "rot") p.rot = d.orig;
      S.undo.pop();
    }
    afterChange();
  }
  /** Fans snap onto a mounting position of a library case when their centre is near one. */
  function mountSnap(p, pr, nx, ny) {
    const part = partOf(p.part), tpl = S.doc.case.source === "library" && caseOf(S.doc.case.id);
    if (!tpl || !isFan(part) || pr.missing) return null;
    const cx = nx + pr.box.w / 2, cy = ny + pr.box.h / 2;
    let best = null, bd = 12;
    for (const m of tpl.mounts || []) {
      if (m.view !== (p.view || "side")) continue;
      for (const lay of m.layouts || []) {
        if (Math.abs(lay.fan_mm - part.dimensions_mm.w) > 2) continue;
        for (const [sx, sy] of lay.snap_mm || []) { const dd = Math.hypot(sx - cx, sy - cy); if (dd < bd) { bd = dd; best = { x: sx - pr.box.w / 2, y: sy - pr.box.h / 2 }; } }
      }
    }
    return best;
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
  function onKey(ev) {
    if (!S.open || !$("#tab-light").classList.contains("active")) return;
    const k = ev.key;
    if (k === " " && !typing(ev.target) && ev.target.tagName !== "BUTTON" && ev.target.tagName !== "SUMMARY") { S.space = true; svgEl().classList.add("pan-ready"); ev.preventDefault(); return; }
    const mod = ev.ctrlKey || ev.metaKey;
    if (mod && !typing(ev.target)) {
      const lk = k.toLowerCase();
      if (lk === "z") { ev.preventDefault(); ev.shiftKey ? redo() : undo(); return; }
      if (lk === "y") { ev.preventDefault(); redo(); return; }
    }
    if (mod && k.toLowerCase() === "s") { ev.preventDefault(); save(); return; }
    if (typing(ev.target) || (ev.target.tagName === "BUTTON" && k === "Enter")) return;
    if (mod && k.toLowerCase() === "d" && S.sel) { ev.preventDefault(); duplicatePart(S.sel); return; }
    if (mod || ev.altKey) return; // Alt+Arrow and friends belong to the browser
    const p = selPart();
    if (k === "Escape") {
      if (S.drag && S.drag.kind !== "pan") { cancelDrag(); ev.preventDefault(); } else if (S.sel) { select(null); ev.preventDefault(); }
      return;
    }
    if ((k === "Delete" || k === "Backspace") && p) { ev.preventDefault(); deletePart(p.id); return; }
    if ((k === "r" || k === "R") && p) { ev.preventDefault(); mutate(() => rotatePart(p, ((Math.round(p.rot / 90) * 90 + (ev.shiftKey ? -90 : 90)) % 360 + 360) % 360)); return; }
    if (k.startsWith("Arrow") && p) {
      ev.preventDefault();
      const st = ev.shiftKey ? 10 : 1;
      mutate(() => movePart(p, p.x + (k === "ArrowRight" ? st : k === "ArrowLeft" ? -st : 0), p.y + (k === "ArrowDown" ? st : k === "ArrowUp" ? -st : 0)), "nudge" + p.id);
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
  function seg(key, items, get, set, aria) {
    const box = E("div", { class: "seg small", role: "group", "aria-label": aria });
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
    const sig = [S.sel, p && p.bind && p.bind.device, p && p.view, !!p, LANG, devices().map((d) => d.id + d.zones.length).join(), S.doc.case.source, S.doc.case.id].join("|");
    if (!force && sig === S.insSig) { S.insSync.forEach((f) => f()); fillDynamic(); return; }
    S.insSig = sig;
    const keepScroll = S.insSel === S.sel;
    S.insSel = S.sel;
    const focusKey = document.activeElement && box.contains(document.activeElement) ? /** @type {any} */ (document.activeElement).dataset.f : "";
    S.insSync = [];
    box.replaceChildren(...(p ? partInspector(p) : caseInspector()), warnBox());
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
    const sz = $(".sk-part-size");
    if (sz && selPart()) { const pr = proj(selPart()); sz.textContent = pr.missing ? "" : t("sk.part.size", { w: fmt(pr.box.w), h: fmt(pr.box.h), n: pr.n }); }
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
    const views = seg("canvas", VIEWS.map((v) => [v, t("sk.view." + v)]), () => S.doc.canvasView, (v) => mutate(() => { S.doc.canvasView = v; }), t("sk.case.canvas"));
    const list = parts();
    return [
      E("section", { class: "sk-sec" }, E("h3", {}, t("sk.case")),
        field(t("sk.case.type"), sel), lib ? E("p", { class: "sub" }, t("sk.case.fixed")) : null,
        field(t("sk.case.dims"), dimsBox), divRow,
        field(t("sk.case.canvas"), views, E("small", {}, t("sk.case.canvas.hint")))),
      E("section", { class: "sk-sec" }, E("h3", {}, t("sk.parts"), list.length ? E("span", { class: "badge sk-b" }, String(list.length)) : null),
        list.length ? E("ul", { class: "sk-plist" }, list.map((q) => {
          const col = q.bind ? devColor(q.bind.device) : null;
          return E("li", {}, E("button", { type: "button", class: "sk-prow", "aria-label": t("sk.parts.item", { n: partTitle(q) }), onclick: () => focusPart(q.id) },
            E("i", { class: "sk-dot" + (col ? "" : " off"), style: col ? { background: col } : {} }),
            E("span", { class: "sk-prow-text" }, E("b", {}, partTitle(q)), E("small", {}, viewName(q.view || "side") + " · " + (q.bind ? t("sk.bound") : t("sk.unbound")))),
            E("span", { class: "sk-prow-n" }, String(proj(q).n))));
        })) : E("p", { class: "sub" }, t("sk.parts.none"))),
    ];
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
      if (id === "custom-box") { c.source = "custom"; c.id = "custom-box"; return; }
      const tpl = caseOf(id);
      if (!tpl) return;
      c.source = "library"; c.id = id; c.dims = { w: tpl.dims.w, h: tpl.dims.h, d: tpl.dims.d }; c.chambers = (tpl.chambers || []).map((q) => ({ x: q.x, w: q.w }));
      const n = fitParts();
      if (n) toast(t("sk.dims.moved", { n }));
    });
    fitView(); renderInspector(true); schedule();
  }

  function partInspector(p) {
    const part = partOf(p.part), view = p.view || "side";
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

    // placement
    const rotRow = E("div", { class: "sk-rotrow" },
      seg("rot", [0, 90, 180, 270].map((d) => [d, d + "°"]), () => ((Math.round(p.rot * 100) / 100) % 360 + 360) % 360, (d) => mutate(() => rotatePart(p, d)), t("sk.f.rot")),
      E("label", { class: "sk-rot-free" }, numInput("rotfree", () => p.rot, (v) => rotatePart(p, clamp(v, -360, 360)), { min: -360, max: 360, aria: t("sk.f.rot.free") }), E("small", {}, "°")));
    out.push(E("section", { class: "sk-sec" },
      field(t("sk.f.view"), seg("view", VIEWS.map((v) => [v, viewName(v)]), () => p.view || "side", (v) => { mutate(() => setPartView(p, v)); setView(v); }, t("sk.f.view"))),
      field(t("sk.f.pos"), E("div", { class: "sk-xy" },
        E("label", {}, E("small", {}, t("sk.f.x")), numInput("x", () => p.x, (v) => movePart(p, v, p.y), { step: 1, aria: "x" })),
        E("label", {}, E("small", {}, t("sk.f.y")), numInput("y", () => p.y, (v) => movePart(p, p.x, v), { step: 1, aria: "y" })))),
      field(t("sk.f.rot"), rotRow),
      switchRow("mirror", t("sk.f.mirror"), t("sk.f.mirror.sub"), () => p.mirror, (on) => { p.mirror = on; }),
      field(t("sk.f.face"), seg("face", VIEWS.map((v) => [v, t("sk.face." + v)]), () => p.face || view, (v) => mutate(() => setFace(p, v)), t("sk.f.face")),
        (p.face && p.face !== view) ? E("small", {}, t("sk.f.face.edge")) : null)));
    if (isFan(part)) {
      out.push(E("section", { class: "sk-sec" }, field(t("sk.f.flow"),
        seg("flow", [["intake", t("sk.f.flow.intake")], ["exhaust", t("sk.f.flow.exhaust")]], () => p.flow || "intake", (v) => mutate(() => { p.flow = v; }), t("sk.f.flow")),
        E("small", {}, t("sk.f.flow.hint")))));
    }
    // LEDs
    const ledSec = E("section", { class: "sk-sec" });
    if (g) {
      ledSec.append(field(t("sk.f.leds"), numInput("leds", () => p.leds || g.default, (v) => { p.leds = clamp(Math.round(v), g.min, g.max); S.test = Math.min(S.test, p.leds - 1); }, { min: g.min, max: g.max, aria: t("sk.f.leds") }),
        E("small", {}, t("sk.f.leds.range", { a: g.min, b: g.max }))));
      if (g.lengthMax > 0) {
        ledSec.append(field(t("sk.f.len"), numInput("len", () => p.len || part.dimensions_mm.w, (v) => {
          const b0 = proj(p).box, cx = b0.x + b0.w / 2, cy = b0.y + b0.h / 2;
          p.len = clamp(v, g.lengthMin, g.lengthMax);
          const b1 = proj(p).box; movePart(p, cx - b1.w / 2, cy - b1.h / 2);
        }, { min: g.lengthMin, max: g.lengthMax, aria: t("sk.f.len") }), E("small", {}, t("sk.f.len.range", { a: g.lengthMin, b: g.lengthMax }))));
      }
    } else ledSec.append(E("p", { class: "sub" }, t("sk.f.leds.fixed", { n: (part.leds.positions_mm || []).length })));
    out.push(ledSec);
    out.push(bindSection(p));
    return out;
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
    const pr = proj(p), info = chains().info.get(p.id), n = pr.n;
    S.test = clamp(S.test, 0, Math.max(n - 1, 0));
    const { d, z } = findZone(p.bind.device, p.bind.zone);
    const saved = S.stored && savedHas(p);
    let why = "";
    if (!p.bind.zone || !n) why = t("sk.test.need");
    else if (!saved || isDirty()) why = t("sk.test.dirty");
    else if (!d || !z || d.state !== "ok") why = t("sk.test.inactive");
    const zi = info ? info.start + S.test : S.test;
    const key = [S.test, n, why, zi, LANG].join("|");
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
      E("small", {}, why || t("sk.test.hint", { n: S.test + 1, z: zi + 1 })));
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
  function setView(v) {
    if (S.view === v) return;
    endTest();
    S.view = v;
    const sp = S.sel && partById(S.sel);
    if (!sp || (sp.view || "side") !== v) S.sel = null;
    renderViews(); afterChange(true); fitIfNeeded();
  }
  function fitIfNeeded() { if (!S.vp[S.view]) fitView(); draw(); }
  const cvIcon = () => sv("svg", { class: "sk-cv", viewBox: "0 0 24 24", width: 12, height: 12, fill: "none", stroke: "currentColor", "stroke-width": 2.4, "stroke-linecap": "round", "stroke-linejoin": "round", "aria-hidden": "true" }, sv("path", { d: "M3 5h18v12H3zM9 21h6" }));
  /** The view tabs are kept (focus stays on them); only labels, counts and states change. */
  function renderViews() {
    const box = $("#sk-views");
    if (box.children.length !== VIEWS.length) {
      box.setAttribute("aria-orientation", "horizontal");
      box.replaceChildren(...VIEWS.map((v) => E("button", { type: "button", role: "tab", "data-v": v, onclick: () => setView(v) })));
    }
    VIEWS.forEach((v, i) => {
      const b = box.children[i], n = parts().filter((q) => (q.view || "side") === v).length, on = S.view === v, cv = S.doc.canvasView === v;
      b.className = on ? "active" : "";
      b.setAttribute("aria-selected", String(on));
      b.tabIndex = on ? 0 : -1;
      const title = t("sk.view." + v + ".long") + (cv ? " · " + t("sk.view.canvas") : "");
      b.title = title; b.setAttribute("aria-label", title + (n ? " (" + n + ")" : ""));
      b.replaceChildren(...[t("sk.view." + v), n ? E("small", { class: "sk-count" }, String(n)) : null, cv ? cvIcon() : null].filter((x) => x != null));
    });
  }
  /** Arrow keys, Home and End move between the view tabs. */
  function onViewsKey(ev) {
    const i = VIEWS.indexOf(S.view);
    const next = ev.key === "ArrowRight" || ev.key === "ArrowDown" ? (i + 1) % VIEWS.length : ev.key === "ArrowLeft" || ev.key === "ArrowUp" ? (i + VIEWS.length - 1) % VIEWS.length : ev.key === "Home" ? 0 : ev.key === "End" ? VIEWS.length - 1 : -1;
    if (next < 0 || ev.altKey || ev.ctrlKey || ev.metaKey) return;
    ev.preventDefault(); ev.stopPropagation(); // the arrows must not nudge the selected part
    setView(VIEWS[next]);
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
    $("#sk-hint").textContent = t("sk.hint"); $("#sk-hint").title = t("sk.hint");
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
    schedule();
  }

  // ---------- load, save, delete ----------
  function adopt(sketch) {
    S.doc = sketch ? cleanDoc(sketch) : newDoc();
    S.stored = !!sketch;
    S.saved = docJSON();
    S.undo = []; S.redo = []; S.lastKey = "";
    S.sel = null; S.test = 0; S.rev++;
  }
  async function load() {
    S.busy = true; S.err = ""; S.loadFailed = false; renderHead();
    try {
      const [cat, sk, light] = await Promise.all([api("GET", "/api/lighting/catalog"), api("GET", "/api/lighting/sketch"), api("GET", "/api/lighting").catch(() => null)]);
      S.cat = cat; projCache = new WeakMap(); chainCache = { rev: -1, v: null };
      if (cat.limits) S.lim = { ...S.lim, ...cat.limits };
      if (light && typeof lv !== "undefined") { lv = light; }
      adopt(sk && sk.sketch);
      S.view = S.doc.parts.length ? S.doc.canvasView : "side";
      S.vp = {};
    } catch (e) {
      S.err = t("sk.err.load", { e: e.message });
      S.loadFailed = true;
      adopt(null); // an old document must not stay: it could be saved over the stored sketch
      S.vp = {};
    }
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
      toast(t("sk.saved"));
    } catch (e) { S.err = t("sk.err.save", { e: e.message }); }
    S.busy = false;
    renderAll();
    refreshEntry();
  }
  async function remove() {
    if (!S.stored || S.busy || S.loadFailed || !confirm(t("sk.delete.confirm") + (isDirty() ? "\n\n" + t("sk.delete.dirty") : ""))) return;
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
  function discard() {
    if (S.busy || !isDirty() || !confirm(t("sk.discard.confirm"))) return;
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
  async function importFile(file) {
    if (!file) return;
    if (file.size > 1 << 20) { toast(t("sk.import.big"), true); return; }
    let raw;
    try { raw = JSON.parse(await file.text()); } catch (e) { toast(t("sk.import.err", { e: e.message }), true); return; }
    const bad = (m) => toast(t("sk.import.err", { e: m }), true);
    if (!raw || typeof raw !== "object" || raw.version !== 1 || !raw.case || typeof raw.case !== "object" || !raw.case.dims || !Array.isArray(raw.parts)) { bad("version, case, parts"); return; }
    const d = raw.case.dims;
    if (![d.w, d.h, d.d].every((v) => Number.isFinite(v) && v > 0 && v <= S.lim.maxDimMm)) { bad("case.dims"); return; }
    if (raw.parts.length > S.lim.maxParts) { bad(t("sk.max.parts", { n: S.lim.maxParts })); return; }
    if ((parts().length || isDirty()) && !confirm(t("sk.import.confirm"))) return;
    const { doc, skipped, fixed } = sanitizeSketch(raw);
    endTest();
    mutate(() => { S.doc = doc; S.sel = null; });
    S.vp = {}; fitView();
    renderAll();
    toast(t("sk.import.ok"));
    const notes = [skipped > 0 ? t("sk.import.skipped", { n: skipped }) : "", fixed > 0 ? t("sk.import.fixed", { n: fixed }) : ""].filter(Boolean);
    if (notes.length) setTimeout(() => toast(notes.join(" "), true), 2400);
  }
  const ID_RE = /^[a-z0-9][a-z0-9._-]{0,56}$/;
  /** Text as the server accepts it: printable characters only, at most max characters. */
  const printable = (v, max) => Array.from(v.replace(/\p{C}/gu, "").replace(/\p{Z}/gu, (m) => (m === " " ? m : " "))).slice(0, max).join("");
  /** An imported file brought to the rules the server enforces: enums, ranges, ids, text. Counts what was dropped or adjusted. */
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
    let kase;
    if (tpl) {
      kase = { source: "library", id: rc.id, dims: { w: tpl.dims.w, h: tpl.dims.h, d: tpl.dims.d }, chambers: (tpl.chambers || []).map((q) => ({ x: q.x, w: q.w })) };
    } else {
      if (rc.source === "library") fixed++; // a case this library does not know becomes a custom box of that size
      const dims = { w: r2(num(rc.dims.w)), h: r2(num(rc.dims.h)), d: r2(num(rc.dims.d)) };
      const ch = (Array.isArray(rc.chambers) ? rc.chambers : []).filter((q) => q && Number.isFinite(q.x) && Number.isFinite(q.w) && q.x >= 0 && q.w > 0 && q.x + q.w <= dims.w + 1e-9);
      if ((Array.isArray(rc.chambers) ? rc.chambers.length : 0) !== Math.min(ch.length, S.lim.maxChambers)) fixed++;
      kase = { source: "custom", id: "custom-box", dims, chambers: ch.slice(0, S.lim.maxChambers).map((q) => ({ x: q.x, w: q.w })) };
    }
    const size = (v) => (v === "front" ? { w: kase.dims.w, h: kase.dims.h } : v === "side" ? { w: kase.dims.d, h: kase.dims.h } : { w: kase.dims.w, h: kase.dims.d });
    const m = S.lim.marginMm, ids = new Set(), out = [];
    for (const q of raw.parts) {
      const part = q && typeof q === "object" ? partOf(q.part) : null;
      if (!part || typeof q.id !== "string" || !ID_RE.test(q.id) || ids.has(q.id)) { skipped++; continue; }
      ids.add(q.id);
      const view = VIEWS.includes(q.view) ? q.view : "side";
      if (q.view != null && q.view !== "" && q.view !== view) fixed++;
      const face = VIEWS.includes(q.face) && q.face !== view ? q.face : "";
      if (q.face != null && q.face !== "" && q.face !== face && q.face !== view) fixed++;
      const vs = size(view), g = gen(part), o = {
        id: q.id, part: q.part, view, x: lim(q.x, -m, vs.w + m), y: lim(q.y, -m, vs.h + m), rot: lim(q.rot, -360, 360), face,
        mirror: q.mirror === true, flow: isFan(part) ? (q.flow === "exhaust" ? "exhaust" : "intake") : "",
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
      out.push(o);
    }
    const doc = cleanDoc({ version: 1, case: kase, canvasView: raw.canvasView, parts: out });
    return { doc, skipped, fixed };
  }

  // ---------- open, close ----------
  function renderAll() {
    if (!S.open || !S.doc) return;
    renderHead(); renderViews(); renderPalette(); renderInspector(true); applyChrome();
    if (!S.vp[S.view]) fitView();
    draw();
  }
  function applyChrome() {
    $("#sk-snap").checked = S.snap; $("#sk-grid").checked = S.grid;
    $("#sk-search").value = S.search;
  }
  function openEditor() {
    S.open = true;
    $("#tab-light").classList.add("sketching");
    $("#sketch-view").hidden = false;
    document.body.classList.add("sketching");
    window.scrollTo(0, 0);
    measure();
    load();
  }
  async function closeEditor(force) {
    if (!force && isDirty() && !confirm(t("sk.leave"))) return false;
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

  // ---------- wiring ----------
  $("#sketch-open").addEventListener("click", openEditor);
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
  // leaving the lighting tab (or the language) with unsaved changes
  $("#tabs").addEventListener("click", (e) => {
    const b = e.target.closest && e.target.closest("[data-tab]");
    if (S.open && b && b.dataset.tab !== "light") {
      if (isDirty() && !confirm(t("sk.leave"))) { e.stopPropagation(); e.preventDefault(); } else endTest();
    }
  }, true);
  for (const b of $$("[data-lang]")) b.addEventListener("click", () => { refreshEntry(); if (S.open && S.doc) { $("#sk-empty").dataset.key = ""; renderAll(); } });
  refreshEntry();
})();
