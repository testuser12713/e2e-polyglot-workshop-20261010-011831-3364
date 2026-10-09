import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useSession } from "../auth/session";
import { ApiError, apiFetch } from "../lib/api";
import "./WorkshopOrders.css";

type OrderStatus =
  | "angefragt"
  | "bestätigt"
  | "in Arbeit"
  | "fertig"
  | "abgeholt";

const STATUS_OPTIONS: OrderStatus[] = [
  "angefragt",
  "bestätigt",
  "in Arbeit",
  "fertig",
  "abgeholt",
];

const STATUS_SLUG: Record<string, string> = {
  angefragt: "angefragt",
  "bestätigt": "bestaetigt",
  "in Arbeit": "inarbeit",
  fertig: "fertig",
  abgeholt: "abgeholt",
};

interface Vehicle {
  license_plate: string;
  brand: string;
  model: string;
  mileage: number;
}

interface Customer {
  name: string;
  email: string;
  phone: string;
}

interface OrderItem {
  id: number;
  kind: string;
  description: string;
  hours?: number;
  quantity?: number;
  unit_price_cents?: number;
  total_cents: number;
}

interface StatusEvent {
  from_status: string | null;
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

interface OrderListResponse {
  orders: OrderDetail[];
}

const DEBOUNCE_MS = 250;

/**
 * VAT rate used to derive the gross amount of an order from its positions.
 *
 * The shared contract's OrderDetail carries no total, only the net position
 * totals; 19 % is the product-wide VAT rate (SPEC AC-06). Orders without any
 * position show no amount.
 */
const VAT_RATE = 0.19;

function formatDate(value: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value);
  if (!match) {
    return value;
  }
  return `${match[3]}.${match[2]}.${match[1]}`;
}

function formatCents(cents: number): string {
  return new Intl.NumberFormat("de-DE", {
    style: "currency",
    currency: "EUR",
  }).format(cents / 100);
}

function orderGrossCents(order: OrderDetail): number | null {
  if (!order.items || order.items.length === 0) {
    return null;
  }
  const net = order.items.reduce((sum, item) => sum + (item.total_cents ?? 0), 0);
  return net + Math.round(net * VAT_RATE);
}

function orderAmount(order: OrderDetail): string {
  const gross = orderGrossCents(order);
  return gross === null ? "—" : formatCents(gross);
}

function StatusBadge({ status }: { status: string }) {
  const slug = STATUS_SLUG[status] ?? "angefragt";
  return (
    <span className={`wo-badge wo-badge--${slug}`}>
      <span className="wo-badge__dot" aria-hidden="true" />
      {status}
    </span>
  );
}

function EmptyState({
  filtered,
  onReset,
}: {
  filtered: boolean;
  onReset: () => void;
}) {
  return (
    <div className="wo-empty" role="status">
      <svg
        className="wo-empty__icon"
        xmlns="http://www.w3.org/2000/svg"
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill="none"
        aria-hidden="true"
      >
        <path
          d="M4 13l1.5-5A2 2 0 0 1 7.4 6.5h9.2a2 2 0 0 1 1.9 1.5L20 13v4a1 1 0 0 1-1 1h-1v-1.5H6V18H5a1 1 0 0 1-1-1v-4Z"
          stroke="currentColor"
          strokeWidth="1.6"
          strokeLinejoin="round"
        />
        <path d="M4 13h4l1 2h6l1-2h4" stroke="currentColor" strokeWidth="1.6" />
      </svg>
      <div className="wo-empty__title">
        {filtered ? "Keine Aufträge für diesen Filter" : "Noch keine Aufträge"}
      </div>
      <div className="wo-empty__desc">
        {filtered
          ? "Kein Auftrag passt zu den gewählten Filtern."
          : "Sobald ein Auftrag eingeht, erscheint er hier."}
      </div>
      {filtered && (
        <button type="button" className="btn btn-secondary" onClick={onReset}>
          Filter zurücksetzen
        </button>
      )}
    </div>
  );
}

export default function WorkshopOrders() {
  const { token, logout } = useSession();
  const navigate = useNavigate();

  const [plateInput, setPlateInput] = useState("");
  const [plateQuery, setPlateQuery] = useState("");
  const [status, setStatus] = useState("");
  const [orders, setOrders] = useState<OrderDetail[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const timer = setTimeout(() => {
      setPlateQuery(plateInput.trim().toUpperCase());
    }, DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [plateInput]);

  useEffect(() => {
    if (!token) {
      setLoading(false);
      return;
    }

    const controller = new AbortController();
    const params = new URLSearchParams();
    if (status) {
      params.set("status", status);
    }
    if (plateQuery) {
      params.set("license_plate", plateQuery);
    }
    const query = params.toString();
    const path = `/api/workshop/orders${query ? `?${query}` : ""}`;

    setLoading(true);
    setError(null);

    apiFetch<OrderListResponse>(path, { token, signal: controller.signal })
      .then((data) => {
        setOrders(data?.orders ?? []);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) {
          return;
        }
        if (err instanceof ApiError && err.status === 401) {
          logout();
          navigate("/werkstatt/anmeldung", { replace: true });
          return;
        }
        setError(
          err instanceof ApiError
            ? err.message
            : "Die Aufträge konnten nicht geladen werden.",
        );
        setLoading(false);
      });

    return () => controller.abort();
  }, [status, plateQuery, token, logout, navigate]);

  const hasFilter = Boolean(status || plateQuery);

  function resetFilters() {
    setPlateInput("");
    setPlateQuery("");
    setStatus("");
  }

  function clearPlate() {
    setPlateInput("");
    setPlateQuery("");
  }

  function renderTable() {
    return (
      <div className="wo-table-wrap">
        <table className="wo-table">
          <thead>
            <tr>
              <th>Auftragsnummer</th>
              <th>Kennzeichen</th>
              <th className="wo-md-hide">Fahrzeug</th>
              <th>Status</th>
              <th className="wo-md-hide">Wunschtermin</th>
              <th className="wo-num">Betrag brutto</th>
              <th>Aktion</th>
            </tr>
          </thead>
          <tbody>
            {orders.map((order) => (
              <tr key={order.order_number}>
                <td className="wo-num">
                  <Link to={`/werkstatt/auftraege/${order.order_number}`}>
                    {order.order_number}
                  </Link>
                </td>
                <td>{order.vehicle.license_plate}</td>
                <td className="wo-md-hide">
                  {order.vehicle.brand} {order.vehicle.model}
                </td>
                <td>
                  <StatusBadge status={order.status} />
                </td>
                <td className="wo-md-hide">{formatDate(order.requested_date)}</td>
                <td className="wo-num">{orderAmount(order)}</td>
                <td>
                  <Link
                    className="wo-action"
                    to={`/werkstatt/auftraege/${order.order_number}`}
                  >
                    Öffnen
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  }

  function renderCards() {
    return (
      <div className="wo-cards">
        {orders.map((order) => (
          <div className="card wo-order-card" key={order.order_number}>
            <div className="wo-order-card__line1">
              <Link
                className="wo-order-card__num"
                to={`/werkstatt/auftraege/${order.order_number}`}
              >
                {order.order_number}
              </Link>
              <StatusBadge status={order.status} />
            </div>
            <div className="wo-order-card__line2">
              {order.vehicle.license_plate} · {order.vehicle.brand}{" "}
              {order.vehicle.model}
            </div>
            <div className="wo-order-card__line3">{orderAmount(order)}</div>
            <div className="wo-order-card__actions">
              <Link
                className="btn btn-secondary btn-block"
                to={`/werkstatt/auftraege/${order.order_number}`}
              >
                Auftrag öffnen
              </Link>
            </div>
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="container">
      <section className="page-header">
        <h1 className="page-title">Auftragsliste</h1>
      </section>

      <section className="card wo-orders-card">
        <form
          className="wo-filter-bar"
          onSubmit={(event) => event.preventDefault()}
        >
          <div className="wo-field wo-field--search">
            <label className="wo-visually-hidden" htmlFor="wo-search">
              Nach Kennzeichen suchen
            </label>
            <input
              id="wo-search"
              className="wo-input"
              type="search"
              placeholder="Nach Kennzeichen suchen"
              autoComplete="off"
              value={plateInput}
              onChange={(event) => setPlateInput(event.target.value)}
            />
          </div>
          <div className="wo-field wo-field--status">
            <label className="wo-visually-hidden" htmlFor="wo-status">
              Nach Status filtern
            </label>
            <div className="wo-select-wrap">
              <select
                id="wo-status"
                className="wo-select"
                value={status}
                onChange={(event) => setStatus(event.target.value)}
              >
                <option value="">Alle Status</option>
                {STATUS_OPTIONS.map((option) => (
                  <option key={option} value={option}>
                    {option}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </form>

        {hasFilter && (
          <div className="wo-chip-row">
            {status && (
              <span className="wo-chip">
                Status: {status}
                <button
                  type="button"
                  className="wo-chip__remove"
                  aria-label="Statusfilter entfernen"
                  onClick={() => setStatus("")}
                >
                  ✕
                </button>
              </span>
            )}
            {plateQuery && (
              <span className="wo-chip">
                Kennzeichen: {plateQuery}
                <button
                  type="button"
                  className="wo-chip__remove"
                  aria-label="Kennzeichenfilter entfernen"
                  onClick={clearPlate}
                >
                  ✕
                </button>
              </span>
            )}
            <button
              type="button"
              className="wo-text-button"
              onClick={resetFilters}
            >
              Filter zurücksetzen
            </button>
          </div>
        )}

        {error && (
          <div className="wo-alert" role="alert">
            {error}
          </div>
        )}

        {loading ? (
          <p className="wo-loading">Aufträge werden geladen …</p>
        ) : orders.length === 0 ? (
          <EmptyState filtered={hasFilter} onReset={resetFilters} />
        ) : (
          <>
            {renderTable()}
            {renderCards()}
          </>
        )}
      </section>
    </div>
  );
}
