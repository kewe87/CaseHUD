# Fehlerbehebung und FAQ

[Handbuch](README.md) · Zurück: [Kommandozeile](kommandozeile.md) · Weiter: [Selbst bauen und Roadmap](selbst-bauen.md)

## Displays

**Kein unterstütztes Display gefunden.**
1. USB-Kabel der LCD-Lüfter bzw. des Pumpenkopfs prüfen; den Controller direkt am Mainboard anschließen, nicht über Hub oder Verteiler.
2. Herstellersoftware für die Displays beenden ([Andere Software](einstellungen.md#andere-software)).
3. Oben rechts **Nach LCDs suchen** wählen. Nach fünf erfolglosen automatischen Versuchen stoppt die Suche („Gerät neu einstecken oder „Neu suchen“ wählen“).

**Der Universal Screen 8.8" zeigt nichts / hängt im Monitor-Modus.** **Zurückholen** im Band „Monitor-Modus“ ([Monitor-Modus](displays.md#monitor-modus)).

**Ein experimentelles Gerät wird nicht angesprochen.** Es muss unter **Einstellungen → Geräte → Experimentelle Geräte** freigegeben sein ([Experimentelle Geräte](einstellungen.md#experimentelle-geräte)).

**Videos laufen nicht.** **ffmpeg** installieren (`winget install ffmpeg` oder `ffmpeg.exe` neben `CaseHUD.exe`). Bilder und GIFs gehen ohne.

**Eine Live-Vorschau fehlt („zu viele CaseHUD-Fenster offen“).** Andere CaseHUD-Fenster schließen; es wird erneut versucht.

**Die Displays sollen mit dem Bildschirmschoner ausgehen (oder nicht).** Die Option **LCDs mit dem Bildschirmschoner ausschalten** unter **Einstellungen → Allgemein** prüfen.

## Sensoren und FPS

**„Sensoren offline“ in der Statusanzeige.** Unter **Einstellungen → Datenquellen → Sensoren** **Verbindung testen**. Läuft der Dienst `CaseHUDSensors` (Windows-Dienste)? Bei der portablen ZIP muss LibreHardwareMonitor mit **Options → Remote Web Server → Run** laufen ([Sensoren](sensoren.md)).

**FPS fehlen.** **FPS-Messung einrichten**, danach in Windows ab- und anmelden; der Status „Berechtigung fehlt“ weist auf die fehlende Gruppenmitgliedschaft hin ([FPS und Latenz](sensoren.md#fps-und-latenz)).

**Ein Sensor erscheint nicht im Design.** Pfad mit `casehud-cli.exe sensors` prüfen; Alternativen mit `|` angeben.

## Beleuchtung

**Nichts leuchtet.**
1. Ist der Schalter **Beleuchtung steuern** eingeschaltet? („Aus: kein Gerätezugriff durch CaseHUD“)
2. Steuert ein anderes Programm das Gerät („Belegt“)? Programm beenden oder **Trotzdem ansteuern**.
3. Bei ARGB-Anschlüssen des Mainboards ist die **LED-Anzahl** zu setzen („Keine LED-Anzahl gesetzt“; [Zonen und LED-Anzahl](beleuchtung.md#zonen-und-led-anzahl)).
4. Ist die Quelle des Geräts **Aus** oder **Nicht angesteuert** ([Woher kommt die Farbe?](beleuchtung.md#woher-kommt-die-farbe))?
5. Teile der Skizze müssen mit einer Zone **verbunden** sein ([Teile verbinden](beleuchtung.md#teile-verbinden)).

**RAM leuchtet nicht.** Die SMBus-Brücke muss eingeschaltet sein ([RAM-Beleuchtung](beleuchtung.md#ram-beleuchtung)).

**Ambilight oder Audio-Visualizer lassen sich nicht nutzen.** Zustimmung unter **Einstellungen → Info & Hilfe → Bildschirm- und Audio-Effekte erlauben**.

## Kühlung

**Die Lüfter reagieren nicht auf die Kurven.** Im **Testbetrieb** gibt CaseHUD nichts aus: **Einstellungen → Bereiche → CaseHUD regelt die Lüfter** einschalten ([Regelbetrieb und Testbetrieb](kuehlung.md#regelbetrieb-und-testbetrieb)). Läuft ein anderes Lüfterprogramm oder ist der Haftungshinweis unbestätigt, bleibt die Ausgabe blockiert.

**Kein Reiter „Kühlung“.** Der Reiter erscheint nur mit gefundenem Lüfter-Controller oder eingerichteter Kühlung; der Bereich darf nicht abgeschaltet sein.

**„Pumpe läuft nicht, vermutlich defekt“.** Last reduzieren und die Pumpe prüfen; die Lüfter laufen auf sicherer Drehzahl ([Sicherheit](kuehlung.md#sicherheit)).

**„Nicht freigegeben“ / „Nicht abgeglichen“ an der Pumpe.** **Anschluss freigeben** (Administratorabfrage bestätigen), danach **Erneut prüfen**. Bis dahin regelt das Mainboard die Pumpe.

**„Sensordienst antwortet nicht“.** Dienst „CaseHUD Sensors“ in den Windows-Diensten prüfen.

**Speichern ist gesperrt.** Die Probleme stehen in der Liste über dem Plan (z. B. fehlender Sensor, Pumpe unter 30 %, Kurve ohne Bezugssensor).

## Installation und Update

**SmartScreen warnt.** Die Programme sind noch nicht code-signiert. Prüfsummen und Signatur: [SECURITY.md](../../SECURITY.md).

**Update schlägt fehl.** **Erneut versuchen** im Hinweisband oder das Setup von [Releases](https://github.com/kewe87/CaseHUD/releases) manuell ausführen. Windows fragt nach Administratorrechten.

## Hilfe holen

1. **Einstellungen → Info & Hilfe → Diagnose exportieren** erzeugt eine bereinigte ZIP-Datei (Namen, Pfade, Adressen und Tokens entfernt). Vor der Weitergabe prüfen.
2. Issue auf GitHub eröffnen: [github.com/kewe87/CaseHUD/issues](https://github.com/kewe87/CaseHUD/issues). Für Geräte-Tests das Formular „Hardware-Testbericht“ nutzen, mit Gerätename und USB-VID:PID (Geräte-Manager → Eigenschaften → Details → Hardware-IDs).
3. Sicherheitslücken nicht öffentlich melden, sondern vertraulich über [SECURITY.md](../../SECURITY.md).
