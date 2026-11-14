# Displays

[Handbuch](README.md) · Zurück: [Erste Schritte](erste-schritte.md) · Weiter: [Szenen](szenen.md)

Der Bereich **Display** umfasst die Reiter **Displays**, **Szenen**, **Designs** und **Medien**. Er lässt sich unter [Einstellungen → Bereiche](einstellungen.md#bereiche) abschalten.

## Displays im Überblick

Der Reiter **Displays** zeigt jeden LCD-Lüfter, Pumpenkopf und jedes Case-Screen mit Live-Vorschau. Lüfter sind nach Port gruppiert („UNI FAN TL LCD · Port 2“).

![Reiter Displays mit Live-Vorschau](img/displays-uebersicht.png)
<!-- BILD: Reiter Displays mit mehreren Lüfter-LCDs (gruppiert nach Port) und einem Breitbild-Display, alle mit Live-Vorschau -->

Oben rechts im Reiter:

- **Anordnung**: Ports und Nummern der Lüfter ([Anordnung](#anordnung))
- **Alle identifizieren**: zeigt auf jedem Display seine Nummer

Leere Ansicht („Kein unterstütztes Display gefunden“): USB-Kabel der LCD-Lüfter bzw. des Pumpenkopfs prüfen, Herstellersoftware für die Displays beenden, danach oben rechts erneut suchen.

## Display einstellen

1. Im Reiter **Displays** auf ein Display klicken. Die Seitenleiste öffnet sich.
2. **Name** vergeben (z. B. „Front oben“). **Identifizieren** zeigt die Nummer auf dem Display.
3. Unter **Inhalt** **HUD**, **Medien** oder **Aus** wählen ([Inhalt](#inhalt)).
4. **Farbschema**, **Helligkeit** und **Drehung** einstellen.
5. Änderungen gelten sofort. Mit **Auf alle {n} Displays übernehmen** wird die Einstellung auf alle Displays übertragen (Design, Datei oder Aus).

![Seitenleiste eines Displays](img/display-seitenleiste.png)
<!-- BILD: Seitenleiste eines Lüfter-LCDs mit Name, Inhalt, Design-Auswahl, Farbschema, Helligkeit, Drehung -->

### Inhalt

| Wahl | Wirkung |
|---|---|
| **HUD** | Ein Sensor-Design (mitgeliefert oder eigen), optional mit animiertem Hintergrund und Bloom |
| **Medien** | Ein Bild, GIF oder Video aus dem Reiter **Medien** |
| **Aus** | Display schwarz |

Weitere Felder je nach Gerät:

- **Farbschema**: **Dunkel** oder **Hell**. Die Farben werden für helle Hintergründe automatisch angepasst (ideal für weiße Builds).
- **Hintergrund**: Keiner, Drehen, Atmen, Radar, Orbit, Aurora, Synthwave, Digital Rain, CRT. „★ Passend zum Design“ markiert den empfohlenen. **Stärke** regelt die Intensität.
- **Bloom**: Weichzeichnung mit Leuchten heller Bereiche; beruhigt unruhige GIFs.
- **Drehung**: Bildausrichtung. Bei hochkant montierten Breitbild-Displays gilt 90° = rechte Kante oben, 270° = linke Kante oben; die Felder werden übereinander angeordnet.
- **Drehung im Gerät (Test)**: Das Display dreht das Bild bei 180° selbst (spart CPU). Wird im Gerät gespeichert und nur bei Änderung gesendet.
- **Uhr anzeigen, wenn CaseHUD nicht läuft**: Das Display zeigt seine eigene Uhr; die Einstellung wird im Display gespeichert.
- **Firmware**: zeigt die Version. Updates gibt es beim Hersteller; CaseHUD installiert keine Firmware. Eine optionale Prüfung auf neuere Versionen steht unter [Einstellungen → Allgemein](einstellungen.md#allgemein).

Steuert die Rotation oder die Automatik gerade die Displays, ist das Farbschema gespeichert und gilt danach.

### Lüfter-Set

**Stil für alle Lüfter** (je Port-Gruppe) setzt CPU, GPU und Kühlmittel auf drei LCD-Lüfter im gleichen Look. Ohne Kühlmittelsensor steht statt des Kühlmittels die Gesamtleistung; Bar-Displays erhalten dann vier Quadrate ohne Kühlmittel.

## Anordnung

Legt fest, welcher Lüfter welche Nummer trägt. Die Nummern bestimmen die Reihenfolge in CaseHUD und gelten nur dort; ins Gerät wird nichts geschrieben.

1. **Anordnung** im Reiter **Displays** wählen (oder **Einstellungen → Allgemein → Anordnung öffnen**).
2. Einen Lüfter auswählen: seine Nummer erscheint auf dem Display.
3. Neue Nummer wählen. Ist sie vergeben, tauschen beide Lüfter.
4. **Zurück zu den Displays**. **Nummern zurücksetzen** stellt die Standardnummern wieder her.

![Anordnung der Lüfter am Controller](img/displays-anordnung.png)
<!-- BILD: Ansicht Anordnung mit Controller, Ports 1 bis 4 und den Lüftern je Port -->

Grenzen und Hinweise:

- Controller direkt an einen USB-Anschluss des Mainboards anschließen, nicht über Hub oder Verteiler.
- Port 1: nur Lüfter ohne Display. Lüfter mit Display an Port 2 bis 4.
- Höchstens 3 Lüfter mit Display pro Port und 13 insgesamt, bis zu 10 Lüfter pro Port.
- **Lüfter ohne Display suchen** liest die Lüfterliste einmalig vom Controller (nur lesend); andere Programme, die den Controller nutzen, können kurz gestört werden.
- **Identifizieren** lässt die Lüfter kurz leuchten oder blinken: kurzer Schreibzugriff auf den Controller, keine dauerhafte Änderung.

## Breitbild-Displays und Aufteilung

Das Universal Screen 8.8" (1920×480) lässt sich in Felder aufteilen. Jedes Feld ist ein virtuelles Display mit eigenem Design, Farbschema und Hintergrund.

1. Display im Reiter **Displays** öffnen.
2. Unter **Anzeige** **Einzelnes Design** oder ein **Layout** wählen. Alternativ unter **Fertige Zusammenstellungen** eine Vorlage wählen: **Übersicht** (vier Quadrate), **Gaming**, **Wasserkühlung** (nur mit Kühlmittelsensor), **Uhr + Werte**. Ohne Kühlmittelsensor ersetzen die Vorlagen das Kühlmittel durch Leistung bzw. CPU-Takt.
3. Mit **Aufteilung wählen** eine Aufteilung setzen: 4 Quadrate, 3 Felder, 2 Felder, 2 groß + 4 klein, 1 groß + 6 klein, 8 klein.
4. Felder anklicken („Felder · Klick zum Bearbeiten“) und je Feld ein Design wählen. **Alle Designs zeigen** blendet auch Designs anderer Form ein; sie werden eingepasst. **Wie Display** übernimmt das Farbschema des Displays.
5. **Als Layout speichern** legt die Aufteilung unter **Designs → Layouts** ab. **Aufteilung auf alle {n} Breitbild-Displays übernehmen** überträgt sie auf alle Breitbild-Displays (nicht auf Lüfter).

Eigene Layouts: **Designs → Layouts → Neues Layout**. Änderungen an einem Layout gelten für alle Displays, die es nutzen. Wird ein Layout gelöscht, behalten die Displays die Felder als eigene Aufteilung.

## Pixeluhren

Am Ende des Reiters **Displays**: Gruppe **Pixeluhren**. CaseHUD zeigt CPU, GPU, Kühlmittel (falls vorhanden) oder FPS als Apps auf AWTRIX-Uhren im Netzwerk. Unterstützt: Ulanzi TC001 (AWTRIX 3 oder AWTRIX NG) und TC002 (AWTRIX NG). Das Modell wird erkannt, Icons und Schrift werden angepasst. Die Apps werden beim Beenden von CaseHUD entfernt.

1. **Uhren suchen** wählen (UDP 4210/4211 und HTTP im lokalen Netz, nur auf Klick).
2. Gefundene Uhr **Hinzufügen**.
3. Wird keine Uhr gefunden: Netzwerk und Stromversorgung prüfen oder die Adresse unter **Adresse eingeben** (IP oder Hostname, z. B. `awtrix_1a2b3c.local`) eintragen und **Prüfen & hinzufügen** wählen.
4. Je Uhr die Apps aktivieren: CPU, GPU, Last CPU + GPU, Kühlmittel (falls vorhanden), FPS, PC-Leistung.

## Designs

Der Reiter **Designs** enthält über 30 mitgelieferte Designs in vier Sets und alle eigenen Designs.

| Set | Charakter |
|---|---|
| **Clean** | Reduziert |
| **Neon** | Futuristisches HUD mit Segmenten und Leuchteffekt |
| **Retro** | Bernstein-Röhrenmonitor mit LED-Balken und Scanlines |
| **Cyberpunk** | Gelbes Terminal-HUD mit Glitch-Effekt, passend zu Digital Rain |

Inhalte der Sets: CPU + GPU (Temperatur oder Last), Gaming mit FPS und Latenz, Kühlmittelkreislauf, Leistung in Watt, Gesamtleistung, VRAM, GPU-Hotspot, Uhr, Icons, Dashboards und Breitbild-Designs. **Vorschau:** schaltet die Galerie zwischen **Dunkel** und **Hell**.

![Designs in Dunkel und Hell](img/designs-galerie.png)
<!-- BILD: Reiter Designs mit Sets, Vorschau dunkel/hell und den Schaltflächen Community, Importieren, Neues Design -->

### Design bearbeiten

Mitgelieferte Designs lassen sich mit **Anpassen** als eigene Kopie („– angepasst“) bearbeiten; ein neues Design entsteht mit **Neues Design**.

1. Im Editor Widgets über die Leiste unten hinzufügen: **Ring**, **Wert**, **Balken**, **Diagramm**, **Text**, **Uhr**.
2. Widget in der Vorschau auswählen und ziehen. Unter **Widget** Position (X, Y, Breite, Höhe), **Beschriftung**, **Farbe**, **Sensor** und weitere Felder setzen. **Ebenen** ordnet die Reihenfolge.
3. Unter **Design** Hintergrund, Akzent, Text, Beschriftung und Kachelfarbe festlegen. Die Option **Scanlines (Röhren-Look)** erzeugt den Röhren-Look.
4. **Speichern**. **Rückgängig** mit Strg+Z, **Exportieren** speichert das Design als JSON.

Hinweise:

- Designs werden in 480×480 gezeichnet und auf jedes Display skaliert. **Runde Vorschau** zeigt den Ausschnitt runder Displays.
- Sensorfelder: aus der Liste wählen oder Pfad eingeben, mehrere Alternativen mit `|` trennen ([Sensoren](sensoren.md)).
- **Warnung ab** färbt gelb, **Kritisch ab** rot (0 = aus). **Niedrig ist schlecht** kehrt dies um (z. B. FPS, Pumpe).
- **Zweiter Wert** zeigt unter dem Hauptwert z. B. eine Temperatur.
- Ringe: **Ringstärke**, **Leuchteffekt**, **Segmente**, **Skalenstriche**, **Farbe fließend nach Wert**.
- Uhr: **Format** `15:04` = Uhrzeit, `15:04:05` mit Sekunden, `Monday, 02.01.` = Wochentag + Datum.

### Teilen und Community

- **Exportieren** / **Importieren**: Designs als JSON weitergeben, auch von einer KI geschriebene Designs. Ein importiertes Design wird erst mit **Speichern** übernommen.
- **Community** öffnet die Galerie mit geteilten Designs; **Importieren** legt ein Design unter **Eigene** ab. Eigene Designs lassen sich per Pull Request im Repository [CaseHUD-Library](https://github.com/kewe87/CaseHUD-Library) einreichen. Die Galerie wird nur auf Klick geladen.

### Layouts

Die Ansicht **Layouts** (Umschalter oben im Reiter **Designs**) listet **Mitgelieferte Layouts** und **Eigene Layouts** mit Anzahl der Displays, die sie nutzen. Siehe [Breitbild-Displays und Aufteilung](#breitbild-displays-und-aufteilung).

## Medien

Der Reiter **Medien** verwaltet Bilder, GIFs und Videos für die Displays.

1. **Hinzufügen** wählen oder Dateien in die Fläche ziehen („Bilder, GIFs oder Videos hierher ziehen“). Format: PNG, JPG, GIF, MP4; der Zuschnitt erfolgt automatisch.
2. Datei einem Display zuweisen: Seitenleiste des Displays → Inhalt **Medien** → Datei wählen. Medien lassen sich auch als Hintergrund eines Designs verwenden.

Videos benötigen **ffmpeg** (siehe [Voraussetzungen](erste-schritte.md#voraussetzungen)). Mit der Option **Video im Display abspielen (spart CPU/USB)** unter [Einstellungen → Anzeige](einstellungen.md#anzeige) spielen Balkendisplays Videos selbst ab; ohne ffmpeg werden Einzelbilder gesendet.

## Startbild und Ruhebild

Die Geräte-Schreibvorgänge sind standardmäßig aus und unter [Einstellungen → Geräte](einstellungen.md#geräte) einschaltbar:

- **CaseHUD-Logo als Startbild**: schreibt einmalig das Logo (oder ein Neutralbild) als Start- und Ruhebild **dauerhaft** in jede UNI FAN TL LCD; nur begrenzt oft möglich. Angezeigt wird es, wenn CaseHUD nicht läuft.
- Der Universal Screen 8.8" zeigt beim Beenden von CaseHUD das CaseHUD-Logo (normales Bild, es wird nichts gespeichert).
- **Seriennummer für neue Displays vergeben**: nur für Displays ohne Seriennummer, einmalig und dauerhaft. Vorhandene Seriennummern werden nie geändert.

## Monitor-Modus

Hängt der Universal Screen 8.8" im Monitor-Modus (z. B. nach der Herstellersoftware), zeigt der Reiter **Displays** das Band „Universal Screen 8.8" im Monitor-Modus“. **Zurückholen** startet das Display neu; es erscheint danach wieder als CaseHUD-Display.

## Unterstützte Geräte

Die Liste der unterstützten und getesteten Geräte steht in der [README](../../README.md#unterstützte-geräte). Experimentelle Geräte werden erst nach Freigabe unter [Einstellungen → Geräte](einstellungen.md#geräte) angesprochen; die Treiber beschreibt [DRIVERS.md](../DRIVERS.md).
