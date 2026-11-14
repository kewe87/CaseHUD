# Szenen

[Handbuch](README.md) · Zurück: [Displays](displays.md) · Weiter: [Beleuchtung](beleuchtung.md)

Der Reiter **Szenen** (Bereich **Display**) bündelt drei Teile: **Szenen**, **Automatik** und **Rotation**. Er setzt angeschlossene Displays voraus („Noch keine Displays angeschlossen“ sonst).

![Reiter Szenen](img/szenen-uebersicht.png)
<!-- BILD: Reiter Szenen mit Szenenliste, Automatik (Regeln) und Rotation -->

## Szenen speichern und anwenden

Eine Szene speichert die komplette Belegung der Displays. Szenen bleiben getrennt von der Rotation und sind in Automatik und Rotation verwendbar.

1. Displays wie gewünscht einstellen ([Displays](displays.md#display-einstellen)).
2. Im Reiter **Szenen** einen Namen eingeben (z. B. „Gaming“) und **Aktuelle Belegung speichern** wählen. Existiert der Name schon, fragt CaseHUD vor dem Überschreiben.
3. In der Liste je Szene:
   - **Anwenden**: übernimmt die Szene als normale Einstellung der Displays. Die bisherige Einstellung wird ersetzt und sollte vorher als Szene gespeichert werden.
   - **In Rotation**: Aus = die Szene wird in der Rotation übersprungen (nur für Automatik nutzbar).
   - **Umbenennen** und Löschen. Wird eine Szene von Regeln der Automatik verwendet, werden diese Regeln beim Löschen übersprungen. „Aktiv“ und „Automatik“ markieren die gerade gezeigte bzw. von der Automatik genutzte Szene.

**Gaming-Szene anlegen** erzeugt aus der aktuellen Einrichtung die Szene „Gaming“: FPS und GPU auf den Lüftern, Gaming-Aufteilung auf Breitbild-Displays. Sie ist nicht in der Rotation.

## Automatik

Die Automatik schaltet eine Szene, solange ein Spiel läuft, und danach zurück.

1. Schalter **Automatik einschalten** aktivieren.
2. **Regel hinzufügen**. Je Regel:
   - **Bedingung**: **ein Spiel läuft** oder **ein bestimmtes Spiel läuft** (dann **Programmname des Spiels**, z. B. `eldenring.exe`, oder **Programm wählen …**).
   - **zeige Szene**: eine gespeicherte Szene oder **Normale Einstellung (Rotation pausiert)**.
3. Regeln werden von oben nach unten geprüft; die erste zutreffende gilt. Reihenfolge mit **Nach oben** / **Nach unten**, Regel mit **Regel an/aus** deaktivieren.
4. Ohne Treffer gilt die Rotation (falls aktiv) oder die normale Einstellung je Display („Sonst:“).

Unter der Regelliste zeigt „Gerade erkannt: …“ das aktuelle Spiel; **Regel für „{Spiel}“ anlegen** erzeugt direkt eine Regel dafür.

### Spielerkennung

Spiele werden per FPS-Messung erkannt (Vordergrund, Vollbild; siehe [FPS & Latenz](sensoren.md#fps-und-latenz)). Eine eingebaute Liste normaler Programme (Browser, Terminal, Editoren, Launcher) ist ausgenommen. Eigene Ausnahmen: **Ignorierte Programme** (**Ignorieren** beim erkannten Programm).

### Verzögerung

Unter **Verzögerung** gegen Flackern bei Alt-Tab:

- **Spiel muss laufen für**: 1, 3, 5 oder 10 s
- **Nach Spielende zurück nach**: 3, 10, 30 s oder 1 min

Die FPS-Erkennung braucht zusätzlich ca. 5 s.

### Manuelle Änderung während der Automatik

Das Band „Automatik aktiv: Szene …“ zeigt die laufende Regel („Regel {n} trifft zu“). Eine manuelle Änderung während der Automatik gilt nur für dieses Display und bleibt nach dem Spiel erhalten; die Szene bleibt unverändert. Das Band bietet **Szene bearbeiten …** (sichert die bisherige Einstellung als Szene „Normal (Sicherung)“ und übernimmt die Szene zum Bearbeiten; danach unter **Szenen** unter gleichem Namen speichern) und **Automatik pausieren**.

## Rotation

Die Rotation wechselt im Leerlauf zwischen Szenen oder lässt Designs reihum wandern. Sie pausiert, solange die Automatik eine Szene zeigt.

1. Schalter **Rotation einschalten** aktivieren.
2. **Art** wählen:
   - **Szenen**: Wechsel zwischen den gespeicherten Szenen. Es müssen mindestens zwei Szenen mit **In Rotation** vorhanden sein; die Reihenfolge entspricht der Szenenliste.
   - **Reihum**: Bei jedem Wechsel rückt jedes Display um ein Design weiter. Designs unter **Designs für Reihum** mit **Design hinzufügen …** wählen.
3. **Wechsel alle**: 5 s, 10 s, 15 s, 30 s, 1 min oder 5 min.
4. **Übergang**: Überblenden, Schieben (↓ ↑ ← →), Blättern, Zoom.
5. Optional **Nacheinander wechseln**: Displays wechseln nacheinander von oben nach unten; die Reihenfolge steht unter **Reihenfolge der Displays (oben nach unten)**.
6. **Nächster Wechsel** löst einen Wechsel sofort aus.

Der Status zeigt „Läuft“, „Pausiert (Spiel aktiv)“ oder „Pausiert – Automatik zeigt „{Szene}““. Ein Chip im Reiter **Displays** („Rotation läuft“) führt zu den Szenen.
