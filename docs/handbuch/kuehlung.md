# Kühlung

[Handbuch](README.md) · Zurück: [Beleuchtung](beleuchtung.md) · Weiter: [Einstellungen](einstellungen.md)

> **Achtung:** Eine falsch eingestellte Kühlung kann Hardware überhitzen. Kurven und Sensoren vor dem ersten Regelbetrieb im [Testbetrieb](#regelbetrieb-und-testbetrieb) prüfen. Nutzung auf eigene Gefahr.

CaseHUD steuert TL-Lüfter (am Lüfter-Controller) und die Pumpe an einem Mainboard-Anschluss selbst; ein zusätzliches Lüfterprogramm ist nicht nötig. Der Reiter **Kühlung** ist ein Schaltplan: Sensoren, Kurven und Ausgänge werden mit Kabeln verbunden. Der Bereich lässt sich unter [Einstellungen → Bereiche](einstellungen.md#bereiche) abschalten.

Der Reiter erscheint nur, wenn ein Lüfter-Controller gefunden wurde oder eine Kühlung eingerichtet ist.

![Schaltplan der Kühlung](img/kuehlung-schaltplan.png)
<!-- BILD: Reiter Kühlung mit drei Spalten Sensoren, Kurven, Lüfter und Pumpe, verbunden durch Kabel; oben Preset-Auswahl und Status „Testbetrieb“ -->

## Regelbetrieb und Testbetrieb

| Modus | Wirkung |
|---|---|
| **Testbetrieb** (Standard) | CaseHUD rechnet nur („würde {d} %“); es gibt keine Ausgabe an Lüfter oder Pumpe. Die bisherige Lüftersteuerung bleibt aktiv. |
| **Regelbetrieb** („Aktiv“) | CaseHUD regelt Lüfter und Pumpe nach den Kurven. |

Umschalten: **Einstellungen → Bereiche → CaseHUD regelt die Lüfter** (erscheint unter **Kühlung**). Der Schalter ist aus = Testbetrieb. Die Statusanzeige im Reiter **Kühlung** zeigt „Aktiv“ oder „Testbetrieb“; „Gespeichert: Regelbetrieb, derzeit nicht wirksam“ weist auf einen noch blockierten Regelbetrieb hin.

1. Schaltplan einrichten und im Testbetrieb prüfen (Kurven zeigen „würde … %“).
2. Andere Lüfterprogramme beenden.
3. **CaseHUD regelt die Lüfter** einschalten und die Rückfrage bestätigen („Ab dem Speichern steuert CaseHUD die Lüfter …“).
4. Zurück in den Testbetrieb: Schalter ausschalten. Die Lüfter wechseln auf ihre sichere Drehzahl und behalten sie, bis ein anderes Programm sie regelt.

Regelbetrieb ist blockiert (keine Ausgabe, Meldung im Reiter), solange

- ein anderes Lüfterprogramm läuft („{Programm} läuft. Keine Ausgabe an Lüfter; CaseHUD übernimmt nach Beenden des Programms.“),
- der Haftungshinweis nicht bestätigt ist oder
- die Prüfung auf andere Lüfterprogramme nicht möglich ist.

## Schaltplan

Der Plan hat drei Spalten: **Sensoren**, **Kurven**, **Lüfter und Pumpe**. Ein Kabel von Anschluss zu Anschluss ziehen; ein Klick auf ein Kabel trennt es, ein Klick auf einen Baustein zeigt seine Einstellungen im Seitenpanel.

1. Unter **Sensoren** **Sensor hinzufügen …** wählen. Sensoren sind nach Hardware gruppiert (Prozessor, Grafikkarte, Mainboard, Arbeitsspeicher, Laufwerke, Weitere); **{Gruppe}: alle hinzufügen** nimmt eine ganze Gruppe auf.
2. **Neue Kurve** anlegen oder ein Kabel von einem Sensor in die Spalte **Kurven** ziehen („Kabel von einem Sensor hierher ziehen, um eine anzulegen“).
3. Kurve anklicken und im Seitenpanel **Art der Kurve** und Werte einstellen ([Kurvenarten](#kurvenarten)).
4. Ein Kabel von der Kurve zu einem Lüfter in der Spalte **Lüfter und Pumpe** ziehen (oder im Seitenpanel der Kurve **Lüfter anschließen …** wählen). Pro Lüfter gibt es eine Kurve; mehrere Lüfter können an einer Kurve hängen.
5. **Speichern**. **Verwerfen** nimmt ungespeicherte Änderungen zurück. **Aufräumen** ordnet den Plan.

Hinweise:

- Mehrere Sensoren an einer Kurve: der heißeste zählt („Sensoren (der heißeste zählt)“).
- Lüfter ohne Kurve werden nicht geregelt („Nicht geregelt“, „Ohne Kurve“). Beim Speichern warnt CaseHUD: Der Lüfter ist dann auf sicherer Drehzahl, ohne Regelung und ohne Überwachung.
- Lüfter am TL-Controller erscheinen als „Port {p} · Lüfter {f}“, die Pumpe als **Pumpe**. Ausgänge lassen sich benennen.
- **Speichern** ist gesperrt, solange der Plan Probleme enthält; sie stehen in einer Liste über dem Plan (z. B. „Pumpe „{n}“: mindestens 30 % erforderlich“).
- Mit **Entfernen** wird ein Baustein vom Plan genommen, mit **Vom Plan nehmen** ein Sensor.

## Kurvenarten

Jede Kurve ist ein eigenes Objekt. Die **Art der Kurve** bestimmt, wie aus der Sensortemperatur ein Wert wird. Der **Typ der Vorgabe** („Die Kurve gibt vor“) ist **Prozent** (alle Lüfter bekommen denselben Prozentwert) oder **Drehzahl** (alle Lüfter erreichen dieselbe Drehzahl; Umrechnung über die [Kalibrierung](#kalibrierung) des jeweiligen Lüfters, Kalibrierung erforderlich).

| Art | Beschreibung |
|---|---|
| **Gezeichnete Linie** | Beliebig viele Punkte, durch Geraden verbunden. Punkte ziehen, Doppelklick fügt einen hinzu (oder **Punkt hinzufügen**), Rechtsklick entfernt ihn. Achse: Temperatur in °C. |
| **Gerade Linie** | Eine gerade Linie zwischen zwei Punkten: **Unten: Temperatur** / **Unten: Wert** bis **Oben: Temperatur** / **Oben: Wert**. |
| **Abstand** | Folgt der Differenz zweier Sensoren, z. B. Kühlmittel über Raumluft; gilt unabhängig von der Jahreszeit. Erstes Kabel: Messwert, zweites Kabel: Bezug (**Bezugssensor**). Der Bezugssensor darf nicht auch gemessen werden. |
| **Fester Wert** | Konstanter **Wert**, kein Sensor nötig. |
| **Schalter** | Zwei Stufen: schaltet hoch ab **Last ab (°C)** und zurück erst bei **Leerlauf bis (°C)**; dazwischen bleibt die Stufe unverändert. Werte: **Wert im Leerlauf**, **Wert unter Last**. |
| **Thermostat** | Ermittelt automatisch den niedrigsten Wert, der die **Zieltemperatur (°C)** hält. Einstellungen: **Ruhig bis (°C)**, **Kleinster Wert**, **Größter Wert**, **Schritt**, **Spielraum unter dem Ziel (°C)**, **Reagiert nach (s)**. |
| **Kombination** | Verrechnet mehrere Kurven (**Kurven**, **Kurve hinzufügen …**) per **Rechnung**: Größter Wert, Kleinster Wert, Summe, Mittelwert, Erste minus Rest. Fällt der Sensor einer Kurve aus, wechseln die Lüfter auf die sichere Drehzahl. Prozent und Drehzahl lassen sich nicht mischen. |
| **Folgt einem Lüfter** | Wie ein anderer Lüfter (**Folgt**), optional mit **Abstand**, wahlweise als Anteil in %. Der gefolgte Lüfter muss an einer Kurve hängen. |

**Regelverhalten** (aufklappbar, bei Kurven mit Sensor): **Steigen ab (°C)**, **Fallen ab (°C)**, **Steigen nach (s)**, **Fallen nach (s)**, **Vorausschau (s)** dämpfen das Reagieren auf schnelle Temperaturänderungen; **Abstand auch an den Enden** gilt für die Art **Abstand**.

Weitere Aktionen im Seitenpanel einer Kurve: **Duplizieren**, **Kurve löschen**, **Name**.

## Lüfter und Pumpe

Im Seitenpanel eines Ausgangs:

- **Typ** (nur am Lüfter-Controller): **Lüfter**, **Gehäuselüfter (läuft immer)** oder **Pumpe**. Eine am TL-Anschluss hängende Pumpe muss als Typ **Pumpe** eingetragen sein.
- **Darf stillstehen**: Der Lüfter steht, solange die Kurve höchstens den Wert **Stopp bei (%)** vorgibt (dafür die Kurve am kalten Ende auf 0 setzen). Bei Übertemperatur oder Sensorausfall läuft er trotzdem an.
- **Steuerung**: **Kurve** oder **Manuell** ([Manueller Modus](#manueller-modus)).
- **Regelverhalten** (aufklappbar): **Minimum (%)**, **Stopp bei (%)**, **Start mit (%)**, **Sichere Drehzahl (%)**, **Schritt hoch (%/s)**, **Schritt runter (%/s)** und **Versatz (%)** (Versatz je Ausgang, auch negativ).
- Der Lüfter fährt nach Übernahme oder Neuaufbau sanft vom aktuellen Wert an, statt zu springen.

## Manueller Modus

Statt einer Kurve lässt sich jeder Ausgang auf einen festen Wert setzen.

1. Ausgang anklicken, bei **Steuerung** **Manuell** wählen.
2. **Wert** und **Einheit** (Prozent oder Drehzahl) einstellen. Drehzahl ist nur bei kalibriertem Lüfter möglich („Drehzahl nur bei kalibriertem Lüfter“).
3. **Speichern**. Der Plan zeigt „Manuell · {Wert} {Einheit}“.

Regeln: Der Wert muss über 0 liegen, in Prozent höchstens 100, und nicht unter dem **Minimum** des Ausgangs. Die Pumpe läuft auch im manuellen Modus nie unter 30 % und wird bei Stillstand alarmiert (siehe [Sicherheit](#sicherheit)).
Temperaturwächter: Ein manueller Ausgang (Pumpe oder Lüfter) wird über die Sensoren aller Kurven überwacht. Meldet einer davon drei Zyklen in Folge 100 °C, läuft der Ausgang mit 100 %, der Plan zeigt „Übertemperatur, volle Drehzahl“ und das Alarm-Banner erscheint. Wird es wieder kühl, kehrt der Ausgang schrittweise zum manuellen Wert zurück. Hat keine Kurve einen Sensor, zeigt der Plan „ohne Temperaturwächter“.

## Kalibrierung

Die Kalibrierung misst Start-, Stopp- und Drehzahlverlauf eines Lüfters und liefert Vorschläge für **Stopp bei**, **Start mit** und **Minimum**. Sie ist die Grundlage für Drehzahl-Kurven.

1. Im Seitenpanel des Lüfters **Lüfter kalibrieren** wählen (oder in der Kopfleiste **Alle Lüfter kalibrieren**).
2. Die Rückfrage lesen und bestätigen.
3. Ablauf je Lüfter:
   1. **Abwärts**: Die Drehzahl sinkt von 100 % schrittweise bis zum Stillstand.
   2. **Stillstand**: mindestens 30 s bei 0 %.
   3. **Wiederanlauf**: aufwärts in 1-%-Schritten, je Schritt mindestens 30 s.
4. Der Lauf dauert meist unter 10 Minuten, bei trägen Lüftern länger; der Lüfter steht dabei zeitweise. Die übrigen Lüfter regeln normal weiter.
5. Nach Abschluss erscheinen Messwerte („Stoppwert“, „Startwert“) und ein **Vorschlag** („Stopp bei {a} %, Start mit {b} %“ bzw. „Minimum {m} %“). **Übernehmen** setzt die Werte, **Speichern** wendet sie an. Ohne gefundenen Wiederanlauf gibt es keinen Vorschlag für Stillstand. Gehäuselüfter laufen immer; vorgeschlagen wird nur ein Minimum.
6. **Kalibrierung entfernen** verwirft die Messung.

**Alle Lüfter kalibrieren** misst nacheinander alle Lüfter und Gehäuselüfter (nicht die Pumpe), jeweils wie bei der Einzelkalibrierung. Zwischen zwei Lüftern liegt eine Abkühlpause von 30 s.

**Abbruchbedingungen und Abbrechen:**

- Bei einem Temperaturanstieg um 10 °C oder ab 80 °C wird abgebrochen.
- **Abbrechen und verwerfen** beendet den Lauf, nichts wird gespeichert. **Abbrechen, Fertige behalten** speichert die bereits fertigen Lüfter.
- Eine Kalibrierung ist nur im Regelbetrieb möglich, nur mit gespeicherten Änderungen und nur für Ausgänge mit Kurve oder manuellem Wert.

**Pumpen-Minimum kalibrieren:** Die Pumpe wird von 100 % in 5-%-Schritten abgesenkt, nie unter 30 %. Beim ersten Anzeichen eines Einbruchs der Drehzahl geht sie sofort auf 100 % zurück. Der Kühlmitteldurchfluss sinkt währenddessen; nur bei geringer Last starten. Ergebnis: „Niedrigster stabiler Wert“.

## Presets

Presets speichern die komplette Kühlungseinstellung und schalten sie um. Sie sind auch über API und MCP ansprechbar ([API & MCP](api-mcp.md)).

1. Einstellung wie gewünscht anlegen und **Speichern**.
2. In der Auswahl **Preset** **Speichern unter …** wählen und einen **Namen** vergeben.
3. Umschalten: Preset in der Auswahl wählen. Das aktive trägt „Aktiv“; abweichende Einstellungen erscheinen als „Angepasst“. Ungespeicherte Änderungen gehen beim Wechsel verloren (Rückfrage).
4. **Verwalten**: **Überschreiben**, **Umbenennen**, **Löschen**.

Höchstens 32 Presets. Ändert sich die Einstellung zwischenzeitlich (z. B. durch einen Presetwechsel von außen), meldet der Plan „Einstellungen zwischenzeitlich geändert“ und bietet **Neu laden, Änderungen verwerfen**.

## Sicherheit

### Pumpe

- Die Pumpe läuft **nie unter 30 %** und **nie aus**. Diese Grenze gilt für Kurven, den manuellen Modus, die Kalibrierung und die API.
- Steht die Pumpe, ist sie defekt: Es gibt einen **Alarm**, und alle Lüfter an Kurven wechseln auf ihre sichere Drehzahl.
- Mit Kurve oder manuellem Wert regelt CaseHUD die Pumpe. Ohne beides bleibt sie beim Mainboard und wird nur überwacht („Nur überwacht“). Endet CaseHUD oder läuft ein anderes Lüfterprogramm, regelt wieder das Mainboard.
- **Abgleich:** Vor dem Regeln beweist CaseHUD einmalig, dass Anschluss und Drehzahlsensor der Pumpe zusammengehören. Dafür hebt es den Anschluss einmal an (um 20 Prozentpunkte, ab 80 % direkt auf 100 %), wartet, bis der Drehzahlsensor folgt, und gibt den Anschluss danach an das Mainboard zurück; die Pumpe läuft dabei also kurz schneller. Läuft die Pumpe schon (fast) mit voller Drehzahl, wird sie stattdessen kurz abgesenkt, nie unter ihr Minimum und nur bei kühlem System. Das Ergebnis wird mit den Namen von Sensor, Anschluss und Mainboard gespeichert (`cooling_match.json` im Datenordner) und nur bei Änderung wiederholt.
- **Voraussetzungen:** Eine Kurve mit Temperatursensor muss vorhanden sein (sonst „Keine Temperaturquelle für die Prüfung“), kein Kalibrierlauf darf laufen und kein anderes Lüfterprogramm zuständig sein. Der Lauf endet, wenn die Temperatur 75 °C erreicht oder um 10 °C steigt oder ein Sensor ausfällt; der Anschluss geht dann sofort zurück an das Mainboard. Brachte der Lauf kein Ergebnis, versucht CaseHUD es nach 10 Minuten erneut.
- **Status in der Oberfläche:** „Nicht abgeglichen“ (steht aus), „Abgleich läuft“, nach Erfolg regelt CaseHUD die Pumpe, nach Misserfolg („Drehzahlsensor und Anschluss gehören nicht nachweislich zusammen“) behält das Mainboard die Pumpe (sie läuft weiter); **Erneut prüfen** wiederholt den Lauf (erst nach dem Speichern). Fehlt der Kanal auf dem Mainboard, gilt der Abgleich sofort als fehlgeschlagen.
- **Freigabe:** Der Mainboard-Anschluss muss einmal für CaseHUD freigegeben werden („Nicht freigegeben“, **Anschluss freigeben**). Windows fragt dafür nach Administratorrechten; CaseHUD startet sich dazu selbst mit `--allow-cooling-header`. Die Freigabe wird im Datenordner des Sensor-Dienstes (`C:\ProgramData\CaseHUD`) abgelegt. Eine neue Anfrage ist frühestens nach 30 s möglich; ist der Anschluss beim Abgleich noch nicht freigegeben, prüft CaseHUD nach 30 s erneut. Bis zur Freigabe regelt das Mainboard die Pumpe.
- Der **Regelkanal** lässt sich bei Bedarf wählen (nur Kanäle desselben Chips); eine neue Wahl wird nach dem Speichern geprüft und braucht erneut die Freigabe.
- Für die Pumpe sind **Drehzahlsensor der Pumpe**, **Minimum (%)** (mindestens 30) und **Gilt als stehend unter (RPM)** einstellbar.
- **Überhitzungsschutz (jeder Ausgang):** Erreicht der Sensor einer Kurve 100 °C drei Zyklen lang, geht der Ausgang auf 100 %, unabhängig von Kurve und Schrittgrenzen; manuelle Ausgänge werden über die Sensoren aller Kurven überwacht. Dazu erscheinen „Übertemperatur, volle Drehzahl“ am Ausgang und das Alarm-Banner.

### Alarme

Ein Alarmband („Zur Kühlung“) erscheint auf jeder Seite. Gemeldet werden u. a.:

- Pumpe läuft nicht, vermutlich defekt („Last reduzieren und Pumpe prüfen“); Pumpendrehzahl nicht lesbar
- Sensor für einen Lüfter fehlt oder liefert keine neuen Werte (Lüfter auf sicherer Drehzahl)
- Lüfter steht trotz Drehzahlvorgabe oder nimmt keine Drehzahl an
- Lüfter-Controller nicht erreichbar
- Sensordienst antwortet nicht (Dienst „CaseHUD Sensors“ in den Windows-Diensten prüfen)
- Lüfterregelung reagiert nicht oder wurde angehalten (CaseHUD neu starten)
- Einstellungen konnten nicht geladen werden; `cooling.json` beschädigt, Betrieb mit der letzten Sicherung (die defekte Datei bleibt im Datenordner)
- ein anderes Lüfterprogramm läuft, oder der Haftungshinweis ist nicht bestätigt

### Sichere Drehzahl und Wächterprozess

- Jeder geregelte Ausgang hat eine **Sichere Drehzahl (%)**, mindestens 30 %. Bei Sensorausfall, Pumpenalarm und ähnlichen Störungen wechseln die Lüfter darauf.
- Der Lüfter-Controller behält den zuletzt gesetzten Wert, wenn CaseHUD abstürzt. Deshalb startet CaseHUD im Regelbetrieb einen eigenen **Wächterprozess** (`CaseHUD-Guard`). Endet CaseHUD ohne sauberes Beenden, setzt er die Lüfter auf sichere Werte (nie niedriger als zuletzt).
- Beim Abmelden und Herunterfahren endet die Steuerung sauber.
- Eine Pumpe an einem Mainboard-Anschluss übernimmt bei Ausfall von CaseHUD das Mainboard.

### Andere Lüfterprogramme

Solange ein anderes Lüfterprogramm läuft, hält sich CaseHUD aus der Lüftersteuerung heraus und meldet „{Programm} läuft, keine Regelung durch CaseHUD“. Lüfter bleiben mindestens auf sicherer Drehzahl, bis das andere Programm übernimmt oder beendet wird. Zwei Regelungen gleichzeitig arbeiten gegeneinander; die Lüfterfunktion des anderen Programms abschalten oder das Programm beenden ([Andere Software](einstellungen.md#andere-software)).
