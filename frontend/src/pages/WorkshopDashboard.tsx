import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useSession } from "../auth/session";
import { ApiError, apiFetch } from "../lib/api";
import "./WorkshopDashboard.css";

interface DashboardResponse {
  open_orders: number;
  completed_today: number;
  revenue_month_cents: number;
}

type OrderStatus = "angefragt" | "bestätigt" | "in Arbeit" | "fertig" | "abgeholt";

interface OrderItem {
  id: number;
  kind: string;
  description: string;
  hours?: number;
  quantity?: number;
  unit_price_cents?: number;
  total_cents: number;
}

interface OrderDetail {
  order_number: string;
  status: OrderStatus;
  requested_date: string;
  problem_description: string;
  created_at: string;
  customer: { name: string; email: string; phone: string };
  vehicle: { license_plate: string; brand: string; model: string; mileage: number };
  items: OrderItem[];
  history: unknown[];
}

interface OrderListResponse {
  orders: OrderDetail[];
}

const RECENT_ORDERS_LIMIT = 5;

const STATUS_LABELS: Record<OrderStatus, string> = {
  angefragt: "angefragt",
  bestätigt: "bestätigt",
  "in Arbeit": "in Arbeit",
  fertig: "fertig",
  abgeholt: "abgeholt",
};

const STATUS_CLASSES: Record<OrderStatus, string> = {
  angefragt: "wd-status--angefragt",
  bestätigt: "wd-status--bestaetigt",
  "in Arbeit": "wd-status--inArbeit",
  fertig: "wd-status--fertig",
  abgeholt: "wd-status--abgeholt",
};

const euroFormatter = new Intl.NumberFormat("de-DE", {
  style: "currency",
  currency: "EUR",
});

/** Formats whole cents in German euro notation, e.g. 1842350 -> "18.423,50 €". */
export function formatEuro(cents: number): string {
  return euroFormatter.format(cents / 100);
}

/** Formats a date as "TT.MM.JJJJ, HH:MM Uhr" in Europe/Berlin (AC: one format product-wide). */
export function formatStand(date: Date): string {
  const parts = new Intl.DateTimeFormat("de-DE", {
    timeZone: "Europe/Berlin",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).formatToParts(date);
  const value = (type: string) => parts.find((part) => part.type === type)?.value ?? "";
  return `${value("day")}.${value("month")}.${value("year")}, ${value("hour")}:${value("minute")} Uhr`;
}

function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return error.message;
  }
  return "Die Werkstatt-API ist nicht erreichbar.";
}

interface StatTileProps {
  label: string;
  loading: boolean;
  value: string | null;
  subline: string;
}

function StatTile({ label, loading, value, subline }: StatTileProps) {
  return (
    <div className="wd-stat-tile">
      <div className="wd-stat-label">{label}</div>
      {loading ? (
        <>
          <span className="wd-skeleton" aria-hidden="true" data-testid="stat-skeleton" />
          <span className="wd-skeleton wd-skeleton--sub" aria-hidden="true" />
        </>
      ) : (
        <>
          <div className="wd-stat-value">{value}</div>
          <div className="wd-stat-sub">{subline}</div>
        </>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: OrderStatus }) {
  const label = STATUS_LABELS[status];
  if (!label) {
    return <span className="wd-badge">{status}</span>;
  }
  return (
    <span className={`wd-badge ${STATUS_CLASSES[status]}`}>
      <span className="wd-badge__dot" aria-hidden="true" />
      {label}
    </span>
  );
}

function vehicleLine(order: OrderDetail): string {
  return [order.vehicle.brand, order.vehicle.model].filter(Boolean).join(" ");
}

export default function WorkshopDashboard() {
  const { token } = useSession();

  const [dashboard, setDashboard] = useState<DashboardResponse | null>(null);
  const [dashboardLoading, setDashboardLoading] = useState(true);
  const [dashboardError, setDashboardError] = useState<string | null>(null);
  const [stand, setStand] = useState<string>("");

  const [recentOrders, setRecentOrders] = useState<OrderDetail[]>([]);
  const [ordersLoading, setOrdersLoading] = useState(true);
  const [ordersError, setOrdersError] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setDashboardLoading(true);
    setDashboardError(null);

    apiFetch<DashboardResponse>("/api/workshop/dashboard", { token })
      .then((data) => {
        if (cancelled) {
          return;
        }
        setDashboard(data);
        setStand(formatStand(new Date()));
      })
      .catch((error: unknown) => {
        if (cancelled) {
          return;
        }
        setDashboardError(errorMessage(error));
      })
      .finally(() => {
        if (!cancelled) {
          setDashboardLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [token]);

  useEffect(() => {
    let cancelled = false;
    setOrdersLoading(true);
    setOrdersError(false);

    apiFetch<OrderListResponse>("/api/workshop/orders", { token })
      .then((data) => {
        if (cancelled) {
          return;
        }
        const newest = [...(data.orders ?? [])]
          .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))
          .slice(0, RECENT_ORDERS_LIMIT);
        setRecentOrders(newest);
      })
      .catch(() => {
        if (cancelled) {
          return;
        }
        setOrdersError(true);
      })
      .finally(() => {
        if (!cancelled) {
          setOrdersLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [token]);

  const revenue = dashboard ? formatEuro(dashboard.revenue_month_cents) : "—";

  return (
    <div className="container">
      <section className="page-header">
        <h1 className="page-title">Dashboard</h1>
      </section>

      {dashboardError && (
        <div className="wd-alert" role="alert">
          {dashboardError}
        </div>
      )}

      <section className="wd-stat-tiles" aria-busy={dashboardLoading}>
        <StatTile
          label="Offene Aufträge"
          loading={dashboardLoading}
          value={dashboard ? String(dashboard.open_orders) : "—"}
          subline={stand}
        />
        <StatTile
          label="Heute fertig geworden"
          loading={dashboardLoading}
          value={dashboard ? String(dashboard.completed_today) : "—"}
          subline={stand}
        />
        <StatTile
          label="Umsatz laufender Monat"
          loading={dashboardLoading}
          value={revenue}
          subline={stand}
        />
      </section>

      <section className="card">
        <div className="wd-card-header">
          <h2 className="wd-card-title">Neueste Aufträge</h2>
          <div className="wd-card-action">
            <Link className="btn btn-secondary" to="/werkstatt/auftraege">
              Alle Aufträge
            </Link>
          </div>
        </div>

        {ordersError ? (
          <div className="wd-alert" role="alert">
            Die neuesten Aufträge konnten nicht geladen werden.
          </div>
        ) : ordersLoading ? (
          <p className="wd-empty" aria-busy="true">
            Aufträge werden geladen …
          </p>
        ) : recentOrders.length === 0 ? (
          <p className="wd-empty">Noch keine Aufträge</p>
        ) : (
          <div className="wd-table-scroll">
            <table className="wd-data-table">
              <thead>
                <tr>
                  <th>Auftragsnummer</th>
                  <th>Kennzeichen</th>
                  <th>Fahrzeug</th>
                  <th>Status</th>
                  <th className="wd-num">Betrag brutto</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {recentOrders.map((order) => (
                  <tr key={order.order_number}>
                    <td>
                      <Link
                        className="wd-order-link"
                        to={`/werkstatt/auftraege/${order.order_number}`}
                      >
                        {order.order_number}
                      </Link>
                    </td>
                    <td>{order.vehicle.license_plate}</td>
                    <td>{vehicleLine(order)}</td>
                    <td>
                      <StatusBadge status={order.status} />
                    </td>
                    <td className="wd-num wd-muted">—</td>
                    <td>
                      <Link
                        className="link-button"
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
        )}
      </section>
    </div>
  );
}
