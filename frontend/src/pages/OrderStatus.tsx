import { useState, type FormEvent } from "react";
import { ApiError, apiFetch } from "../lib/api";
import "./OrderStatus.css";

interface Customer {
  name: string;
  email: string;
  phone: string;
}

interface Vehicle {
  license_plate: string;
  brand: string;
  model: string;
  mileage: number;
}

interface StatusEvent {
  from_status: string | null;
  to_status: string;
  changed_by: string;
  changed_at: string;
}

interface OrderDetail {
  order_number: string;
  status: string;
  requested_date: string;
  problem_description: string;
  created_at: string;
  customer: Customer;
  vehicle: Vehicle;
  items: unknown[];
  history: StatusEvent[];
}

interface InvoiceLine {
  description: string;
  quantity: number;
  unit_price_cents: number;
  total_cents: number;
}

interface Invoice {
  invoice_number: string;
  order_number: string;
  issued_at: string;
  lines: InvoiceLine[];
  labor_cents: number;
  parts_cents: number;
  net_cents: number;
  vat_cents: number;
  gross_cents: number;
}

const STATUS_FLOW = ["angefragt", "bestätigt", "in Arbeit", "fertig", "abgeholt"] as const;
const STATUS_CLASS = ["angefragt", "bestaetigt", "inArbeit", "fertig", "abgeholt"] as const;

type StatusName = (typeof STATUS_FLOW)[number];

const NOT_FOUND_MESSAGE =
  "Zu dieser Auftragsnummer und diesem Kennzeichen liegt kein Auftrag vor.";
const NO_INVOICE_MESSAGE =
  "Noch keine Rechnung vorhanden — sie erscheint, sobald der Auftrag fertig ist.";

const dateTimeFormat = new Intl.DateTimeFormat("de-DE", {
  timeZone: "Europe/Berlin",
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
});

const currencyFormat = new Intl.NumberFormat("de-DE", {
  style: "currency",
  currency: "EUR",
});

const decimalFormat = new Intl.NumberFormat("de-DE", {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const integerFormat = new Intl.NumberFormat("de-DE", {
  maximumFractionDigits: 0,
});

function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return `${dateTimeFormat.format(date)} Uhr`;
}

function formatCents(cents: number): string {
  return currencyFormat.format(cents / 100);
}

function formatHours(hours: number): string {
  return `${decimalFormat.format(hours)} h`;
}

function formatQuantity(quantity: number): string {
  return `${integerFormat.format(quantity)} Stück`;
}

function formatMileage(km: number): string {
  return `${integerFormat.format(km)} km`;
}

/** Labour lines arrive with fractional hours, part lines with whole counts. */
function formatLineQuantity(quantity: number): string {
  return Number.isInteger(quantity) ? formatQuantity(quantity) : formatHours(quantity);
}

function statusIndex(status: string): number {
  const index = STATUS_FLOW.indexOf(status as StatusName);
  return index < 0 ? 0 : index;
}

function statusClass(status: string): string {
  return STATUS_CLASS[statusIndex(status)];
}

function isCompleted(status: string): boolean {
  return statusIndex(status) >= STATUS_FLOW.indexOf("fertig");
}

function orderPath(orderNumber: string, licensePlate: string): string {
  return `/api/customer/orders/${encodeURIComponent(orderNumber)}?license_plate=${encodeURIComponent(licensePlate)}`;
}

function invoicePath(orderNumber: string, licensePlate: string): string {
  return `/api/customer/orders/${encodeURIComponent(orderNumber)}/invoice?license_plate=${encodeURIComponent(licensePlate)}`;
}

interface TimelineEntry {
  status: StatusName;
  reached: boolean;
  changedAt?: string;
  actor?: string;
}

function buildTimeline(order: OrderDetail): TimelineEntry[] {
  const current = statusIndex(order.status);
  const events = new Map<string, StatusEvent>();
  for (const event of order.history ?? []) {
    events.set(event.to_status, event);
  }
  return STATUS_FLOW.map((status, index) => {
    const reached = index <= current;
    const event = events.get(status);
    const changedAt = event?.changed_at ?? (status === "angefragt" ? order.created_at : undefined);
    const actor = event?.changed_by ?? (status === "angefragt" ? "System" : undefined);
    return {
      status,
      reached,
      changedAt: reached ? changedAt : undefined,
      actor: reached ? actor : undefined,
    };
  });
}

function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`status-badge status-${statusClass(status)}`}>
      <span className="dot" aria-hidden="true" />
      {status}
    </span>
  );
}

function Timeline({ entries }: { entries: TimelineEntry[] }) {
  return (
    <ul className="timeline">
      {entries.map((entry) => (
        <li
          key={entry.status}
          className={`timeline-item tl-${statusClass(entry.status)}${entry.reached ? " is-reached" : ""}`}
        >
          <div className="timeline-row">
            <StatusBadge status={entry.status} />
            {entry.reached && entry.changedAt && (
              <span className="timeline-time">{formatDateTime(entry.changedAt)}</span>
            )}
            {entry.reached && (
              <span className="timeline-actor">von {entry.actor ?? "System"}</span>
            )}
          </div>
        </li>
      ))}
    </ul>
  );
}

function InvoiceSection({ order, invoice }: { order: OrderDetail; invoice: Invoice }) {
  const vehicle = order.vehicle;
  return (
    <div className="card-stack">
      <div className="card-header">
        <div>
          <h2 className="card-title">Rechnung zu Auftrag {order.order_number}</h2>
          <p className="card-sub">
            {vehicle.license_plate} · {vehicle.brand} {vehicle.model}
          </p>
        </div>
      </div>

      <div className="table-scroll">
        <table className="data-table">
          <thead>
            <tr>
              <th>Beschreibung</th>
              <th className="num">Menge</th>
              <th className="num">Einzelpreis (netto)</th>
              <th className="num">Summe</th>
            </tr>
          </thead>
          <tbody>
            {invoice.lines.map((line, index) => (
              <tr key={`${line.description}-${index}`}>
                <td>{line.description}</td>
                <td className="num">{formatLineQuantity(line.quantity)}</td>
                <td className="num">{formatCents(line.unit_price_cents)}</td>
                <td className="num">{formatCents(line.total_cents)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="invoice-summary">
        <div className="invoice-row">
          <span>Netto</span>
          <span className="is-num">{formatCents(invoice.net_cents)}</span>
        </div>
        <div className="invoice-row">
          <span>{"Mehrwertsteuer (19\u00A0%)"}</span>
          <span className="is-num">{formatCents(invoice.vat_cents)}</span>
        </div>
        <div className="invoice-total">
          <span>Brutto</span>
          <span className="is-num">{formatCents(invoice.gross_cents)}</span>
        </div>
      </div>
    </div>
  );
}

export default function OrderStatus() {
  const [orderNumber, setOrderNumber] = useState("");
  const [licensePlate, setLicensePlate] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [alert, setAlert] = useState<string | null>(null);
  const [orderNumberError, setOrderNumberError] = useState<string | null>(null);
  const [licensePlateError, setLicensePlateError] = useState<string | null>(null);
  const [order, setOrder] = useState<OrderDetail | null>(null);
  const [invoice, setInvoice] = useState<Invoice | null>(null);
  const [invoiceState, setInvoiceState] = useState<
    "idle" | "loading" | "ready" | "empty" | "error"
  >("idle");
  const [invoiceError, setInvoiceError] = useState<string | null>(null);

  function resetResult() {
    setOrder(null);
    setInvoice(null);
    setInvoiceError(null);
    setInvoiceState("idle");
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const num = orderNumber.trim();
    const plate = licensePlate.trim();
    const numError = num ? null : "Bitte die Auftragsnummer angeben.";
    const plateError = plate ? null : "Bitte das Kennzeichen angeben.";
    setOrderNumberError(numError);
    setLicensePlateError(plateError);
    setAlert(null);

    if (numError || plateError) {
      resetResult();
      return;
    }

    setSubmitting(true);
    resetResult();

    try {
      const detail = await apiFetch<OrderDetail>(orderPath(num, plate));
      setOrder(detail);

      if (isCompleted(detail.status)) {
        setInvoiceState("loading");
        try {
          const loaded = await apiFetch<Invoice>(
            invoicePath(detail.order_number, detail.vehicle.license_plate),
          );
          setInvoice(loaded);
          setInvoiceState("ready");
        } catch (error) {
          setInvoice(null);
          if (error instanceof ApiError && error.status === 404) {
            setInvoiceState("empty");
          } else {
            setInvoiceError(
              error instanceof ApiError
                ? error.message
                : "Die Rechnung konnte nicht geladen werden.",
            );
            setInvoiceState("error");
          }
        }
      }
    } catch (error) {
      resetResult();
      if (error instanceof ApiError && error.status === 404) {
        setAlert(NOT_FOUND_MESSAGE);
      } else if (error instanceof ApiError) {
        setAlert(error.message);
      } else {
        setAlert("Unerwarteter Fehler der Werkstatt-API.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  const timeline = order ? buildTimeline(order) : [];

  return (
    <div className="container order-status">
      <h1 className="page-title">Status abrufen</h1>

      <section className="card form-card">
        <form onSubmit={handleSubmit} noValidate>
          {alert && (
            <div className="alert alert-danger" role="alert">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                width="16"
                height="16"
                viewBox="0 0 16 16"
                fill="none"
                aria-hidden="true"
              >
                <circle cx="8" cy="8" r="7" stroke="currentColor" strokeWidth="1.6" />
                <path
                  d="M8 5v3.5M8 11h.01"
                  stroke="currentColor"
                  strokeWidth="1.6"
                  strokeLinecap="round"
                />
              </svg>
              <div>{alert}</div>
            </div>
          )}

          <div className="field-grid">
            <div className="field">
              <label htmlFor="lookup-number">
                Auftragsnummer<span className="required-marker">erforderlich</span>
              </label>
              <input
                id="lookup-number"
                name="order"
                type="text"
                autoComplete="off"
                placeholder="AUF-2026-000000"
                value={orderNumber}
                className={orderNumberError ? "is-invalid" : undefined}
                aria-invalid={orderNumberError ? true : undefined}
                aria-describedby={orderNumberError ? "lookup-number-error" : undefined}
                onChange={(event) => {
                  setOrderNumber(event.target.value);
                  if (orderNumberError) {
                    setOrderNumberError(null);
                  }
                }}
              />
              {orderNumberError && (
                <p className="field-error" id="lookup-number-error">
                  {orderNumberError}
                </p>
              )}
            </div>

            <div className="field">
              <label htmlFor="lookup-plate">
                Kennzeichen<span className="required-marker">erforderlich</span>
              </label>
              <input
                id="lookup-plate"
                name="plate"
                type="text"
                autoComplete="off"
                placeholder="z. B. B-AB 1234"
                value={licensePlate}
                className={licensePlateError ? "is-invalid" : undefined}
                aria-invalid={licensePlateError ? true : undefined}
                aria-describedby={licensePlateError ? "lookup-plate-error" : undefined}
                onChange={(event) => {
                  setLicensePlate(event.target.value);
                  if (licensePlateError) {
                    setLicensePlateError(null);
                  }
                }}
              />
              {licensePlateError && (
                <p className="field-error" id="lookup-plate-error">
                  {licensePlateError}
                </p>
              )}
            </div>
          </div>

          <p className="field-help">Beispiel: AUF-2026-000125 · M-EF 9012</p>

          <div className="field">
            <button
              type="submit"
              className={`btn btn-primary${submitting ? " is-loading" : ""}`}
              disabled={submitting}
            >
              <span className="btn-label">Status abrufen</span>
              <span className="dots" aria-hidden="true">
                <i />
                <i />
                <i />
              </span>
            </button>
          </div>
        </form>
      </section>

      {order && (
        <section className="card card-stack" aria-label="Auftragsstatus">
          <div className="card-header">
            <div>
              <h2 className="card-title">Auftrag {order.order_number}</h2>
              <p className="card-sub">
                {order.vehicle.license_plate} · {order.vehicle.brand}{" "}
                {order.vehicle.model} · {formatMileage(order.vehicle.mileage)}
              </p>
            </div>
            <StatusBadge status={order.status} />
          </div>

          <Timeline entries={timeline} />

          {isCompleted(order.status) && (
            <>
              {invoiceState === "ready" && invoice && (
                <InvoiceSection order={order} invoice={invoice} />
              )}
              {invoiceState === "empty" && (
                <div className="empty-state">
                  <p className="empty-desc">{NO_INVOICE_MESSAGE}</p>
                </div>
              )}
              {invoiceState === "error" && (
                <div className="alert alert-danger" role="alert">
                  <div>{invoiceError}</div>
                </div>
              )}
            </>
          )}
        </section>
      )}
    </div>
  );
}
