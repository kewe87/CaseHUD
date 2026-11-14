# Beleuchtung

[Handbuch](README.md) · Zurück: [Szenen](szenen.md) · Weiter: [Kühlung](kuehlung.md)

Der Reiter **Beleuchtung** steuert Gehäuse-, RAM- und Lüfterbeleuchtung mit einem gemeinsamen Effekt. Der Bereich ist optional und lässt sich unter [Einstellungen → Bereiche](einstellungen.md#bereiche) abschalten. CaseHUD sendet nichts an Beleuchtungsgeräte, bis die Beleuchtung eingeschaltet ist.

Unterstützte Geräte:

- UNI FAN TL Controller (Lüfter-LEDs)
- ARGB-Anschlüsse und Onboard-Licht von ASUS-Mainboards
- RAM-Riegel (nur über die optionale SMBus-Brücke des Sensor-Dienstes, siehe [SMBUS-BRIDGE.md](../SMBUS-BRIDGE.md))
- LED-Rahmen des Universal Screen 8.8"

![Reiter Beleuchtung](img/beleuchtung-uebersicht.png)
<!-- BILD: Reiter Beleuchtung mit aufgeklapptem Gehäuse und Effekt-Animation, rechts Effekt-Einstellungen, darunter „Woher kommt die Farbe?“ -->

## Beleuchtung einschalten

1. Im Reiter **Beleuchtung** den Schalter **Beleuchtung steuern** aktivieren. Aus bedeutet: kein Gerätezugriff durch CaseHUD.
2. Beim ersten Mal erscheint eine einmalige Bestätigung („Beleuchtung“): CaseHUD steuert dann die Beleuchtung der erkannten Geräte (z. B. Mainboard, Lüfter-Controller, RAM). Je nach Einstellung „Mit Mainboard synchronisieren“ können Beleuchtungseinstellungen auch nach dem Beenden von CaseHUD aktiv bleiben. **Verstanden – einschalten** bestätigt.
3. Der Status zeigt „Aktiv – {n} Geräte gefunden“. **Neu suchen** sucht die Geräte erneut.
4. Erscheint „Keine Beleuchtungs-Hardware freigegeben“, die gefundenen Geräte unter [Weitere Einstellungen](#weitere-einstellungen) aktivieren (z. B. TL-Lüfter-Beleuchtung).

Steuert ein anderes Programm dasselbe Gerät, steht am Gerät „Belegt“ mit „Wird von {Programm} gesteuert“. **Trotzdem ansteuern** ist möglich, kann aber flackern.

## Aufgeklapptes Gehäuse

Das Gehäuse wird aufgeklappt gezeigt: Seitenansicht in der Mitte, Deckel oben, Boden unten, Rückseite und Front daneben. Der aktive Effekt läuft als Animation über alle Flächen; jedes Teil zeigt die Farbe an seiner Stelle, auch bei ausgeschalteter Beleuchtung (Simulation).

- **Live**: Farben, die die Geräte gerade bekommen. **Vorschau**: berechnete Vorschau des Effekts, die Geräte bekommen gerade nichts. Weitere Zustände: **Beleuchtung aus**, **Pausiert** (Displays aus), **Keine Live-Daten**.
- Legende: **verbunden** und **nicht verbunden**. Nicht verbundene Teile sind markiert; „{n} Teile nicht verbunden – Teil anklicken“ führt zu [Teile verbinden](#teile-verbinden).
- Bildschirm- und Audio-Effekte entstehen erst beim Abspielen und werden nicht vorab berechnet („Keine Vorschau“).
- Ohne Skizze zeigt die Fläche „Noch keine Skizze“ mit **Erkannte Geräte übernehmen** und **Skizze zeichnen**.

## Effekt wählen

Rechts neben dem Gehäuse:

1. **Effekt** wählen. Verfügbar sind: Regenbogen, Einfarbig, Atmen, Farbwelle, Temperatur, Farbwechsel, Meteor, Umlauf, Füllen, Füllen und verblassen, Pendel, Springen, Stapeln, Funkeln, Streifen, Sanduhr, Funke, Treffen und mischen, Regenbogen mit Lücken, Bildschirm-Ambilight, Audio-Visualizer.
2. **Helligkeit** einstellen.
3. Unter **Einstellungen des Effekts** Parameter setzen (z. B. Geschwindigkeit, Breite, Farbe, Farben 1–6, Richtung, Sensor und Werte für niedrige und hohe Farbe beim Effekt **Temperatur**). Die Parameter hängen vom Effekt ab.
4. Unter [Weitere Einstellungen](#weitere-einstellungen) festlegen, was **Beim Beenden** geschieht.

## Woher kommt die Farbe?

Dieser Abschnitt zeigt, aus welcher Quelle jedes Gerät seine Farben bezieht. Jedes Gerät hat genau eine Quelle, verbunden per Kabel mit seinen Zonen. Ein Klick auf das Gerät öffnet das Seitenpanel; dort werden Quelle, Zonen und Einstellungen gewählt.

| Quelle | Bedeutung |
|---|---|
| **CaseHUD-Effekt** | Farbe aus dem Effekt, je LED nach ihrer Stelle im Gehäuse |
| **Mainboard** | Folgt dem ARGB-Anschluss des Mainboards; bleibt nach dem Beenden bestehen |
| **Eigener Effekt** | Effekt, den der Controller selbst erzeugt |
| **Aus** | Gerät bleibt dunkel; Effekt und Einstellungen bleiben gespeichert |
| **Nicht angesteuert** | CaseHUD sendet nichts an dieses Gerät |

![Woher kommt die Farbe?](img/beleuchtung-quellen.png)
<!-- BILD: Abschnitt „Woher kommt die Farbe?“ mit Geräten, Quellen und Kabeln zu den Zonen, Seitenpanel mit Gerätedetails geöffnet -->

Das Seitenpanel (**Details zu {Gerät}**) enthält **Quelle**, **Zonen** (mit Teilen je Zone, „kein Teil zugeordnet“ oder „LED-Anzahl fehlt“), **Weitere Geräteeinstellungen** und **Lage auf der Effektfläche** (jeder Punkt ist eine LED, wie der Effekt sie sieht). Eine Zone ohne Teil folgt dem Effekt, liegt aber ohne eigenes Layout als Linie in der Mitte der Effektfläche.

Hinweise zu Quellen:

- **Sync-Anschluss am Mainboard**: Der Lüfter-Controller gibt das Signal des ARGB-Anschlusses parallel an alle Ports weiter. Die Auswahl legt diesen Anschluss im Layout mit der Lüfteranzahl an (je 26 LEDs, in Kettenreihenfolge). Ist die Lüfteranzahl unbekannt, zuerst die Lüfter einlesen (Displays → Anordnung).
- Der Lüfter-Controller speichert jede Beleuchtungsänderung dauerhaft im Flash-Speicher; häufige Änderungen verschleißen ihn. Empfohlen: Quelle **Mainboard**. Wechselt ein TL-Controller in einen Modus, der jede Änderung im Controller speichert, erscheint ein Hinweis („Controller-Speicher“, **Verstanden – umstellen**); Änderungen werden gebündelt gesendet.
  Beim TL-Controller betrifft das die Quellen **CaseHUD-Effekt** (eine Farbe je LED-Ring, höchstens alle 30 s gesendet) und **Eigener Effekt** (Effekt aus dem Controller); **Mainboard** schreibt nichts in den Controller-Speicher.
- Das Ausschalten eines Controller-Geräts wird im Controller gespeichert („Gerät bleibt dunkel, auch nach einem Neustart“).
- **Ports** (TL-Controller, Quelle **Eigener Effekt**): Jeder Port des Controllers kann einen eigenen Effekt und eine eigene Richtung haben. Je Port eine Zeile, z. B. „Port 1 · Seite 1–3“, mit Effekt und Richtung; **wie alle Ports** übernimmt die Einstellung des Geräts. Mit **Farben und Richtung aus CaseHUD** kommen Farben (bis zu vier) und Tempo aus dem CaseHUD-Effekt und die Richtung je Port aus der Gehäuseskizze: Läuft der Effekt über die Lüfter eines Ports nach unten, bekommt der Port „runter“. Läuft er in keiner Richtung über sie, leuchten sie als Ganzes (Atmen). Unter jeder Zeile steht, was der Port gerade nutzt und woher der Wert kommt. Eigene Werte je Port haben Vorrang. Gesendet wird nur an die Ports, deren Licht sich tatsächlich ändert; jede Änderung wird im Controller gespeichert. Nicht verfügbar bei **Hardware-Effekt über alle Lüfter**.

## Teile verbinden

Ein Teil leuchtet nur, wenn es mit einer LED-Zone eines Geräts verbunden ist.

1. Im aufgeklappten Gehäuse ein nicht verbundenes Teil anklicken (oder den Chip „{n} Teile nicht verbunden“).
2. Im Dialog **Verbinden mit** die Zone wählen. Zonen, die für das Teil zu kurz sind, sind mit „zu kurz, höchstens {m} LEDs“ gekennzeichnet. **– nicht verbunden –** löst die Verbindung.
3. **Im Editor öffnen** springt zum Teil in der [Skizze](#skizze-und-editor).

### Ausrichtung prüfen

Für jedes verbundene Teil prüft **Ausrichtung prüfen**, ob es so eingebaut ist, wie die Skizze es zeigt. Erreichbar im Dialog **Verbinden mit**, in der Teileliste unter einer Zone im Geräte-Panel, im Editor unter **LED testen** und nach dem automatischen Zuordnen als Schritt für alle zugeordneten Teile.

- Am Teil und in der Vorschau erscheint für höchstens 60 s ein Prüfmuster. Im Gerät wird nichts gespeichert.
- Lüfter und Ringe: oben Rot, rechts Grün, unten Blau, links Gelb, LED 1 weiß. Streifen: LED 1 grün, letzte LED rot, dazwischen ein Verlauf.
- Weicht das Teil von der Vorschau ab, die passende Antwort wählen: **Um 90°/180°/270° gedreht** (im Uhrzeigersinn, von vorn gesehen), **Gespiegelt**, beim Streifen **Anfang am anderen Ende**. Drehung, Spiegelung bzw. Laufrichtung in der Skizze werden korrigiert und das Muster erscheint erneut, bis **Passt** gewählt ist.
- Parallel angeschlossene Teile (z. B. weitere Lüfter im Sync-Modus des Lüfter-Controllers) zeigen die Farben ihres Vorbilds und lassen sich nicht einzeln korrigieren. Alternative: Controller-Modus mit direkter Port-Ansteuerung, dann nur 2 Farben je Lüfter; die Einstellung wird in den Controller-Speicher geschrieben.
- Ein Teil ohne Verbindung hat keine Prüfung.

Schneller: **Erkannte Geräte übernehmen** legt zu den erkannten Geräten passende Teile an. Der Dialog zeigt den Vorschlag; die Positionen lassen sich danach in der Skizze ändern. Fehlt ein passendes Teil im Katalog, steht ein Hinweis unter **Hinweise** (z. B. „Kein Mainboard mit {n} LEDs im Katalog“).

## Zonen und LED-Anzahl

ARGB-Anschlüsse des Mainboards melden die LED-Anzahl nicht; ohne Einstellung leuchtet dort nichts („Keine LED-Anzahl gesetzt“). Je Anschluss gibt es zwei Wege:

- **Bauteile wählen**: Bauteile in der Reihenfolge am Anschluss hinzufügen; die Summe ihrer LEDs ergibt die LED-Anzahl. **parallel geschaltet (Splitter)**: Alle Bauteile hängen am selben Signal, das längste zählt. Fehlt ein Bauteil, unter **Vorlagen** die Library aktualisieren.
- **LED-Anzahl ermitteln**: Zahl erhöhen (**Eine mehr**, **Zehn mehr**), bis die letzte LED am Kettenende gerade leuchtet. Die rote LED ist die gewählte, davor leuchten zwei blaue; sie erscheint nur, wenn die LED existiert. Nur Vorschau, im Gerät wird nichts gespeichert. Am Ende **Übernehmen**.
- Bei Zonen mit **Kalibrieren** zeigt ein Testlicht die LEDs nacheinander; die rote LED wandert mit, erlischt sie, ist das Ende erreicht. Das Testlicht schreibt nur kurz an den Controller.

**Vorlage anwenden**: Fertige Einstellungen für bekannte Hardware setzen LED-Anzahl und Layout der gelisteten Zonen; Effekt und Helligkeit bleiben unverändert. Bestehende Layouts dieser Zonen werden ersetzt (Rückfrage). Sieht die Vorlage einen Sync-Anschluss vor, folgt der Lüfter-Controller danach diesem Mainboard-Anschluss; die Vorlage nennt ihn.
Die Karte **Vorlage anwenden: {Name}** erscheint bei aktiver Beleuchtung unter den Knöpfen von **Woher kommt die Farbe?** und über den Geräten, sobald eine Vorlage zur erkannten Hardware passt.

## Skizze und Editor

Der Editor („Gehäuse-Skizze“) zeichnet das Gehäuse und setzt Teile maßstabsgetreu hinein. Die Effekte folgen dann den echten Positionen der LEDs.

![Editor der Gehäuse-Skizze](img/beleuchtung-editor.png)
<!-- BILD: Gehäuse-Skizze-Editor mit Bauteile-Liste links, Zeichenfläche mit Reiter je Fläche in der Mitte, Eigenschaften rechts -->

1. Im Reiter **Beleuchtung** **Skizze öffnen** (bzw. **Skizze bearbeiten**) wählen.
2. Rechts unter **Gehäuse** das Gehäuse wählen ([Gehäusevorlagen](#gehäusevorlagen)) und Maße einstellen.
3. Links unter **Bauteile** ein Teil auf das Gehäuse ziehen oder anklicken. Suche: „Hersteller, Modell, Kategorie …“. Unter **Allgemein** stehen Lüfter 120/140 mm (LED-Ring), RAM-Riegel, LED-Streifen, Grafikkarte (LED-Rahmen), Pumpe (runde LED-Fläche) und Verteilerplatte (Distroplate, LED-Rahmen); weitere Teile kommen aus der Community-Library. Kategorien u. a.: Lüfter, Lüfter mit Display, Arbeitsspeicher, LED-Streifen, Grafikkarte, Pumpe / Kühler, Verteilerplatte (Distroplate), CPU-Kühler, Display, Mainboard, Controller.
4. Oberhalb der Zeichenfläche die Ansicht wählen: ein Reiter je Gehäusefläche (**Seitenansicht**, **Deckel**, **Boden**, **Front**, **Rückseite**, **Trägerseite**), der nur die Teile dieser Fläche zeigt. Die Trägerseite ist die Rückseite des Mainboard-Trägers, gegenüber der Seitenansicht.
5. Teil auswählen und unter **Eigenschaften** einstellen (siehe unten).
6. **Speichern**. Erst damit wird die Skizze übernommen. **Verwerfen** nimmt alle ungespeicherten Änderungen zurück; „Nicht gespeichert“ zeigt offene Änderungen an.

Eigenschaften eines Teils:

- **Name**, **Fläche** (auf welcher Fläche des aufgeklappten Gehäuses das Teil liegt und die Farbe des Effekts aufnimmt)
- **Position (Mitte, mm)**: x ab Seitenansicht, y ab Deckel, z ab Rückseite; Abstand zu den Wänden wird mitgezeigt. **Schnellausrichtung**: Senkrecht, Waagerecht, Am Boden.
- **Drehung** (**90° drehen** oder Grad), **Spiegeln** (kehrt die Richtung der LED-Reihenfolge um), **LED-Seite zeigt nach**, bei Lüftern **Luftstrom** (Ansaugen / Ausblasen; Ausblasen dreht die LED-Reihenfolge des Rings)
- **LED-Anzahl** und **Länge (mm)** bei Teilen mit variabler Größe
- **Anschluss**: **Gerät** und **Zone**, **Reihenfolge in der Kette**, **Parallel zum Vorgänger** (z. B. über einen Splitter). Gebundene Teile nehmen die Farbe der Zone an; ungebundene leuchten nicht.
- **LED testen**: **Testlicht** lässt die LED {n} des Teils kurz leuchten (nichts wird dauerhaft geändert). Das Teil muss vorher gebunden und die Skizze gespeichert sein.
- **Ebene**, **Sichtbar** (ausgeblendete Teile bleiben in der Skizze und leuchten weiter), **Montiert auf** (das Teil bewegt sich mit seinem Träger)

Bedienung der Zeichenfläche: Rad zoomt, Leertaste + Ziehen verschiebt, Umschalt + Klick oder Rahmen wählt mehrere Teile, Strg+G verschweißt zu einer Gruppe (Strg+Umschalt+G löst sie), Alt + Klick wählt ein einzelnes Teil, Alt beim Ziehen übergeht die Kollisionsprüfung, Pfeiltasten verschieben um 1 mm (Umschalt: 10 mm), Bild ↑/↓ ändert die Tiefe, R dreht, Entf löscht. **Raster fangen** und **Raster** sind schaltbar; **Einpassen** zeigt alles. **Rückgängig** Strg+Z, **Wiederholen** Strg+Y.

Der Bereich **Hinweise** warnt z. B. vor überlappenden Teilen, Teilen außerhalb des Gehäuses, ungebundenen Teilen („leuchtet nicht“) und zu vielen LEDs in einer Zone.

**Importieren** / **Exportieren** tauschen die Skizze als JSON (höchstens 1 MiB; Skizzen im alten Format Version 1 lassen sich nicht importieren). Importierte Skizzen werden mit **Speichern** übernommen. **Skizze löschen** entfernt die Skizze; gebundene Zonen fallen auf ihr Standard-Layout zurück.

## Gehäusevorlagen

Gehäuse und Hardware lassen sich aus Vorlagen übernehmen:

- **Gehäuse aus der Bibliothek**: In der Skizze unter **Gehäuse** → **Gehäuse wählen** → **Aus der Bibliothek** ein Modell wählen. Die Maße kommen dann aus der Bibliothek. **Eigene Box** erlaubt freie **Außenmaße (mm)** (Breite, Höhe, Tiefe), optional mit **Kammertrennung** und Angabe, wo die **Seitenansicht** liegt (links/rechts, von vorne gesehen).
- **Vorlage anwenden**: Fertige Einstellungen für bekannte Hardware, siehe [Zonen und LED-Anzahl](#zonen-und-led-anzahl).

Die Bibliothek wird mit **Library aktualisieren** (unter [Effekt-Vorlagen](#effekt-vorlagen)) von GitHub geladen. Offline gelten die eingebauten Bauteile.

## Effekt-Vorlagen

Im aufklappbaren Bereich **Vorlagen** liegen Effekt-Vorlagen aus der Community-Bibliothek und eigene.

1. **Vorlagen** öffnen und eine Vorlage anwenden: Sie setzt Effekt und Einstellungen; Geräte und Layouts bleiben unverändert. Kennt der Effekt Einstellungen der Vorlage nicht, werden sie ausgelassen.
2. Eigene Vorlage: Namen eingeben und **Als Vorlage speichern**. Eigene Vorlagen lassen sich löschen; Bibliotheksvorlagen sind mit **Bibliothek** gekennzeichnet, eigene mit **Eigene**.
3. **Library aktualisieren** lädt Community-Vorlagen und -Bauteile von GitHub (nur öffentliche Dateien, nichts wird gesendet). Die Zeile darunter zeigt Stand, Anzahl Bauteile und Vorlagen.

Vorlagen sind auch über die API ansprechbar ([API & MCP](api-mcp.md)).

## Bildschirm-Ambilight und Audio-Visualizer

Zwei optionale Effekte leiten die LED-Farben aus dem Bildschirminhalt (**Bildschirm-Ambilight**) bzw. aus dem Ton des PCs (**Audio-Visualizer**) ab. Weil das private Daten sind, gibt es sie erst nach einer einmaligen Zustimmung.

1. Effekt wählen. Es erscheint die einmalige Bestätigung „Bildschirm und Audio“. **Verstanden – erlauben** stimmt zu.
2. Bilder und Ton werden nur im Arbeitsspeicher verarbeitet, nie gespeichert, protokolliert, exportiert oder über das Netzwerk gesendet; gelesen wird nur, solange der Effekt läuft. Bei diesen Effekten nimmt die Diagnose keinen Datenverkehr der Beleuchtungsgeräte auf.
3. Zurücknehmen: **Einstellungen → Info & Hilfe → Bildschirm- und Audio-Effekte erlauben** ausschalten. Ein laufender Effekt wird auf den Standardeffekt zurückgestellt.

Einstellungen **Bildschirm-Ambilight**: **Bildschirm** (0 = Hauptbildschirm), **Bereich** (**Ränder (Ambilight)** oder **Ganzes Bild**), **Sättigung (%)**, **Aufnahmerate (Bilder/s)**. Einstellungen **Audio-Visualizer**: **Modus** (**Spektrum** oder **Puls**), **Bänder**, **Empfindlichkeit (%)**, **Abklingen**, **Regenbogenfarben**.

## Weitere Einstellungen

Aufklappbarer Bereich **Weitere Einstellungen** unten im Reiter:

- **Beim Beenden**: **Aus**, **Letzte Farben halten** oder **Eigener Effekt der Geräte**. Gilt beim Beenden von CaseHUD und beim Ausschalten der Beleuchtung. Bei „Eigener Effekt der Geräte“ schalten Geräte ohne eigenen Effekt aus.
- **TL-Lüfter-Beleuchtung (experimentell)**: Steuert den TL-Lüfter-Controller. Im Standardmodus „Mit Mainboard synchronisieren“ sendet CaseHUD beim Start einen einzelnen Befehl; danach folgen die Lüfter dem Mainboard. Nötig ist dafür das Sync-Kabel vom ARGB-Anschluss des Mainboards zum Controller; ohne Kabel einen anderen Modus wählen. Die Modi „CaseHUD-Effekte“ und „Effekte des Controllers“ speichern jede Änderung im Flash-Speicher des Controllers und erfordern eine Bestätigung. „Blinken“ und das Testlicht schreiben nur kurz an den Controller. Noch nicht auf allen Systemen getestet.

## RAM-Beleuchtung

RAM-Beleuchtung braucht die SMBus-Brücke im Sensor-Dienst. Sie ist standardmäßig aus und wird per Datei `C:\ProgramData\CaseHUD\sensors.json` mit `"smbusBridge": true` eingeschaltet. Einzelheiten und Sicherheitsregeln: [SMBUS-BRIDGE.md](../SMBUS-BRIDGE.md).
