# Selbst bauen und Roadmap

[Handbuch](README.md) · Zurück: [Fehlerbehebung und FAQ](fehlerbehebung.md)

## Selbst bauen

```bash
go test ./...
GOOS=windows GOARCH=amd64 go build -o casehud.exe ./cmd/casehud
GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui" -o CaseHUD.exe ./cmd/casehud-gui
go run ./cmd/casehud-gui --demo       # GUI mit simulierten LCDs, ohne Hardware
```

Releases baut GitHub Actions (`.github/workflows/release.yml`): Entweder einen Tag `vX.Y.Z` pushen oder den Workflow „Release“ manuell mit dem Tag als Eingabe starten. Heraus kommen Setup und portable ZIP für **Windows x64** samt signierten Prüfsummen unter Releases. Andere Plattformen werden nicht gebaut.

Die Kommandozeile der gebauten Programme: [Kommandozeile](kommandozeile.md). Treiber: [DRIVERS.md](../DRIVERS.md).

## Roadmap

- **1.0 – Erstes Release:** Displays (UNI FAN TL LCD, Universal Screen 8.8"; weitere experimentell), Rotation, Szenen und Automatik, AWTRIX-Pixeluhren, optionale MCP-Schnittstelle und lokale API (Rechte je Programm), Community-Galerie, Setup mit Sensor-Dienst, Sauberes Update und Deinstallieren, Hotplug, Standby und automatische Wiederverbindung, Displays aus bei Bildschirmschoner, Erkennung von Herstellersoftware mit Beenden-Dialog (nur auf Klick), optionale Beleuchtung (TL Controller, Mainboard-ARGB, RAM über die SMBus-Brücke, 8.8"-Rahmen), Anzeige der Geräte-Firmware mit optionaler Prüfung, Ersteinrichtung, signierte Prüfsummen, Diagnose-Export und Issue-Vorlagen für Geräte-Tests.
- **1.1 – Beleuchtung ausbauen:** Gehäuse-Skizze und Layout-Editor für die LEDs, Bildschirm-Ambilight und Audio-Visualizer. Noch offen (Ideen, keine Zusagen): weitere Geräte und Controller.
- **1.2 – Kühlung und Schnittstellen:** eigene Steuerung von TL-Lüftern und Pumpe, neu gestaltete Beleuchtungsseite, API v1 mit Kopplung, MCP und KI-Verbindung, abschaltbare Kernbereiche.
- **Ideen für später – nach Nachfrage:** Alarme (Display pulsiert bei kritischer Temperatur), Zeitpläne, abgeleitete Sensoren wie ΔT; neue Widgets (Laufwerke, Netzwerk), „Now Playing“, Home Assistant / MQTT, eigene HTTP/JSON-Quellen; weitere Hersteller und Display-Typen – nur mit Testern aus der Community, die die Hardware besitzen.

Fehlt ein Display? Issue mit Gerätename und USB-VID:PID eröffnen – mit Tester geht es schneller.
