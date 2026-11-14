# Erste Schritte

[Handbuch](README.md) · Weiter: [Displays](displays.md)

## Voraussetzungen

- Windows 10 oder 11, x64 (nur Windows)
- WebView2-Runtime (bei Windows 11 enthalten)
- Für Videos auf den Displays: **ffmpeg** (`winget install ffmpeg` oder `ffmpeg.exe` neben `CaseHUD.exe` legen). Bilder und GIFs funktionieren ohne.

Die Programme sind noch nicht code-signiert; Windows SmartScreen kann warnen. Prüfsummen und Signatur: [SECURITY.md](../../SECURITY.md).

## Installation

1. Unter [Releases](https://github.com/kewe87/CaseHUD/releases) `CaseHUD-Setup-x.y.z.exe` herunterladen und ausführen.
2. Das Setup installiert in einem Schritt alles Nötige: CaseHUD, den Sensor-Dienst (LibreHardwareMonitor ist eingebaut), den Hardware-Treiber PawnIO und die Berechtigung für die FPS-Messung. Optional: Autostart.
3. Herstellersoftware, die dieselben Displays oder LEDs ansteuert, beenden. CaseHUD erkennt solche Programme und fragt, ob es sie schließen soll (siehe [Andere Software](einstellungen.md#andere-software)).
4. CaseHUD startet nach dem Setup und zeigt alle gefundenen LCDs. Schließen des Fensters lässt die Anzeige im Tray weiterlaufen.

**Das Setup installiert:** CaseHUD nach `C:\Program Files\CaseHUD`, den Dienst `CaseHUDSensors` (läuft als SYSTEM), den Treiber PawnIO (nur falls noch nicht vorhanden), PresentMon für die FPS-Messung und eine Firewall-Regel für die AWTRIX-Suche (UDP 4211, nur private Netzwerke). Optional: Autostart und die FPS-Berechtigung (Mitgliedschaft in der Windows-Gruppe „Leistungsprotokollbenutzer“ / Performance Log Users; danach ab- und anmelden).

### Portable ZIP

Ohne Installation läuft CaseHUD aus der ZIP. Die Sensoren kommen dann von einem laufenden LibreHardwareMonitor: **Options → Remote Web Server → Run**. Die ZIP aktualisiert sich nicht selbst; das neue Setup lässt sich einfach darüber installieren.

## Ersteinrichtung

Beim ersten Start erscheinen zwei Dialoge nacheinander.

1. **Haftungsausschluss** („Vor der Gerätesteuerung lesen“): **Verstanden – weiter** bestätigt, **Beenden** schließt CaseHUD. Ohne Zustimmung spricht CaseHUD kein Gerät an.
2. **Einrichtung** („Bereiche wählen“): Die drei Kernbereiche **Display**, **Beleuchtung** und **Kühlung** sind vorgewählt. Nicht benötigte Bereiche abwählen, mindestens einer bleibt aktiv. **Übernehmen** speichert, **Überspringen** lässt alles an.

Der Dialog erscheint nur einmal für eine neue Konfiguration. Die Auswahl lässt sich jederzeit unter [Einstellungen → Bereiche](einstellungen.md#bereiche) ändern.

Danach:

1. Tab **Displays**: Prüfen, ob alle LCDs erscheinen. Fehlt eines, USB-Kabel prüfen und oben rechts **Nach LCDs suchen** wählen.
2. Pro Display Design oder Medien wählen ([Displays](displays.md)).
3. Beleuchtung und Kühlung sind zunächst ausgeschaltet bzw. im Testbetrieb. Sie werden erst nach ausdrücklicher Freigabe wirksam ([Beleuchtung](beleuchtung.md), [Kühlung](kuehlung.md#regelbetrieb-und-testbetrieb)).

## Oberfläche im Überblick

![Oberfläche mit Reitern und Statusanzeige](img/oberflaeche-ueberblick.png)
<!-- BILD: Hauptfenster mit allen Reitern (Displays, Szenen, Beleuchtung, Kühlung, Designs, Medien, Einstellungen) und Statusanzeige oben rechts -->

| Reiter | Inhalt | Kapitel |
|---|---|---|
| **Displays** | Alle LCDs mit Live-Vorschau, Inhalt je Display, Anordnung, Pixeluhren | [Displays](displays.md) |
| **Szenen** | Gespeicherte Belegungen, Automatik, Rotation | [Szenen](szenen.md) |
| **Beleuchtung** | Effekt auf dem aufgeklappten Gehäuse, Quellen, Skizze | [Beleuchtung](beleuchtung.md) |
| **Kühlung** | Schaltplan aus Sensoren, Kurven, TL-Lüftern und Pumpe | [Kühlung](kuehlung.md) |
| **Designs** | Mitgelieferte und eigene Designs, Layouts, Community | [Displays](displays.md#designs) |
| **Medien** | Bilder, GIFs, Videos | [Displays](displays.md#medien) |
| **Einstellungen** | Bereiche, Datenquellen, Geräte, Schnittstellen, Info | [Einstellungen](einstellungen.md) |

Reiter abgeschalteter Bereiche fehlen. Der Reiter **Kühlung** erscheint nur, wenn ein Lüfter-Controller gefunden wurde oder eine Kühlung eingerichtet ist.

Oben rechts:

- **Statusanzeige** („{n} Displays · Sensoren OK“): Klick öffnet die Einstellungen.
- **Herz**: Open-Source-Projekte, auf denen CaseHUD aufbaut.
- **Nach LCDs suchen**: sucht die Displays neu.
- **DE / EN**: Sprache der Oberfläche. Ohne gespeicherte Wahl folgt sie der Windows-Anzeigesprache (Deutsch, sonst Englisch); Wochentage und Dezimalzeichen auf den Displays richten sich danach.

Ein Hinweisband zeigt verfügbare Updates (**Neuerungen**, **Jetzt aktualisieren**), gefundene KI-Apps und einen Kühlungsalarm (**Zur Kühlung**).

## Update

1. Meldet CaseHUD eine neue Version, im Hinweisband **Jetzt aktualisieren** wählen.
2. CaseHUD lädt das Setup von GitHub, prüft die Signatur der Prüfsummen und den Hash des Setups und startet es. Windows fragt nach Administratorrechten.
3. CaseHUD startet danach neu; Einstellungen und Autostart bleiben erhalten.

Alternativ das neue `CaseHUD-Setup-x.y.z.exe` über die bestehende Installation ausführen. Die Update-Prüfung ist unter [Einstellungen → Allgemein](einstellungen.md#allgemein) abschaltbar.

Hinweise zum Update auf 1.2.0:

- Läuft ein anderes Lüfterprogramm, hält sich CaseHUD aus der Lüftersteuerung heraus. Für die eigene Steuerung das andere Programm beenden und die Kühlung in CaseHUD einrichten.
- Eine Pumpe am Mainboard-Anschluss wird nach dem Update einmal neu geprüft und läuft dabei kurz mit voller Drehzahl.
- Bestehende Gehäuse-Skizzen werden übernommen; Teile ohne Zuordnung zu einer LED-Zone sind auf der Beleuchtungsseite markiert.

## Deinstallation und Datenordner

**Datenordner:** `%APPDATA%\casehud` (Einstellungen, Designs, Medien). Mit `--data ORDNER` lässt sich ein anderer wählen. Er bleibt bei der Deinstallation erhalten.

**Die Deinstallation entfernt** Programm, Dienst, Firewall-Regel und den Autostart-Eintrag des deinstallierenden Kontos.

**Es bleiben zurück:**

- der Datenordner
- `C:\ProgramData\CaseHUD` (nur für die optionale `sensors.json` der SMBus-Brücke und die freigegebenen Mainboard-Lüfteranschlüsse; für normale Benutzer nur lesbar)
- der Treiber PawnIO (eigenständig, ggf. von anderer Software genutzt; unter „Apps“ separat entfernbar)
- die Gruppenmitgliedschaft
- dauerhafte Schreibvorgänge im Gerät, etwa das Startbild der Lüfter

Die Kommandozeilenoptionen von `CaseHUD.exe` stehen unter [Kommandozeile](kommandozeile.md).
