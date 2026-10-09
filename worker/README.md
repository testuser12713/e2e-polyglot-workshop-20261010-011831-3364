# Rechnungs-Worker

Python-Dienst des Kfz-Werkstatt-Kundenportals. Er lauscht auf der
Valkey-Liste `invoices:queue`, nimmt je eine Nachricht
(`{"order_id": int, "order_number": str}`) ab und verarbeitet sie in drei
Schritten: Rechnung berechnen, Rechnung speichern, Kundenbenachrichtigung in
die Outbox legen. Anschließend wird die Nachricht bestätigt (der blockierende
`BLPOP` entfernt sie bereits atomar).

In diesem Sprint ist das der lauffähige Skelett-Stand: Queue, Konfiguration
und Datenbank-Anbindung sind echt, die drei Verarbeitungsschritte
(`invoice.build_invoice`, `store.save_invoice`, `outbox.enqueue_notification`)
sind als Stubs mit finaler Signatur verdrahtet und werden von eigenen Tickets
gefüllt.

## Voraussetzungen

- Python 3.13+
- PostgreSQL 18 und Valkey 9.1 (die Adressen kommen aus der Umgebung)

## Installation

```bash
cd worker
python3 -m pip install -e .
```

## Konfiguration

Der Worker liest die Konfiguration aus der Umgebung (niemals aus dem
Repository):

| Variable            | Bedeutung                                              |
| ------------------- | ------------------------------------------------------ |
| `DATABASE_URL`      | PostgreSQL-Verbindung (erforderlich)                   |
| `VALKEY_URL`        | Valkey-Verbindung (erforderlich)                       |
| `HOURLY_RATE_CENTS` | Stundensatz in ganzen Cent (Standard: `8900`)          |

`RUN.json` im Repository-Wurzelverzeichnis deklariert diese Variablen
(`DATABASE_URL`/`VALKEY_URL` als `${service:db.url}`/`${service:queue.url}`).
Fehlt eine erforderliche Variable, startet der Worker nicht und nennt sie in
der Fehlermeldung.

## Starten

Aus dem Verzeichnis `worker/` (dort liegt auch `pyproject.toml`), mit den oben
genannten Variablen:

```bash
cd worker
DATABASE_URL=postgresql://app:app@localhost:5432/app \
VALKEY_URL=redis://localhost:6379/0 \
HOURLY_RATE_CENTS=8900 \
python3 -m worker
```

Der Worker meldet `worker listening on invoices:queue` und verarbeitet danach
eintreffende Nachrichten. Der Datenbank-Container und der Valkey-Container
werden mit `docker compose up -d` (siehe `compose.yaml`) gestartet.

## Tests

```bash
cd worker
PYTHONPATH=. pytest
```

Die Tests prüfen das Parsen und Validieren von Warteschlangen-Nachrichten, den
einheitlichen Fehlerkörper und – sofern `VALKEY_URL` gesetzt ist – den echten
Verbrauch einer Nachricht aus Valkey.
