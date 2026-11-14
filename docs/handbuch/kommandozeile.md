# Kommandozeile

[Handbuch](README.md) · Zurück: [Sensoren](sensoren.md) · Weiter: [Fehlerbehebung und FAQ](fehlerbehebung.md)

Es gibt zwei Programme: `CaseHUD.exe` (die grafische Oberfläche) und `casehud-cli.exe` (für Skripte und zur Fehlersuche).

## GUI: CaseHUD.exe

| Option | Wirkung |
|---|---|
| `--demo` | simulierte LCDs und Sensoren, keine Hardware nötig |
| `--demo-setup` | mit `--demo`: zeigt den Einrichtungsdialog des ersten Starts |
| `--minimized` | startet im Tray, ohne das Fenster zu öffnen (der Autostart nutzt das) |
| `--background` | wie `--minimized`; läuft CaseHUD schon, beendet sich der zweite Start still, ohne das Fenster zu zeigen |
| `--no-tray` | kein Tray-Symbol, solange keine Taskleiste da ist (Shells ohne Taskleiste); legt Explorer später die Taskleiste an (Wechsel in den Desktop-Modus), erscheint das Symbol; das Fenster öffnet sonst die API oder ein zweiter Start |
| `--port N` | Port der Oberfläche auf 127.0.0.1 (Standard: zufällig) |
| `--data ORDNER` | anderer Datenordner |
| `--set-api=on` / `off` | schaltet die API in den Einstellungen um und beendet sich (vom Installer genutzt) |
| `--quit` | beendet ein laufendes CaseHUD sauber (vom Installer genutzt) |

CaseHUD startet außerdem selbst zwei interne Aufrufe, die nicht von Hand gedacht sind: `--cooling-guard` (Wächterprozess der Kühlung, siehe [Sicherheit](kuehlung.md#sicherheit)) und `--allow-cooling-header` (gibt einen Mainboard-Lüfteranschluss frei; braucht Administratorrechte).

Stille Installation: `CaseHUD-Setup-x.y.z.exe /VERYSILENT`; mit `/API=1` wird zusätzlich die API eingeschaltet ([API und MCP](api-mcp.md)).

## CLI: casehud-cli.exe

```powershell
.\casehud-cli.exe devices                 # gefundene LCDs anzeigen
.\casehud-cli.exe sensors                 # alle Sensoren von LibreHardwareMonitor anzeigen
.\casehud-cli.exe run --mock              # Testanzeige mit simulierten Werten auf allen LCDs
.\casehud-cli.exe preview --out test.png  # ein Bild mit Live-Werten rendern
```

Weitere Befehle:

| Befehl | Wirkung |
|---|---|
| `version` | Version ausgeben |
| `mcp [--data DIR] [--client APP]` | MCP-Server auf stdio für KI-Apps (CaseHUD muss mit eingeschalteter MCP-Schnittstelle laufen; [API und MCP](api-mcp.md)) |
| `init [--layout PATH]` | Standard-Layoutdatei schreiben |
| `preview [--layout PATH] [--mock] [--out FILE]` | ein Bild in eine PNG-Datei rendern |
| `run [--layout PATH] [--mock] [--png FILE] [--brightness N]` | kontinuierlich auf alle LCDs rendern (oder mit `--png` in eine Datei) |
| `sensors [--url URL]` | Sensoren von LibreHardwareMonitor listen |

Sensoren liest die CLI von LibreHardwareMonitor (Options → Remote Web Server → Run, Standard `http://localhost:8085/data.json`). `--mock` nutzt simulierte Werte.

## Selbst bauen

Siehe [Selbst bauen](selbst-bauen.md#selbst-bauen).
