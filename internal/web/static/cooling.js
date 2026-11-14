"use strict";

// ---------- Cooling: a wiring plan of sensors, curves and fans ----------
Object.assign(STRINGS.de, {
  "tab.cooling": "Kühlung",
  "cool.sub": "Testbetrieb: Berechnung ohne Ausgabe an Lüfter oder Pumpe. Die bisherige Lüftersteuerung bleibt aktiv.",
  "cool.subLive": "Regelbetrieb nach Kurven. Bei Ausfall von CaseHUD übernimmt das Mainboard eine Pumpe an einem Mainboard-Anschluss.",
  "cool.blocked.program": "{p} läuft. Keine Ausgabe an Lüfter; CaseHUD übernimmt nach Beenden des Programms.",
  "cool.blocked.disclaimer": "Keine Ausgabe an Lüfter, bis der Haftungshinweis bestätigt ist.",
  "cool.blocked.check": "Keine Ausgabe an Lüfter: Prüfung auf andere Lüfterprogramme nicht möglich.",
  "cool.hint": "Kabel von Anschluss zu Anschluss ziehen. Ein Klick auf ein Kabel trennt es, ein Klick auf einen Baustein zeigt seine Einstellungen.",
  "cool.colSensors": "Sensoren",
  "cool.panelSensor": "Sensor", "cool.panelCurve": "Kurve", "cool.panelFan": "Lüfter", "cool.panelPump": "Pumpe",
  "cool.colCurves": "Kurven",
  "cool.colOutputs": "Lüfter und Pumpe",
  "cool.fanName": "Port {p} · Lüfter {f}",
  "cool.pumpName": "Pumpe",
  "cool.curveName": "Kurve {n}",
  "cool.curveAdd": "Neue Kurve",
  "cool.curveDrop": "Keine Kurve. Kabel von einem Sensor hierher ziehen, um eine anzulegen.",
  "cool.sensorDrop": "Kein Sensor auf dem Plan.",
  "cool.sensorAdd": "Sensor hinzufügen…",
  "cool.sensorPick": "Sensor wählen…",
  "cool.sensorMissing": "Nicht verfügbar: {k}",
  "cool.pumpAdd": "Pumpe hinzufügen",
  "cool.type": "Typ",
  "cool.type.fan": "Lüfter",
  "cool.type.case": "Gehäuselüfter (läuft immer)",
  "cool.type.pump": "Pumpe",
  "cool.mayStand": "Darf stillstehen",
  "cool.mayStandHint": "Lüfter steht, solange die Kurve höchstens {p} % vorgibt (dafür am kalten Ende auf 0 setzen). Bei Übertemperatur oder Sensorausfall läuft er trotzdem an.",
  "cool.standing": "Steht",
  "cool.addAll": "{g}: alle hinzufügen",
  "cool.groupCount": "{n} Sensoren, heißester zählt",
  "cool.groupClose": "{g} zusammenklappen",
  "cool.group.ram": "Arbeitsspeicher",
  "cool.group.cpu": "Prozessor",
  "cool.group.gpu": "Grafikkarte",
  "cool.group.drive": "Laufwerke",
  "cool.group.board": "Mainboard",
  "cool.group.other": "Weitere",
  "cool.problem.pumpCurve": "Pumpe „{n}“ am Lüfter-Controller: Kurve erforderlich.",
  "cool.problem.pumpHeader": "Pumpe „{n}“ nicht regelbar: Drehzahlsensor gehört zu keinem Mainboard-Anschluss. Kurve trennen, dann wird sie nur überwacht.",
  "cool.problem.pumpLow": "Pumpe „{n}“: mindestens 30 % erforderlich.",
  "cool.problem.caseStops": "Gehäuselüfter „{n}“ läuft immer: Minimum über 0 % erforderlich.",
  "cool.tidy": "Aufräumen",
  "cool.free": "Ohne Kurve",
  "cool.unregulated": "Nicht geregelt",
  "cool.sensorGone": "Sensor fehlt",
  "cool.subIdle": "Regelbetrieb aktiv, aber kein Lüfter ist mit einer Kurve verbunden. Die Lüfter behalten ihre letzte Drehzahl.",
  "cool.parkAsk": "{n} ist danach mit keiner Kurve verbunden: sichere Drehzahl, keine Regelung, keine Überwachung. Trotzdem speichern?",
  "cool.waiting": "Erster Durchlauf ausstehend…",
  "cool.wouldSet": "würde {d} %",
  "cool.wouldRpm": "würde {t} RPM ({d} %)",
  "cool.target": "Soll {t} RPM",
  "cool.targetActual": "Soll {t} RPM · Ist {a} RPM",
  "cool.manualWould": "würde manuell {v} {u}",
  "cool.heldValue": "gehalten {v}",
  "cool.cal.othersPreview": "Die übrigen Lüfter bleiben unverändert.",
  "cool.status.pending": "Gespeichert: Regelbetrieb, derzeit nicht wirksam",
  "cool.watchOnly": "Nur überwacht",
  "cool.alarm.pump": "{n} läuft nicht, vermutlich defekt. Last reduzieren und Pumpe prüfen.",
  "cool.alarm.pumpUnknown": "Drehzahl von {n} nicht lesbar. Pumpenbetrieb nicht prüfbar.",
  "cool.alarm.stale": "Lüfterregelung reagiert nicht. Lüfter behalten den letzten Wert.",
  "cool.alarm.config": "Lüfterregelung inaktiv: Einstellungen konnten nicht geladen werden.",
  "cool.alarm.program": "{p} läuft, keine Regelung durch CaseHUD. Lüfter mindestens auf sicherer Drehzahl, bis {p} übernimmt oder beendet wird.",
  "cool.alarm.check": "Keine Regelung: Prüfung auf andere Lüfterprogramme nicht möglich.",
  "cool.alarm.disclaimer": "Keine Regelung, bis der Haftungshinweis bestätigt ist.",
  "cool.alarm.device": "Lüfter-Controller nicht erreichbar. {n} und weitere Lüfter werden nicht geregelt.",
  "cool.alarm.write": "{n} nimmt keine Drehzahl an.",
  "cool.alarm.sensor": "Sensor für {n} fehlt oder liefert keine neuen Werte. Lüfter auf sicherer Drehzahl.",
  "cool.alarm.stalled": "{n} steht trotz Drehzahlvorgabe.",
  "cool.alarm.service": "Sensordienst antwortet nicht: Temperaturen und Drehzahlen fehlen. Lüfter auf sicherer Drehzahl. Dienst „CaseHUD Sensors“ in den Windows-Diensten prüfen.",
  "cool.alarm.configBackup": "cooling.json beschädigt, Betrieb mit der letzten Sicherung. Defekte Datei als {n} im Datenordner. Einstellungen prüfen und neu speichern.",
  "cool.alarm.halted": "Lüfterregelung angehalten, CaseHUD läuft weiter. Lüfter (und Pumpe) werden nicht geregelt. CaseHUD neu starten.",
  "cool.alarmOpen": "Zur Kühlung",
  "cool.stalled": "Steht trotz Vorgabe",
  "cool.writeFailed": "Nimmt keine Drehzahl an",
  "cool.sensorLost": "Sensor fehlt, sichere Drehzahl",
  "cool.pumpDown": "Läuft nicht, vermutlich defekt",
  "cool.emergency": "Übertemperatur, volle Drehzahl",
  "cool.unguarded": "ohne Temperaturwächter",
  "cool.alarm.emergency": "Übertemperatur: {n} auf voller Drehzahl. Last reduzieren und Kühlung prüfen.",
  "cool.notReleased": "Nicht freigegeben",
  "cool.releaseHint": "Das Mainboard regelt die Pumpe bis zur Freigabe des Anschlusses.",
  "cool.release": "Anschluss freigeben",
  "cool.notMatched": "Nicht abgeglichen",
  "cool.matching": "Abgleich läuft",
  "cool.matchHint": "Drehzahlsensor und Anschluss gehören nicht nachweislich zusammen. Das Mainboard regelt die Pumpe weiter.",
  "cool.recheck": "Erneut prüfen",
  "cool.recheckSave": "Erst speichern",
  "cool.control": "Regelkanal",
  "cool.controlDerived": "Vom Drehzahlsensor abgeleitet ({k})",
  "cool.controlMissing": "Kanal nicht gefunden ({k})",
  "cool.channel": "Kanal {n}",
  "cool.controlHint": "Nur Kanäle desselben Chips. Eine neue Wahl wird nach dem Speichern geprüft und braucht die Freigabe des Anschlusses.",
  "cool.rpmUnknown": "Pumpendrehzahl unbekannt",
  "cool.deviceLost": "Controller nicht erreichbar",
  "cool.held": "Wird kalibriert",
  "cool.badfile": "Einstellungen nicht ladbar, keine Regelung: {e}",
  "cool.noFans": "Kein Lüfter-Controller gefunden.",
  "cool.fansHeld": "Lüfterliste nach Bestätigung des Hinweises, sofern kein anderes Lüfterprogramm läuft.",
  "cool.close": "Schließen",
  "cool.remove": "Entfernen",
  "cool.disconnect": "Trennen",
  "cool.name": "Name",
  "cool.current": "Aktuell",
  "cool.usedBy": "Hängt an",
  "cool.connectCurve": "An Kurve anschließen…",
  "cool.removeSensor": "Vom Plan nehmen",
  "cool.unit": "Die Kurve gibt vor",
  "cool.unitPercent": "Prozent",
  "cool.unitRpm": "Drehzahl",
  "cool.unitPercentHint": "Alle Lüfter bekommen denselben Prozentwert.",
  "cool.unitRpmHint": "Alle Lüfter erreichen dieselbe Drehzahl. Umrechnung über die Kalibrierung des jeweiligen Lüfters; Kalibrierung erforderlich.",
  "cool.curve": "Verlauf",
  "cool.curveHint": "Punkte ziehen. Doppelklick fügt einen Punkt hinzu, Rechtsklick entfernt ihn.",
  "cool.curveTemp": "Temperatur in °C",
  "cool.pointAdd": "Punkt hinzufügen",
  "cool.sensors": "Sensoren (der heißeste zählt)",
  "cool.attached": "Angeschlossen",
  "cool.attachOutput": "Lüfter anschließen…",
  "cool.duplicate": "Duplizieren",
  "cool.copyOf": "{n} (Kopie)",
  "cool.deleteCurve": "Kurve löschen",
  "cool.advanced": "Regelverhalten",
  "cool.riseBy": "Steigen ab (°C)",
  "cool.fallBy": "Fallen ab (°C)",
  "cool.riseAfter": "Steigen nach (s)",
  "cool.fallAfter": "Fallen nach (s)",
  "cool.lead": "Vorausschau (s)",
  "cool.stepUp": "Schritt hoch (%/s)",
  "cool.stepDown": "Schritt runter (%/s)",
  "cool.minDuty": "Minimum (%)",
  "cool.stopAt": "Stopp bei (%)",
  "cool.startDuty": "Start mit (%)",
  "cool.safeDuty": "Sichere Drehzahl (%)",
  "cool.stoppedBelow": "Gilt als stehend unter (RPM)",
  "cool.rpmSensor": "Drehzahlsensor der Pumpe",
  "cool.curveOf": "Kurve",
  "cool.noSource": "Keine Temperaturquelle für die Prüfung",
  "cool.why.noSource": "Keine Temperaturquelle in der Konfiguration: Eine Kurve mit Sensor anlegen.",
  "cool.drive": "Steuerung",
  "cool.drive.curve": "Kurve",
  "cool.drive.manual": "Manuell",
  "cool.manualValue": "Wert",
  "cool.manualUnit": "Einheit",
  "cool.manualNode": "Manuell · {v} {u}",
  "cool.manualNoSpeeds": "Drehzahl nur bei kalibriertem Lüfter.",
  "cool.problem.manualRange": "„{n}“: Wert über 0 erforderlich, in Prozent höchstens 100.",
  "cool.problem.manualRpm": "„{n}“: Drehzahl braucht einen kalibrierten Lüfter.",
  "cool.problem.manualMin": "„{n}“: Wert unter dem Minimum ({m} %).",
  "cool.noCurve": "Keine (wird nicht geregelt)",
  "cool.pumpHint": "Mit Kurve oder manuellem Wert regelt CaseHUD die Pumpe, ohne beides bleibt sie beim Mainboard und wird nur überwacht. Bei Ende von CaseHUD oder laufendem anderem Lüfterprogramm regelt wieder das Mainboard. Eine Pumpe lässt sich nicht abschalten: Steht sie, ist sie defekt (Alarm), und alle Lüfter an Kurven wechseln auf ihre sichere Drehzahl.",
  "cool.pumpHintController": "Pumpe am Lüfter-Controller, folgt ihrer Kurve. Eine Pumpe lässt sich nicht abschalten: Steht sie, ist sie defekt (Alarm), und alle Lüfter an Kurven wechseln auf ihre sichere Drehzahl.",
  "cool.pumpDefect": "{n} läuft nicht, vermutlich defekt. Keine Kühlmittelzirkulation, alle Lüfter auf sicherer Drehzahl. Last reduzieren und Pumpe prüfen.",
  "cool.speeds": "Kalibriert: {n} Punkte, {a}–{b} RPM",
  "cool.noSpeeds": "Noch nicht kalibriert.",
  "cool.cal.fan": "Lüfter kalibrieren",
  "cool.cal.pump": "Pumpen-Minimum kalibrieren",
  "cool.cal.all": "Alle Lüfter kalibrieren",
  "cool.cal.stop": "Abbrechen und verwerfen",
  "cool.cal.stopKeep": "Abbrechen, Fertige behalten",
  "cool.cal.stopTip": "Beendet den Lauf, nichts daraus wird gespeichert.",
  "cool.cal.stopKeepTip": "Beendet den Lauf, die bereits fertigen Lüfter werden gespeichert.",
  "cool.cal.title": "{n} wird kalibriert",
  "cool.cal.titlePump": "{n}: Pumpen-Minimum wird kalibriert",
  "cool.cal.fanOf": "Lüfter {i} von {c}",
  "cool.cal.others": "Die übrigen Lüfter werden weiter geregelt.",
  "cool.cal.othersPump": "Die Lüfter werden weiter geregelt.",
  "cool.cal.progress": "Fortschritt",
  "cool.cal.ph.down": "Abwärts · Stufe {s} von {S}",
  "cool.cal.ph.pump": "Pumpe · Stufe {s} von {S}",
  "cool.cal.ph.stop": "Stillstand halten · {h} von {H} s",
  "cool.cal.ph.start": "Anlauf · {d} % (bis höchstens {m} %)",
  "cool.cal.ph.wait": "Wird vorbereitet",
  "cool.cal.ph.pause": "Pause vor dem nächsten Lüfter",
  "cool.cal.now": "Aktuelle Stufe {d} % · {r} RPM",
  "cool.cal.steps": "Stufen",
  "cool.cal.clear": "Kalibrierung entfernen",
  "cool.noEffect": "Kurve wirkt nicht",
  "cool.heldNow": "Wird kalibriert · {d} % · {r} RPM",
  "cool.cal.fanAsk": "{n} wird kalibriert.\n\n1. Abwärts: Die Drehzahl sinkt von 100 % schrittweise bis zum Stillstand.\n2. Stillstand: mindestens 30 s bei 0 %.\n3. Wiederanlauf: aufwärts in 1-%-Schritten, je Schritt mindestens 30 s.\n\nDauer: meist unter 10 Minuten, bei trägen Lüftern länger; der Lüfter steht dabei zeitweise. Die übrigen Lüfter regeln normal weiter. Bei einem Temperaturanstieg um 10 °C oder ab 80 °C wird abgebrochen. Die Ergebnisse erscheinen nur als Vorschlag.\n\n{note}Starten?",
  "cool.cal.pumpNote": "Hinweis: Hängt an einem TL-Anschluss eine Pumpe, muss sie als Typ „Pumpe“ eingetragen sein. Ein Lüfter wird hier bis 0 % abgesenkt, eine Pumpe würde stehen.",
  "cool.cal.pumpNoteGeneric": "Hinweis: Hängt an diesem Anschluss eine Pumpe, nicht kalibrieren: Die Kalibrierung hält Lüfter bis zum Stillstand an.",
  "cool.cal.allAsk": "Alle {c} Lüfter und Gehäuselüfter werden nacheinander kalibriert, jeweils wie bei der Einzelkalibrierung (meist unter 10 Minuten je Lüfter). Die Pumpe gehört nicht dazu. Es wird immer nur ein Lüfter kalibriert, die übrigen regeln normal weiter. Zwischen zwei Lüftern liegt eine Abkühlpause von 30 s. Abbrechen beendet den ganzen Durchlauf: verwerfen oder die fertigen Lüfter behalten.\n\n{note}Starten?",
  "cool.cal.pumpAsk": "ACHTUNG, Pumpe: {n} wird von 100 % in 5-%-Schritten abgesenkt, nie unter {f} %. Beim ersten Anzeichen eines Einbruchs der Drehzahl geht sie sofort auf 100 % zurück. Währenddessen sinkt der Kühlmitteldurchfluss. Nur bei geringer Last starten. Temperaturanstieg und Pumpenalarm werden überwacht. Starten?",
  "cool.cal.done": "{n}: Kalibrierung abgeschlossen. Der Vorschlag steht im Seitenbereich.",
  "cool.cal.doneAll": "{k} von {c} Lüftern kalibriert. Die Vorschläge stehen in den Seitenbereichen.",
  "cool.cal.outcomeFail": "{n}: {e}",
  "cool.cal.canceled": "Abgebrochen. Nichts gespeichert.",
  "cool.cal.canceledKept": "Abgebrochen. {k} fertige Lüfter gespeichert.",
  "cool.cal.fail": "{n}: Kalibrierung beendet, nichts übernommen: {e}",
  "cool.cal.tableFail": "{n}: Drehzahltabelle nicht übernommen, Kalibrierung gespeichert: {e}",
  "cool.cal.stopsAt": "Stoppwert: {v} % (niedrigster Wert mit stabiler Drehzahl)",
  "cool.cal.never": "Stoppt nicht: läuft auch bei 0 % weiter.",
  "cool.cal.unheld": "Stillstand nicht haltbar: Der Lüfter lief trotz wiederholtem Stopp von selbst wieder an.",
  "cool.cal.startsAt": "Startwert: {v} % (läuft nach Stillstand dauerhaft an)",
  "cool.cal.noStart": "Kein Anlauf bis 50 %: Lüfter prüfen.",
  "cool.cal.when": "Kalibriert am {d}",
  "cool.cal.suggestStand": "Vorschlag: Stopp bei {a} %, Start mit {b} %",
  "cool.cal.suggestMin": "Vorschlag: Minimum {m} %",
  "cool.cal.suggestNoStand": "Kein Vorschlag für Stillstand: Ohne gefundenen Wiederanlauf ist er nicht sicher.",
  "cool.cal.caseNote": "Gehäuselüfter laufen im Betrieb immer: Vorgeschlagen wird nur ein Minimum.",
  "cool.cal.apply": "Übernehmen",
  "cool.cal.applied": "Übernommen. Zum Anwenden speichern.",
  "cool.cal.appliedSafe": "Übernommen, Sicherheitswert auf {s} % angehoben. Zum Anwenden speichern.",
  "cool.cal.raiseSafe": "Der Sicherheitswert ({s} %) wird mit auf {m} % angehoben.",
  "cool.cal.pumpMin": "Niedrigster stabiler Wert: {v} %",
  "cool.cal.pumpLimit": "Bis zur Untergrenze ({v} %) stabil: kein Einbruch in allen Stufen.",
  "cool.why.running": "Eine Kalibrierung läuft.",
  "cool.why.dirty": "Zuerst die Änderungen speichern.",
  "cool.why.noCurve": "Der Ausgang braucht eine Kurve oder einen manuellen Wert.",
  "cool.why.preview": "Nur im Regelbetrieb möglich (Einstellungen: CaseHUD regelt die Lüfter).",
  "cool.why.blocked": "Ein anderes Lüfterprogramm ist zuständig.",
  "cool.why.notReleased": "Der Mainboard-Anschluss der Pumpe ist noch nicht freigegeben.",
  "cool.why.matching": "Der Abgleich der Pumpe läuft noch.",
  "cool.why.notMatched": "Der Abgleich der Pumpe ist fehlgeschlagen: Sensor und Anschluss gehören nicht zusammen.",
  "cool.why.pending": "Der Abgleich der Pumpe steht noch aus.",
  "cool.why.pumpDown": "Die Pumpe steht. Zuerst beheben.",
  "cool.why.noFans": "Kein Lüfter an einer Kurve.",
  "cool.why.wait": "Der Zustand wird geladen.",
  "cool.problem.noSensor": "„{n}“: Lüfter ohne Sensor.",
  "cool.problem.unmeasured": "„{n}“ gibt eine Drehzahl vor, aber „{o}“ ist nicht kalibriert.",
  "cool.problem.pumpSensor": "„{n}“: Drehzahlsensor erforderlich.",
  "cool.problem.pumpRpm": "Die Pumpe „{n}“ kann keiner Drehzahl-Kurve folgen.",
  "cool.modeLive": "CaseHUD regelt die Lüfter",
  "cool.kind": "Art der Kurve",
  "cool.kind.drawn": "Gezeichnete Linie",
  "cool.kind.straight": "Gerade Linie",
  "cool.kind.gap": "Abstand",
  "cool.kind.fixed": "Fester Wert",
  "cool.kind.switch": "Schalter",
  "cool.kind.thermostat": "Thermostat",
  "cool.kind.combined": "Kombination",
  "cool.kind.follow": "Folgt einem Lüfter",
  "cool.kindHint.drawn": "Beliebig viele Punkte, durch Geraden verbunden.",
  "cool.kindHint.straight": "Eine gerade Linie zwischen zwei Punkten.",
  "cool.kindHint.gap": "Folgt der Differenz zweier Sensoren, z. B. Kühlmittel über Raumluft; gilt unabhängig von der Jahreszeit. Erstes Kabel: Messwert, zweites Kabel: Bezug.",
  "cool.kindHint.fixed": "Konstanter Wert, kein Sensor nötig.",
  "cool.reference": "Bezugssensor",
  "cool.referencePick": "Sensor wählen…",
  "cool.problem.noReference": "„{n}“ hat noch keinen Bezugssensor.",
  "cool.problem.sameReference": "„{n}“: Der Bezugssensor darf nicht auch gemessen werden.",
  "cool.kindHint.switch": "Zwei Stufen: Hochschalten ab Lasttemperatur, Zurückschalten erst bei Leerlauftemperatur. Dazwischen bleibt die Stufe unverändert.",
  "cool.kindHint.thermostat": "Ermittelt automatisch den niedrigsten Wert, der die Zieltemperatur hält.",
  "cool.kindHint.combined": "Verrechnet mehrere Kurven. Bei Sensorausfall in einer Kurve wechseln die Lüfter auf die sichere Drehzahl.",
  "cool.kindHint.follow": "Wie ein anderer Lüfter, optional mit Abstand.",
  "cool.linearFrom": "Unten: Temperatur",
  "cool.linearFromValue": "Unten: Wert",
  "cool.linearTo": "Oben: Temperatur",
  "cool.linearToValue": "Oben: Wert",
  "cool.flatValue": "Wert",
  "cool.idleTemp": "Leerlauf bis (°C)",
  "cool.loadTemp": "Last ab (°C)",
  "cool.idleValue": "Wert im Leerlauf",
  "cool.loadValue": "Wert unter Last",
  "cool.autoTarget": "Zieltemperatur (°C)",
  "cool.autoIdle": "Ruhig bis (°C)",
  "cool.autoMin": "Kleinster Wert",
  "cool.autoMax": "Größter Wert",
  "cool.autoStep": "Schritt",
  "cool.autoDeadband": "Spielraum unter dem Ziel (°C)",
  "cool.autoReact": "Reagiert nach (s)",
  "cool.combineFn": "Rechnung",
  "cool.combine.max": "Größter Wert",
  "cool.combine.min": "Kleinster Wert",
  "cool.combine.sum": "Summe",
  "cool.combine.average": "Mittelwert",
  "cool.combine.subtract": "Erste minus Rest",
  "cool.combineInputs": "Kurven",
  "cool.combineAdd": "Kurve hinzufügen…",
  "cool.combineOf": "{f} aus {n}",
  "cool.followWhom": "Folgt",
  "cool.followPick": "Lüfter wählen…",
  "cool.followOffset": "Abstand",
  "cool.followShare": "Abstand als Anteil in %",
  "cool.follows": "folgt {n}",
  "cool.followsOffset": "folgt {n}, {o}",
  "cool.flatShort": "immer {v}",
  "cool.triggerShort": "ab {l}°: {a} · bis {i}°: {b}",
  "cool.autoShort": "hält {l}°, {a} bis {b}",
  "cool.colDerived": "Abgeleitet",
  "cool.offset": "Versatz (%)",
  "cool.endsStrict": "Abstand auch an den Enden",
  "cool.problem.combineEmpty": "„{n}“ hat noch keine Kurven.",
  "cool.problem.combineUnit": "„{n}“ mischt Prozent und Drehzahl.",
  "cool.problem.loop": "„{n}“ verweist auf sich selbst.",
  "cool.problem.syncNone": "„{n}“ folgt noch keinem Lüfter.",
  "cool.problem.syncFree": "„{n}“ folgt „{o}“, der mit keiner Kurve verbunden ist.",
  "cool.problem.idleLoad": "„{n}“: Die untere Temperatur muss unter der oberen liegen.",
  "cool.problem.deadband": "„{n}“: Der Spielraum ist größer als der Abstand zwischen den Temperaturen.",
  "cool.problem.range": "„{n}“: Der kleinste Wert muss unter dem größten liegen.",
  "cool.problem.step": "„{n}“: Der Schritt muss größer als 0 sein.",
  "cool.save": "Speichern",
  "cool.reset": "Verwerfen",
  "cool.saved": "Gespeichert",
  "cool.goLive": "Ab dem Speichern steuert CaseHUD die Lüfter. Andere Lüfterprogramme müssen beendet sein. Kurven und Sensoren vorher im Testbetrieb prüfen. Fortfahren?",
  "cool.leaveLive": "Beim Speichern wechseln die Lüfter auf ihre sichere Drehzahl und behalten sie, bis ein anderes Programm sie regelt. Fortfahren?",
  "cool.preset": "Preset",
  "cool.preset.custom": "Angepasst",
  "cool.preset.saveAs": "Speichern unter…",
  "cool.preset.manage": "Verwalten",
  "cool.preset.name": "Name",
  "cool.preset.ok": "Speichern",
  "cool.preset.cancel": "Abbrechen",
  "cool.preset.overwrite": "Überschreiben",
  "cool.preset.rename": "Umbenennen",
  "cool.preset.delete": "Löschen",
  "cool.preset.active": "Aktiv",
  "cool.preset.askOverwrite": "„{n}“ mit der aktuellen Konfiguration überschreiben?",
  "cool.preset.askDelete": "„{n}“ löschen?",
  "cool.preset.askDiscard": "Ungespeicherte Änderungen gehen beim Wechsel verloren. Fortfahren?",
  "cool.preset.saveFirst": "Erst speichern",
  "cool.preset.limit": "Höchstens {n} Presets",
  "cool.preset.activated": "Preset „{n}“ aktiv",
  "cool.preset.stored": "Gespeichert",
  "cool.preset.deleted": "Gelöscht",
  "cool.preset.error": "Presets nicht lesbar, nichts wird überschrieben: {e}",
  "cool.stale": "Einstellungen zwischenzeitlich geändert (z. B. durch Presetwechsel). Eigene Änderungen sind noch vorhanden.",
  "cool.reload": "Neu laden, Änderungen verwerfen",
  "cool.conflict": "Nicht gespeichert: Die Einstellungen haben sich geändert.",
});
Object.assign(STRINGS.en, {
  "tab.cooling": "Cooling",
  "cool.sub": "Test mode: calculation only, no output to fans or pump. The current fan control stays active.",
  "cool.subLive": "Active control by curves. If CaseHUD fails, the mainboard takes over a pump connected to it.",
  "cool.blocked.program": "{p} is running. No output to fans; CaseHUD takes over once the program has exited.",
  "cool.blocked.disclaimer": "No output to fans until the disclaimer is confirmed.",
  "cool.blocked.check": "No output to fans: check for other fan programs not possible.",
  "cool.hint": "Pull cables from port to port. Clicking a cable cuts it, clicking a block shows its settings.",
  "cool.colSensors": "Sensors",
  "cool.panelSensor": "Sensor", "cool.panelCurve": "Curve", "cool.panelFan": "Fan", "cool.panelPump": "Pump",
  "cool.colCurves": "Curves",
  "cool.colOutputs": "Fans and pump",
  "cool.fanName": "Port {p} · Fan {f}",
  "cool.pumpName": "Pump",
  "cool.curveName": "Curve {n}",
  "cool.curveAdd": "New curve",
  "cool.curveDrop": "No curve. Pull a cable from a sensor to here to create one.",
  "cool.sensorDrop": "No sensor on the plan.",
  "cool.sensorAdd": "Add sensor…",
  "cool.sensorPick": "Choose a sensor…",
  "cool.sensorMissing": "Not available: {k}",
  "cool.pumpAdd": "Add a pump",
  "cool.type": "Type",
  "cool.type.fan": "Fan",
  "cool.type.case": "Case fan (always runs)",
  "cool.type.pump": "Pump",
  "cool.mayStand": "May stand still",
  "cool.mayStandHint": "The fan stops while the curve specifies {p} % or less (set it to 0 at the cold end for this). On overtemperature or sensor loss it still starts.",
  "cool.standing": "Stopped",
  "cool.addAll": "{g}: add all",
  "cool.groupCount": "{n} sensors, the hottest counts",
  "cool.groupClose": "Collapse {g}",
  "cool.group.ram": "Memory",
  "cool.group.cpu": "Processor",
  "cool.group.gpu": "Graphics card",
  "cool.group.drive": "Drives",
  "cool.group.board": "Mainboard",
  "cool.group.other": "Other",
  "cool.problem.pumpCurve": "Pump “{n}” on the fan controller: curve required.",
  "cool.problem.pumpHeader": "Pump “{n}” cannot be controlled: its speed sensor does not belong to a mainboard header. Disconnect the curve to monitor it only.",
  "cool.problem.pumpLow": "Pump “{n}”: at least 30 % required.",
  "cool.problem.caseStops": "Case fan “{n}” always runs: minimum above 0 % required.",
  "cool.tidy": "Tidy up",
  "cool.free": "No curve",
  "cool.unregulated": "Not controlled",
  "cool.sensorGone": "Sensor missing",
  "cool.subIdle": "Active control is on, but no fan is connected to a curve. The fans keep their last speed.",
  "cool.parkAsk": "{n} will not be connected to any curve afterwards: safe speed, no control, no monitoring. Save anyway?",
  "cool.waiting": "Waiting for the first cycle…",
  "cool.wouldSet": "would set {d} %",
  "cool.wouldRpm": "would set {t} RPM ({d} %)",
  "cool.target": "Target {t} RPM",
  "cool.targetActual": "Target {t} RPM · Actual {a} RPM",
  "cool.manualWould": "would set manually {v} {u}",
  "cool.heldValue": "held {v}",
  "cool.cal.othersPreview": "The other fans stay unchanged.",
  "cool.status.pending": "Saved: active control, not in effect at the moment",
  "cool.watchOnly": "Watched only",
  "cool.alarm.pump": "{n} is not running, probably defective. Reduce load and check the pump.",
  "cool.alarm.pumpUnknown": "Speed of {n} not readable. Pump operation cannot be verified.",
  "cool.alarm.stale": "Fan control not responding. Fans keep their last value.",
  "cool.alarm.config": "Fan control inactive: settings could not be loaded.",
  "cool.alarm.program": "{p} is running, CaseHUD is not controlling. Fans stay at safe speed or above until {p} takes over or exits.",
  "cool.alarm.check": "No control: check for other fan programs not possible.",
  "cool.alarm.disclaimer": "No control until the disclaimer is confirmed.",
  "cool.alarm.device": "Fan controller not reachable. {n} and other fans are not controlled.",
  "cool.alarm.write": "{n} does not accept a speed.",
  "cool.alarm.sensor": "Sensor for {n} missing or not delivering new values. Fans at safe speed.",
  "cool.alarm.stalled": "{n} is stopped despite a speed target.",
  "cool.alarm.service": "Sensor service not responding: temperatures and speeds are missing. Fans at safe speed. Check the “CaseHUD Sensors” service in Windows Services.",
  "cool.alarm.configBackup": "cooling.json damaged, running on the last backup. Damaged file kept as {n} in the data folder. Check the settings and save again.",
  "cool.alarm.halted": "Fan control stopped while CaseHUD keeps running. Fans (and pump) are not controlled. Restart CaseHUD.",
  "cool.alarmOpen": "Open cooling",
  "cool.stalled": "Stopped despite target",
  "cool.writeFailed": "Does not accept a speed",
  "cool.sensorLost": "Sensor missing, safe speed",
  "cool.pumpDown": "Not running, probably defective",
  "cool.emergency": "Overheating, full speed",
  "cool.unguarded": "no temperature guard",
  "cool.alarm.emergency": "Overheating: {n} at full speed. Reduce load and check the cooling.",
  "cool.notReleased": "Not released",
  "cool.releaseHint": "The mainboard controls the pump until the header is released.",
  "cool.release": "Release header",
  "cool.notMatched": "Not matched",
  "cool.matching": "Matching",
  "cool.matchHint": "The speed sensor and the header could not be shown to belong together. The mainboard keeps controlling the pump.",
  "cool.recheck": "Check again",
  "cool.recheckSave": "Save first",
  "cool.control": "Control channel",
  "cool.controlDerived": "Derived from the speed sensor ({k})",
  "cool.controlMissing": "Channel not found ({k})",
  "cool.channel": "Channel {n}",
  "cool.controlHint": "Only channels of the same chip. A new choice is checked after saving and needs the header to be released.",
  "cool.rpmUnknown": "Pump speed unknown",
  "cool.deviceLost": "Controller not reachable",
  "cool.held": "Being calibrated",
  "cool.badfile": "Settings could not be loaded, no control: {e}",
  "cool.noFans": "No fan controller found.",
  "cool.fansHeld": "Fan list appears after the disclaimer is confirmed and no other fan program is running.",
  "cool.close": "Close",
  "cool.remove": "Remove",
  "cool.disconnect": "Disconnect",
  "cool.name": "Name",
  "cool.current": "Now",
  "cool.usedBy": "Feeds",
  "cool.connectCurve": "Connect to curve…",
  "cool.removeSensor": "Take off the plan",
  "cool.unit": "The curve asks for",
  "cool.unitPercent": "Percent",
  "cool.unitRpm": "Speed",
  "cool.unitPercentHint": "All fans get the same percentage.",
  "cool.unitRpmHint": "All fans reach the same speed. Converted through each fan's calibration; calibration is required.",
  "cool.curve": "Shape",
  "cool.curveHint": "Drag the points. Double-click adds a point, right-click removes it.",
  "cool.curveTemp": "Temperature in °C",
  "cool.pointAdd": "Add point",
  "cool.sensors": "Sensors (the hottest one counts)",
  "cool.attached": "Connected",
  "cool.attachOutput": "Connect a fan…",
  "cool.duplicate": "Duplicate",
  "cool.copyOf": "{n} (copy)",
  "cool.deleteCurve": "Delete curve",
  "cool.advanced": "Behaviour",
  "cool.riseBy": "Rise from (°C)",
  "cool.fallBy": "Fall from (°C)",
  "cool.riseAfter": "Rise after (s)",
  "cool.fallAfter": "Fall after (s)",
  "cool.lead": "Look ahead (s)",
  "cool.stepUp": "Step up (%/s)",
  "cool.stepDown": "Step down (%/s)",
  "cool.minDuty": "Minimum (%)",
  "cool.stopAt": "Stop at (%)",
  "cool.startDuty": "Start with (%)",
  "cool.safeDuty": "Safe speed (%)",
  "cool.stoppedBelow": "Counts as stopped below (RPM)",
  "cool.rpmSensor": "Pump speed sensor",
  "cool.curveOf": "Curve",
  "cool.noSource": "No temperature source for the check",
  "cool.why.noSource": "No temperature source in the configuration: add a curve with a sensor.",
  "cool.drive": "Control",
  "cool.drive.curve": "Curve",
  "cool.drive.manual": "Manual",
  "cool.manualValue": "Value",
  "cool.manualUnit": "Unit",
  "cool.manualNode": "Manual · {v} {u}",
  "cool.manualNoSpeeds": "Speed needs a calibrated fan.",
  "cool.problem.manualRange": "“{n}”: value above 0 required, at most 100 in percent.",
  "cool.problem.manualRpm": "“{n}”: speed needs a calibrated fan.",
  "cool.problem.manualMin": "“{n}”: value below the minimum ({m} %).",
  "cool.noCurve": "None (not controlled)",
  "cool.pumpHint": "With a curve or a manual value CaseHUD controls the pump; without either it stays with the mainboard and is only monitored. When CaseHUD exits or another fan program runs, the mainboard takes over again. A pump cannot be switched off: if it stops, it is defective (alarm), and all fans on curves go to their safe speed.",
  "cool.pumpHintController": "Pump on the fan controller, follows its curve. A pump cannot be switched off: if it stops, it is defective (alarm), and all fans on curves go to their safe speed.",
  "cool.pumpDefect": "{n} is not running, probably defective. No coolant circulation, all fans at safe speed. Reduce load and check the pump.",
  "cool.speeds": "Calibrated: {n} points, {a}–{b} RPM",
  "cool.noSpeeds": "Not calibrated yet.",
  "cool.cal.fan": "Calibrate fan",
  "cool.cal.pump": "Calibrate pump minimum",
  "cool.cal.all": "Calibrate all fans",
  "cool.cal.stop": "Cancel and discard",
  "cool.cal.stopKeep": "Cancel, keep finished",
  "cool.cal.stopTip": "Ends the run; nothing from it is saved.",
  "cool.cal.stopKeepTip": "Ends the run; the fans that are already finished are saved.",
  "cool.cal.title": "{n} is being calibrated",
  "cool.cal.titlePump": "{n}: calibrating the pump minimum",
  "cool.cal.fanOf": "Fan {i} of {c}",
  "cool.cal.others": "The other fans stay under control.",
  "cool.cal.othersPump": "The fans stay under control.",
  "cool.cal.progress": "Progress",
  "cool.cal.ph.down": "Down · step {s} of {S}",
  "cool.cal.ph.pump": "Pump · step {s} of {S}",
  "cool.cal.ph.stop": "Holding standstill · {h} of {H} s",
  "cool.cal.ph.start": "Restart · {d} % (up to {m} %)",
  "cool.cal.ph.wait": "Preparing",
  "cool.cal.ph.pause": "Pause before the next fan",
  "cool.cal.now": "Current step {d} % · {r} RPM",
  "cool.cal.steps": "Steps",
  "cool.cal.clear": "Remove calibration",
  "cool.noEffect": "curve not in effect",
  "cool.heldNow": "Being calibrated · {d} % · {r} RPM",
  "cool.cal.fanAsk": "{n} will be calibrated.\n\n1. Down: the speed is lowered from 100 % in steps until it stands still.\n2. Standstill: at least 30 s at 0 %.\n3. Restart: upwards in 1 % steps, at least 30 s per step.\n\nDuration: usually under 10 minutes, longer for slow fans; the fan stands still for part of it. The other fans keep running normally. The run aborts when the temperature rises by 10 °C or reaches 80 °C. The results are only offered as a suggestion.\n\n{note}Start?",
  "cool.cal.pumpNote": "Note: if a pump is connected to a TL port, it must be entered with the type “Pump”. A fan is lowered to 0 % here, a pump would stop.",
  "cool.cal.pumpNoteGeneric": "Note: if a pump is connected to this port, do not calibrate: calibration stops fans down to standstill.",
  "cool.cal.allAsk": "All {c} fans and case fans are calibrated one after the other, each like a single calibration (usually under 10 minutes per fan). The pump is not included. Only one fan is calibrated at a time; the others keep running normally. There is a cool-down pause of 30 s between two fans. Canceling ends the whole run: discard it or keep the finished fans.\n\n{note}Start?",
  "cool.cal.pumpAsk": "CAUTION, pump: {n} is lowered from 100 % in 5 % steps, never below {f} %. At the first sign of its speed collapsing it goes straight back to 100 %. The coolant flow drops meanwhile. Start only under low load. Temperature rise and pump alarm are monitored. Start?",
  "cool.cal.done": "{n}: calibration finished. The suggestion is in the side panel.",
  "cool.cal.doneAll": "{k} of {c} fans calibrated. The suggestions are in the side panels.",
  "cool.cal.outcomeFail": "{n}: {e}",
  "cool.cal.canceled": "Canceled. Nothing saved.",
  "cool.cal.canceledKept": "Canceled. {k} finished fans saved.",
  "cool.cal.fail": "{n}: calibration ended, nothing taken over: {e}",
  "cool.cal.tableFail": "{n}: speed table not taken over, calibration saved: {e}",
  "cool.cal.stopsAt": "Stop value: {v} % (lowest value with a stable speed)",
  "cool.cal.never": "Does not stop: keeps turning at 0 %.",
  "cool.cal.unheld": "Standstill not kept: the fan started again by itself despite repeated stops.",
  "cool.cal.startsAt": "Start value: {v} % (keeps turning after standstill)",
  "cool.cal.noStart": "No start up to 50 %: check the fan.",
  "cool.cal.when": "Calibrated on {d}",
  "cool.cal.suggestStand": "Suggestion: stop at {a} %, start with {b} %",
  "cool.cal.suggestMin": "Suggestion: minimum {m} %",
  "cool.cal.suggestNoStand": "No suggestion for standing still: without a restart found it is not safe.",
  "cool.cal.caseNote": "Case fans always run in operation: only a minimum is suggested.",
  "cool.cal.apply": "Apply",
  "cool.cal.applied": "Applied. Save to use it.",
  "cool.cal.appliedSafe": "Applied, safe duty raised to {s} %. Save to use it.",
  "cool.cal.raiseSafe": "The safe duty ({s} %) is raised to {m} % as well.",
  "cool.cal.pumpMin": "Lowest stable value: {v} %",
  "cool.cal.pumpLimit": "Stable down to the limit ({v} %): no collapse in any step.",
  "cool.why.running": "A calibration is running.",
  "cool.why.dirty": "Save the changes first.",
  "cool.why.noCurve": "The output needs a curve or a manual value.",
  "cool.why.preview": "Only possible in active control (settings: CaseHUD controls the fans).",
  "cool.why.blocked": "Another fan program is in charge.",
  "cool.why.notReleased": "The pump's mainboard header is not released yet.",
  "cool.why.matching": "The pump check is still running.",
  "cool.why.notMatched": "The pump check failed: sensor and header do not belong together.",
  "cool.why.pending": "The pump check is still pending.",
  "cool.why.pumpDown": "The pump is standing. Fix that first.",
  "cool.why.noFans": "No fan on a curve.",
  "cool.why.wait": "The state is loading.",
  "cool.problem.noSensor": "“{n}”: fans without a sensor.",
  "cool.problem.unmeasured": "“{n}” asks for a speed, but “{o}” is not calibrated.",
  "cool.problem.pumpSensor": "“{n}”: speed sensor required.",
  "cool.problem.pumpRpm": "The pump “{n}” cannot follow a speed curve.",
  "cool.modeLive": "CaseHUD controls the fans",
  "cool.kind": "Kind of curve",
  "cool.kind.drawn": "Drawn line",
  "cool.kind.straight": "Straight line",
  "cool.kind.gap": "Distance",
  "cool.kind.fixed": "Fixed value",
  "cool.kind.switch": "Switch",
  "cool.kind.thermostat": "Thermostat",
  "cool.kind.combined": "Combination",
  "cool.kind.follow": "Follows a fan",
  "cool.kindHint.drawn": "Any number of points, joined by straight lines.",
  "cool.kindHint.straight": "A straight line between two points.",
  "cool.kindHint.gap": "Follows the difference between two sensors, e.g. coolant above room air; independent of the season. First cable: measured sensor, second cable: reference.",
  "cool.kindHint.fixed": "Constant value, no sensor required.",
  "cool.reference": "Reference sensor",
  "cool.referencePick": "Pick a sensor…",
  "cool.problem.noReference": "“{n}” has no reference sensor yet.",
  "cool.problem.sameReference": "“{n}”: the reference sensor must not be measured as well.",
  "cool.kindHint.switch": "Two levels: switches up at the load temperature and back only at the idle temperature. In between the level is unchanged.",
  "cool.kindHint.thermostat": "Determines the lowest value that holds the target temperature.",
  "cool.kindHint.combined": "Combines several curves. If a sensor of one curve is lost, the fans go to their safe speed.",
  "cool.kindHint.follow": "Same as another fan, optionally with an offset.",
  "cool.linearFrom": "Low: temperature",
  "cool.linearFromValue": "Low: value",
  "cool.linearTo": "High: temperature",
  "cool.linearToValue": "High: value",
  "cool.flatValue": "Value",
  "cool.idleTemp": "Idle up to (°C)",
  "cool.loadTemp": "Load from (°C)",
  "cool.idleValue": "Value at idle",
  "cool.loadValue": "Value under load",
  "cool.autoTarget": "Target temperature (°C)",
  "cool.autoIdle": "Quiet up to (°C)",
  "cool.autoMin": "Lowest value",
  "cool.autoMax": "Highest value",
  "cool.autoStep": "Step",
  "cool.autoDeadband": "Margin below the target (°C)",
  "cool.autoReact": "Reacts after (s)",
  "cool.combineFn": "Calculation",
  "cool.combine.max": "Highest value",
  "cool.combine.min": "Lowest value",
  "cool.combine.sum": "Sum",
  "cool.combine.average": "Average",
  "cool.combine.subtract": "First minus the rest",
  "cool.combineInputs": "Curves",
  "cool.combineAdd": "Add a curve…",
  "cool.combineOf": "{f} of {n}",
  "cool.followWhom": "Follows",
  "cool.followPick": "Pick a fan…",
  "cool.followOffset": "Distance",
  "cool.followShare": "Distance as a share in %",
  "cool.follows": "follows {n}",
  "cool.followsOffset": "follows {n}, {o}",
  "cool.flatShort": "always {v}",
  "cool.triggerShort": "from {l}°: {a} · to {i}°: {b}",
  "cool.autoShort": "holds {l}°, {a} to {b}",
  "cool.colDerived": "Derived",
  "cool.offset": "Offset (%)",
  "cool.endsStrict": "Distance also at the ends",
  "cool.problem.combineEmpty": "“{n}” has no curves yet.",
  "cool.problem.combineUnit": "“{n}” combines percent and speed.",
  "cool.problem.loop": "“{n}” refers to itself.",
  "cool.problem.syncNone": "“{n}” follows no fan yet.",
  "cool.problem.syncFree": "“{n}” follows “{o}”, which is not connected to a curve.",
  "cool.problem.idleLoad": "“{n}”: the lower temperature must be below the upper one.",
  "cool.problem.deadband": "“{n}”: the margin is larger than the distance between the temperatures.",
  "cool.problem.range": "“{n}”: the lowest value must be below the highest.",
  "cool.problem.step": "“{n}”: the step must be above 0.",
  "cool.save": "Save",
  "cool.reset": "Discard",
  "cool.saved": "Saved",
  "cool.goLive": "From saving on, CaseHUD drives the fans. Other fan programs must be closed. Check curves and sensors in test mode first. Continue?",
  "cool.leaveLive": "On saving, the fans go to their safe speed and stay there until another program controls them. Continue?",
  "cool.preset": "Preset",
  "cool.preset.custom": "Custom",
  "cool.preset.saveAs": "Save as…",
  "cool.preset.manage": "Manage",
  "cool.preset.name": "Name",
  "cool.preset.ok": "Save",
  "cool.preset.cancel": "Cancel",
  "cool.preset.overwrite": "Overwrite",
  "cool.preset.rename": "Rename",
  "cool.preset.delete": "Delete",
  "cool.preset.active": "Active",
  "cool.preset.askOverwrite": "Overwrite “{n}” with the current configuration?",
  "cool.preset.askDelete": "Delete “{n}”?",
  "cool.preset.askDiscard": "Unsaved changes are lost when switching. Continue?",
  "cool.preset.saveFirst": "Save first",
  "cool.preset.limit": "At most {n} presets",
  "cool.preset.activated": "Preset “{n}” active",
  "cool.preset.stored": "Saved",
  "cool.preset.deleted": "Deleted",
  "cool.preset.error": "Presets unreadable, nothing is overwritten: {e}",
  "cool.stale": "Settings changed in the meantime (e.g. by a preset switch). Unsaved edits are still present.",
  "cool.reload": "Reload, discard changes",
  "cool.conflict": "Not saved: the settings have changed.",
});
applyI18n();

// ---------- state ----------
let coolSetup = null;     // the setup being edited; null until loaded
let coolView = null;      // the latest state of the loop
let coolDirty = false;
let coolLive = false;
let coolBlocked = false;
let coolMeasuring = false;
let coolLoading = false;
let coolSel = null;       // selected block: {kind, id}
let coolOpenGroups = new Set(); // groups of sensors shown one by one
let coolSensorKeys = [];  // sensors on the plan, in the order shown
let coolOutputIds = [];   // outputs in the order shown
let coolParts = new Map(); // "kind:id" -> the block and its ports
let coolCables = [];      // what is drawn between the blocks
let coolObserver = null;
let coolPanelEditor = null;
let coolDragging = null;

// Nothing is loaded at first (and again if the first load fails): then there is nothing, not an error.
const coolCurves = () => (coolSetup ? coolSetup.config.curves : []);
const coolOutputs = () => (coolSetup ? coolSetup.config.outputs : []);
const coolCurve = (id) => coolCurves().find((c) => c.id === id);
const coolOutput = (id) => coolOutputs().find((o) => o.id === id);
const coolFansOn = (id) => coolOutputs().filter((o) => o.curve === id);
// An output follows a curve or a fixed value; with neither nothing regulates it.
const coolDriven = (o) => !!(o.curve || o.manual);
const coolManualUnit = (o) => (o.manual && o.manual.unit === "rpm" ? "rpm" : "percent");

// The duty a manual value stands for: percent as it is, a speed through the output's table.
function coolManualDuty(o) {
  const m = o.manual;
  if (!m) return null;
  if (coolManualUnit(o) === "percent") return Math.ceil(m.value);
  const sp = o.speeds || [];
  if (sp.length < 2) return null;
  if (m.value <= 0) return 0;
  if (m.value <= sp[0][1]) return sp[0][0];
  for (let i = 1; i < sp.length; i++) {
    if (m.value <= sp[i][1]) return sp[i - 1][0] + Math.ceil((m.value - sp[i - 1][1]) / (sp[i][1] - sp[i - 1][1]) * (sp[i][0] - sp[i - 1][0]));
  }
  return 100;
}

// ---------- kinds of curves ----------
const COOL_KINDS = ["drawn", "straight", "gap", "fixed", "switch", "thermostat", "combined", "follow"];
const COOL_COMBINES = ["max", "min", "sum", "average", "subtract"];
const coolKind = (c) => c.kind || "drawn";
const coolIsGap = (c) => coolKind(c) === "gap";
const coolHasShape = (c) => coolKind(c) === "drawn" || coolKind(c) === "straight" || coolIsGap(c);
const coolTakesSensors = (c) => coolHasShape(c) || coolKind(c) === "switch" || coolKind(c) === "thermostat";
const coolIsCombined = (c) => coolKind(c) === "combined";
// A curve keeps its sensors while another kind is tried out, but they do not count then.
const coolSources = (c) => (!coolTakesSensors(c) ? [] : coolIsGap(c) && c.reference ? [...c.sources, c.reference] : c.sources);
const coolCombinedWith = (id) => coolCurves().filter((c) => coolIsCombined(c) && (c.inputs || []).includes(id));

// The curves this one takes its value from.
function coolNeeds(c) {
  if (coolIsCombined(c)) return (c.inputs || []).map(coolCurve).filter(Boolean);
  if (coolKind(c) === "follow") {
    const o = coolOutput(c.follow);
    const on = o && coolCurve(o.curve);
    return on ? [on] : [];
  }
  return [];
}

function coolEndsUpOnItself(c) {
  const walk = (x, seen) => coolNeeds(x).some((n) => n === c || (!seen.has(n) && (seen.add(n), walk(n, seen))));
  return walk(c, new Set());
}

function coolLacksSensor(c, seen = new Set()) {
  if (seen.has(c)) return false;
  seen.add(c);
  return (coolTakesSensors(c) && !c.sources.length) || (coolIsGap(c) && !c.reference) || coolNeeds(c).some((n) => coolLacksSensor(n, seen));
}

function coolSetKind(c, kind) {
  const was = coolKind(c);
  if (was === kind) return;
  const first = c.points[0], last = c.points[c.points.length - 1];
  c.kind = kind;
  if (kind === "straight") c.points = [first, last];
  // Distances are small numbers, a shape for temperatures would never move.
  if (kind === "gap" && last[0] > 40) c.points = c.unit === "rpm" ? [[0, 500], [5, 700], [10, 1100], [15, 1600]] : [[0, 25], [5, 35], [10, 60], [15, 100]];
  if (kind === "gap") c.sources = c.sources.filter((k) => k !== c.reference);
  if (kind === "fixed" && !c.value) c.value = Math.round(coolCurveValue(c, (first[0] + last[0]) / 2));
  if (kind === "switch" || kind === "thermostat") {
    if (!c.load_temp) c.load_temp = Math.round(first[0] + (last[0] - first[0]) * 0.7);
    if (!c.idle_temp) c.idle_temp = Math.round(first[0] + (last[0] - first[0]) * 0.3);
    if (!c.load_value) c.load_value = last[1];
    if (!c.idle_value) c.idle_value = Math.min(first[1], last[1] - 1);
  }
  if (kind === "thermostat") {
    if (!c.step) c.step = c.unit === "rpm" ? 50 : 2;
    if (c.deadband === undefined) c.deadband = 2;
    if (!c.react_after) c.react_after = 5;
  }
  if (kind === "combined") {
    if (!c.combine) c.combine = "max";
    if (!c.inputs) c.inputs = [];
  }
}

function coolValueText(c, v) {
  return `${Math.round(v)}${c.unit === "rpm" ? " RPM" : " %"}`;
}

// What a curve without a shape does, in a few words.
function coolCurveSummary(c) {
  const v = (x) => coolValueText(c, x || 0);
  switch (coolKind(c)) {
    case "fixed": return t("cool.flatShort", { v: v(c.value) });
    case "switch": return t("cool.triggerShort", { l: c.load_temp, a: v(c.load_value), i: c.idle_temp, b: v(c.idle_value) });
    case "thermostat": return t("cool.autoShort", { l: c.load_temp, a: v(c.idle_value), b: v(c.load_value) });
    case "combined": return t("cool.combineOf", { f: t("cool.combine." + (c.combine || "max")), n: (c.inputs || []).length });
    case "follow": {
      const o = coolOutput(c.follow);
      const name = o ? o.name || o.id : "–";
      if (!c.offset) return t("cool.follows", { n: name });
      const shown = (c.offset > 0 ? "+" : "") + (c.proportional ? c.offset + " %" : coolValueText(c, c.offset));
      return t("cool.followsOffset", { n: name, o: shown });
    }
  }
  return "";
}

// Settings of a new output or curve: a careful start that assumes no particular system.
function coolingFanTitle(id) {
  const m = /^port(\d+)-fan(\d+)$/.exec(id);
  return m ? t("cool.fanName", { p: m[1], f: m[2] }) : id;
}

function coolingNewFan(id) {
  return { id, name: coolingFanTitle(id), curve: "", step_up: 5, step_down: 3, min_duty: 20, stop_at: 0, start_duty: 0, safe_duty: 70 };
}

function coolingNewPump(id) {
  return { id, name: t("cool.pumpName"), curve: "", pump: true, rpm_sensor: "", step_up: 5, step_down: 3, min_duty: 50, safe_duty: 100, stopped_below_rpm: 0 };
}

// The shape a new curve starts with, also given to a loaded curve that has none.
const coolDefaultPoints = () => [[30, 30], [50, 45], [70, 75], [85, 100]];

function coolingNewCurve() {
  let n = coolCurves().length + 1;
  while (coolCurves().some((c) => c.id === "c" + n)) n++;
  return {
    id: "c" + n, name: t("cool.curveName", { n }), sources: [], unit: "percent",
    points: coolDefaultPoints(),
    rise_by: 2, fall_by: 3, rise_after: 3, fall_after: 10, lead_ticks: 0,
  };
}

function coolChanged() {
  coolDirty = true;
  const problems = coolingProblems();
  const save = $("#cool-save");
  if (save) save.disabled = problems.length > 0;
  const reset = $("#cool-reset");
  if (reset) reset.disabled = false;
  const box = $("#cool-problems");
  if (box) {
    box.hidden = !problems.length;
    box.replaceChildren(...problems.map((p) => el("div", {}, p)));
  }
  coolUpdateMeasureUI();
  coolPresetsDirty();
}

// What the server would refuse, said in terms of the plan.
function coolingProblems() {
  const out = [];
  if (!coolSetup) return out;
  for (const c of coolCurves()) {
    const n = c.name || c.id;
    const kind = coolKind(c);
    if (kind === "combined") {
      if (!(c.inputs || []).length) out.push(t("cool.problem.combineEmpty", { n }));
      if (coolNeeds(c).some((x) => (x.unit === "rpm") !== (c.unit === "rpm"))) out.push(t("cool.problem.combineUnit", { n }));
    }
    if (kind === "follow") {
      const o = coolOutput(c.follow);
      if (!o) out.push(t("cool.problem.syncNone", { n }));
      else if (!coolDriven(o)) out.push(t("cool.problem.syncFree", { n, o: o.name || o.id }));
    }
    if (kind === "gap" && c.reference && c.sources.includes(c.reference)) out.push(t("cool.problem.sameReference", { n }));
    if (kind === "straight") {
      const [a, b] = c.points;
      if (!(a[0] < b[0])) out.push(t("cool.problem.idleLoad", { n }));
      if (a[1] > b[1]) out.push(t("cool.problem.range", { n }));
    }
    if (kind === "switch" || kind === "thermostat") {
      if (!(c.idle_temp < c.load_temp)) out.push(t("cool.problem.idleLoad", { n }));
      if (kind === "switch" ? c.idle_value > c.load_value : !(c.idle_value < c.load_value)) out.push(t("cool.problem.range", { n }));
    }
    if (kind === "thermostat") {
      if (!(c.step > 0)) out.push(t("cool.problem.step", { n }));
      if (c.idle_temp < c.load_temp && !((c.deadband || 0) < c.load_temp - c.idle_temp)) out.push(t("cool.problem.deadband", { n }));
    }
    if (coolEndsUpOnItself(c)) out.push(t("cool.problem.loop", { n }));
    const fans = coolFansOn(c.id);
    if (!fans.length) continue;
    if (coolIsGap(c) && c.sources.length && !c.reference) out.push(t("cool.problem.noReference", { n }));
    else if (coolLacksSensor(c)) out.push(t("cool.problem.noSensor", { n }));
    if (c.unit !== "rpm") continue;
    for (const o of fans) {
      if (o.pump) out.push(t("cool.problem.pumpRpm", { n: o.name || o.id }));
      else if (!o.speeds || !o.speeds.length) out.push(t("cool.problem.unmeasured", { n: c.name || c.id, o: o.name || o.id }));
    }
  }
  for (const o of coolOutputs()) {
    const n = o.name || o.id, onController = coolOnController(o);
    if (o.pump && !onController && !o.rpm_sensor) out.push(t("cool.problem.pumpSensor", { n }));
    if (o.pump && onController && !coolDriven(o)) out.push(t("cool.problem.pumpCurve", { n }));
    if (o.manual) {
      const m = o.manual, duty = coolManualDuty(o);
      if (!(m.value > 0) || (coolManualUnit(o) === "percent" && m.value > 100)) out.push(t("cool.problem.manualRange", { n }));
      else if (duty === null) out.push(t("cool.problem.manualRpm", { n }));
      else if (o.pump && duty < 30) out.push(t("cool.problem.pumpLow", { n }));
      else if (duty < (o.min_duty || 0)) out.push(t("cool.problem.manualMin", { n, m: o.min_duty }));
    }
    // The same rules the server applies before it stores a live setup.
    if (coolSetup.config.mode === "live" && o.pump) {
      if (!onController && coolDriven(o) && o.rpm_sensor && !/^\/lpc\/[a-z0-9]{1,32}\/[0-9]{1,2}\/fan\/[0-9]{1,2}$/.test(o.rpm_sensor)) out.push(t("cool.problem.pumpHeader", { n }));
      else if ((onController || coolDriven(o)) && !(o.min_duty >= 30)) out.push(t("cool.problem.pumpLow", { n }));
    }
    if (coolOutputType(o) === "case" && !(o.min_duty >= 1)) out.push(t("cool.problem.caseStops", { n }));
  }
  return out;
}

// ---------- readings ----------
const coolTemps = () => (coolSetup ? coolSetup.temps : []);

function coolSensorValue(key) {
  // Once the loop has reported, only its readings count: a sensor it no longer
  // has is gone, not at its old value.
  if (coolView) return (coolView.sensors || {})[key];
  const s = coolTemps().find((x) => x.key === key);
  return s ? s.value : undefined;
}

// What the hardware calls a sensor: the whole way there, and its last part.
function coolSensorFull(key) {
  const s = coolTemps().find((x) => x.key === key);
  return s ? s.name : key;
}

function coolSensorShort(key) {
  return coolSensorFull(key).split(/\/(?![^(]*\))/).pop();
}

function coolSensorOwnName(key) {
  return ((coolSetup && coolSetup.config.sensor_names) || {})[key] || "";
}

function coolSensorName(key) {
  return coolSensorOwnName(key) || coolSensorShort(key);
}

// Limits and the like are listed with the temperatures, but nothing follows them.
function coolTempsOffered() {
  return coolTemps().filter((s) => !/\b(limit|resolution|critical|warning)\b/i.test(coolSensorShort(s.key)));
}

// ---------- sensors in groups ----------
function coolHardwareOf(key) {
  const first = key.split("/")[1] || "";
  if (first === "memory") return "ram";
  if (first === "amdcpu" || first === "intelcpu") return "cpu";
  if (first.startsWith("gpu")) return "gpu";
  if (first === "nvme" || first === "hdd" || first === "ssd" || first === "storage") return "drive";
  if (first === "lpc" || first === "mainboard") return "board";
  return "other";
}

// A sensor stands alone if it is more than one of many: the user named it, it
// measures the water, or a curve compares against just this one.
function coolGroupOf(key) {
  if (coolSensorOwnName(key)) return null;
  if (/\b(t ?sensor|water|coolant|liquid)\b/i.test(coolSensorShort(key))) return null;
  if (coolCurves().some((c) => c.reference === key)) return null;
  const g = coolHardwareOf(key);
  return g === "other" ? null : g; // nothing says that unknown sensors belong together
}

// The rows of the sensor column: single sensors, and groups of at least two.
function coolSensorRows() {
  const groups = new Map();
  for (const key of coolSensorKeys) {
    const g = coolGroupOf(key);
    if (g) groups.set(g, [...(groups.get(g) || []), key]);
  }
  const rows = [], placed = new Set();
  for (const key of coolSensorKeys) {
    const g = coolGroupOf(key);
    if (!g || groups.get(g).length < 2) rows.push({ key });
    else if (!placed.has(g)) {
      placed.add(g);
      rows.push({ group: g, keys: groups.get(g) });
    }
  }
  return rows;
}

function coolHottest(keys) {
  const values = keys.map(coolSensorValue);
  return values.includes(undefined) ? undefined : Math.max(...values);
}

// The hottest sensor, or null if one is missing: the fans go to their safe speed then.
function coolCurveTemp(c, seen = new Set()) {
  if (seen.has(c)) return null;
  seen.add(c);
  let top = null;
  for (const key of coolTakesSensors(c) ? c.sources : []) {
    const v = coolSensorValue(key);
    if (v === undefined) return null;
    if (top === null || v > top) top = v;
  }
  if (coolIsGap(c)) {
    const from = c.reference ? coolSensorValue(c.reference) : undefined;
    return top === null || from === undefined ? null : top - from;
  }
  for (const n of coolNeeds(c)) {
    const v = coolCurveTemp(n, seen);
    if (v !== null && (top === null || v > top)) top = v;
  }
  return top;
}

function coolCurveValue(c, temp) {
  const p = c.points;
  if (temp <= p[0][0]) return p[0][1];
  for (let i = 1; i < p.length; i++) {
    if (temp <= p[i][0]) return p[i - 1][1] + ((temp - p[i - 1][0]) / (p[i][0] - p[i - 1][0])) * (p[i][1] - p[i - 1][1]);
  }
  return p[p.length - 1][1];
}

// The colour of a temperature, for cables and sensor blocks. The number is always shown as well.
const COOL_HEAT = [[30, [34, 211, 238]], [50, [167, 139, 250]], [70, [250, 176, 5]], [90, [244, 63, 94]]];
function coolHeat(temp) {
  if (temp === null || temp === undefined) return "var(--faint-deco)";
  if (temp <= COOL_HEAT[0][0]) return `rgb(${COOL_HEAT[0][1]})`;
  for (let i = 1; i < COOL_HEAT.length; i++) {
    const [t1, c1] = COOL_HEAT[i], [t0, c0] = COOL_HEAT[i - 1];
    if (temp <= t1) {
      const f = (temp - t0) / (t1 - t0);
      return `rgb(${c0.map((v, j) => Math.round(v + (c1[j] - v) * f))})`;
    }
  }
  return `rgb(${COOL_HEAT[COOL_HEAT.length - 1][1]})`;
}

// ---------- curve drawing ----------
const CURVE_W = 336, CURVE_H = 200, CURVE_LEFT = 44, CURVE_RIGHT = 10, CURVE_TOP = 28, CURVE_BOTTOM = 26;

function svgEl(tag, attrs = {}) {
  const e = document.createElementNS("http://www.w3.org/2000/svg", tag);
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
  return e;
}

function coolRpmTop(c) {
  const on = coolFansOn(c.id).concat(coolOutputs()).filter((o) => o.speeds && o.speeds.length);
  return on.length ? on[0].speeds[on[0].speeds.length - 1][1] : 2000;
}

function curveScale(c, w = CURVE_W, h = CURVE_H, left = CURVE_LEFT, right = CURVE_RIGHT, top = CURVE_TOP, bottom = CURVE_BOTTOM) {
  const temps = c.points.map((p) => p[0]);
  const rpm = c.unit === "rpm";
  const x0 = coolIsGap(c) ? 0 : Math.min(20, ...temps), x1 = coolIsGap(c) ? Math.max(30, ...temps) : Math.max(100, ...temps);
  const y1 = rpm ? Math.max(coolRpmTop(c), ...c.points.map((p) => p[1])) : 100;
  const iw = w - left - right, ih = h - top - bottom;
  return {
    x0, x1, y1, unit: rpm ? "RPM" : "%", step: rpm ? 10 : 1,
    x: (v) => left + ((Math.min(x1, Math.max(x0, v)) - x0) / (x1 - x0)) * iw,
    y: (v) => top + ih - (v / y1) * ih,
    temp: (px) => x0 + ((px - left) / iw) * (x1 - x0),
    value: (py) => ((top + ih - py) / ih) * y1,
  };
}

function curveLine(c, s) {
  const pts = [[s.x0, c.points[0][1]], ...c.points, [s.x1, c.points[c.points.length - 1][1]]];
  return pts.map((p) => `${s.x(p[0]).toFixed(1)},${s.y(p[1]).toFixed(1)}`).join(" ");
}

function curveEditor(c, onChange) {
  const svg = svgEl("svg", { viewBox: `0 0 ${CURVE_W} ${CURVE_H}`, class: "cool-curve", role: "img" });
  let drag = -1, mark = null;
  const pointerAt = (ev) => {
    const b = svg.getBoundingClientRect();
    return [(ev.clientX - b.left) * (CURVE_W / b.width), (ev.clientY - b.top) * (CURVE_H / b.height)];
  };
  const tick = (x, y, anchor, text) => {
    const label = svgEl("text", { x, y, class: "cool-tick", "text-anchor": anchor });
    label.textContent = text;
    svg.append(label);
  };
  const place = () => {
    if (!mark) return;
    const temp = coolCurveTemp(c);
    mark.style.display = temp === null ? "none" : "";
    if (temp === null) return;
    const s = curveScale(c);
    mark.setAttribute("transform", `translate(${s.x(temp).toFixed(1)} ${s.y(coolCurveValue(c, temp)).toFixed(1)})`);
  };
  const draw = () => {
    const s = curveScale(c);
    svg.replaceChildren();
    for (let i = 0; i <= 4; i++) {
      const v = (s.y1 * i) / 4;
      svg.append(svgEl("line", { x1: CURVE_LEFT, x2: CURVE_W - CURVE_RIGHT, y1: s.y(v), y2: s.y(v), class: i ? "cool-grid" : "cool-grid cool-grid-base" }));
      tick(CURVE_LEFT - 12, s.y(v) + 4, "end", Math.round(v));
    }
    for (let v = Math.ceil(s.x0 / 20) * 20; v <= s.x1; v += 20) {
      svg.append(svgEl("line", { x1: s.x(v), x2: s.x(v), y1: CURVE_TOP, y2: CURVE_H - CURVE_BOTTOM, class: "cool-grid" }));
      const edge = s.x(v) + 14 > CURVE_W;
      tick(edge ? CURVE_W - 2 : s.x(v), CURVE_H - 10, edge ? "end" : "middle", v + "°");
    }
    tick(2, 12, "start", s.unit);
    svg.append(svgEl("polyline", { points: curveLine(c, s), class: "cool-line" }));
    mark = svgEl("g", { class: "cool-mark" });
    mark.append(svgEl("circle", { r: 9, class: "cool-mark-glow" }), svgEl("circle", { r: 4, class: "cool-mark-dot" }));
    svg.append(mark);
    place();
    c.points.forEach((p, i) => {
      const dot = svgEl("circle", { cx: s.x(p[0]), cy: s.y(p[1]), r: 7, class: "cool-dot" });
      dot.addEventListener("pointerdown", (ev) => {
        ev.preventDefault();
        svg.setPointerCapture(ev.pointerId);
        drag = i;
      });
      dot.addEventListener("contextmenu", (ev) => {
        ev.preventDefault();
        if (c.points.length > 1) {
          c.points.splice(i, 1);
          draw();
          onChange();
        }
      });
      const title = svgEl("title");
      title.textContent = `${p[0]} °C → ${p[1]} ${s.unit}`;
      dot.append(title);
      svg.append(dot);
    });
  };
  svg.addEventListener("pointermove", (ev) => {
    if (drag < 0) return;
    const s = curveScale(c);
    const [px, py] = pointerAt(ev);
    const prev = c.points[drag - 1], next = c.points[drag + 1];
    const temp = Math.round(Math.min(next ? next[0] - 1 : 125, Math.max(prev ? prev[0] + 1 : 1, s.temp(px))));
    const value = Math.min(next ? next[1] : s.y1, Math.max(prev ? prev[1] : 0, s.value(py)));
    c.points[drag] = [temp, Math.round(value / s.step) * s.step];
    draw();
    onChange();
  });
  const end = () => { drag = -1; };
  svg.addEventListener("pointerup", end);
  svg.addEventListener("pointercancel", end);
  svg.addEventListener("dblclick", (ev) => {
    const s = curveScale(c);
    const [px, py] = pointerAt(ev);
    const temp = Math.round(s.temp(px));
    let at = c.points.findIndex((p) => p[0] >= temp);
    if (at < 0) at = c.points.length;
    if (c.points[at] && c.points[at][0] === temp) return;
    const prev = c.points[at - 1], next = c.points[at];
    const value = Math.min(next ? next[1] : s.y1, Math.max(prev ? prev[1] : 0, s.value(py)));
    c.points.splice(at, 0, [temp, Math.round(value / s.step) * s.step]);
    draw();
    onChange();
  });
  draw();
  return { svg, draw, place };
}

// The same points as numbers: exact values and a way that needs no mouse.
function curveTable(c, changed) {
  const box = el("div", { class: "cool-points" });
  const fill = () => {
    const unit = c.unit === "rpm" ? "RPM" : "%";
    box.replaceChildren(...c.points.map((p) => el("span", { class: "cool-point" },
      el("input", { type: "number", value: p[0], min: 1, max: 125, "aria-label": t("cool.curveTemp"),
        onchange: (ev) => { p[0] = +ev.target.value; changed(); } }),
      el("span", { class: "cool-sep" }, "°C →"),
      el("input", { type: "number", value: p[1], min: 0, "aria-label": unit,
        onchange: (ev) => { p[1] = +ev.target.value; changed(); } }),
      el("span", { class: "cool-unit" }, unit),
    )),
    el("button", { type: "button", onclick: () => {
      const last = c.points[c.points.length - 1];
      c.points.push([Math.min(125, last[0] + 5), last[1]]);
      fill();
      changed();
    } }, t("cool.pointAdd")));
  };
  fill();
  return { box, fill };
}

function coolMiniGraph(c) {
  const W = 168, H = 44;
  const s = curveScale(c, W, H, 1, 1, 3, 3);
  const line = curveLine(c, s);
  const svg = svgEl("svg", { viewBox: `0 0 ${W} ${H}`, class: "cool-mini", "aria-hidden": "true" });
  svg.append(
    svgEl("polygon", { points: `1,${H - 3} ${line} ${W - 1},${H - 3}`, class: "cool-mini-fill" }),
    svgEl("polyline", { points: line, class: "cool-mini-line" }));
  const mark = svgEl("circle", { r: 3.5, class: "cool-mark-dot", style: "display:none" });
  svg.append(mark);
  return { svg, mark, scale: s };
}

// ---------- the plan: blocks ----------
function coolPort(side, onDown) {
  return el("span", { class: `cool-port cool-port-${side}`, "aria-hidden": "true", onpointerdown: onDown });
}

function coolBlock(kind, id, cls, children, ports) {
  const act = ports.activate || (() => coolSelect(kind, id));
  const node = el("div", {
    class: `cool-node ${cls}`, tabindex: 0, role: "button", "data-kind": kind, "data-id": id,
    onclick: (ev) => { ev.stopPropagation(); if (!coolDragging) act(); },
    onkeydown: (ev) => { if (ev.key === "Enter" || ev.key === " ") { ev.preventDefault(); act(); } },
    onpointerenter: () => { if (!coolDragging) coolHighlight(`${kind}:${id}`); },
    onpointerleave: () => { if (!coolDragging) coolHighlight(null); },
  }, children, ports.in, ports.out);
  coolParts.set(`${kind}:${id}`, { node, in: ports.in, out: ports.out });
  return node;
}

function coolSensorBlock(key) {
  const value = el("span", { class: "cool-node-value" });
  const node = coolBlock("sensor", key, "cool-sensor-node", [
    el("span", { class: "cool-node-title", title: coolSensorFull(key) }, coolSensorName(key)), value,
  ], { out: coolPort("out", (ev) => coolStartCable(ev, { kind: "sensor", id: key })) });
  coolParts.get(`sensor:${key}`).value = value;
  return node;
}

function coolGroupBlock(row) {
  const value = el("span", { class: "cool-node-value" });
  const first = row.keys[0];
  const node = coolBlock("sensor", first, "cool-sensor-node cool-group-node", [
    el("span", { class: "cool-node-title", title: row.keys.map(coolSensorFull).join("\n") }, `${t("cool.group." + row.group)} ▸`),
    value,
    el("span", { class: "cool-node-sub" }, t("cool.groupCount", { n: row.keys.length })),
  ], {
    out: coolPort("out", (ev) => coolStartCable(ev, { kind: "sensor", id: first, ids: row.keys })),
    activate: () => {
      coolOpenGroups.add(row.group);
      coolRenderBoard();
    },
  });
  const part = coolParts.get(`sensor:${first}`);
  part.value = value;
  part.members = row.keys;
  for (const key of row.keys) coolParts.set(`sensor:${key}`, part);
  return node;
}

function coolSensorRow(row) {
  if (!row.group) return coolSensorBlock(row.key);
  if (!coolOpenGroups.has(row.group)) return coolGroupBlock(row);
  const name = t("cool.group." + row.group);
  return el("div", { class: "cool-group-open" },
    el("button", { type: "button", class: "cool-group-head", title: t("cool.groupClose", { g: name }), onclick: () => {
      coolOpenGroups.delete(row.group);
      coolRenderBoard();
    } }, `${name} ▾`),
    row.keys.map(coolSensorBlock));
}

function coolCurveBlock(c) {
  const mini = coolHasShape(c) ? coolMiniGraph(c) : null;
  const summary = mini ? null : el("span", { class: "cool-node-kind" }, coolCurveSummary(c));
  const readout = el("span", { class: "cool-node-sub" });
  const takesCables = coolTakesSensors(c) || coolIsCombined(c);
  const node = coolBlock("curve", c.id, "cool-curve-node", [
    el("span", { class: "cool-node-title" }, c.name || c.id),
    mini ? mini.svg : summary, readout,
  ], {
    in: takesCables ? coolPort("in", (ev) => coolStartCable(ev, { kind: "curve-in", id: c.id })) : null,
    out: coolPort("out", (ev) => coolStartCable(ev, { kind: "curve", id: c.id })),
  });
  Object.assign(coolParts.get(`curve:${c.id}`), { mini, summary, readout });
  return node;
}

const COOL_ROTOR = "M12 10c0-4 1-7 4-7s3 4 0 6l-4 1zM14 12c4 0 7 1 7 4s-4 3-6 0l-1-4zM12 14c0 4-1 7-4 7s-3-4 0-6l4-1zM10 12c-4 0-7-1-7-4s4-3 6 0l1 4z";
const COOL_DROP = "M12 3c3.5 4.4 6 7.7 6 11a6 6 0 0 1-12 0c0-3.3 2.5-6.6 6-11z";

function coolOutputBlock(o) {
  const icon = svgEl("svg", { viewBox: "0 0 24 24", class: "cool-rotor", "aria-hidden": "true" });
  icon.append(svgEl("path", { d: o.pump ? COOL_DROP : COOL_ROTOR }));
  const sub = el("span", { class: "cool-node-sub" });
  const bar = el("span", { class: "cool-duty-bar" }, el("i"));
  const node = coolBlock("output", o.id, "cool-output-node" + (o.pump ? " cool-pump-node" : ""), [
    icon,
    el("span", { class: "cool-node-text" }, el("span", { class: "cool-node-title", title: o.id }, o.name || o.id), sub),
    bar,
  ], { in: coolPort("in", (ev) => coolStartCable(ev, { kind: "output", id: o.id })) });
  Object.assign(coolParts.get(`output:${o.id}`), { sub, bar: bar.firstChild, icon });
  return node;
}

// The order that lets the cables run straight: every block next to what it hangs on.
function coolTidy(dropUnused) {
  const curves = coolCurves();
  const at = new Map(curves.map((c, i) => [c.id, i]));
  const used = new Map();
  curves.forEach((c, i) => {
    for (const key of coolSources(c)) {
      if (!used.has(key)) used.set(key, []);
      used.get(key).push(i);
    }
  });
  const mean = (l) => l.reduce((a, b) => a + b, 0) / l.length;
  const connected = [...used.keys()].sort((a, b) => mean(used.get(a)) - mean(used.get(b)));
  const extras = dropUnused ? [] : coolSensorKeys.filter((k) => !used.has(k));
  coolSensorKeys = [...connected, ...extras];
  const rank = (o) => (o.curve && at.has(o.curve) ? at.get(o.curve) : curves.length) * 2 + (o.pump ? 1 : 0);
  coolOutputIds = coolOutputs().map((o, i) => [o, i]).sort((a, b) => rank(a[0]) - rank(b[0]) || a[1] - b[1]).map((x) => x[0].id);
}

function coolRenderBoard() {
  const board = $("#cool-board");
  if (!board || !coolSetup) return;
  coolParts = new Map();
  const onPlan = new Set(coolSensorKeys);
  const free = coolTempsOffered().filter((s) => !onPlan.has(s.key));
  for (const o of coolOutputs()) if (!coolOutputIds.includes(o.id)) coolOutputIds.push(o.id);
  coolOutputIds = coolOutputIds.filter((id) => coolOutput(id));
  const hasPump = !!coolOutput("pump");
  const head = (key) => el("h3", { class: "cool-col-head" }, t(key));
  const addSensor = el("select", { "aria-label": t("cool.sensorAdd"), onchange: (ev) => {
    const v = ev.target.value;
    if (v.startsWith("group:")) coolSensorKeys.push(...free.filter((s) => coolHardwareOf(s.key) === v.slice(6)).map((s) => s.key));
    else if (v) coolSensorKeys.push(v);
    coolRenderBoard();
  } }, el("option", { value: "" }, t("cool.sensorAdd")), [...new Set(free.map((s) => coolHardwareOf(s.key)))].map((g) => {
    const list = free.filter((s) => coolHardwareOf(s.key) === g);
    return el("optgroup", { label: t("cool.group." + g) },
      list.length > 1 ? el("option", { value: "group:" + g }, t("cool.addAll", { g: t("cool.group." + g) })) : null,
      list.map((s) => el("option", { value: s.key, title: s.name }, `${coolSensorName(s.key)} (${Math.round(s.value)} °C)`)));
  }));
  const cables = svgEl("svg", { class: "cool-cables", "aria-hidden": "true" });
  // Curves built from others get a column of their own, so their cables run forwards.
  const built = coolCurves().filter((c) => coolIsCombined(c) || coolKind(c) === "follow");
  const plain = coolCurves().filter((c) => !built.includes(c));
  board.classList.toggle("cool-has-derived", built.length > 0);
  board.replaceChildren(cables,
    el("div", { class: "cool-col cool-col-sensors" }, head("cool.colSensors"),
      el("div", { class: "cool-col-body" }, coolSensorKeys.length ? coolSensorRows().map(coolSensorRow) : el("p", { class: "cool-empty" }, t("cool.sensorDrop"))),
      el("div", { class: "cool-col-foot" }, free.length ? addSensor : null)),
    el("div", { class: "cool-col cool-col-curves" }, head("cool.colCurves"),
      el("div", { class: "cool-col-body" }, plain.length ? plain.map(coolCurveBlock) : el("p", { class: "cool-empty" }, t("cool.curveDrop"))),
      el("div", { class: "cool-col-foot" }, el("button", { type: "button", onclick: () => {
        const c = coolingNewCurve();
        coolCurves().push(c);
        coolChanged();
        coolSel = { kind: "curve", id: c.id };
        coolRenderBoard();
        coolRenderPanel();
      } }, "+ " + t("cool.curveAdd")))),
    ...(built.length ? [el("div", { class: "cool-col cool-col-derived" }, head("cool.colDerived"),
      el("div", { class: "cool-col-body" }, built.map(coolCurveBlock)),
      el("div", { class: "cool-col-foot" }))] : []),
    el("div", { class: "cool-col cool-col-outputs" }, head("cool.colOutputs"),
      el("div", { class: "cool-col-body" }, coolOutputIds.map((id) => coolOutputBlock(coolOutput(id))),
        coolOutputs().length ? null : el("p", { class: "cool-empty" }, t(coolSetup.fansHeld ? "cool.fansHeld" : "cool.noFans"))),
      el("div", { class: "cool-col-foot" }, hasPump ? null : el("button", { type: "button", onclick: () => {
        const p = coolingNewPump("pump");
        coolOutputs().push(p);
        coolChanged();
        coolSel = { kind: "output", id: p.id };
        coolRenderBoard();
        coolRenderPanel();
      } }, "+ " + t("cool.pumpAdd"))),
    ),
  );
  coolDrawCables();
  coolApplyLive();
  coolHighlight(null);
}

// One decimal, the decimal separator of the interface language, a space before the unit: 35,7 °C.
function coolDeg(v) {
  return v.toLocaleString(LANG === "de" ? "de-DE" : "en-GB", { minimumFractionDigits: 1, maximumFractionDigits: 1 }) + " °C";
}

// ---------- the plan: cables ----------
// The cables are drawn inside the border of the board.
function coolBoardBox(board) {
  const r = board.getBoundingClientRect();
  // The cables live in the scrolled content: on a phone the board scrolls sideways inside its frame.
  return { left: r.left + board.clientLeft - board.scrollLeft, top: r.top + board.clientTop - board.scrollTop };
}

function coolPortAt(port, box) {
  const r = port.getBoundingClientRect();
  return [r.left + r.width / 2 - box.left, r.top + r.height / 2 - box.top];
}

function coolCablePath(a, b) {
  const pull = Math.max(36, Math.abs(b[0] - a[0]) * 0.5);
  return `M${a[0]} ${a[1]} C${a[0] + pull} ${a[1]} ${b[0] - pull} ${b[1]} ${b[0]} ${b[1]}`;
}

function coolDrawCables() {
  const board = $("#cool-board");
  const svg = board && board.querySelector(".cool-cables");
  if (!svg || !board.offsetWidth) return;
  svg.style.width = "";
  svg.style.width = board.scrollWidth + "px";
  const box = coolBoardBox(board);
  svg.replaceChildren();
  coolCables = [];
  const add = (fromKey, toKey, def) => {
    const from = coolParts.get(fromKey), to = coolParts.get(toKey);
    if (!from || !to) return;
    const d = coolCablePath(coolPortAt(from.out, box), coolPortAt(to.in, box));
    const hit = svgEl("path", { d, class: "cool-cable-hit" });
    const base = svgEl("path", { d, class: "cool-cable" });
    const flow = svgEl("path", { d, class: "cool-cable-flow" });
    hit.addEventListener("click", (ev) => {
      ev.stopPropagation();
      def.cut();
      coolChanged();
      coolRenderBoard();
      coolRenderPanel();
    });
    const cable = { ...def, fromKey, toKey, base, flow, hit, group: svgEl("g", { class: "cool-cable-group" }) };
    hit.addEventListener("pointerenter", () => cable.group.classList.add("cool-cable-over"));
    hit.addEventListener("pointerleave", () => cable.group.classList.remove("cool-cable-over"));
    cable.group.append(base, flow, hit);
    svg.append(cable.group);
    coolCables.push(cable);
  };
  for (const c of coolCurves()) {
    const drawn = new Set(); // a closed group has one cable to a curve, however many sensors it brings
    for (const key of coolSources(c)) {
      const part = coolParts.get(`sensor:${key}`);
      const keys = part && part.members ? part.members.filter((k) => coolSources(c).includes(k)) : [key];
      if (drawn.has(keys[0])) continue;
      drawn.add(keys[0]);
      add(`sensor:${key}`, `curve:${c.id}`, { type: "sensor", curve: c, keys, cut: () => {
        c.sources = c.sources.filter((k) => !keys.includes(k));
        if (keys.includes(c.reference)) c.reference = "";
      } });
    }
    if (coolIsCombined(c)) {
      for (const id of c.inputs || []) {
        if (coolCurve(id)) add(`curve:${id}`, `curve:${c.id}`, { type: "input", curve: coolCurve(id), cut: () => { c.inputs = c.inputs.filter((k) => k !== id); } });
      }
    }
  }
  for (const o of coolOutputs()) {
    if (o.curve) add(`curve:${o.curve}`, `output:${o.id}`, { type: "output", curve: coolCurve(o.curve), output: o, cut: () => { o.curve = ""; } });
  }
  coolApplyCables();
}

function coolOutputState(o) {
  const v = coolView && (coolView.outputs || []).find((x) => x.id === o.id);
  return v || null;
}

function coolApplyCables() {
  for (const cable of coolCables) {
    let temp, level = 0;
    if (cable.type === "sensor") {
      temp = coolHottest(cable.keys);
      const top = coolCurveTemp(cable.curve);
      cable.group.classList.toggle("cool-cable-lead", temp !== undefined && temp === top);
      cable.group.classList.toggle("cool-cable-dead", temp === undefined);
      level = temp === undefined ? 0 : 2;
    } else if (cable.type === "input") {
      temp = coolCurveTemp(cable.curve);
      cable.group.classList.add("cool-cable-lead");
      level = temp === null ? 0 : 2;
    } else {
      temp = coolCurveTemp(cable.curve);
      cable.group.classList.add("cool-cable-lead");
      const s = coolOutputState(cable.output);
      const duty = coolShownDuty(cable.output, s);
      level = duty <= 0 ? 0 : duty < 35 ? 1 : duty < 70 ? 2 : 3;
    }
    cable.group.style.setProperty("--cable", coolHeat(temp === undefined ? null : temp));
    if (cable.level !== level) {
      cable.level = level;
      cable.group.dataset.flow = level;
    }
  }
}

// ---------- the plan: live values ----------
function coolApplyLive() {
  if (!coolSetup) return;
  const board = $("#cool-board");
  if (!board) return;
  const outputs = (coolView && coolView.outputs) || [];
  board.classList.toggle("cool-running", outputs.some((o) => o.temp !== null || o.duty > 0));
  board.classList.toggle("cool-is-live", !!(coolView && coolView.live));
  for (const key of coolSensorKeys) {
    const part = coolParts.get(`sensor:${key}`);
    if (!part) continue;
    const v = coolHottest(part.members || [key]);
    part.value.textContent = v === undefined ? "–" : coolDeg(v);
    part.node.style.setProperty("--heat", coolHeat(v === undefined ? null : v));
    part.node.classList.toggle("cool-missing", v === undefined);
  }
  for (const c of coolCurves()) {
    const part = coolParts.get(`curve:${c.id}`);
    if (!part) continue;
    const temp = coolCurveTemp(c);
    part.node.style.setProperty("--heat", coolHeat(temp));
    const unit = c.unit === "rpm" ? " RPM" : " %";
    if (part.summary) part.summary.textContent = coolCurveSummary(c);
    if (temp === null) {
      part.readout.textContent = coolSources(c).length || coolNeeds(c).length ? t("cool.sensorGone") : "";
      if (part.mini) part.mini.mark.style.display = "none";
    } else if (part.mini) {
      const value = coolCurveValue(c, temp);
      // The server's value (rise/fall limits, minimum) can differ from the raw curve value: say so.
      let held = "";
      if (c.unit !== "rpm") {
        const driven = coolFansOn(c.id).map((o) => ({ o, st: coolOutputState(o) })).find((x) => x.st && !x.o.pump && !x.st.held && !x.st.unregulated && !x.st.sensorLost);
        if (driven && Math.round(driven.st.duty) !== Math.round(value)) held = " (" + t("cool.heldValue", { v: Math.round(driven.st.duty) + " %" }) + ")";
      }
      part.readout.textContent = `${coolIsGap(c) ? "Δ " : ""}${coolDeg(temp)} → ${Math.round(value)}${unit}${held}`;
      part.mini.mark.style.display = "";
      part.mini.mark.setAttribute("cx", part.mini.scale.x(temp).toFixed(1));
      part.mini.mark.setAttribute("cy", part.mini.scale.y(value).toFixed(1));
    } else {
      part.readout.textContent = coolDeg(temp);
    }
  }
  const live = !!(coolView && coolView.live);
  for (const o of coolOutputs()) {
    const part = coolParts.get(`output:${o.id}`);
    if (!part) continue;
    const s = coolOutputState(o);
    let text = coolDriven(o) || o.pump ? "–" : t(live ? "cool.unregulated" : "cool.free"), alarm = false;
    const manualText = () => t(live ? "cool.manualNode" : "cool.manualWould", { v: o.manual.value, u: coolManualUnit(o) === "rpm" ? "RPM" : "%" });
    if (o.manual) text = manualText();
    if (s) {
      const blockedHere = coolView && coolView.blocked;
      alarm = !!(s.pumpDown || s.emergency || s.sensorLost || s.stalled || s.writeFailed || (o.pump && s.rpm === null) || (s.deviceLost && !blockedHere));
      if ((s.notReleased || s.notMatched) && !s.pumpDown && !s.emergency) alarm = false;
      if (s.pumpDown) text = t("cool.pumpDown");
      else if (s.emergency) text = t("cool.emergency");
      else if (s.sensorLost) text = t("cool.sensorLost");
      else if (s.notMatched) text = [t("cool.notMatched"), s.rpm === null ? "" : s.rpm + " RPM", t("cool.noEffect")].filter(Boolean).join(" · ");
      else if (s.notReleased) text = [t("cool.notReleased"), s.rpm === null ? "" : s.rpm + " RPM", t("cool.noEffect")].filter(Boolean).join(" · ");
      else if (s.noSource) text = t("cool.noSource");
      else if (s.matching) text = t("cool.matching");
      else if (o.pump && s.rpm === null) text = t("cool.rpmUnknown");
      else if (s.deviceLost && !blockedHere) text = t("cool.deviceLost");
      else if (s.writeFailed) text = t("cool.writeFailed");
      else if (s.stalled) text = t("cool.stalled");
      else if (coolCalibrating(o)) text = t("cool.heldNow", { d: coolCalibrating(o).duty, r: coolCalibrating(o).rpm });
      else if (s.unregulated || (o.pump && !coolDriven(o))) text = `${t("cool.unregulated")}${s.rpm === null ? "" : " · " + s.rpm + " RPM"}`;
      else if (o.manual) text = s.unguarded ? `${manualText()} · ${t("cool.unguarded")}` : manualText();
      else if (live && !o.pump && o.curve && s.duty === 0) text = t("cool.standing");
      else {
        const cv = o.curve && coolCurve(o.curve);
        if (cv && cv.unit === "rpm" && s.targetRpm) {
          text = !live ? t("cool.wouldRpm", { t: s.targetRpm, d: s.duty })
            : s.rpm !== null ? t("cool.targetActual", { t: s.targetRpm, a: s.rpm }) : t("cool.target", { t: s.targetRpm });
        } else {
          const parts = [live ? s.duty + " %" : t("cool.wouldSet", { d: s.duty })];
          if (s.rpm !== null) parts.push(s.rpm + " RPM");
          else if (s.targetRpm) parts.push("≈ " + s.targetRpm + " RPM");
          text = parts.join(" · ");
        }
      }
    }
    part.sub.textContent = text;
    part.node.classList.toggle("cool-alarm-node", alarm);
    part.node.classList.toggle("cool-unreleased-node", !!(s && (s.notReleased || s.notMatched) && !s.pumpDown));
    part.node.classList.toggle("cool-free", !coolDriven(o) && !o.pump);
    const duty = coolShownDuty(o, s);
    part.node.classList.toggle("cool-calibrating-node", !!coolCalibrating(o));
    part.bar.style.width = duty + "%";
    const spin = duty <= 0 ? "" : duty < 35 ? "1" : duty < 70 ? "2" : "3";
    if (part.node.dataset.spin !== spin) part.node.dataset.spin = spin;
  }
  coolApplyCables();
  if (coolPanelEditor) coolPanelEditor.place();
  const release = $("#cool-release");
  if (release && coolSel && coolSel.kind === "output") {
    const s = coolOutputState(coolOutput(coolSel.id) || {});
    release.hidden = !(s && s.notReleased);
  }
  const recheck = $("#cool-recheck");
  if (recheck && coolSel && coolSel.kind === "output") {
    const s = coolOutputState(coolOutput(coolSel.id) || {});
    recheck.hidden = !(s && s.notMatched);
    const rb = recheck.querySelector("button");
    if (rb) {
      rb.disabled = coolDirty;
      rb.title = coolDirty ? t("cool.recheckSave") : "";
    }
  }
  const runBox = $(".cool-panel-run");
  if (runBox) {
    const o = coolOutput(runBox.dataset.for), m = o && coolCalibrating(o);
    runBox.hidden = !m;
    if (m) {
      runBox.replaceChildren(
        el("strong", {}, t("cool.heldNow", { d: m.duty, r: m.rpm })),
        el("span", {}, coolPhaseInfo(m).text),
        el("span", { class: "cool-hint" }, coolOthersNote(m)));
    }
  }
  const now = $("#cool-panel-now");
  if (now && coolSel && coolSel.kind === "sensor") {
    const v = coolSensorValue(coolSel.id);
    now.textContent = v === undefined ? "–" : coolDeg(v);
  }
}

// Hovering or selecting a block lights up everything that hangs together with it.
function coolHighlight(key) {
  const board = $("#cool-board");
  if (!board) return;
  const focus = key || (coolSel && `${coolSel.kind}:${coolSel.id}`);
  const lit = new Set();
  if (focus && coolParts.has(focus)) {
    lit.add(focus);
    const [kind, ...rest] = focus.split(":");
    const id = rest.join(":");
    // Towards the sensors and towards the fans, through combinations on the way.
    const towardsSensors = (c, seen = new Set()) => {
      if (seen.has(c)) return;
      seen.add(c);
      lit.add(`curve:${c.id}`);
      for (const s of coolSources(c)) lit.add(`sensor:${s}`);
      if (coolIsCombined(c)) coolNeeds(c).forEach((n) => towardsSensors(n, seen));
    };
    const towardsFans = (c, seen = new Set()) => {
      if (seen.has(c)) return;
      seen.add(c);
      lit.add(`curve:${c.id}`);
      for (const o of coolFansOn(c.id)) lit.add(`output:${o.id}`);
      coolCombinedWith(c.id).forEach((m) => towardsFans(m, seen));
    };
    if (kind === "curve") {
      towardsSensors(coolCurve(id));
      towardsFans(coolCurve(id));
    }
    if (kind === "sensor") {
      const keys = coolParts.get(focus).members || [id];
      keys.forEach((k) => lit.add(`sensor:${k}`));
      coolCurves().filter((c) => coolSources(c).some((k) => keys.includes(k))).forEach((c) => towardsFans(c));
    }
    if (kind === "output") {
      const c = coolCurve(coolOutput(id).curve);
      if (c) towardsSensors(c);
    }
  }
  board.classList.toggle("cool-focus", lit.size > 0);
  // The sensors of a closed group share a block: it is lit if any of them is.
  for (const part of coolParts.values()) part.node.classList.remove("cool-lit", "cool-selected");
  for (const [k, part] of coolParts) {
    if (lit.has(k)) part.node.classList.add("cool-lit");
    if (coolSel && k === `${coolSel.kind}:${coolSel.id}`) part.node.classList.add("cool-selected");
  }
  for (const c of coolCables) c.group.classList.toggle("cool-cable-lit", lit.has(c.fromKey) && lit.has(c.toKey));
}

function coolSelect(kind, id) {
  coolSel = coolSel && coolSel.kind === kind && coolSel.id === id ? null : { kind, id };
  coolHighlight(null);
  coolRenderPanel();
}

// ---------- the plan: pulling a cable ----------
function coolWire(from, node) {
  const target = node.dataset.id;
  const addInput = (m, id) => {
    if (!m.inputs) m.inputs = [];
    if (m.id !== id && !m.inputs.includes(id)) m.inputs.push(id);
  };
  if (from.kind === "curve-in" && coolIsCombined(coolCurve(from.id))) return addInput(coolCurve(from.id), target);
  if (from.kind === "curve" && node.dataset.kind === "curve") return addInput(coolCurve(target), from.id);
  const sensor = from.kind === "sensor" ? from.id : from.kind === "curve-in" ? target : null;
  if (sensor !== null) {
    const curve = coolCurve(from.kind === "sensor" ? target : from.id);
    if (!curve) return;
    // A cable from a closed group brings all of its sensors.
    const part = coolParts.get(`sensor:${sensor}`);
    const members = (part && part.members) || [sensor];
    for (const key of members) {
      if (curve.sources.includes(key) || curve.reference === key) continue;
      if (members.length === 1 && coolIsGap(curve) && curve.sources.length && !curve.reference) curve.reference = key;
      else curve.sources.push(key);
    }
    return;
  }
  const output = coolOutput(from.kind === "output" ? from.id : target);
  if (output) {
    output.curve = from.kind === "output" ? target : from.id;
    delete output.manual;
  }
}

// Which block a cable pulled from this port may end on.
function coolCableFits(from, node) {
  const kind = node.dataset.kind;
  const curve = kind === "curve" ? coolCurve(node.dataset.id) : null;
  switch (from.kind) {
    case "sensor": return !!curve && coolTakesSensors(curve);
    case "curve-in": return coolIsCombined(coolCurve(from.id)) ? !!curve && curve.id !== from.id : kind === "sensor";
    case "curve": return kind === "output" || (!!curve && coolIsCombined(curve) && curve.id !== from.id);
    default: return !!curve;
  }
}

function coolStartCable(ev, from) {
  if (ev.button !== 0) return;
  ev.preventDefault();
  ev.stopPropagation();
  const board = $("#cool-board");
  const svg = board.querySelector(".cool-cables");
  const picked = from.kind === "output" && coolOutput(from.id).curve ? coolOutput(from.id) : null;
  // A cable is held by its other end: pulling at a connected fan unplugs it.
  const anchorPort = picked ? coolParts.get(`curve:${picked.curve}`).out
    : from.kind === "sensor" || from.kind === "curve" ? coolParts.get(`${from.kind}:${from.id}`).out
    : coolParts.get(`${from.kind === "curve-in" ? "curve" : "output"}:${from.id}`).in;
  const anchorIsOut = anchorPort.classList.contains("cool-port-out");
  const startX = ev.clientX, startY = ev.clientY;
  const temp = svgEl("path", { class: "cool-cable-pull" });
  let moved = false;
  const targetAt = (x, y) => {
    const hit = document.elementFromPoint(x, y);
    const node = hit && hit.closest(".cool-node");
    return node && coolCableFits(from, node) ? node : null;
  };
  const draw = (x, y) => {
    const box = coolBoardBox(board);
    const target = targetAt(x, y);
    let end = [x - box.left, y - box.top];
    for (const part of coolParts.values()) part.node.classList.remove("cool-target-over");
    if (target) {
      const part = coolParts.get(`${target.dataset.kind}:${target.dataset.id}`);
      part.node.classList.add("cool-target-over");
      end = coolPortAt(anchorIsOut ? part.in : part.out, box);
    }
    const anchor = coolPortAt(anchorPort, box);
    temp.setAttribute("d", anchorIsOut ? coolCablePath(anchor, end) : coolCablePath(end, anchor));
  };
  const begin = () => {
    moved = true;
    coolDragging = from;
    board.classList.add("cool-pulling");
    svg.append(temp);
    for (const part of coolParts.values()) part.node.classList.toggle("cool-target", coolCableFits(from, part.node));
    if (picked) {
      const held = coolCables.find((c) => c.type === "output" && c.output === picked);
      if (held) held.group.style.display = "none";
    }
    coolHighlight(null);
  };
  const move = (e) => {
    if (!moved && Math.hypot(e.clientX - startX, e.clientY - startY) < 4) return;
    if (!moved) begin();
    draw(e.clientX, e.clientY);
  };
  const stop = () => {
    window.removeEventListener("pointermove", move);
    window.removeEventListener("pointerup", up);
    window.removeEventListener("keydown", key);
    window.removeEventListener("pointercancel", cancel);
    board.classList.remove("cool-pulling");
    temp.remove();
    for (const part of coolParts.values()) part.node.classList.remove("cool-target", "cool-target-over");
    setTimeout(() => { coolDragging = null; }, 0);
  };
  const cancel = () => {
    stop();
    if (moved) coolDrawCables();
  };
  const key = (e) => { if (e.key === "Escape") cancel(); };
  const up = (e) => {
    stop();
    if (!moved) return;
    const target = targetAt(e.clientX, e.clientY);
    const wiring = () => JSON.stringify([coolCurves().map((c) => [c.sources, c.inputs, c.reference]), coolOutputs().map((o) => o.curve)]);
    const before = wiring();
    if (target) {
      coolWire(from, target);
    } else if (picked) {
      picked.curve = "";
    } else if (from.kind === "sensor" && coolOverEmptyCurves(e.clientX, e.clientY)) {
      const c = coolingNewCurve();
      c.sources.push(...(from.ids || [from.id]));
      coolCurves().push(c);
      coolSel = { kind: "curve", id: c.id };
    }
    if (wiring() !== before) coolChanged();
    coolRenderBoard();
    coolRenderPanel();
  };
  window.addEventListener("pointermove", move);
  window.addEventListener("pointerup", up);
  window.addEventListener("pointercancel", cancel);
  window.addEventListener("keydown", key);
}

function coolOverEmptyCurves(x, y) {
  const col = $(".cool-col-curves");
  if (!col) return false;
  const r = col.getBoundingClientRect();
  return x >= r.left && x <= r.right && y >= r.top && y <= r.bottom;
}

// ---------- details ----------
function coolingSensorSelect(value, sensors, onPick, placeholder) {
  const known = sensors.some((s) => s.key === value);
  const sel = el("select", { onchange: (ev) => onPick(ev.target.value) },
    placeholder ? el("option", { value: "" }, placeholder) : null,
    !known && value ? el("option", { value }, t("cool.sensorMissing", { k: value })) : null,
    sensors.map((s) => el("option", { value: s.key }, `${s.name} (${Math.round(s.value)})`)));
  sel.value = value || "";
  return sel;
}

function coolingNumber(o, key, label, opts = {}) {
  return el("label", { class: "cool-field" }, el("span", {}, t(label)),
    el("input", { type: "number", value: o[key] || 0, ...(opts.negative ? {} : { min: 0 }), step: "any",
      onchange: (ev) => {
        o[key] = +ev.target.value;
        coolChanged();
        coolApplyLive();
      } }));
}

// The fields of a curve that is not drawn as a shape.
function coolKindFields(c) {
  const numbers = (list) => el("div", { class: "cool-fields" }, list.map(([k, l, opts]) => coolingNumber(c, k, l, opts)));
  switch (coolKind(c)) {
    case "straight": {
      const part = (i, j, label) => el("label", { class: "cool-field" }, el("span", {}, t(label)),
        el("input", { type: "number", value: c.points[i][j], min: 0, step: "any", onchange: (ev) => {
          c.points[i][j] = +ev.target.value;
          coolRewire();
        } }));
      return el("div", { class: "cool-fields" }, part(0, 0, "cool.linearFrom"), part(0, 1, "cool.linearFromValue"),
        part(1, 0, "cool.linearTo"), part(1, 1, "cool.linearToValue"));
    }
    case "gap": {
      const pick = el("select", { onchange: (ev) => {
        c.reference = ev.target.value;
        if (c.reference && !coolSensorKeys.includes(c.reference)) coolSensorKeys.push(c.reference);
        coolRewire();
      } }, el("option", { value: "" }, t("cool.referencePick")),
      coolTempsOffered().filter((x) => !c.sources.includes(x.key)).map((x) => el("option", { value: x.key, title: x.name }, coolSensorName(x.key))));
      pick.value = c.reference || "";
      return el("label", { class: "cool-field" }, el("span", {}, t("cool.reference")), pick);
    }
    case "fixed": return numbers([["value", "cool.flatValue"]]);
    case "switch": return numbers([["load_temp", "cool.loadTemp"], ["load_value", "cool.loadValue"],
      ["idle_temp", "cool.idleTemp"], ["idle_value", "cool.idleValue"], ["rise_after", "cool.riseAfter"], ["fall_after", "cool.fallAfter"]]);
    case "thermostat": return numbers([["load_temp", "cool.autoTarget"], ["idle_temp", "cool.autoIdle"], ["idle_value", "cool.autoMin"],
      ["load_value", "cool.autoMax"], ["step", "cool.autoStep"], ["deadband", "cool.autoDeadband"], ["react_after", "cool.autoReact"]]);
    case "combined": {
      const fn = el("select", { onchange: (ev) => { c.combine = ev.target.value; coolChanged(); coolApplyLive(); } },
        COOL_COMBINES.map((m) => el("option", { value: m }, t("cool.combine." + m))));
      fn.value = c.combine || "max";
      const others = coolCurves().filter((x) => x !== c && !(c.inputs || []).includes(x.id));
      return el("div", { class: "cool-stack" },
        el("label", { class: "cool-field" }, el("span", {}, t("cool.combineFn")), fn),
        el("div", { class: "cool-field" }, el("span", {}, t("cool.combineInputs")),
          (c.inputs || []).map((id) => coolRow((coolCurve(id) || { name: id }).name || id, () => {
            c.inputs = c.inputs.filter((k) => k !== id);
            coolRewire();
          })),
          others.length ? el("select", { onchange: (ev) => {
            if (ev.target.value) {
              if (!c.inputs) c.inputs = [];
              c.inputs.push(ev.target.value);
            }
            coolRewire();
          } }, el("option", { value: "" }, t("cool.combineAdd")), others.map((x) => el("option", { value: x.id }, x.name || x.id))) : null));
    }
    case "follow": {
      const pick = el("select", { onchange: (ev) => { c.follow = ev.target.value; coolRewire(); } },
        el("option", { value: "" }, t("cool.followPick")),
        coolOutputs().filter((o) => o.curve !== c.id).map((o) => el("option", { value: o.id }, o.name || o.id)));
      pick.value = c.follow || "";
      return el("div", { class: "cool-stack" },
        el("label", { class: "cool-field" }, el("span", {}, t("cool.followWhom")), pick),
        coolingNumber(c, "offset", "cool.followOffset", { negative: true }),
        el("label", { class: "cool-mode" }, el("input", { type: "checkbox", class: "switch", checked: !!c.proportional,
          onchange: (ev) => { c.proportional = ev.target.checked; coolChanged(); coolApplyLive(); } }), t("cool.followShare")));
    }
  }
  return null;
}

function coolingNameField(o) {
  return el("label", { class: "cool-field" }, el("span", {}, t("cool.name")),
    el("input", { value: o.name || "", onchange: (ev) => {
      o.name = ev.target.value;
      coolChanged();
      coolRenderBoard();
    } }));
}

function coolRow(text, onRemove) {
  return el("div", { class: "cool-row" }, el("span", { class: "cool-row-text", title: text }, text),
    el("button", { type: "button", "aria-label": t("cool.disconnect"), title: t("cool.disconnect"), onclick: onRemove }, "✕"));
}

function coolRewire() {
  coolChanged();
  coolRenderBoard();
  coolRenderPanel();
}

function coolSensorPanel(key) {
  const curves = coolCurves().filter((c) => coolSources(c).includes(key));
  const others = coolCurves().filter((c) => coolTakesSensors(c) && !c.sources.includes(key));
  return [
    el("p", { class: "sub cool-break" }, coolSensorFull(key)),
    el("label", { class: "cool-field" }, el("span", {}, t("cool.name")),
      el("input", { value: coolSensorOwnName(key), placeholder: coolSensorShort(key), maxlength: 60, onchange: (ev) => {
        const names = { ...coolSetup.config.sensor_names };
        const name = ev.target.value.trim();
        if (name) names[key] = name;
        else delete names[key];
        coolSetup.config.sensor_names = names;
        coolRewire();
      } })),
    el("div", { class: "cool-field" }, el("span", {}, t("cool.current")), el("strong", { id: "cool-panel-now" }, "–")),
    el("div", { class: "cool-field" }, el("span", {}, t("cool.usedBy")),
      curves.map((c) => coolRow(c.name || c.id, () => {
        c.sources = c.sources.filter((k) => k !== key);
        if (c.reference === key) c.reference = "";
        coolRewire();
      })),
      others.length ? el("select", { onchange: (ev) => {
        const c = coolCurve(ev.target.value);
        if (c) c.sources.push(key);
        coolRewire();
      } }, el("option", { value: "" }, t("cool.connectCurve")), others.map((c) => el("option", { value: c.id }, c.name || c.id))) : null),
    el("button", { type: "button", class: "cool-panel-end", onclick: () => {
      for (const c of coolCurves()) {
        c.sources = c.sources.filter((k) => k !== key);
        if (c.reference === key) c.reference = "";
      }
      coolSensorKeys = coolSensorKeys.filter((k) => k !== key);
      coolSel = null;
      coolRewire();
    } }, t("cool.removeSensor")),
  ];
}

function coolSetUnit(c, unit) {
  if ((c.unit === "rpm") === (unit === "rpm")) return;
  const top = coolRpmTop(c);
  c.points = c.points.map(([temp, v]) => [temp, unit === "rpm"
    ? Math.round(((v / 100) * top) / 10) * 10
    : Math.min(100, Math.round((v / top) * 100))]);
  const scale = (v) => (unit === "rpm" ? Math.round(((v / 100) * top) / 10) * 10 : Math.min(100, Math.round((v / top) * 100)));
  for (const k of ["value", "idle_value", "load_value", "step"]) if (c[k]) c[k] = scale(c[k]);
  c.offset = 0;
  c.unit = unit;
}

function coolCurvePanel(c) {
  const refreshMini = () => {
    const part = coolParts.get(`curve:${c.id}`);
    if (part && part.mini) {
      const next = coolMiniGraph(c);
      part.mini.svg.replaceWith(next.svg);
      part.mini = next;
      coolApplyLive();
    }
  };
  let table;
  const editor = curveEditor(c, () => { table.fill(); coolChanged(); refreshMini(); });
  table = curveTable(c, () => { editor.draw(); coolChanged(); refreshMini(); });
  const drawn = coolKind(c) === "drawn" || coolIsGap(c);
  coolPanelEditor = drawn ? editor : null;
  const kindPick = el("select", { onchange: (ev) => {
    coolSetKind(c, ev.target.value);
    coolChanged();
    coolRewire();
  } }, COOL_KINDS.map((k) => el("option", { value: k }, t("cool.kind." + k))));
  kindPick.value = coolKind(c);
  const offCurve = coolOutputs().filter((o) => o.curve !== c.id);
  const sensors = coolTempsOffered().filter((s) => !c.sources.includes(s.key));
  const unitButton = (unit, label) => el("button", {
    type: "button", class: c.unit === unit || (unit === "percent" && c.unit !== "rpm") ? "active" : "",
    "aria-pressed": c.unit === unit || (unit === "percent" && c.unit !== "rpm") ? "true" : "false",
    onclick: () => {
      coolSetUnit(c, unit);
      coolRewire();
    },
  }, t(label));
  return [
    coolingNameField(c),
    el("label", { class: "cool-field" }, el("span", {}, t("cool.kind")), kindPick,
      el("span", { class: "cool-hint" }, t("cool.kindHint." + coolKind(c)))),
    el("div", { class: "cool-field" }, el("span", {}, t("cool.unit")),
      el("div", { class: "seg small" }, unitButton("percent", "cool.unitPercent"), unitButton("rpm", "cool.unitRpm")),
      el("span", { class: "cool-hint" }, t(c.unit === "rpm" ? "cool.unitRpmHint" : "cool.unitPercentHint"))),
    drawn ? el("div", { class: "cool-field" }, el("span", {}, t("cool.curve")), editor.svg,
      el("span", { class: "cool-hint" }, t("cool.curveHint")), table.box) : coolKindFields(c),
    coolIsGap(c) ? coolKindFields(c) : null,
    coolTakesSensors(c) ? el("div", { class: "cool-field" }, el("span", {}, t("cool.sensors")),
      c.sources.map((key) => coolRow(coolSensorName(key), () => {
        c.sources = c.sources.filter((k) => k !== key);
        coolRewire();
      })),
      coolingSensorSelect("", sensors, (v) => {
        if (v) {
          c.sources.push(v);
          if (!coolSensorKeys.includes(v)) coolSensorKeys.push(v);
        }
        coolRewire();
      }, t("cool.sensorAdd"))) : null,
    el("div", { class: "cool-field" }, el("span", {}, t("cool.attached")),
      coolFansOn(c.id).map((o) => coolRow(o.name || o.id, () => {
        o.curve = "";
        coolRewire();
      })),
      offCurve.length ? el("select", { onchange: (ev) => {
        const o = coolOutput(ev.target.value);
        if (o) o.curve = c.id;
        coolRewire();
      } }, el("option", { value: "" }, t("cool.attachOutput")), offCurve.map((o) => el("option", { value: o.id }, o.name || o.id))) : null),
    coolHasShape(c) ? el("details", {}, el("summary", {}, t("cool.advanced")),
      el("div", { class: "cool-fields" }, [
        ["rise_by", "cool.riseBy"], ["fall_by", "cool.fallBy"], ["rise_after", "cool.riseAfter"],
        ["fall_after", "cool.fallAfter"], ["lead_ticks", "cool.lead"],
      ].map(([k, l]) => coolingNumber(c, k, l))),
      el("label", { class: "cool-mode" }, el("input", { type: "checkbox", class: "switch", checked: !!c.distance_at_ends,
        onchange: (ev) => { c.distance_at_ends = ev.target.checked; coolChanged(); } }), t("cool.endsStrict"))) : null,
    el("div", { class: "cool-panel-end cool-row-actions" },
      el("button", { type: "button", onclick: () => {
        const copy = { ...coolingNewCurve(), ...JSON.parse(JSON.stringify(c)) };
        copy.id = coolingNewCurve().id;
        copy.name = t("cool.copyOf", { n: c.name || c.id });
        coolCurves().push(copy);
        coolSel = { kind: "curve", id: copy.id };
        coolRewire();
      } }, t("cool.duplicate")),
      el("button", { type: "button", onclick: () => {
        for (const o of coolFansOn(c.id)) o.curve = "";
        for (const m of coolCombinedWith(c.id)) m.inputs = m.inputs.filter((k) => k !== c.id);
        coolSetup.config.curves = coolCurves().filter((x) => x !== c);
        coolSel = null;
        coolRewire();
      } }, t("cool.deleteCurve"))),
  ];
}

function coolOnController(o) {
  return /^port[1-4]-fan[1-9][0-9]?$/.test(o.id);
}

function coolOutputType(o) {
  return o.pump ? "pump" : o.role === "case" ? "case" : "fan";
}

// Each type comes with what it must not do: a pump and a case fan never stand.
function coolSetOutputType(o, type) {
  delete o.role;
  if (type !== "pump") delete o.control_key;
  o.pump = type === "pump";
  if (type === "fan") {
    o.stopped_below_rpm = 0;
    return;
  }
  o.stop_at = 0;
  o.start_duty = 0;
  if (type === "case") {
    o.role = "case";
    o.stopped_below_rpm = 0;
    o.min_duty = Math.max(o.min_duty || 0, 20);
  } else {
    o.min_duty = Math.max(o.min_duty || 0, 50);
    o.safe_duty = 100;
    o.offset = 0;
  }
}

function coolSetMayStand(o, yes) {
  if (yes) {
    o.min_duty = 0;
    o.stop_at = o.stop_at || 6;
    o.start_duty = o.start_duty || 15;
  } else {
    o.min_duty = 20;
    o.stop_at = 0;
    o.start_duty = 0;
  }
}

// Outputs whose release is being asked for; the UAC prompt can stay open for a long time.
const coolReleasing = new Set();

function coolReleaseBox(o) {
  const button = el("button", { type: "button", class: "cool-release-btn", disabled: coolReleasing.has(o.id),
    onclick: async () => {
      coolReleasing.add(o.id);
      button.disabled = true;
      try {
        await api("POST", "/api/cooling/release-pump", { id: o.id });
      } catch (err) {
        toast(err.message, true);
      } finally {
        coolReleasing.delete(o.id);
        button.disabled = false;
      }
    } }, t("cool.release"));
  return el("div", { id: "cool-release", class: "cool-release", hidden: true },
    el("p", { class: "cool-hint" }, t("cool.releaseHint")), button);
}

const COOL_PUMP_SENSOR = /^(\/lpc\/[a-z0-9]{1,32}\/[0-9]{1,2})\/fan\/([0-9]{1,2})$/;

function coolDerivedControl(sensor) {
  const m = COOL_PUMP_SENSOR.exec(sensor || "");
  return m ? `${m[1]}/control/${m[2]}` : "";
}

const coolControlOf = (o) => o.control_key || coolDerivedControl(o.rpm_sensor);

// The channels a pump may be given: those of the sensor's chip that no other pump uses.
// The server checks the same rule.
function coolControlChoices(o) {
  const m = COOL_PUMP_SENSOR.exec(o.rpm_sensor || "");
  if (!m) return [];
  const taken = new Set(coolOutputs().filter((x) => x !== o && x.pump && !coolOnController(x)).map(coolControlOf));
  const own = coolDerivedControl(o.rpm_sensor);
  return (coolSetup.controls || []).filter((c) => c.key.startsWith(m[1] + "/control/") && c.key !== own && !taken.has(c.key));
}

function coolControlLabel(c) {
  const name = (c.name || "").split("/").pop() || t("cool.channel", { n: c.key.split("/").pop() });
  return c.duty === null || c.duty === undefined ? name : `${name} · ${c.duty} %`;
}

function coolControlSelect(o) {
  const own = coolDerivedControl(o.rpm_sensor);
  const ownInfo = (coolSetup.controls || []).find((c) => c.key === own);
  const choices = coolControlChoices(o);
  const chosen = o.control_key || "";
  const select = el("select", { id: "cool-control", onchange: (ev) => {
    if (ev.target.value) o.control_key = ev.target.value;
    else delete o.control_key;
    coolChanged();
    coolApplyLive();
  } },
  el("option", { value: "" }, t("cool.controlDerived", { k: ownInfo ? coolControlLabel(ownInfo) : own.split("/").slice(-2).join("/") })),
  chosen && !choices.some((c) => c.key === chosen) ? el("option", { value: chosen }, t("cool.controlMissing", { k: chosen.split("/").slice(-2).join("/") })) : null,
  choices.map((c) => el("option", { value: c.key }, coolControlLabel(c))));
  select.value = chosen;
  return select;
}

// Shown while the check of the pump's header failed: the board keeps the pump.
function coolRecheckBox(o) {
  const button = el("button", { type: "button", class: "cool-release-btn",
    onclick: async () => {
      button.disabled = true;
      try {
        await api("POST", "/api/cooling/recheck-pump", { id: o.id });
      } catch (err) {
        toast(err.message, true);
      } finally {
        button.disabled = false;
      }
    } }, t("cool.recheck"));
  return el("div", { id: "cool-recheck", class: "cool-release", hidden: true },
    el("p", { class: "cool-hint" }, t("cool.matchHint")),
    el("div", { class: "cool-channel-row" },
      el("label", { class: "cool-field" }, el("span", {}, t("cool.control")), coolControlSelect(o)), button),
    el("p", { class: "cool-hint" }, t("cool.controlHint")));
}

// Why an output cannot be calibrated now ("" if it can); the words are cool.why.*.
function coolMeasureReason(o) {
  if (!coolView || !coolView.outputs) return "wait";
  if (coolMeasuring) return "running";
  if (coolDirty) return "dirty";
  if (!coolDriven(o)) return "noCurve";
  const s = coolOutputState(o);
  if (!s) return "dirty";
  if (s.measureBlock) return s.measureBlock;
  if (o.pump && s.pumpDown) return "pumpDown";
  return "";
}

// The same for the run over all fans: possible if any fan can be measured.
function coolMeasureAllReason() {
  if (!coolView || !coolView.outputs) return "wait";
  if (coolMeasuring) return "running";
  if (coolDirty) return "dirty";
  const fans = coolOutputs().filter((o) => !o.pump && coolDriven(o));
  if (!fans.length) return "noFans";
  const reasons = fans.map(coolMeasureReason);
  return reasons.some((r) => !r) ? "" : reasons[0];
}

// Enables or disables the buttons and says why, from the state as it is now.
function coolUpdateMeasureUI() {
  for (const b of $$(".cool-measure-btn")) {
    const o = coolOutput(b.dataset.id);
    const why = o ? coolMeasureReason(o) : "dirty";
    b.disabled = !!why;
    b.title = why ? t("cool.why." + why) : "";
    const note = $$(".cool-why").find((n) => n.dataset.for === b.dataset.id);
    if (note) {
      // "Save first" is said once, at the top; the button carries it as its tooltip.
      note.textContent = why && why !== "dirty" ? t("cool.why." + why) : "";
      note.hidden = !why || why === "dirty";
    }
  }
  const all = $("#cool-measure-all");
  if (all) {
    const why = coolMeasureAllReason();
    all.disabled = !!why;
    all.title = why ? t("cool.why." + why) : "";
    const note = $("#cool-all-why");
    note.textContent = why ? t("cool.why." + why) : "";
    note.hidden = !why;
  }
}

// The pump warning of the calibration dialogs: always shown, concrete where a pump is set up.
function coolPumpNote() {
  return t(coolOutputs().some((o) => o.pump) ? "cool.cal.pumpNote" : "cool.cal.pumpNoteGeneric") + "\n\n";
}

async function coolStartMeasure(url, body, question) {
  if (!confirm(question)) return;
  try {
    await api("POST", url, body);
    coolMeasuring = true; // the next poll confirms; until then no second start
    coolUpdateMeasureUI();
  } catch (err) {
    toast(err.message, true);
  }
}

// What the last calibration found, the suggestion made from it, and the button.
function coolCalibrationBox(o) {
  const c = o.calibration;
  const sg = (coolSetup.suggestions || {})[o.id];
  const type = coolOutputType(o);
  const lines = [];
  if (c && c.kind === "pump") {
    lines.push(t(c.at_limit ? "cool.cal.pumpLimit" : "cool.cal.pumpMin", { v: c.at_limit ? 30 : c.minimum }));
  } else if (c) {
    if (c.stops === "yes") lines.push(t("cool.cal.stopsAt", { v: c.stop }));
    else lines.push(t(c.stops === "never" ? "cool.cal.never" : "cool.cal.unheld"));
    if (c.stops === "yes") lines.push(t(c.starts ? "cool.cal.startsAt" : "cool.cal.noStart", { v: c.start }));
  }
  const stands = type === "fan" && !o.min_duty;
  const apply = (fn, done) => el("button", { type: "button", class: "sm", onclick: () => {
    fn();
    coolRewire();
    toast(done || t("cool.cal.applied"));
  } }, t("cool.cal.apply"));
  let suggestion = null;
  // Applied already (the values equal the suggestion): the line is gone, also after a reload.
  const safeNow = o.safe_duty || 100;
  const applied = sg && (stands && sg.stopAt
    ? o.stop_at === sg.stopAt && o.start_duty === sg.startDuty
    : sg.minDuty && (!stands || o.pump) && o.min_duty === sg.minDuty && safeNow >= sg.minDuty);
  if (!c || applied) {
    suggestion = null;
  } else if (sg && stands && sg.stopAt) {
    suggestion = el("div", { class: "cool-speeds cool-suggest" }, el("span", {}, t("cool.cal.suggestStand", { a: sg.stopAt, b: sg.startDuty })),
      apply(() => { o.stop_at = sg.stopAt; o.start_duty = sg.startDuty; }));
  } else if (sg && sg.minDuty && (!stands || o.pump)) {
    // The measured value is never lowered to fit: a minimum above the safe duty
    // would be refused, and a safe duty below the minimum has no meaning, so the
    // safe duty goes up with it.
    const m = sg.minDuty;
    const safe = o.safe_duty || 100;
    const raise = m > safe;
    suggestion = el("div", { class: "cool-speeds cool-suggest" },
      el("span", {}, t("cool.cal.suggestMin", { m }) + (raise ? " " + t("cool.cal.raiseSafe", { s: safe, m }) : "")),
      apply(() => {
        o.min_duty = m;
        if (raise) o.safe_duty = m;
      }, raise ? t("cool.cal.appliedSafe", { s: m }) : null));
  } else if (c && stands && c.kind === "fan" && c.stops === "yes") {
    suggestion = el("p", { class: "cool-hint" }, t("cool.cal.suggestNoStand"));
  }
  const id = o.id;
  // Only the display data of the measurement goes: the speed table and the values stay.
  const clear = c ? el("button", { type: "button", class: "sm cool-cal-clear", onclick: () => {
    delete o.calibration;
    coolRewire();
  } }, t("cool.cal.clear")) : null;
  const run = o.pump
    ? () => coolStartMeasure("/api/cooling/measure-pump", { id }, t("cool.cal.pumpAsk", { n: o.name || id, f: 30 }))
    : () => coolStartMeasure("/api/cooling/measure", { id }, t("cool.cal.fanAsk", { n: o.name || id, note: coolPumpNote() }));
  return el("div", { class: "cool-cal" },
    ...lines.map((l) => el("p", { class: "cool-hint cool-cal-line" }, l)),
    c && c.at ? el("p", { class: "cool-hint" }, t("cool.cal.when", { d: new Date(c.at).toLocaleString(LANG === "de" ? "de-DE" : "en-GB") })) : null,
    type === "case" && c ? el("p", { class: "cool-hint" }, t("cool.cal.caseNote")) : null,
    suggestion,
    clear,
    el("button", { type: "button", class: "cool-measure-btn", "data-id": id, disabled: true, onclick: run }, t(o.pump ? "cool.cal.pump" : "cool.cal.fan")),
    el("p", { class: "cool-hint cool-why", "data-for": id, hidden: true }));
}

// Value and unit of a manual output. Speeds need the output's measured table.
function coolManualFields(o) {
  const measured = !!(o.speeds && o.speeds.length >= 2);
  if (!measured && o.manual.unit === "rpm") o.manual.unit = "percent";
  const unit = el("select", { class: "cool-manual-unit", onchange: (ev) => {
    o.manual.unit = ev.target.value;
    coolRewire();
  } }, el("option", { value: "percent" }, t("cool.unitPercent")),
  el("option", { value: "rpm", ...(measured ? {} : { disabled: true }) }, t("cool.unitRpm")));
  unit.value = coolManualUnit(o);
  return el("div", { class: "cool-manual" },
    el("div", { class: "cool-fields" },
      el("label", { class: "cool-field" }, el("span", {}, t("cool.manualValue")),
        el("input", { type: "number", class: "cool-manual-value", value: o.manual.value, min: 0, step: "any", onchange: (ev) => {
          o.manual.value = +ev.target.value;
          coolChanged();
          coolApplyLive();
        } })),
      el("label", { class: "cool-field" }, el("span", {}, t("cool.manualUnit")), unit)),
    measured ? null : el("p", { class: "cool-hint" }, t("cool.manualNoSpeeds")));
}

function coolOutputPanel(o) {
  const speeds = o.speeds && o.speeds.length
    ? t("cool.speeds", { n: o.speeds.length, a: o.speeds[0][1], b: o.speeds[o.speeds.length - 1][1] })
    : t("cool.noSpeeds");
  const curve = el("select", { onchange: (ev) => {
    o.curve = ev.target.value;
    coolRewire();
  } }, el("option", { value: "" }, t("cool.noCurve")), coolCurves().map((c) => el("option", { value: c.id }, c.name || c.id)));
  curve.value = o.curve || "";
  const driveSelect = el("select", { onchange: (ev) => {
    if (ev.target.value === "manual") {
      o.curve = "";
      o.manual = { value: Math.max(50, o.min_duty || 0), unit: "percent" };
    } else {
      delete o.manual;
    }
    coolRewire();
  } }, ["curve", "manual"].map((x) => el("option", { value: x }, t("cool.drive." + x))));
  driveSelect.value = o.manual ? "manual" : "curve";
  const detected = coolSetup.fans.includes(o.id);
  const type = coolOutputType(o);
  const steps = [["step_up", "cool.stepUp"], ["step_down", "cool.stepDown"]];
  const stands = type === "fan" && !o.min_duty;
  const fields = o.pump
    ? [["min_duty", "cool.minDuty"], ["safe_duty", "cool.safeDuty"], ["stopped_below_rpm", "cool.stoppedBelow"], ...steps]
    : [...(stands ? [["stop_at", "cool.stopAt"], ["start_duty", "cool.startDuty"]] : [["min_duty", "cool.minDuty"]]), ["safe_duty", "cool.safeDuty"], ...steps, ["offset", "cool.offset", { negative: true }]];
  const typeSelect = el("select", { onchange: (ev) => {
    coolSetOutputType(o, ev.target.value);
    coolRewire();
  } }, ["fan", "case", "pump"].map((x) => el("option", { value: x }, t("cool.type." + x))));
  typeSelect.value = type;
  return [
    coolingNameField(o),
    el("p", { class: "sub cool-break" }, o.id),
    el("div", { class: "cool-panel-run", "data-for": o.id, hidden: true }),
    o.pump && !coolOnController(o) ? coolReleaseBox(o) : null,
    o.pump && !coolOnController(o) ? coolRecheckBox(o) : null,
    coolOnController(o) ? el("label", { class: "cool-field" }, el("span", {}, t("cool.type")), typeSelect) : null,
    type === "fan" && !o.manual ? el("label", { class: "cool-check" },
      el("input", { type: "checkbox", ...(stands ? { checked: true } : {}), onchange: (ev) => {
        coolSetMayStand(o, ev.target.checked);
        coolRewire();
      } }), t("cool.mayStand")) : null,
    stands && !o.manual ? el("p", { class: "cool-hint" }, t("cool.mayStandHint", { p: o.stop_at || 0 })) : null,
    el("label", { class: "cool-field" }, el("span", {}, t("cool.drive")), driveSelect),
    o.manual ? coolManualFields(o) : el("label", { class: "cool-field" }, el("span", {}, t("cool.curveOf")), curve),
    o.pump && !coolOnController(o) ? el("label", { class: "cool-field" }, el("span", {}, t("cool.rpmSensor")),
      coolingSensorSelect(o.rpm_sensor, coolSetup.rpms, (v) => {
        o.rpm_sensor = v;
        if (o.control_key && !coolControlChoices(o).some((c) => c.key === o.control_key)) delete o.control_key; // another chip
        coolChanged();
      }, t("cool.sensorPick"))) : null,
    o.pump ? el("p", { class: "cool-hint" }, t(coolOnController(o) ? "cool.pumpHintController" : "cool.pumpHint")) : el("div", { class: "cool-speeds" },
      el("span", { class: "sub" }, speeds)),
    coolCalibrationBox(o),
    el("details", {}, el("summary", {}, t("cool.advanced")),
      el("div", { class: "cool-fields" }, fields.map(([k, l, opts]) => coolingNumber(o, k, l, opts)))),
    detected ? null : el("button", { type: "button", class: "cool-panel-end", onclick: () => {
      for (const c of coolCurves()) if (c.follow === o.id) c.follow = "";
      coolSetup.config.outputs = coolOutputs().filter((x) => x !== o);
      coolSel = null;
      coolRewire();
    } }, t("cool.remove")),
  ];
}

function coolRenderPanel() {
  const panel = $("#cool-panel");
  if (!panel) return;
  coolPanelEditor = null;
  const curve = coolSel && coolSel.kind === "curve" ? coolCurve(coolSel.id) : null;
  const output = coolSel && coolSel.kind === "output" ? coolOutput(coolSel.id) : null;
  const sensor = coolSel && coolSel.kind === "sensor" && coolSensorKeys.includes(coolSel.id) ? coolSel.id : null;
  if (!curve && !output && sensor === null) {
    coolSel = null;
    coolPanelShown = "";
    panel.hidden = true;
    panel.replaceChildren();
    return;
  }
  panel.hidden = false;
  const title = curve ? "cool.panelCurve" : output ? (output.pump ? "cool.panelPump" : "cool.panelFan") : "cool.panelSensor";
  panel.replaceChildren(
    el("div", { class: "cool-panel-head" },
      el("h3", { class: "cool-panel-title" }, t(title)),
      el("button", { type: "button", class: "ghost icon-btn sm cool-panel-close", "aria-label": t("cool.close"), title: t("cool.close"),
        onclick: () => { coolSel = null; coolRenderPanel(); coolHighlight(null); } }, "✕")),
    ...(curve ? coolCurvePanel(curve) : output ? coolOutputPanel(output) : coolSensorPanel(sensor)).filter(Boolean));
  coolHighlight(null);
  coolApplyLive();
  coolUpdateMeasureUI();
  // Below the plan (narrow window) the panel is out of sight: bring it into view once per selection.
  const shown = coolSel.kind + ":" + coolSel.id;
  if (shown !== coolPanelShown && matchMedia("(max-width: 1180px)").matches) panel.scrollIntoView({ block: "nearest" });
  coolPanelShown = shown;
}

let coolPanelShown = "";
// Esc closes the side panel (a dialog or menu that is open takes the key first).
document.addEventListener("keydown", (ev) => {
  if (ev.key !== "Escape" || ev.defaultPrevented || !coolSel) return;
  const panel = $("#cool-panel");
  if (!panel || panel.hidden || document.querySelector("dialog[open]") || ev.target.closest("select")) return;
  coolSel = null;
  coolRenderPanel();
  coolHighlight(null);
});

// ---------- presets ----------
let coolPresets = null;    // {presets, active, error, max, nameMax}
let coolPresetEdit = null; // null, {kind: "save"} or {kind: "rename", id}
let coolManage = false;
let coolStale = false;     // the stored settings changed under unsaved edits
let coolPresetsLoading = false;

async function loadCoolPresets() {
  if (coolPresetsLoading) return;
  coolPresetsLoading = true;
  try {
    coolPresets = await api("GET", "/api/cooling/presets");
    renderCoolPresets();
  } catch (err) {
    toast(err.message, true);
  } finally {
    coolPresetsLoading = false;
  }
}

// Every route answers with the list as it is afterwards.
async function coolPresetCall(method, path, body, doneKey, vars) {
  try {
    coolPresets = await api(method, path, body);
    if (doneKey) toast(t(doneKey, vars));
    return true;
  } catch (err) {
    toast(err.message, true);
    loadCoolPresets();
    return false;
  } finally {
    renderCoolPresets();
  }
}

const coolPresetUrl = (id, tail = "") => `/api/cooling/presets/${encodeURIComponent(id)}${tail}`;
const coolPresetName = (id) => coolPresets.presets.find((p) => p.id === id)?.name || id;

async function coolActivatePreset(id) {
  if (coolDirty && !confirm(t("cool.preset.askDiscard"))) {
    renderCoolPresets();
    return;
  }
  if (await coolPresetCall("POST", coolPresetUrl(id, "/activate"), undefined, "cool.preset.activated", { n: coolPresetName(id) })) {
    coolDirty = false;
    coolStale = false;
    await loadCoolingSetup();
  }
}

// The stored settings can be kept as a preset only when the plan shows them.
function coolPresetsCanKeep() {
  return !coolDirty && !!coolPresets && !coolPresets.error && coolOutputs().some((o) => coolDriven(o) || o.pump);
}

// Buttons that act on the stored settings wait for the edits to be saved.
function coolPresetsDirty() {
  for (const b of $$(".cool-needs-clean")) {
    b.disabled = coolDirty || b.dataset.off === "1";
    if (coolDirty) b.title = t("cool.preset.saveFirst");
    else if (b.title === t("cool.preset.saveFirst")) b.title = "";
  }
}

function coolNameForm(initial, onOk, onCancel) {
  const input = el("input", { type: "text", maxlength: coolPresets.nameMax, value: initial, placeholder: t("cool.preset.name"), "aria-label": t("cool.preset.name") });
  const ok = () => {
    const name = input.value.trim();
    if (name) onOk(name);
  };
  input.addEventListener("keydown", (ev) => {
    if (ev.key === "Enter") ok();
    if (ev.key === "Escape") onCancel();
  });
  requestAnimationFrame(() => { input.focus(); input.select(); });
  return el("div", { class: "cool-preset-form" }, input,
    el("button", { type: "button", class: "sm primary", onclick: ok }, t("cool.preset.ok")),
    el("button", { type: "button", class: "sm ghost", onclick: onCancel }, t("cool.preset.cancel")));
}

function coolPresetItem(p) {
  const active = p.id === coolPresets.active;
  if (coolPresetEdit?.kind === "rename" && coolPresetEdit.id === p.id) {
    return el("div", { class: "cool-preset-item" }, coolNameForm(p.name, async (name) => {
      coolPresetEdit = null;
      await coolPresetCall("PUT", coolPresetUrl(p.id), { name }, "cool.preset.stored");
    }, () => { coolPresetEdit = null; renderCoolPresets(); }));
  }
  return el("div", { class: "cool-preset-item" },
    el("span", { class: "name", title: p.name }, p.name),
    active ? el("span", { class: "badge on" }, t("cool.preset.active")) : null,
    el("button", { type: "button", class: "sm cool-needs-clean", "data-off": active ? "1" : null, disabled: coolDirty || active,
      title: coolDirty ? t("cool.preset.saveFirst") : null, onclick: async () => {
        if (confirm(t("cool.preset.askOverwrite", { n: p.name }))) await coolPresetCall("POST", coolPresetUrl(p.id, "/overwrite"), undefined, "cool.preset.stored");
      } }, t("cool.preset.overwrite")),
    el("button", { type: "button", class: "sm", onclick: () => { coolPresetEdit = { kind: "rename", id: p.id }; renderCoolPresets(); } }, t("cool.preset.rename")),
    el("button", { type: "button", class: "sm danger", onclick: async () => {
      if (confirm(t("cool.preset.askDelete", { n: p.name }))) await coolPresetCall("DELETE", coolPresetUrl(p.id), undefined, "cool.preset.deleted");
    } }, t("cool.preset.delete")));
}

function renderCoolPresets() {
  const box = $("#cool-presets");
  if (!coolPresets || !coolSetup) {
    box.hidden = true;
    box.replaceChildren();
    return;
  }
  box.hidden = false;
  const p = coolPresets;
  const full = p.presets.length >= p.max;
  const saveOff = !coolPresetsCanKeep() || full;
  const select = el("select", { id: "cool-preset-select", "aria-label": t("cool.preset"), disabled: !p.presets.length || !!p.error },
    p.active ? null : el("option", { value: "", disabled: true }, t("cool.preset.custom")),
    p.presets.map((x) => el("option", { value: x.id }, x.name)));
  select.value = p.active || "";
  select.addEventListener("change", () => coolActivatePreset(select.value));
  if (!p.presets.length) coolManage = false;
  box.replaceChildren(...[
    el("div", { class: "cool-preset-row" },
      el("span", { class: "cool-preset-label" }, t("cool.preset")),
      select,
      el("button", { type: "button", id: "cool-preset-save", class: "sm cool-needs-clean", "data-off": saveOff ? "1" : null, disabled: saveOff,
        title: coolDirty ? t("cool.preset.saveFirst") : full ? t("cool.preset.limit", { n: p.max }) : null,
        onclick: () => { coolPresetEdit = { kind: "save" }; renderCoolPresets(); } }, t("cool.preset.saveAs")),
      el("button", { type: "button", id: "cool-preset-manage", class: "sm ghost", "aria-expanded": coolManage ? "true" : "false", disabled: !p.presets.length,
        onclick: () => { coolManage = !coolManage; renderCoolPresets(); } }, t("cool.preset.manage"))),
    p.error ? el("div", { class: "cool-problems" }, el("div", {}, t("cool.preset.error", { e: p.error }))) : null,
    coolPresetEdit?.kind === "save" ? coolNameForm("", async (name) => {
      coolPresetEdit = null;
      await coolPresetCall("POST", "/api/cooling/presets", { name }, "cool.preset.stored");
    }, () => { coolPresetEdit = null; renderCoolPresets(); }) : null,
    coolManage && p.presets.length ? el("div", { class: "cool-preset-list" }, p.presets.map(coolPresetItem)) : null,
    coolStale ? el("div", { class: "cool-problems cool-stale" },
      el("div", {}, t("cool.stale")),
      el("button", { type: "button", id: "cool-reload", class: "sm", onclick: () => { coolDirty = false; coolStale = false; loadCoolingSetup(); } }, t("cool.reload"))) : null,
  ].filter(Boolean));
}

// The language changed: redraw the text of the plan, the preset bar and the open panel from the current state.
function coolRelang() {
  if (coolSetup) {
    renderCoolingSetup();
    renderCoolPresets();
    coolRenderPanel();
  }
  if (coolView) renderCooling(coolView);
}

// ---------- setup ----------
function renderCoolingSetup() {
  const box = $("#cool-setup");
  if (!coolSetup) {
    box.replaceChildren();
    return;
  }
  const cfg = coolSetup.config;
  const problems = coolingProblems();
  box.replaceChildren(
    el("div", { class: "cool-bar" },
      el("p", { class: "sub cool-bar-hint" }, t("cool.hint")),
      el("span", { id: "cool-mode-state", class: "badge cool-mode-state" }),
      el("button", { type: "button", onclick: () => {
        const before = new Map([...coolParts].map(([k, p]) => [k, p.node.getBoundingClientRect().top]));
        coolTidy(true);
        coolRenderBoard();
        coolFlip(before);
      } }, t("cool.tidy")),
      el("button", { type: "button", id: "cool-measure-all", disabled: true, onclick: () => {
        const n = coolOutputs().filter((o) => !o.pump && coolDriven(o)).length;
        coolStartMeasure("/api/cooling/measure-all", {}, t("cool.cal.allAsk", { c: n, note: coolPumpNote() }));
      } }, t("cool.cal.all")),
      el("div", { class: "cool-bar-end" },
        el("button", { type: "button", id: "cool-reset", disabled: !coolDirty, onclick: () => {
          coolDirty = false;
          loadCoolingSetup();
        } }, t("cool.reset")),
        el("button", { type: "button", id: "cool-save", class: "primary", disabled: !coolDirty || problems.length > 0, onclick: saveCoolingSetup }, t("cool.save")))),
    el("p", { id: "cool-all-why", class: "cool-hint", hidden: true }),
    el("div", { id: "cool-problems", class: "cool-problems", hidden: !problems.length }, problems.map((p) => el("div", {}, p))),
    ...(coolSetup.fansHeld && cfg.outputs.length ? [el("p", { class: "sub" }, t("cool.fansHeld"))] : []),
    el("div", { class: "cool-layout" },
      el("div", { id: "cool-board", class: "cool-board", onclick: (ev) => {
        if (coolDragging || !coolSel || ev.target.closest("button, select")) return;
        coolSel = null;
        coolRenderPanel();
        coolHighlight(null);
      } }),
      el("aside", { id: "cool-panel", class: "cool-panel", hidden: true })),
  );
  coolRenderBoard();
  coolRenderPanel();
  coolUpdateModeChip();
  coolUpdateMeasureUI();
  if (coolObserver) coolObserver.disconnect();
  coolObserver = new ResizeObserver(() => coolDrawCables());
  coolObserver.observe($("#cool-board"));
}

// Blocks slide from where they were to where they now belong.
function coolFlip(before) {
  if (matchMedia("(prefers-reduced-motion: reduce)").matches) return;
  const cables = $(".cool-cables");
  if (cables) cables.animate([{ opacity: 0 }, { opacity: 0, offset: 0.6 }, { opacity: 1 }], { duration: 420 });
  for (const [k, part] of coolParts) {
    const was = before.get(k);
    if (was === undefined) continue;
    const dy = was - part.node.getBoundingClientRect().top;
    if (dy) part.node.animate([{ transform: `translateY(${dy}px)` }, { transform: "none" }], { duration: 320, easing: "cubic-bezier(.2,.8,.2,1)" });
  }
}

// The server decides what is valid. A refusal leaves the old settings running.
async function saveCoolingSetup() {
  // A fan taken off its curve keeps the safe speed the loop leaves it at. That
  // is the right end for a loop but not obvious from pulling a cable.
  if (coolSetup.config.mode === "live" && coolSetup.savedMode === "live") {
    const left = coolOutputs().filter((o) => !o.pump && !coolDriven(o) && coolSetup.savedOnCurve.has(o.id));
    if (left.length && !confirm(t("cool.parkAsk", { n: left.map((o) => o.name || o.id).join(", ") }))) return;
  }
  try {
    await api("PUT", "/api/cooling/config", coolSetup.config, false, { "If-Match": String(coolSetup.revision) });
  } catch (err) {
    if (err.status === 409) {
      // Changed meanwhile: the edits stay, the page offers to reload.
      coolStale = true;
      renderCoolPresets();
      toast(t("cool.conflict"), true);
      return;
    }
    toast(err.message, true);
    return;
  }
  coolDirty = false;
  toast(t("cool.saved"));
  await loadCoolingSetup();
}

// quiet leaves the page alone unless the list of fans has changed.
async function loadCoolingSetup(quiet) {
  if (coolLoading) return;
  coolLoading = true;
  try {
    const set = await api("GET", "/api/cooling/setup");
    if (quiet && coolDirty) return;
    if (quiet && coolSetup && set.fansHeld === coolSetup.fansHeld && JSON.stringify(set.fans) === JSON.stringify(coolSetup.fans)) return;
    set.savedMode = set.config.mode || "preview";
    set.savedOnCurve = new Set(set.config.outputs.filter(coolDriven).map((o) => o.id));
    set.config.curves = set.config.curves || [];
    for (const c of set.config.curves) {
      c.sources = c.sources || [];
      // A curve saved as another kind may have no shape; switching back needs one.
      if (!c.points || !c.points.length) c.points = coolDefaultPoints();
    }
    // Every fan found is on the plan; saving it unconnected changes nothing.
    for (const id of set.fans) if (!set.config.outputs.some((o) => o.id === id)) set.config.outputs.push(coolingNewFan(id));
    coolSetup = set;
    coolPresets = set.presets;
    coolStale = false;
    coolSensorKeys = coolSensorKeys.filter((k) => set.temps.some((s) => s.key === k));
    coolTidy(false);
    renderCoolingSetup();
    renderCoolPresets();
  } catch (err) {
    toast(err.message, true);
  } finally {
    coolLoading = false;
  }
}

// ---------- status ----------
function coolingName(id) {
  const o = coolSetup && coolSetup.config.outputs.find((x) => x.id === id);
  return (o && o.name) || id;
}

// The calibration that drives output o right now, or null.
// What happens to the other fans during a calibration depends on the mode in effect.
function coolOthersNote(m) {
  if (!(coolView && coolView.live)) return t("cool.cal.othersPreview");
  return t(m.kind === "pump" ? "cool.cal.othersPump" : "cool.cal.others");
}

function coolCalibrating(o) {
  const s = coolOutputState(o);
  const m = coolView && coolView.measure;
  return s && s.held && m && m.running && m.id === o.id ? m : null;
}

// What the output shows as its duty: the set one while it is calibrated, nothing
// while the control does not act on it (pump not released or not matched).
function coolShownDuty(o, s) {
  if (!s) return 0;
  const m = coolCalibrating(o);
  if (m) return m.duty;
  if (s.unregulated || (o.pump && !coolDriven(o))) return 0;
  if (s.notReleased || s.notMatched || s.matching) return 0;
  if (o.pump && s.rpm === null) return 0; // speed unknown: no bar
  return s.duty;
}

// Phase and step of a running calibration in words, and how far the phase is (0-1, null: unknown).
// Step/Steps mean the duties of the way down only; standstill and restart count in seconds and percent.
function coolPhaseInfo(m) {
  const v = { s: (m.step || 0) + 1, S: m.steps || 1, h: m.heldSec || 0, H: m.holdSec || 30, d: m.duty, m: m.steps || 50 };
  switch (m.phase) {
    case "down": case "pump": return { text: t("cool.cal.ph." + m.phase, v), frac: v.s / v.S };
    case "stop": return { text: t("cool.cal.ph.stop", v), frac: v.h / v.H };
    case "start": return { text: t("cool.cal.ph.start", v), frac: m.duty / v.m };
  }
  return { text: t((m.index || 0) > 0 ? "cool.cal.ph.pause" : "cool.cal.ph.wait"), frac: null };
}

let coolRun = null; // the parts of the card of a running calibration

function coolBuildRun(box, m) {
  const stop = (keep) => api("DELETE", "/api/cooling/measure" + (keep ? "?keep=finished" : "")).catch((e) => toast(e.message, true));
  const r = {
    sig: [m.all ? 1 : 0, m.kind].join("|"),
    title: el("strong", { class: "cool-run-title" }),
    fan: el("span", { class: "cool-run-fan" }),
    phase: el("span", { class: "cool-run-phase" }),
    now: el("span", { class: "cool-run-now" }),
    bar: el("progress", { class: "cool-run-bar", max: 1, "aria-label": t("cool.cal.progress") }),
    fanBar: m.all ? el("progress", { class: "cool-run-bar cool-run-fanbar", max: 1 }) : null,
    steps: el("div", { class: "cool-run-steps" }),
    stepsKey: "",
  };
  const note = el("span", { class: "cool-hint cool-run-note" }, coolOthersNote(m));
  const buttons = el("div", { class: "cool-run-buttons" },
    el("button", { type: "button", id: "cool-measure-stop", class: "cool-run-stop", title: t("cool.cal.stopTip"), onclick: () => stop(false) }, t("cool.cal.stop")),
    m.all ? el("button", { type: "button", id: "cool-measure-stop-keep", class: "cool-run-stop", title: t("cool.cal.stopKeepTip"), onclick: () => stop(true) }, t("cool.cal.stopKeep")) : null);
  box.replaceChildren(
    el("div", { class: "cool-run-head" }, r.title, r.fan),
    ...[r.phase, r.now, r.bar, r.fanBar, r.steps, note, buttons].filter(Boolean));
  coolRun = r;
}

// The card of a running calibration is built once and updated in place: a
// button that is replaced every second cannot be clicked reliably.
function coolUpdateRun(box, m) {
  if (!coolRun || coolRun.sig !== [m.all ? 1 : 0, m.kind].join("|") || !box.contains(coolRun.title)) coolBuildRun(box, m);
  const r = coolRun, name = coolingName(m.id);
  r.title.textContent = t(m.kind === "pump" ? "cool.cal.titlePump" : "cool.cal.title", { n: name });
  r.fan.textContent = m.all ? t("cool.cal.fanOf", { i: (m.index || 0) + 1, c: (m.queue || []).length }) : "";
  const ph = coolPhaseInfo(m);
  r.phase.textContent = ph.text;
  r.now.textContent = m.phase ? t("cool.cal.now", { d: m.duty, r: m.rpm }) : "";
  if (ph.frac === null) r.bar.removeAttribute("value");
  else r.bar.value = Math.min(1, Math.max(0, ph.frac));
  if (r.fanBar) {
    const n = (m.queue || []).length || 1;
    r.fanBar.value = (m.index || 0) / n;
    r.fanBar.title = t("cool.cal.fanOf", { i: (m.index || 0) + 1, c: n });
  }
  const res = m.results || [];
  const key = JSON.stringify(res) + "|" + m.phase;
  if (key !== r.stepsKey) {
    r.stepsKey = key;
    const down = m.phase === "down" || m.phase === "pump";
    r.steps.replaceChildren(...(res.length ? [el("span", { class: "cool-run-steps-label" }, t("cool.cal.steps"))] : []),
      ...res.map((x, i) => el("span", { class: "cool-run-step" + (down && i === res.length - 1 ? " cool-run-step-now" : "") }, `${x.duty} % → ${x.rpm}`)));
  }
}

function renderCoolingMeasure(m) {
  const box = $("#cool-measure");
  if (!m || !m.id) {
    box.hidden = true;
    coolRun = null;
    return;
  }
  const name = coolingName(m.id);
  box.hidden = false;
  box.classList.toggle("cool-alarm", !!m.error);
  box.classList.toggle("cool-run-card", !!m.running);
  if (m.running) {
    coolUpdateRun(box, m);
    return;
  }
  coolRun = null;
  const outcomes = m.outcomes || [];
  const lines = [];
  if (m.canceled) {
    const k = outcomes.filter((o) => o.calibration).length;
    lines.push(k ? t("cool.cal.canceledKept", { k }) : t("cool.cal.canceled"));
  }
  else if (m.error) lines.push(t("cool.cal.fail", { n: name, e: m.error }));
  else if (m.all) lines.push(t("cool.cal.doneAll", { k: outcomes.filter((o) => o.calibration).length, c: (m.queue || []).length }));
  else lines.push(t("cool.cal.done", { n: name }));
  for (const o of outcomes) {
    if (o.error) lines.push(t(o.calibration ? "cool.cal.tableFail" : "cool.cal.outcomeFail", { n: coolingName(o.id), e: o.error }));
  }
  box.replaceChildren(...lines.map((l) => el("span", {}, l)));
}

function renderCooling(view) {
  coolView = view;
  const msg = $("#cool-msg");
  const outputs = view.outputs || [];
  const started = outputs.some((o) => o.temp !== null || o.duty > 0 || o.sensorLost);
  const blocked = view.blocked ? t("cool.blocked." + view.blocked, { p: view.program || "" }) : "";
  const brokenPump = outputs.find((o) => o.pumpDown);
  const defect = brokenPump ? t("cool.pumpDefect", { n: brokenPump.name || brokenPump.id }) : "";
  msg.hidden = !defect && !view.error && !blocked && (started || outputs.length === 0);
  msg.textContent = defect || (view.error ? t("cool.badfile", { e: view.error }) : blocked || t("cool.waiting"));
  msg.classList.toggle("cool-alarm", !!defect || !!blocked || !!view.error);
  const regulating = outputs.some((o) => !o.pump) || (coolSetup?.config?.outputs || []).some((o) => o.pump && coolDriven(o));
  $("#cool-sub").textContent = t(!view.live ? "cool.sub" : regulating ? "cool.subLive" : "cool.subIdle");
  const measuring = !!(view.measure && view.measure.running);
  coolUpdateModeChip();
  renderCoolingMeasure(view.measure);
  coolApplyLive();

  // The setup is not redrawn while being edited: a finished measurement or a
  // changed state only updates it when no change would be lost.
  const live = !!view.live, isBlocked = !!view.blocked;
  const changed = live !== coolLive || isBlocked !== coolBlocked || measuring !== coolMeasuring;
  if (coolMeasuring && !measuring && coolDirty) coolTakeMeasurement();
  coolLive = live;
  coolBlocked = isBlocked;
  coolMeasuring = measuring;
  coolUpdateMeasureUI();
  if (!coolSetup || (changed && !coolDirty)) loadCoolingSetup();

  // Something else stored settings (a preset through the API, a mode switch):
  // without edits the page follows, with edits it says so and keeps them.
  if (coolSetup && view.revision && view.revision !== coolSetup.revision) {
    if (!coolDirty) loadCoolingSetup();
    else if (!coolStale) { coolStale = true; renderCoolPresets(); }
  }
  if (coolPresets && (view.preset || "") !== coolPresets.active) loadCoolPresets();
}

// A measurement ended during editing: its result joins the edits, or the next
// save would throw it away.
async function coolTakeMeasurement() {
  try {
    const set = await api("GET", "/api/cooling/setup");
    if (!coolSetup) return;
    for (const saved of set.config.outputs) {
      const o = coolOutput(saved.id);
      if (o) {
        o.speeds = saved.speeds;
        o.calibration = saved.calibration;
      }
    }
    coolSetup.suggestions = set.suggestions;
    // The measurement stored the settings once; only that is not a foreign change.
    if (coolSetup && set.revision === coolSetup.revision + 1) {
      coolSetup.revision = set.revision;
      if (coolStale) { coolStale = false; renderCoolPresets(); }
    }
    coolRenderPanel();
  } catch (err) {
    toast(err.message, true);
  }
}

// An alarm is shown on every tab: nobody keeps the cooling page open to wait for one.
function coolShowAlarm(alarm) {
  const box = $("#cool-alarm-banner");
  box.hidden = !alarm;
  if (alarm) $("#cool-alarm-text").textContent = t("cool.alarm." + alarm.kind, { n: alarm.name || "", p: alarm.program || "" });
}
$("#cool-alarm-open").addEventListener("click", () => $('[data-tab="cooling"]').click());

// The mode is switched in the settings. Only the mode follows here: edits in
// the plan stay as they are and are neither lost nor saved by the switch.
let coolStoredMode = ""; // the mode the server has stored
// The chip shows what is in effect (the server's view), not only what is stored.
function coolUpdateModeChip() {
  const chip = $("#cool-mode-state");
  if (!chip || !coolSetup) return;
  const saved = coolSetup.savedMode === "live";
  const live = !!(coolView && coolView.live); // "active" only once the loop says so, not from the stored mode
  chip.textContent = t(live ? "cool.status.live" : "cool.status.preview");
  chip.classList.toggle("on", live);
  chip.title = saved && !live ? t("cool.status.pending") : t("cool.status.where");
}
function coolSyncMode(mode) {
  coolStoredMode = mode || "";
  const sw = $("#set-cool-live");
  if (sw && document.activeElement !== sw) sw.checked = mode === "live";
  if (!mode || !coolSetup || coolSetup.savedMode === mode) return;
  coolSetup.savedMode = coolSetup.config.mode = mode;
  coolUpdateModeChip();
  const problems = coolingProblems();
  const save = $("#cool-save");
  if (save) save.disabled = !coolDirty || problems.length > 0;
  const box = $("#cool-problems");
  if (box) {
    box.hidden = !problems.length;
    box.replaceChildren(...problems.map((p) => el("div", {}, p)));
  }
}

$("#set-cool-live").addEventListener("change", async (ev) => {
  const want = ev.target.checked;
  const wasLive = coolStoredMode === "live";
  if (want && !wasLive && !confirm(t("cool.goLive"))) { ev.target.checked = false; return; }
  if (!want && wasLive && !confirm(t("cool.leaveLive"))) { ev.target.checked = true; return; }
  try {
    const res = await api("PUT", "/api/cooling/mode", { mode: want ? "live" : "preview" }, true);
    // The page follows the mode itself (coolSyncMode); a foreign change at the same time stays visible.
    const rev = Number(res.headers.get("ETag"));
    if (coolSetup && rev === coolSetup.revision + 1) {
      coolSetup.revision = rev;
      if (coolStale) { coolStale = false; renderCoolPresets(); }
    }
    toast(t("saved"));
  } catch (err) {
    toast(err.message, true);
    ev.target.checked = !want;
  }
});

// While the tab is open it follows the loop. It keeps asking without a controller
// too: one can be plugged in later, and an alarm must still get through.
async function pollCooling() {
  if (coolSetup && coolSetup.fansHeld && !coolDirty && $("#tab-cooling").classList.contains("active")) loadCoolingSetup(true);
  let view;
  try {
    view = await api("GET", "/api/cooling");
  } catch {
    // No old values, least of all an old "OK", stay on screen.
    if ($("#tab-cooling").classList.contains("active")) renderCooling({ outputs: [] });
    setTimeout(pollCooling, 1000);
    return;
  }
  coolShowAlarm(view.alarm);
  coolSyncMode(view.mode);
  $('[data-tab="cooling"]').hidden = view.disabled || (!view.configured && !view.available);
  if ($('[data-tab="cooling"]').hidden) ensureTabShown();
  if (!view.configured && !view.available) {
    setTimeout(pollCooling, view.disabled ? 1000 : 5000);
    return;
  }
  if ($("#tab-cooling").classList.contains("active")) renderCooling(view);
  setTimeout(pollCooling, 1000);
}
pollCooling();
