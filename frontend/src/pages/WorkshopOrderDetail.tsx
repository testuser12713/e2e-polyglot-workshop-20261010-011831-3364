import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type FormEvent,
} from "react";
import { Link, useParams } from "react-router-dom";
import { useSession } from "../auth/session";
import { ApiError, apiFetch } from "../lib/api";
import "./WorkshopOrderDetail.css";

type OrderStatus =
  | "angefragt"
  | "bestätigt"
  | "in Arbeit"
  | "fertig"
  | "abgeholt";

const STATUS_ORDER: OrderStatus[] = [
  "angefragt",
  "bestätigt",
  "in Arbeit",
  "fertig",
  "abgeholt",
];

const STATUS_CLASS: Record<OrderStatus, string> = {
  angefragt: "angefragt",
  bestätigt: "bestaetigt",
  "in Arbeit": "inArbeit",
  fertig: "fertig",
  abgeholt: "abgeholt",
};

const EDITABLE_STATUSES: OrderStatus[] = ["angefragt", "bestätigt", "in Arbeit"];

const OPEN_ITEMS_FROM_INDEX = STATUS_ORDER.indexOf("fertig");

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

interface OrderItem {
  id: number;
  kind: string;
  description: string;
  hours?: number | null;
  quantity?: number | null;
  unit_price_cents?: number | null;
  total_cents: number;
}

interface StatusEvent {
  from_status: OrderStatus | null;
  to_status: OrderStatus;
  changed_by: string;
  changed_at: string;
}

interface OrderDetail {
  order_number: string;
  status: OrderStatus;
  requested_date: string;
  problem_description: string;
  created_at: string;
  customer: Customer;
  vehicle: Vehicle;
  items: OrderItem[];
  history: StatusEvent[];
}

type ItemKind = "labor" | "part";

interface ItemBody {
  kind: ItemKind;
  description: string;
  hours?: number;
  quantity?: number;
  unit_price_cents: number;
}

type AlertTone = "success" | "danger" | "warning" | "info";

interface AlertMessage {
  tone: AlertTone;
  message: string;
}

interface FormErrors {
  description?: string;
  qty?: string;
  price?: string;
}

const currencyFmt = new Intl.NumberFormat("de-DE", {
  style: "currency",
  currency: "EUR",
});
const hoursFmt = new Intl.NumberFormat("de-DE", {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});
const intFmt = new Intl.NumberFormat("de-DE", { maximumFractionDigits: 0 });
const timestampFmt = new Intl.DateTimeFormat("de-DE", {
  timeZone: "Europe/Berlin",
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
});

function formatCents(cents: number | null | undefined): string {
  return currencyFmt.format((cents ?? 0) / 100);
}

function formatDate(value: string | null | undefined): string {
  if (!value) {
    return "—";
  }
  const match = /^(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2}))?/.exec(value);
  if (!match) {
    return value;
  }
  const [, year, month, day, hour, minute] = match;
  const date = `${day}.${month}.${year}`;
  return hour ? `${date}, ${hour}:${minute} Uhr` : date;
}

function formatTimestamp(value: string | null | undefined): string {
  if (!value) {
    return "—";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return `${timestampFmt.format(parsed)} Uhr`;
}

function kindLabel(kind: string): string {
  return kind === "part" ? "Teile" : "Arbeitszeit";
}

function quantityLabel(item: OrderItem): string {
  if (item.kind === "part") {
    return `${intFmt.format(item.quantity ?? 0)} Stück`;
  }
  return `${hoursFmt.format(item.hours ?? 0)} h`;
}

function parseNumber(value: string): number {
  return Number.parseFloat(value.trim().replace(/\./g, "").replace(",", "."));
}

function toInputPrice(cents: number | null | undefined): string {
  if (cents == null) {
    return "";
  }
  return (cents / 100).toFixed(2).replace(".", ",");
}

function StatusBadge({ status }: { status: OrderStatus }) {
  return (
    <span className={`od-badge od-badge--${STATUS_CLASS[status]}`}>
      <span className="od-badge__dot" aria-hidden="true" />
      {status}
    </span>
  );
}

function AlertIcon({ tone }: { tone: AlertTone }) {
  if (tone === "success") {
    return (
      <svg
        className="od-alert__icon"
        xmlns="http://www.w3.org/2000/svg"
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        aria-hidden="true"
      >
        <circle cx="8" cy="8" r="7" stroke="currentColor" strokeWidth="1.6" />
        <path
          d="M5 8.2l2.1 2.1L11 6"
          stroke="currentColor"
          strokeWidth="1.6"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    );
  }
  if (tone === "warning") {
    return (
      <svg
        className="od-alert__icon"
        xmlns="http://www.w3.org/2000/svg"
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        aria-hidden="true"
      >
        <path
          d="M8 2l6 11H2L8 2z"
          stroke="currentColor"
          strokeWidth="1.6"
          strokeLinejoin="round"
        />
        <path
          d="M8 7v3M8 12h.01"
          stroke="currentColor"
          strokeWidth="1.6"
          strokeLinecap="round"
        />
      </svg>
    );
  }
  if (tone === "info") {
    return (
      <svg
        className="od-alert__icon"
        xmlns="http://www.w3.org/2000/svg"
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        aria-hidden="true"
      >
        <circle cx="8" cy="8" r="7" stroke="currentColor" strokeWidth="1.6" />
        <path
          d="M8 7.5V11M8 5h.01"
          stroke="currentColor"
          strokeWidth="1.6"
          strokeLinecap="round"
        />
      </svg>
    );
  }
  return (
    <svg
      className="od-alert__icon"
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
  );
}

export default function WorkshopOrderDetail() {
  const { orderNumber = "" } = useParams<{ orderNumber: string }>();
  const { token } = useSession();

  const [order, setOrder] = useState<OrderDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [alerts, setAlerts] = useState<AlertMessage[]>([]);

  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [formKind, setFormKind] = useState<ItemKind>("labor");
  const [formDescription, setFormDescription] = useState("");
  const [formQty, setFormQty] = useState("");
  const [formPrice, setFormPrice] = useState("");
  const [formErrors, setFormErrors] = useState<FormErrors>({});
  const [savingItem, setSavingItem] = useState(false);

  const [nextStatus, setNextStatus] = useState<OrderStatus | "">("");
  const [savingStatus, setSavingStatus] = useState(false);

  const dialogRef = useRef<HTMLDivElement | null>(null);
  const firstFieldRef = useRef<HTMLSelectElement | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const data = await apiFetch<OrderDetail>(
        `/api/workshop/orders/${encodeURIComponent(orderNumber)}`,
        { token },
      );
      setOrder(data);
    } catch (error) {
      setLoadError(
        error instanceof ApiError
          ? error.message
          : "Der Auftrag konnte nicht geladen werden.",
      );
    } finally {
      setLoading(false);
    }
  }, [orderNumber, token]);

  useEffect(() => {
    void load();
  }, [load]);

  const statusIndex = order ? STATUS_ORDER.indexOf(order.status) : -1;
  const allowedNext: OrderStatus | null =
    statusIndex >= 0 && statusIndex < STATUS_ORDER.length - 1
      ? STATUS_ORDER[statusIndex + 1]
      : null;
  const editable = order != null && EDITABLE_STATUSES.includes(order.status);

  useEffect(() => {
    setNextStatus(allowedNext ?? "");
  }, [allowedNext]);

  const itemsTotal = useMemo(
    () => (order ? order.items.reduce((sum, item) => sum + (item.total_cents ?? 0), 0) : 0),
    [order],
  );

  const invoice = useMemo(() => {
    if (!order) {
      return null;
    }
    const net = itemsTotal;
    const vat = Math.round(net * 0.19);
    return { net, vat, gross: net + vat };
  }, [order, itemsTotal]);

  const showInvoice = statusIndex >= OPEN_ITEMS_FROM_INDEX && invoice != null;

  useEffect(() => {
    if (!modalOpen) {
      return;
    }
    firstFieldRef.current?.focus();
    const node = dialogRef.current;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setModalOpen(false);
        return;
      }
      if (event.key !== "Tab" || !node) {
        return;
      }
      const focusable = node.querySelectorAll<HTMLElement>(
        'button, input, select, textarea, a[href], [tabindex]:not([tabindex="-1"])',
      );
      if (focusable.length === 0) {
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", onKeyDown, true);
    return () => {
      document.removeEventListener("keydown", onKeyDown, true);
    };
  }, [modalOpen]);

  const openCreate = useCallback(() => {
    setEditingId(null);
    setFormKind("labor");
    setFormDescription("");
    setFormQty("");
    setFormPrice("");
    setFormErrors({});
    setModalOpen(true);
  }, []);

  const openEdit = useCallback((item: OrderItem) => {
    setEditingId(item.id);
    setFormKind(item.kind === "part" ? "part" : "labor");
    setFormDescription(item.description);
    setFormQty(
      item.kind === "part"
        ? String(item.quantity ?? "")
        : String(item.hours ?? ""),
    );
    setFormPrice(toInputPrice(item.unit_price_cents));
    setFormErrors({});
    setModalOpen(true);
  }, []);

  const validateForm = useCallback((): boolean => {
    const errors: FormErrors = {};
    if (!formDescription.trim()) {
      errors.description = "Bitte eine Beschreibung angeben.";
    }
    const qty = parseNumber(formQty);
    if (Number.isNaN(qty) || qty <= 0) {
      errors.qty =
        formKind === "labor"
          ? "Bitte eine Arbeitszeit größer 0 angeben."
          : "Bitte eine Menge größer 0 angeben.";
    } else if (formKind === "part" && !Number.isInteger(qty)) {
      errors.qty = "Teile werden in ganzen Stück erfasst.";
    }
    const price = parseNumber(formPrice);
    if (Number.isNaN(price) || price < 0) {
      errors.price = "Bitte einen gültigen Einzelpreis angeben.";
    }
    setFormErrors(errors);
    return Object.keys(errors).length === 0;
  }, [formDescription, formKind, formPrice, formQty]);

  const submitItem = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (!validateForm()) {
        return;
      }
      const qty = parseNumber(formQty);
      const price = parseNumber(formPrice);
      const body: ItemBody = {
        kind: formKind,
        description: formDescription.trim(),
        unit_price_cents: Math.round(price * 100),
      };
      if (formKind === "labor") {
        body.hours = qty;
      } else {
        body.quantity = Math.round(qty);
      }

      const isEdit = editingId != null;
      const path = isEdit
        ? `/api/workshop/orders/${encodeURIComponent(orderNumber)}/items/${editingId}`
        : `/api/workshop/orders/${encodeURIComponent(orderNumber)}/items`;

      setSavingItem(true);
      setAlerts([]);
      try {
        const updated = await apiFetch<OrderDetail>(path, {
          method: isEdit ? "PUT" : "POST",
          body,
          token,
        });
        setOrder(updated);
        setModalOpen(false);
        setAlerts([
          {
            tone: "success",
            message: isEdit ? "Position aktualisiert." : "Position erfasst.",
          },
        ]);
      } catch (error) {
        setAlerts([
          {
            tone: error instanceof ApiError && error.status === 409 ? "warning" : "danger",
            message:
              error instanceof ApiError
                ? error.message
                : "Die Position konnte nicht gespeichert werden.",
          },
        ]);
      } finally {
        setSavingItem(false);
      }
    },
    [
      editingId,
      formDescription,
      formKind,
      formPrice,
      formQty,
      orderNumber,
      token,
      validateForm,
    ],
  );

  const submitStatus = useCallback(async () => {
    if (!allowedNext) {
      return;
    }
    setSavingStatus(true);
    setAlerts([]);
    try {
      const updated = await apiFetch<OrderDetail>(
        `/api/workshop/orders/${encodeURIComponent(orderNumber)}/status`,
        { method: "POST", body: { status: allowedNext }, token },
      );
      setOrder(updated);
      const nextAlerts: AlertMessage[] = [
        { tone: "success", message: `Status gesetzt: ${allowedNext}.` },
      ];
      if (allowedNext === "fertig") {
        nextAlerts.push({
          tone: "info",
          message:
            "Die Rechnung wird im Hintergrund erstellt und erscheint in Kürze.",
        });
      }
      setAlerts(nextAlerts);
    } catch (error) {
      setAlerts([
        {
          tone: error instanceof ApiError && error.status === 409 ? "warning" : "danger",
          message:
            error instanceof ApiError
              ? error.message
              : "Der Status konnte nicht gesetzt werden.",
        },
      ]);
    } finally {
      setSavingStatus(false);
    }
  }, [allowedNext, orderNumber, token]);

  const onKindChange = useCallback((event: ChangeEvent<HTMLSelectElement>) => {
    setFormKind(event.target.value === "part" ? "part" : "labor");
  }, []);

  const alertsRegion = (
    <div className="od-alerts" aria-live="polite">
      {alerts.map((alert, index) => (
        <div
          key={`${alert.tone}-${index}`}
          className={`od-alert od-alert--${alert.tone}`}
          role={alert.tone === "danger" ? "alert" : "status"}
        >
          <AlertIcon tone={alert.tone} />
          <div>{alert.message}</div>
        </div>
      ))}
    </div>
  );

  if (loading) {
    return (
      <div className="container order-detail">
        <Link className="od-back" to="/werkstatt/auftraege">
          ← Zur Auftragsliste
        </Link>
        <p className="card-text">Auftrag wird geladen…</p>
      </div>
    );
  }

  if (!order || loadError) {
    return (
      <div className="container order-detail">
        <Link className="od-back" to="/werkstatt/auftraege">
          ← Zur Auftragsliste
        </Link>
        <div className="od-alerts" aria-live="polite">
          <div className="od-alert od-alert--danger" role="alert">
            <AlertIcon tone="danger" />
            <div>{loadError ?? "Der Auftrag konnte nicht geladen werden."}</div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="container order-detail">
      <Link className="od-back" to="/werkstatt/auftraege">
        ← Zur Auftragsliste
      </Link>

      <div className="order-detail__head">
        <h1 className="page-title">Auftrag {order.order_number}</h1>
        <StatusBadge status={order.status} />
      </div>

      {alertsRegion}

      <div className="order-detail__grid">
        <div className="order-detail__col">
          <section className="card">
            <div className="od-card__head">
              <h2 className="card-title" style={{ margin: 0 }}>
                Fahrzeug &amp; Kunde
              </h2>
            </div>
            <dl className="od-kv">
              <div className="od-kv__row">
                <dt>Kennzeichen</dt>
                <dd>{order.vehicle.license_plate}</dd>
              </div>
              <div className="od-kv__row">
                <dt>Fahrzeug</dt>
                <dd>
                  {order.vehicle.brand} {order.vehicle.model}
                </dd>
              </div>
              <div className="od-kv__row">
                <dt>Kilometerstand</dt>
                <dd>{intFmt.format(order.vehicle.mileage)} km</dd>
              </div>
              <div className="od-kv__row">
                <dt>Wunschtermin</dt>
                <dd>{formatDate(order.requested_date)}</dd>
              </div>
              <div className="od-kv__row">
                <dt>Kunde</dt>
                <dd>{order.customer.name}</dd>
              </div>
              <div className="od-kv__row">
                <dt>E-Mail</dt>
                <dd>{order.customer.email}</dd>
              </div>
              <div className="od-kv__row">
                <dt>Telefon</dt>
                <dd>{order.customer.phone}</dd>
              </div>
            </dl>
            <div className="od-kv__block">
              <div className="od-kv__dt">Problembeschreibung</div>
              <p className="od-kv__dd">{order.problem_description}</p>
            </div>
          </section>

          <section className="card">
            <div className="od-card__head">
              <h2 className="card-title" style={{ margin: 0 }}>
                Positionen
              </h2>
              <button
                type="button"
                className="btn btn-primary"
                onClick={openCreate}
                disabled={!editable}
                title={
                  editable
                    ? undefined
                    : "Positionen können nach dem Fertigstellen nicht mehr geändert werden."
                }
              >
                Position erfassen
              </button>
            </div>
            <div className="od-table-wrap">
              <table className="od-table">
                <thead>
                  <tr>
                    <th>Position</th>
                    <th>Beschreibung</th>
                    <th className="od-num">Menge</th>
                    <th className="od-num">Einzelpreis (netto)</th>
                    <th className="od-num">Summe</th>
                    <th>
                      <span className="od-sr-only">Aktionen</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {order.items.length === 0 && (
                    <tr>
                      <td className="od-table__empty" colSpan={6}>
                        Noch keine Positionen erfasst.
                      </td>
                    </tr>
                  )}
                  {order.items.map((item) => (
                    <tr key={item.id}>
                      <td>{kindLabel(item.kind)}</td>
                      <td>{item.description}</td>
                      <td className="od-num">{quantityLabel(item)}</td>
                      <td className="od-num">
                        {formatCents(item.unit_price_cents)}
                      </td>
                      <td className="od-num">{formatCents(item.total_cents)}</td>
                      <td>
                        {editable ? (
                          <button
                            type="button"
                            className="btn btn-secondary od-btn-sm"
                            onClick={() => openEdit(item)}
                          >
                            Bearbeiten
                          </button>
                        ) : (
                          <span className="od-muted">—</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="od-summary">
              <div className="od-summary__row">
                <span>Zwischensumme (netto)</span>
                <span className="od-num">{formatCents(itemsTotal)}</span>
              </div>
            </div>
          </section>

          {showInvoice && invoice && (
            <section className="card">
              <div className="od-card__head">
                <div>
                  <h2 className="card-title" style={{ margin: 0 }}>
                    Rechnung zu Auftrag {order.order_number}
                  </h2>
                  <p className="od-card__sub">
                    {order.vehicle.license_plate} · {order.vehicle.brand}{" "}
                    {order.vehicle.model}
                  </p>
                </div>
              </div>
              <div className="od-table-wrap">
                <table className="od-table">
                  <thead>
                    <tr>
                      <th>Position</th>
                      <th>Beschreibung</th>
                      <th className="od-num">Menge</th>
                      <th className="od-num">Einzelpreis (netto)</th>
                      <th className="od-num">Summe</th>
                    </tr>
                  </thead>
                  <tbody>
                    {order.items.map((item) => (
                      <tr key={item.id}>
                        <td>{kindLabel(item.kind)}</td>
                        <td>{item.description}</td>
                        <td className="od-num">{quantityLabel(item)}</td>
                        <td className="od-num">
                          {formatCents(item.unit_price_cents)}
                        </td>
                        <td className="od-num">
                          {formatCents(item.total_cents)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <div className="od-summary">
                <div className="od-summary__row">
                  <span>Netto</span>
                  <span className="od-num">{formatCents(invoice.net)}</span>
                </div>
                <div className="od-summary__row">
                  <span>Mehrwertsteuer (19 %)</span>
                  <span className="od-num">{formatCents(invoice.vat)}</span>
                </div>
                <div className="od-summary__total">
                  <span>Brutto</span> &nbsp;&nbsp;
                  <span className="od-num">{formatCents(invoice.gross)}</span>
                </div>
              </div>
            </section>
          )}
        </div>

        <section className="card">
          <div className="od-card__head">
            <h2 className="card-title" style={{ margin: 0 }}>
              Status
            </h2>
          </div>

          <ol className="od-timeline">
            {STATUS_ORDER.map((status, index) => {
              const reached = index <= statusIndex;
              const event = order.history.find(
                (entry) => entry.to_status === status,
              );
              return (
                <li
                  key={status}
                  className={`od-timeline__item od-timeline__item--${STATUS_CLASS[status]}${
                    reached ? " od-timeline__item--reached" : ""
                  }`}
                >
                  <div className="od-timeline__row">
                    <StatusBadge status={status} />
                    {reached && event && (
                      <span className="od-timeline__time">
                        {formatTimestamp(event.changed_at)}
                      </span>
                    )}
                    {reached && event && (
                      <span className="od-timeline__actor">
                        von {event.changed_by}
                      </span>
                    )}
                  </div>
                </li>
              );
            })}
          </ol>

          <div className="od-field" style={{ marginTop: 24, marginBottom: 0 }}>
            <label className="od-field__label" htmlFor="od-next-status">
              Statuswechsel
            </label>
            <div className="od-select-wrap">
              <select
                id="od-next-status"
                className="od-select"
                value={nextStatus}
                onChange={(event) =>
                  setNextStatus(event.target.value as OrderStatus)
                }
                disabled={!allowedNext}
              >
                {STATUS_ORDER.map((status) => (
                  <option
                    key={status}
                    value={status}
                    disabled={status !== allowedNext}
                  >
                    {status}
                  </option>
                ))}
              </select>
            </div>
            <p className="od-help">
              {allowedNext
                ? `Nur „${allowedNext}“ ist als nächster Status wählbar.`
                : "Der Auftrag ist abgeschlossen; kein weiterer Statuswechsel möglich."}
            </p>
          </div>
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => void submitStatus()}
            disabled={!allowedNext || savingStatus}
            style={{ marginTop: 16 }}
          >
            Status setzen
          </button>
        </section>
      </div>

      {modalOpen && (
        <div
          className="od-modal-backdrop"
          onClick={(event) => {
            if (event.target === event.currentTarget) {
              setModalOpen(false);
            }
          }}
        >
          <div
            className="od-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="od-position-modal-title"
            ref={dialogRef}
          >
            <h2 className="od-modal__header" id="od-position-modal-title">
              {editingId != null ? "Position bearbeiten" : "Position erfassen"}
            </h2>
            <form onSubmit={submitItem} noValidate>
              <div className="od-field">
                <label className="od-field__label" htmlFor="od-item-kind">
                  Art der Position
                  <span className="od-required">erforderlich</span>
                </label>
                <div className="od-select-wrap">
                  <select
                    id="od-item-kind"
                    className="od-select"
                    value={formKind}
                    onChange={onKindChange}
                    ref={firstFieldRef}
                  >
                    <option value="labor">Arbeitszeit</option>
                    <option value="part">Teile</option>
                  </select>
                </div>
              </div>

              <div className="od-field">
                <label className="od-field__label" htmlFor="od-item-description">
                  Beschreibung
                  <span className="od-required">erforderlich</span>
                </label>
                <input
                  id="od-item-description"
                  className={`od-input${
                    formErrors.description ? " od-input--invalid" : ""
                  }`}
                  type="text"
                  value={formDescription}
                  onChange={(event) => setFormDescription(event.target.value)}
                />
                {formErrors.description && (
                  <p className="od-error">{formErrors.description}</p>
                )}
              </div>

              <div className="od-field">
                <label className="od-field__label" htmlFor="od-item-qty">
                  {formKind === "labor" ? "Arbeitszeit (Stunden)" : "Menge"}
                  <span className="od-required">erforderlich</span>
                </label>
                <input
                  id="od-item-qty"
                  className={`od-input od-input--num${
                    formErrors.qty ? " od-input--invalid" : ""
                  }`}
                  type="text"
                  inputMode="decimal"
                  value={formQty}
                  onChange={(event) => setFormQty(event.target.value)}
                />
                {formErrors.qty && <p className="od-error">{formErrors.qty}</p>}
              </div>

              <div className="od-field" style={{ marginBottom: 0 }}>
                <label className="od-field__label" htmlFor="od-item-price">
                  Einzelpreis (netto)
                  <span className="od-required">erforderlich</span>
                </label>
                <input
                  id="od-item-price"
                  className={`od-input od-input--num${
                    formErrors.price ? " od-input--invalid" : ""
                  }`}
                  type="text"
                  inputMode="decimal"
                  value={formPrice}
                  onChange={(event) => setFormPrice(event.target.value)}
                />
                {formErrors.price && (
                  <p className="od-error">{formErrors.price}</p>
                )}
              </div>

              <div className="od-modal__footer">
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => setModalOpen(false)}
                >
                  Abbrechen
                </button>
                <button
                  type="submit"
                  className="btn btn-primary"
                  disabled={savingItem}
                >
                  Speichern
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
