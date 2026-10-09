# Design — Project Identity

> This document is project-long-lived. Tokens are not changed without
> the Architect's approval. Developers MUST use these tokens
> instead of improvising their own colors/spacings.

## Style Direction

Calm, workshop-professional: a light, daylight-readable graphite-and-paper interface with a confident signal blue for every action and a warm orange reserved for 'in Arbeit' and attention — precise and quiet like a Stripe dashboard, matter-of-fact like a properly kept service order sheet.

## Colors

- `--color-bg`: **#F6F7F9**
- `--color-surface`: **#FFFFFF**
- `--color-surfaceAlt`: **#EEF1F4**
- `--color-fg`: **#14181F**
- `--color-fgMuted`: **#5B6470**
- `--color-border`: **#DCE1E7**
- `--color-borderStrong`: **#B9C1CB**
- `--color-accent`: **#0B63CE**
- `--color-accentHover`: **#0A55B2**
- `--color-accentActive`: **#08468F**
- `--color-accentSoft`: **#E7F0FD**
- `--color-signal`: **#E8590C**
- `--color-signalSoft`: **#FDEDE3**
- `--color-focus`: **#0B63CE**
- `--color-statusAngefragt`: **#6B7684**
- `--color-statusAngefragtBg`: **#EEF1F4**
- `--color-statusBestaetigt`: **#0B63CE**
- `--color-statusBestaetigtBg`: **#E7F0FD**
- `--color-statusInArbeit`: **#B44A0A**
- `--color-statusInArbeitBg`: **#FDEDE3**
- `--color-statusFertig`: **#1B7F4B**
- `--color-statusFertigBg`: **#E4F4EB**
- `--color-statusAbgeholt`: **#3A4450**
- `--color-statusAbgeholtBg`: **#E9ECEF**
- `--color-success`: **#1B7F4B**
- `--color-successBg`: **#E4F4EB**
- `--color-warning`: **#B7791F**
- `--color-warningBg`: **#FDF6E3**
- `--color-danger`: **#C0342B**
- `--color-dangerBg`: **#FCEBEA**
- `--color-overlay`: **rgba(20, 24, 31, 0.48)**

## Typography

- `font_family`: Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, 'Noto Sans', sans-serif
- `font_note`: Inter self-hosted from the app's own origin (AC-23); never a Google-Fonts/CDN request. Numeric text (amounts, times, km, order numbers) always uses font-variant-numeric: tabular-nums so columns line up.
- `heading_weight`: 600
- `body_weight`: 400
- `label_weight`: 500
- `size_scale`: 12px / 1.4 (meta, table captions), 14px / 1.5 (labels, table cells), 16px / 1.6 (body), 18px / 1.5 (card title), 24px / 1.3 (page title), 32px / 1.2 (dashboard KPI value)
- `heading_color`: fg
- `line_length`: measure 60-75 characters for prose (Impressum, Datenschutzerklärung, Problembeschreibung display)

## Spacing Scale

- `--space-0`: 4px
- `--space-1`: 8px
- `--space-2`: 12px
- `--space-3`: 16px
- `--space-4`: 24px
- `--space-5`: 32px
- `--space-6`: 48px

## Border-Radii

- `--radius-sm`: 6px
- `--radius-md`: 10px
- `--radius-lg`: 16px
- `--radius-pill`: 999px

## Components

### Button

Base: min-height 44px (touch target, AC-15), padding 12px 20px, radius md (10px), font 14px/600, letter-spacing 0, gap 8px between icon and label, no text transform, cursor pointer, transition 120ms ease. Primary: bg=accent, fg=#FFFFFF, hover bg=accentHover, active bg=accentActive + translateY(1px), focus-visible: 2px outline accent with 2px offset, disabled: opacity 0.55, cursor not-allowed, no hover change. Secondary: bg=surface, fg=fg, border 1px borderStrong, hover bg=surfaceAlt, active bg=surfaceAlt + translateY(1px). Tertiary/Text: bg transparent, fg=accent, underline on hover, min-height 44px but padding 12px 8px. Danger: bg=danger, fg=#FFFFFF, hover #A62B23. Sizes: md 44px (default), sm 36px only inside dense table rows, lg 52px for the customer 'Termin anfragen' submit. Full-width below 640px. Loading state: label replaced by 3-dot indicator, button stays disabled and keeps its width. A button whose function is not built yet is NOT rendered as a dead control: either hidden, or visible but disabled with the label suffix ' (bald verfügbar)' and a tooltip naming the reason (AC-14).

### TextField

Label above the field, 14px/500, fg, margin-bottom 4px; optional 'erforderlich' marker as a 12px fgMuted suffix, never a red asterisk alone. Input: height 44px, padding 10px 12px, radius sm (6px), bg=surface, border 1px border, fg 16px/400 (16px avoids iOS zoom), placeholder fgMuted, hover border borderStrong, focus border accent + 2px accentSoft ring (no layout shift). Optional help text 12px fgMuted below, 4px gap. Error state: border 1px danger, ring dangerBg; message below in 13px danger with a 4px gap, replaces the help text. Validation only after the field was blurred with content or after a submit attempt — an untouched form starts neutral, no red anywhere (AC-16). Field groups: 16px to the next field, 24px to the next group. Numeric fields (Stunden, Menge, Einzelpreis, Kilometerstand) right-aligned with tabular-nums, inputmode=decimal/numeric. Labels in German, exactly as the screens show them: 'Kennzeichen', 'Marke', 'Modell', 'Kilometerstand', 'Wunschtermin', 'Problembeschreibung', 'Arbeitszeit (Stunden)', 'Menge', 'Einzelpreis (netto)', 'E-Mail', 'Passwort'.

### Select

Same box metrics as TextField (44px, radius sm, border 1px border). Used for the status filter ('Alle Status', 'angefragt', 'bestätigt', 'in Arbeit', 'fertig', 'abgeholt') and for status transitions, where only the legally allowed next status is offered as an enabled option — everything else is disabled, not removed, so the model stays visible (AC-03). Custom chevron at 16px, fgMuted, 12px from the right edge.

### StatusBadge

Pill (radius pill), padding 4px 10px, 13px/600, always dot 8px + label, dot uses the status colour, background the matching soft token. Exact labels, wording and order fixed product-wide: 'angefragt' (statusAngefragt), 'bestätigt' (statusBestaetigt), 'in Arbeit' (statusInArbeit / signal orange), 'fertig' (statusFertig / green), 'abgeholt' (statusAbgeholt / graphite). Never colour alone as the signal — dot plus word. Not interactive; the status change happens through Button + Select.

### Card

bg=surface, radius lg (16px), border 1px border, padding 24px (16px below 640px), no shadow by default; a raised variant (shadow 0 1px 2px rgba(20,24,31,0.06), 0 8px 24px rgba(20,24,31,0.06)) only for the dashboard KPI tiles and modals. Card header: title 18px/600, optional action button right-aligned on desktop, below the title on mobile. Card body gap 16px. Cards are the mobile form of every table row.

### DataTable

Header row: 12px/600 uppercase fgMuted, letter-spacing 0.04em, border-bottom 1px borderStrong. Cells: 14px/400 fg, padding 12px 16px, row border-bottom 1px border, row hover bg=surfaceAlt, selected row bg=accentSoft. Money and hours right-aligned with tabular-nums. Sticky header on scroll inside 'Auftragsliste'. ≥1024px all columns (Auftragsnummer, Kennzeichen, Fahrzeug, Status, Wunschtermin, Betrag brutto, Aktion); 640-1023px drop Fahrzeug and Wunschtermin; <640px render nothing tabular — each order becomes a Card with line 1 'AUF-JJJJ-NNNNNN' + StatusBadge, line 2 'Kennzeichen · Marke Modell', line 3 Betrag brutto, and the row actions as a full-width Button (AC-15). No horizontal scroll anywhere.

### FilterBar

Sits directly above 'Auftragsliste': search TextField 'Nach Kennzeichen suchen' (left, grows, debounced 250ms) + Select 'Alle Status' (right); stacked full-width below 640px, gap 12px; a chip row below shows the active filter as a removable pill plus 'Filter zurücksetzen' text button; if the filtered result is empty, an EmptyState 'Keine Aufträge für diesen Filter' with the reset action (AC-10).

### StatTile

Dashboard KPI, raised Card variant, padding 24px, min-height 120px: label 14px/500 fgMuted on top, value 32px/600 fg with tabular-nums, one 12px fgMuted subline (e.g. 'Stand 14.03.2025, 09:30 Uhr'). Three tiles in a row ≥1024px, two-column 640-1023px, one column <640px. Exact labels: 'Offene Aufträge' (count), 'Heute fertig geworden' (count), 'Umsatz laufender Monat' (currency format, AC-12).

### Timeline

Statusverlauf of an order (AC-04): vertical rail 2px borderStrong at a 20px gutter, each entry a 12px dot in the status colour (filled = reached, ring only = not yet reached), right of it the StatusBadge plus '<TT.MM.JJJJ, HH:MM Uhr>' in 14px fg and the acting employee as 12px fgMuted ('von Anna Meier'), 16px between entries. All five states always shown in order 'angefragt → bestätigt → in Arbeit → fertig → abgeholt' so the customer sees where the order stands and what comes next.

### InvoiceTable

Rechnungsansicht: line items as a table (Position, Beschreibung, Menge, Einzelpreis, Summe) — 'Arbeitszeit' with '2,50 h', 'Teile' with '4 Stück' / '12,90 €' / '51,60 €', all right-aligned tabular-nums. Below a right-aligned summary block, 8px between rows, separated by a 1px border above it: 'Netto' , 'Mehrwertsteuer (19 %)' , 'Brutto' in 18px/600 fg; every amount through the single currency format rule, values come from the API in whole cents (AC-08). Header line: 'Rechnung zu Auftrag AUF-JJJJ-NNNNNN', plus the vehicle 'Kennzeichen · Marke Modell'.

### BookingForm (Kundenbereich 'Termin anfragen')

Single-column card form, max-width 560px, fields in this order: Name, E-Mail, Telefon, Kennzeichen, Marke, Modell, Kilometerstand, Wunschtermin, Problembeschreibung. Gruppen-Trennung 24px between 'Kontakt', 'Fahrzeug', 'Anliegen' with 14px/600 sub-headings. 'Wunschtermin' is a date-time field with hint 'Wunschtermin, kein fester Termin'; 'Problembeschreibung' is a 4-row textarea with a 500-character counter bottom-right in 12px fgMuted. Submit 'Termin anfragen' = lg primary Button, full-width below 640px. On success the form is replaced by a confirmation panel with the new Auftragsnummer in large tabular type and the note that the status can be tracked with Auftragsnummer + Kennzeichen (AC-01, AC-02).

### StatusLookup (Kundenbereich 'Status abrufen')

Two fields side by side on desktop, stacked on mobile: 'Auftragsnummer' and 'Kennzeichen', plus Button 'Status abrufen'. On success: StatusBadge, Timeline, vehicle line and — only when the state is 'fertig' or 'abgeholt' — the InvoiceTable. A wrong pair yields one inline alert 'Zu dieser Auftragsnummer und diesem Kennzeichen liegt kein Auftrag vor.' in the danger tokens, no field-level blame. The fetch button shows its loading state and stays disabled while running (AC-08, AC-14).

### Alert / ErrorBody

Used for every API error so all endpoints look alike (AC-13, AC-21): full-width block, padding 12px 16px, radius sm, 1px border, left border 3px in the tone colour, icon 16px, message 14px/500, optional second line 13px fgMuted; the body only ever shows the error identifier and the human message — no SQL, no stack traces, no file paths. Variants: danger (fcaeb), warning (warning/warningBg, e.g. 'Zu viele Anmeldeversuche, bitte in einer Minute erneut versuchen.' for 429), success (success/successBg, e.g. 'Auftrag bestätigt.'), info (accent/accentSoft). Field-level errors use the TextField error state instead. Errors appear above the action that caused them, never as a native browser alert.

### Modal / Dialog

For 'Position erfassen' / 'Position bearbeiten' and for confirming a status change. Overlay = overlay token, dialog = raised Card, radius lg, max-width 520px, padding 24px, vertically centred on desktop, as a bottom sheet (radius lg lg 0 0, max-height 90vh, internal scroll) below 640px. Header 18px/600 'Position erfassen', body = TextField/Select groups, footer right-aligned secondary 'Abbrechen' + primary 'Speichern', both min-height 44px, stacked full-width below 640px. Focus is trapped, Escape closes, the first field receives focus, and focus returns to the trigger on close.

### AppShell (Header, Nav, Footer)

Header: bg=surface, border-bottom 1px border, height 64px (56px mobile), product name 'Werkstattportal' 18px/600 left. Customer area nav as two tabs 'Termin anfragen' and 'Status abrufen'; active tab gets a 2px accent underline and fg=accent. Workshop area: nav 'Dashboard', 'Auftragsliste' plus the signed-in e-mail and a 'Abmelden' text button right (only shown in a valid session, AC-09/AC-17). Below 640px the nav collapses to a menu button opening a full-width panel with 44px rows. Login screen is a single centred Card (max-width 380px) with 'E-Mail', 'Passwort', primary 'Anmelden' full-width; on 429 the Alert text names the rate limit. Footer: 1px top border, 14px fgMuted, links 'Impressum' and 'Datenschutzerklärung' — present on EVERY page of both areas (AC-22), with the current year and the workshop name; link targets are internal routes, no external domains.

### EmptyState

Centred block, padding 48px 24px, icon 24px fgMuted, title 18px/600 fg, description 14px fgMuted (max 45ch), optional primary Button, 16px gaps. German copy per case: 'Noch keine Aufträge' (list), 'Keine Aufträge für diesen Filter' (with reset action), 'Noch keine Rechnung vorhanden — sie erscheint, sobald der Auftrag fertig ist.' (customer view).

### Interaction & Accessibility rules

Focus ring 2px accent + 2px offset on every focusable element, keyboard order follows the visual order, all icons are decorative or have aria-labels, icon-only controls carry an accessible name, StatusBadge text meets ≥4.5:1 on its soft background, body text ≥4.5:1 on bg/surface, targets ≥44px (AC-15). Loading is always signalled inside the control that was triggered (spinner in the button, skeleton rows in the table) — never a full-page blocker. Every visible control either does what it says or is clearly disabled as 'bald verfügbar' (AC-14).

### Legal pages (Impressum, Datenschutzerklärung)

Prose pages inside the same AppShell: container max-width 720px, page title 24px/600, sections with 18px/600 sub-headings, body 16px/1.6, line length capped at 75 characters, 24px between sections, no images, no embeds, self-hosted fonts only (AC-23). Same header and footer as everywhere; the footer links point here in every area (AC-22).

## Layout Principles

- Container: max-width 1120px, centred; horizontal padding 16px (<640px), 24px (640-1023px), 32px (≥1024px). Text-only pages (Impressum, Datenschutzerklärung) cap at 720px.
- Breakpoints: <640px single column (mobile, fully operable, AC-15), 640-1023px two columns, ≥1024px 12-column grid at 1120px. Workshop area is desktop-first but every table falls back to stacked cards below 640px — never a horizontal scrollbar.
- Vertical rhythm: section gap 32px mobile / 48px desktop, card and form-group gap 16px, inline gap 8-12px, table and tile padding 16-24px. Only values from the spacing scale are used — no ad-hoc pixels.
- Forms: label above the field, one field per row below 640px and at most two from 640px; primary action sits bottom-left (customer area) or in the card header (workshop area) and is full-width below 640px with min-height 44px. Validation appears only after a field was filled and left or after a submit attempt; an untouched form is neutral (AC-16).
- Every page shares the same header and the same footer; the footer of both areas always links 'Impressum' and 'Datenschutzerklärung' (AC-22). Only one page ever uses a centred single-card layout: the login.
- Amounts, counts and hours use tabular figures everywhere, and every number that arrives from the API in whole cents is formatted on display only — the API's cent values are never re-rounded or sent back as floats.
- ONE format for every date and time in the whole product: 'TT.MM.JJJJ, HH:MM Uhr' in Europe/Berlin, seconds never shown — e.g. '14.03.2025, 09:30 Uhr'. Relative wording ('vor 2 Stunden') is not used.
- ONE format for every amount: German notation with thousands dot, comma decimals and trailing symbol, always two decimals — e.g. '1.234,56 €' (Intl.NumberFormat('de-DE',{style:'currency',currency:'EUR'})); negative values as '-12,90 €'.
- ONE format for every labour time: two decimals plus unit — e.g. '2,50 h'; the hourly rate from the configuration is shown with the same currency rule, e.g. '24,50 € / h'.
- ONE format for every part quantity: integer plus ' Stück' — e.g. '4 Stück'; unit prices use the currency rule.
- ONE format for every odometer reading: thousands dot, no decimals, unit 'km' — e.g. '123.456 km'.
- ONE format for every percentage: figure plus non-breaking space plus '%' — e.g. '19 %'.
- ONE format for every identifier: order number 'AUF-JJJJ-NNNNNN' (e.g. 'AUF-2025-000123'), licence plate uppercase with hyphen (e.g. 'B-AB 1234').
- Order-status wording is fixed product-wide, in this order: 'angefragt', 'bestätigt', 'in Arbeit', 'fertig', 'abgeholt' — the same words in badges, filters, timelines and transitions.
- Accessibility baseline: contrast ≥4.5:1 for text, 2px visible focus ring on every interactive element, all targets ≥44px, status never communicated by colour alone.
- Privacy in the UI (AC-23): fonts and scripts come from the app's own origin only — no CDN, no Google Fonts, no analytics; nothing is embedded from a third-party domain on any page of the customer area.
