# Einstellungen

[Handbuch](README.md) · Zurück: [Kühlung](kuehlung.md) · Weiter: [API & MCP](api-mcp.md)

Der Reiter **Einstellungen** hat links eine Navigation mit den Gruppen **Bereiche**, **Allgemein**, **Datenquellen**, **Geräte**, **Anzeige**, **Schnittstellen**, **KI-Verbindung**, **Zugänge** und **Info & Hilfe**. **Anzeige**, **Schnittstellen** und **Zugänge** erscheinen nur im **Expertenmodus**.

![Einstellungen](img/einstellungen-uebersicht.png)
<!-- BILD: Reiter Einstellungen mit Navigation links und der Gruppe Bereiche -->

## Bereiche

CaseHUD hat drei Kernbereiche, die sich einzeln abschalten lassen: **Display** (Bildschirme, Designs, Medien, Szenen), **Beleuchtung** (RGB-Effekte und Geräte) und **Kühlung** (TL-Lüfter und Pumpe regeln). Mindestens ein Bereich bleibt aktiv („Mindestens ein Bereich muss aktiv bleiben.“).

- Ein abgeschalteter Bereich verliert seinen Reiter und seine Funktionen; die Einrichtung beim ersten Start fragt danach ([Ersteinrichtung](erste-schritte.md#ersteinrichtung)). Über die API antworten Routen eines abgeschalteten Bereichs mit einem Fehler (z. B. `cooling_off`, siehe [API.md](../API.md)).
- Unter **Kühlung** steht der Schalter **CaseHUD regelt die Lüfter**. Aus: Testbetrieb, keine Ausgabe an Lüfter oder Pumpe ([Regelbetrieb und Testbetrieb](kuehlung.md#regelbetrieb-und-testbetrieb)).

## Allgemein

| Einstellung | Wirkung |
|---|---|
| **Expertenmodus** | Zeigt erweiterte Einstellungen |
| **Mit Windows starten** | Start minimiert im Tray; die Displays laufen sofort |
| **Temperatur** | °C oder °F auf Displays und AWTRIX-Uhren |
| **Uhrzeit** | 24 h oder 12 h für Uhr-Designs; Wochentage folgen der Sprache (DE/EN) |
| **Anordnung** | **Anordnung öffnen**: Ports des Lüfter-Controllers und Nummern der Lüfter-Displays ([Anordnung](displays.md#anordnung)) |
| **LCDs mit dem Bildschirmschoner ausschalten** | Während des Windows-Bildschirmschoners sind die Displays schwarz (Helligkeit 0) |
| **Nach Updates suchen** | Tägliche Prüfung auf GitHub. Installation nur nach Bestätigung über **Jetzt aktualisieren** (Signatur wird geprüft) |
| **Firmware-Updates prüfen** | Einmalige Abfrage der Hersteller-Downloadseite beim Start; der Hersteller sieht dabei die IP-Adresse. Es wird nichts heruntergeladen oder installiert. Standardmäßig aus |

## Datenquellen

### Sensoren

Die Karte **Sensoren** zeigt den Status („Verbunden mit dem CaseHUD-Sensordienst · {n} Sensoren“). **Verbindung testen** prüft die Verbindung. Sind keine Sensoren erreichbar:

1. Empfohlen: Installation über das **Setup**. Der Sensordienst ist enthalten und startet automatisch.
2. Alternativ **LibreHardwareMonitor** als Administrator starten und **Options → Remote Web Server → Run** aktivieren.
3. Anschließend **Verbindung testen**.

Im Expertenmodus: **Sensorquelle** (**Automatisch (empfohlen)**, **Nur CaseHUD-Sensordienst**, **Nur LibreHardwareMonitor**, **Eigene Adresse**) und **Adresse** (z. B. `http://192.168.1.20:8085/data.json`). Mehr zu Sensorpfaden: [Sensoren](sensoren.md).

### FPS & Latenz

Bildrate des Spiels im Vordergrund per PresentMon, einschließlich Frame Generation (DLSS/FSR) und PC-Latenz. Alternativ RivaTuner Statistics Server (RTSS), sofern aktiv. **FPS-Messung einrichten** legt die nötige Berechtigung an; danach in Windows ab- und wieder anmelden. Die Schalter **PresentMon** und **RivaTuner Statistics Server** (Expertenmodus) wählen die Quelle. Siehe [Sensoren](sensoren.md#fps-und-latenz).

## Geräte

### Experimentelle Geräte

Geräte ohne Prüfung auf echter Hardware bleiben bis zur Freigabe ausgeschaltet.

1. **Experimentelle Geräte** einschalten. Eine einmalige Bestätigung („Experimentelle Geräte“) weist darauf hin, dass die Treiber ungetestet sind und CaseHUD nichts dauerhaft in die Geräte schreibt. **Verstanden – aktivieren**.
2. Unter **Einzelne Treiber** lässt sich jedes Gerät einzeln abschalten. Karten solcher Geräte tragen das Kennzeichen „experimentell“.
3. **Testbericht schreiben** öffnet das Formular „Hardware-Testbericht“ auf GitHub. Rückmeldungen (funktioniert / funktioniert nicht) sind willkommen.

### Start- und Ruhebild der Lüfter-LCDs

Standardmäßig aus; alle Optionen sind dauerhafte Schreibvorgänge im Gerät ([Start- und Ruhebild](displays.md#startbild-und-ruhebild)):

- **CaseHUD-Logo als Startbild** und **Startbild** (**CaseHUD-Logo** oder **Neutralbild**); **Erneut schreiben** schreibt das Bild noch einmal in alle angeschlossenen Lüfter-LCDs.
- **Seriennummer für neue Displays vergeben**
- **Startlogo 8,8" entfernen** (nur sichtbar, wenn CaseHUD früher ein Startlogo gespeichert hat): löscht diese Datei dauerhaft vom Gerät.

### Andere Software

Programme, die dieselben Geräte steuern können, stören sich gegenseitig (Gerätesperre, überschriebene Anzeige). CaseHUD erkennt bekannte Programme und zeigt oben ein Band („Andere Software steuert dieselben Geräte“).

- **Schließen** beendet das Programm: zuerst regulär (das Fenster wird bis zu 5 Sekunden um Schließen gebeten), danach auf Wunsch erzwungen (**Hart beenden**; ungespeicherte Daten gehen verloren). CaseHUD beendet nie automatisch ein Programm, nur nach Bestätigung und nur Programme dieser Liste.
- Der Dialog öffnet sich beim Start einmal, wenn ein solches Programm läuft; später auftauchende Programme zeigen nur das Band. **Später** schließt den Dialog, das Band bleibt. Das ✕ am Band blendet es aus, bis sich die Liste der laufenden Programme ändert. **Alle schließen** schließt alle schließbaren Programme nacheinander (mit Rückfrage bei hartnäckigen).
- **Ignorieren** (im Dialog) und der Schalter **Nicht schließen** (je Programm unter **Andere Software**) sind dieselbe Einstellung: Das Programm löst danach weder Band noch Dialog aus und wird nie von CaseHUD geschlossen. Der Schalter nimmt es zurück.
- Dienste und Programme mit Administratorrechten kann CaseHUD nicht beenden. Dienste in der Dienste-Verwaltung (`services.msc`) beenden oder auf „Manuell“ stellen; Autostart dauerhaft verhindern über Windows-Einstellungen → Apps → Autostart oder Task-Manager → Autostart.
- Programme aus ungewöhnlichen Speicherorten werden nur nach zusätzlicher Rückfrage geschlossen.
- Unter **Andere Software** in dieser Gruppe stehen die Liste der Programme (läuft / nicht gestartet) und **Beim Start und bei neuen Programmen fragen**. Aus: kein Hinweis, die Liste bleibt erhalten.

Für die Kühlung gilt zusätzlich: Läuft ein anderes Lüfterprogramm, regelt CaseHUD die Lüfter nicht ([Kühlung](kuehlung.md#andere-lüfterprogramme)).

## Anzeige

(Expertenmodus)

- **Aktualisierung der Displays**: Zeichenintervall der Sensor-Designs (**Wie im Design (Standard)**, 0,5 s, 1 s, 2 s, 5 s). Längere Intervalle senken die CPU-Last.
- **Weiche Animationen**: Ringe und Zahlen laufen weich zum neuen Wert (bis 30 Bilder/s, nur bei Änderungen).
- **Video im Display abspielen (spart CPU/USB)**: Balkendisplays spielen Videos selbst ab. Das Video wird einmalig mit ffmpeg umgewandelt und im Datenordner zwischengespeichert; ohne ffmpeg werden Einzelbilder gesendet.

## Schnittstellen

(Expertenmodus) Beide Schalter sind standardmäßig aus und gelten nur lokal (`127.0.0.1`, mit Token).

- **MCP-Schnittstelle für KI-Apps**: ermöglicht MCP-fähigen KI-Apps, Sensoren zu lesen und Designs zu entwerfen; mit dem Recht **Steuern** auch Displays, Beleuchtung und Kühlung zu ändern. Bei eingeschaltetem Schalter zeigt CaseHUD den Eintrag für die MCP-Konfiguration der KI-App (**Kopieren**).
- **Schnittstelle für andere Programme (API)**: Steuerung von CaseHUD durch andere Programme auf diesem PC (z. B. CouchHUD).

Einzelheiten: [API & MCP](api-mcp.md).

## KI-Verbindung

Unter **KI-Verbindung** verbindet CaseHUD installierte KI-Apps mit CaseHUD (MCP), ohne dass Konfigurationsdateien von Hand bearbeitet werden. Unterstützt: Claude Desktop, Claude Code, Cursor, Visual Studio Code, LM Studio. Jede App erscheint als **Nicht installiert**, **Gefunden** oder **Verbunden**.

![KI-Verbindung](img/einstellungen-ki-verbindung.png)
<!-- BILD: Gruppe KI-Verbindung mit Liste der KI-Apps (Verbunden / Gefunden) und der Karte „Zugang für ein anderes Programm“ -->

1. Bei einer gefundenen App **Verbinden** wählen.
2. Rechte wählen: **Nur lesen** (Zustand, Sensoren und Vorschauen abrufen) oder **Steuern** (zusätzlich Displays, Beleuchtung und Kühlung ändern). Das Recht „System“ (Beenden, Updates) wird nie angeboten.
3. **Verbinden** bestätigen. CaseHUD sichert die Konfigurationsdatei der App, trägt nur seinen eigenen Eintrag ein, schaltet die MCP-Schnittstelle ein und legt den Zugriff an. Andere Einträge bleiben unverändert.
4. Die App neu starten; danach ist CaseHUD dort verfügbar.

Weitere Aktionen: **Rechte ändern**, **Trennen** (entfernt den Eintrag mit Sicherung und entzieht den Zugriff). Ist Claude Code nicht im PATH, zeigt CaseHUD den Befehl zum Ausführen an. Existiert in der App bereits ein fremder Eintrag „casehud“, ersetzt CaseHUD ihn nur nach Rückfrage. Konfigurationen, die kein gültiges JSON sind, doppelte Schlüssel enthalten oder eine Verknüpfung sind, werden nicht verändert.

Ein einmaliger Hinweis („KI-App gefunden: {App}“, **Verbinden …**, **Nicht mehr anzeigen**) erscheint beim ersten Start mit einer unterstützten App.

**Zugang für ein anderes Programm** (**Zugang erstellen**): erzeugt für Programme ohne MCP, die die API nutzen, einen eigenen Token. Der Token wird nur einmal angezeigt; die API wird eingeschaltet. Details: [API.md](../API.md#ai-connection).

## Zugänge

(Expertenmodus) Hier stehen alle Programme mit Zugriff auf CaseHUD.

- Programme fragen den Zugriff selbst an. Im CaseHUD-Fenster erscheint „{Programm} möchte auf CaseHUD zugreifen“ (Anfrage über API oder MCP).
  - **Ablehnen**, **Nur lesen** oder **Steuern**.
  - **Auch System erlauben (Beenden, Updates)** gibt es nur zusammen mit **Steuern** und nur für die API.
  - **Merken**: Der Zugriff bleibt nach einem Neustart erhalten. Sonst gilt er, bis CaseHUD beendet wird.
- Je Eintrag: **Rechte ändern** (gilt sofort), **Entziehen** (das Programm verliert den Zugriff sofort; eine neue Anfrage muss erneut bestätigt werden). Angezeigt werden Rechte und „zuletzt verwendet“.
- **Standard-Zugang** (Token aus `api.json`, z. B. für CouchHUD): **Erneuern** ersetzt den Token; der alte endet sofort, Programme, die ihn nur beim Start lesen, müssen neu gestartet werden.

Programme sollten nur bekannte Rechte bekommen: **Nur lesen** reicht zum Anzeigen, **Steuern** nur für Programme, denen du vertraust.

## Info und Hilfe

- **Version** und Link zu GitHub
- **Protokoll** (Expertenmodus): öffnet das Protokoll der Hintergrundaktivität, nützlich zur Fehlersuche
- **Diagnose exportieren**: speichert eine ZIP-Datei mit Protokollende, bereinigten Einstellungen, Versionen, Display-Liste und Beleuchtungsbericht. Namen, Pfade, Adressen und Tokens werden entfernt. Es wird nichts gesendet; Datei vor der Weitergabe prüfen. Optional **Mitschnitt (30 s)**: zeichnet 30 Sekunden lang die Datenpakete zwischen CaseHUD und den Beleuchtungsgeräten auf (nur bei aktiver Beleuchtung; währenddessen z. B. den Effekt ändern). Sie können Hardware-Kennungen des Controllers enthalten.
- **Datenordner** (Expertenmodus): öffnet `%APPDATA%\casehud`
- **Open Source**: Projekte, auf denen CaseHUD aufbaut
- **Lizenz**: © 2026 kewe87 · GNU GPL v3.0
- **Datenschutz** und **Haftungsausschluss**: Texte wie in der [README](../../README.md#netzwerk-und-datenschutz); der Status zeigt, wann dem Haftungsausschluss zugestimmt wurde
- **Bildschirm- und Audio-Effekte erlauben**: Zustimmung für die Beleuchtungseffekte Bildschirm-Ambilight und Audio-Visualizer. Ausschalten widerruft die Erlaubnis und stellt einen laufenden Effekt auf den Standardeffekt zurück ([Bildschirm-Ambilight und Audio-Visualizer](beleuchtung.md#bildschirm-ambilight-und-audio-visualizer)).

## Hilfe aus dem Programm

Neben jeder Seitenüberschrift und neben den Abschnitten (z. B. **Woher kommt die Farbe?**, die Gruppen der Einstellungen) steht ein **?**. Es öffnet im Browser den passenden Abschnitt dieses Handbuchs in der Sprache des Programms, und zwar in der Fassung der installierten Version. Welche Stelle wohin führt, legt [links.json](links.json) fest.
