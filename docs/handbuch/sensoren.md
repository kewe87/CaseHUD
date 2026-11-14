# Sensoren

[Handbuch](README.md) · Zurück: [API und MCP](api-mcp.md) · Weiter: [Kommandozeile](kommandozeile.md)

CaseHUD liest alle Sensoren, die LibreHardwareMonitor kennt: CPU (Intel/AMD), GPU (NVIDIA/AMD/Intel), Mainboard, Kühlmitteltemperatur, Pumpe, Durchfluss, Aquacomputer und mehr. Der Sensor-Dienst enthält LibreHardwareMonitor; es muss nichts extra installiert werden. Sensorquelle und Verbindungstest: [Einstellungen → Datenquellen](einstellungen.md#datenquellen).

## Sensorpfade

Ein Sensor wird über seinen Pfad in LibreHardwareMonitor angesprochen, z. B. `Temperatures/GPU Core`. Alle verfügbaren Sensoren listet die Kommandozeile mit `casehud-cli.exe sensors` ([Kommandozeile](kommandozeile.md)).

- **Alternativen:** Mehrere Pfade mit `|` trennen, der erste vorhandene wird genommen. `Temperatures/CPU Package|Temperatures/Core (Tctl/Tdie)` funktioniert dadurch mit Intel und AMD.
- **Grafikkarten:** Dedizierte Grafikkarten haben Vorrang vor der integrierten Grafik der CPU.
- **Rechnen mit Sensoren:** Sensoren lassen sich verrechnen, z. B. `Powers/Package + Powers/GPU Package` für die Gesamtleistung.

Wo Sensoren gewählt werden:

- im [Design-Editor](displays.md#design-bearbeiten) im Feld **Sensor** (Liste oder eigener Pfad)
- im [Schaltplan der Kühlung](kuehlung.md#schaltplan) über **Sensor hinzufügen …** (Temperaturen, nach Hardware gruppiert)
- beim Beleuchtungseffekt **Temperatur** (**Sensor**)

## Boards mit Kühlmittel-Temperatur-Header (T_Sensor)

Der mitgelieferte Sensor-Dienst enthält bereits den Fix für den Kühlmitteltemperatur-Header von Boards wie dem ASUS ROG STRIX X870E-E/-H und X870-F (in LibreHardwareMonitor selbst noch offen).

## FPS und Latenz

FPS, Frame Generation und PC-Latenz des Spiels im Vordergrund kommen per PresentMon; alternativ liefert RivaTuner Statistics Server (RTSS, z. B. mit MSI Afterburner) die Bildrate. Einrichtung unter **Einstellungen → Datenquellen → FPS & Latenz**:

1. **FPS-Messung einrichten** wählen (legt die nötige Berechtigung an: Mitgliedschaft in der Windows-Gruppe „Leistungsprotokollbenutzer“ / Performance Log Users).
2. In Windows ab- und wieder anmelden.
3. Status „Aktiv · FPS-Sensoren stehen in Designs bereit“ abwarten.

Die FPS-Messung dient auch der Spielerkennung der [Automatik](szenen.md#spielerkennung). Statusmeldungen: „PresentMon.exe fehlt“ (im Release-Paket neben `CaseHUD.exe` enthalten), „Berechtigung fehlt“, „Nur unter Windows verfügbar“.
