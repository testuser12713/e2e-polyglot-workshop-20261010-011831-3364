# Werkstattportal — Web-App

Vite + React + TypeScript Gerüst der Kunden- und Werkstatt-Oberfläche des
Werkstattportals. Die App ist das gemeinsame Shell-Gerüst (Layout, Navigation,
Fußzeile und Routentabelle), in das die einzelnen Seiten ihre Inhalte einhängen.
Sie spricht ausschließlich über `src/lib/api.ts` mit der Werkstatt-API.

## Technischer Aufbau

- **Vite** (Build und Dev-Server)
- **React** + **React Router** (Routentabelle in `src/App.tsx`)
- **TypeScript**
- **Vitest** + **Testing Library** (Tests)
- Schriften (**Inter**) sind über `@fontsource/inter` selbst gehostet und werden
  beim Build in `dist/assets` gebündelt — es wird keine Ressource von einer
  fremden Domain geladen (Google Fonts/CDN werden bewusst nicht verwendet).

## Installation

Voraussetzung: Node.js (aktuelle LTS-Version) mit npm.

```bash
npm install
```

## Entwicklung starten

```bash
npm run dev
```

Der Dev-Server läuft anschließend unter <http://localhost:5173>.

## Produktions-Build

```bash
npm run build
npm run preview
```

`npm run build` erzeugt den statischen Build in `dist/`, `npm run preview`
serviert ihn lokal (Standard-Port 5173).

## Tests

```bash
npm test
```

## Konfiguration

Die Basis-URL der API wird über die Umgebungsvariable `VITE_API_BASE_URL`
gesetzt (sie wird beim Build eingebettet). Im laufenden Produkt liefert der
Run-Contract den Origin des API-Dienstes (`${service:api.origin}`). Solange der
API-Dienst nicht deklariert ist, greift der dokumentierte lokale Fallback
`http://localhost:8080`.

```bash
VITE_API_BASE_URL=http://localhost:8080 npm run dev
```

## Routen

| Route | Seite |
| --- | --- |
| `/` | Startseite mit den Einstiegen in Kunden- und Werkstattbereich |
| `/terminanfrage` | Termin anfragen (Kundenbereich) |
| `/auftragsstatus` | Status abrufen (Kundenbereich) |
| `/impressum` | Impressum |
| `/datenschutz` | Datenschutzerklärung |
| `/werkstatt/anmeldung` | Anmeldung Werkstattbereich |
| `/werkstatt/auftraege` | Auftragsliste (nur mit Sitzung) |
| `/werkstatt/auftraege/:orderNumber` | Auftragsdetails (nur mit Sitzung) |
| `/werkstatt/dashboard` | Dashboard (nur mit Sitzung) |

Die Werkstatt-Routen sind mit `RequireAuth` geschützt: ohne gültige Sitzung
(`useSession`) wird auf `/werkstatt/anmeldung` umgeleitet. Die Fußzeile verlinkt
auf jeder Seite Impressum und Datenschutzerklärung.

## Hinweis zum Umfang

Dieses Ticket liefert das Gerüst. Die einzelnen Seiten sind als Stubs angelegt
und werden von den jeweiligen Feature-Tickets mit Inhalt gefüllt; noch nicht
gebaute Bedienelemente sind sichtbar als „bald verfügbar“ deaktiviert.
