# Kfz-Werkstatt Kundenportal

Ein Kundenportal für eine Kfz-Werkstatt, aufgebaut aus drei eigenständig
laufenden Diensten: einer Werkstatt-API in Go (net/http, pgx) auf PostgreSQL,
einem Rechnungs-Worker in Python, der fertiggestellte Aufträge über eine
Valkey-Warteschlange abarbeitet und Rechnungen samt Kundenbenachrichtigung
erzeugt, sowie einer Web-App mit Vite/React/TypeScript. Kunden fragen Termine
an, verfolgen den Status ihres Auftrags und sehen ihre Rechnung; nach Anmeldung
verwalten Werkstattmitarbeiter Aufträge, Positionen und Status und nutzen ein
Dashboard. Die Oberfläche des Produkts ist deutsch.

Dieses Repository enthält derzeit das Gerüst (Skeleton) der API: die
Konfiguration, das Datenbankschema, die einheitliche Fehlerbehandlung und alle
vertraglich vereinbarten Routen. Die Routen antworten zunächst mit dem
einheitlichen `501 not_implemented`-Fehlerkörper, bis die jeweils zuständigen
Tickets sie implementieren.

## Tech-Stack

- **api**: Go (net/http aus der Standardbibliothek, pgx für PostgreSQL)
- **worker**: Python, Valkey-Client für die Warteschlange, PostgreSQL-Anbindung
- **web**: Vite + React + TypeScript
- **Datenbank**: PostgreSQL 18
- **Warteschlange**: Valkey 9.1

## Voraussetzungen

- Go (aktuelle Version)
- Docker (für PostgreSQL und Valkey über `compose.yaml`)

## Installation

Datenbank und Warteschlange lokal starten:

```bash
docker compose up -d
```

Go-Abhängigkeiten laden:

```bash
cd backend
go mod download
```

## Entwicklung starten

Die API liest ihre gesamte Konfiguration aus Umgebungsvariablen. Zum lokalen
Start die Variablen exportieren und die API starten:

```bash
# PostgreSQL und Valkey aus compose.yaml
export DATABASE_URL="postgresql://app@localhost:5432/app"
export VALKEY_URL="redis://localhost:6379/0"
export WORKSHOP_ADMIN_EMAIL="admin@werkstatt.example"
# Geheimnisse NICHT im Repository ablegen – hier nur für die lokale Sitzung setzen:
export WORKSHOP_ADMIN_PASSWORD="$(openssl rand -hex 16)"
export AUTH_TOKEN_SECRET="$(openssl rand -hex 32)"
export WEB_ORIGIN="http://localhost:5173"
export PORT=8080

cd backend
go run .
```

Beim Start legt die API das gesamte Schema an (idempotent,
`CREATE TABLE IF NOT EXISTS`) und richtet den Werkstatt-Mitarbeiter aus der
Konfiguration ein. Ein fehlender Pflichtwert führt zu einem Abbruch mit einer
Meldung, die die fehlende Variable benennt.

Prüfen, dass die API läuft:

```bash
curl http://localhost:8080/api/health
# -> {"status":"ok"}
```

## Produktion (Build)

```bash
cd backend
go build -o workshop-api .
./workshop-api
```

## Umgebungsvariablen

Jede vom Prozess zum Starten benötigte Variable steht in `RUN.json`.

| Variable | erforderlich | Bedeutung |
| --- | --- | --- |
| `PORT` | nein (Standard 8080) | Port, auf dem die API lauscht |
| `DATABASE_URL` | ja | PostgreSQL-Verbindungszeichenfolge |
| `VALKEY_URL` | ja | Valkey-Verbindungszeichenfolge |
| `WORKSHOP_ADMIN_EMAIL` | ja | E-Mail des beim Start angelegten Mitarbeiters |
| `WORKSHOP_ADMIN_PASSWORD` | ja | Startpasswort des Mitarbeiters (kein Literalwert im Repo) |
| `AUTH_TOKEN_SECRET` | ja | Schlüssel zum Signieren der Sitzungstoken |
| `WEB_ORIGIN` | nein | Origin der Web-App für CORS; ungesetzt = kein Cross-Origin-Zugriff |

## API-Endpunkte

Alle Antworten sind JSON. Jede fehlgeschlagene Anfrage liefert denselben
Fehlerkörper:

```json
{ "error": { "code": "not_found", "message": "not found" } }
```

Codes und HTTP-Status: `validation_error` 400, `unauthorized` 401,
`not_found` 404, `invalid_transition` 409, `rate_limited` 429,
`not_implemented` 501, `internal_error` 500.

| Methode | Pfad | Zweck |
| --- | --- | --- |
| GET | `/api/health` | Health-Check (prüft die Datenbankverbindung) |
| POST | `/api/customer/appointments` | Termin anfragen |
| GET | `/api/customer/orders/{order_number}?license_plate=` | Auftragsstatus abrufen |
| GET | `/api/customer/orders/{order_number}/invoice?license_plate=` | Rechnung abrufen |
| POST | `/api/workshop/login` | Mitarbeiter anmelden |
| GET | `/api/workshop/orders?status=&license_plate=` | Auftragsliste (Filter) |
| GET | `/api/workshop/orders/{order_number}` | Auftragsdetail |
| POST | `/api/workshop/orders/{order_number}/status` | Status setzen |
| POST | `/api/workshop/orders/{order_number}/items` | Position anlegen |
| PUT | `/api/workshop/orders/{order_number}/items/{item_id}` | Position ändern |
| GET | `/api/workshop/dashboard` | Kennzahlen des Dashboards |

Der Werkstattbereich verlangt den Header `Authorization: Bearer <token>`.
Der Statusfluss eines Auftrags ist `angefragt → bestätigt → in Arbeit → fertig
→ abgeholt`; jeder andere Übergang wird mit `409` abgewiesen.

### Beispiele

`GET /api/health` → `200`:

```json
{ "status": "ok" }
```

`POST /api/workshop/login` mit `{"email":"...","password":"..."}` → `200`:

```json
{
  "token": "…",
  "employee": { "id": 1, "name": "…", "email": "…" }
}
```

## Tests

```bash
cd backend
go test ./...
```

Die Datenbank-tests nutzen `TEST_DATABASE_URL`; ist die Variable nicht gesetzt,
werden sie sauber übersprungen.

## Funktionsumfang (Sprint-Ziel)

- Terminanfrage durch Kunden inkl. Kunden- und Fahrzeugdaten
- Statusverfolgung und Rechnungsabruf für Kunden
- Anmeldung und Sitzungsverwaltung für Werkstattmitarbeiter
- Auftragsliste mit Filter nach Status und Suche nach Kennzeichen
- Erfassung von Arbeitszeit- und Teilepositionen
- Statuslebenszyklus mit Verlauf und Warteschlangen-Nachricht
- Rechnungserstellung und Outbox-Benachrichtigung durch den Worker
- Dashboard mit offenen Aufträgen, heute fertiggestellten Aufträgen und Monatsumsatz
- Einheitliche Fehlerbehandlung ohne interne Details in Antworten
