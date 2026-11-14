# API und MCP

[Handbuch](README.md) · Zurück: [Einstellungen](einstellungen.md) · Weiter: [Sensoren](sensoren.md)

CaseHUD lässt sich von außen steuern: über eine lokale HTTP-API (`/api/v1`) für andere Programme und über einen eingebauten MCP-Server für KI-Apps. Alles, was die Oberfläche kann, ist erreichbar, vor allem die Kühlung einschließlich manuellem Modus und Presets. Die vollständige Beschreibung steht in [docs/API.md](../API.md); die maschinenlesbare Fassung liefert `GET /api/v1/openapi.json` (Token mit mindestens „Lesen“).

## Grundsätze

- **Standardmäßig aus.** Die Schalter liegen unter **Einstellungen → Schnittstellen** (Expertenmodus): **Schnittstelle für andere Programme (API)** und **MCP-Schnittstelle für KI-Apps**. Sie lassen sich nur im Fenster ändern, nie über die API.
- **Nur lokal.** Erreichbar ist nur `127.0.0.1`, mit Token. Es gibt keine Cloud-Anbindung.
- **Rechte je Programm.** Jedes Programm bekommt nur die Rechte, die du ihm gibst: **Nur lesen**, **Steuern** und getrennt **System** (Beenden, Updates).
- **Kopplung.** Ein Programm fragt den Zugriff im CaseHUD-Fenster an; die Anfrage bestätigen oder ablehnen ([Zugänge](einstellungen.md#zugänge)). Erteilte Zugriffe sind dort einsehbar und entziehbar.

## API für andere Programme

1. **Einstellungen → Schnittstellen → Schnittstelle für andere Programme (API)** einschalten.
2. Das Programm findet Adresse und Token in `api.json` im Datenordner (`%APPDATA%\casehud\api.json`) und fragt per Kopplung um Zugriff. Alternativ unter **Einstellungen → KI-Verbindung → Zugang für ein anderes Programm → Zugang erstellen** einen eigenen Token anlegen (wird nur einmal angezeigt).
3. Aufrufe gehen an `http://127.0.0.1:<Port>/api/v1/…`.

Erreichbar sind Status, Displays, Designs, Layouts, Szenen, Rotation, Automatik, Beleuchtung (inkl. Vorlagen), Kühlung (inkl. Presets, manuellem Modus und Kalibrierung), Einstellungen sowie Update und Beenden (nur mit dem Recht „System“). Für die stille Installation: `CaseHUD-Setup-x.y.z.exe /VERYSILENT /API=1` schaltet die API ein.

## MCP für KI-Apps

Der eingebaute MCP-Server (`casehud-cli mcp`) bietet Werkzeuge für Status, Kühlung, Beleuchtung, Displays und Szenen. Er koppelt sich wie jedes andere Programm und bekommt höchstens „Steuern“; Beenden, Updates und das Öffnen des Fensters sind ausgeschlossen.

1. **Einstellungen → KI-Verbindung** öffnen und die KI-App **Verbinden** ([KI-Verbindung](einstellungen.md#ki-verbindung)).
2. Die KI-App neu starten.

Alternativ den Eintrag unter **Einstellungen → Schnittstellen** in die MCP-Konfiguration der App kopieren.

Die Werkzeuge tragen Sicherheitsregeln: nur ändern, wenn es verlangt wurde; vor Änderungen die Kühlungskonfiguration lesen; vor einer Messung (Kalibrierung) rückfragen. Eine Pumpe lässt sich auch hier nie unter 30 % setzen.

## Kühlung und Presets per API

- Kühlungsstatus und -konfiguration lesen, Kurven und Ausgänge ändern (Kurve **oder** manueller Wert), Presets wählen, speichern und auslesen, Kalibrierung starten und beenden.
- Ein abgeschalteter Bereich antwortet mit `cooling_off`.

Details und Beispiele: [docs/API.md](../API.md).
