import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Mock } from "vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import "@testing-library/jest-dom/vitest";
import { AuthProvider } from "../auth/session";
import { ApiError, apiFetch } from "../lib/api";
import WorkshopDashboard, { formatEuro } from "./WorkshopDashboard";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return { ...actual, apiFetch: vi.fn() };
});

const mockedApiFetch = apiFetch as unknown as Mock;

interface Order {
  order_number: string;
  status: string;
  requested_date: string;
  problem_description: string;
  created_at: string;
  customer: { name: string; email: string; phone: string };
  vehicle: { license_plate: string; brand: string; model: string; mileage: number };
  items: unknown[];
  history: unknown[];
}

function order(overrides: Partial<Order>): Order {
  return {
    order_number: "AUF-2026-000001",
    status: "angefragt",
    requested_date: "2026-01-05",
    problem_description: "Bremsen quietschen",
    created_at: "2026-01-01T08:00:00Z",
    customer: { name: "Max Mustermann", email: "max@example.com", phone: "0170" },
    vehicle: { license_plate: "B-AB 1234", brand: "VW", model: "Golf 8", mileage: 123456 },
    items: [],
    history: [],
    ...overrides,
  };
}

function mockApi(
  dashboard: unknown,
  orders: unknown = { orders: [] },
): void {
  mockedApiFetch.mockImplementation((path: string) => {
    if (path === "/api/workshop/dashboard") {
      return Promise.resolve(dashboard);
    }
    if (path === "/api/workshop/orders") {
      return Promise.resolve(orders);
    }
    return Promise.reject(new Error(`unexpected path: ${path}`));
  });
}

function renderDashboard() {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={["/werkstatt/dashboard"]}>
        <WorkshopDashboard />
      </MemoryRouter>
    </AuthProvider>,
  );
}

beforeEach(() => {
  window.localStorage.setItem(
    "werkstatt.session",
    JSON.stringify({
      employee: { id: 1, name: "Anna Meier", email: "anna.meier@werkstatt.de" },
      token: "test-token",
    }),
  );
});

afterEach(() => {
  cleanup();
  window.localStorage.clear();
  vi.clearAllMocks();
});

describe("WorkshopDashboard", () => {
  it("shows the page title and the three StatTile labels", async () => {
    mockApi({ open_orders: 12, completed_today: 3, revenue_month_cents: 1842350 });
    renderDashboard();

    expect(screen.getByRole("heading", { name: "Dashboard" })).toBeInTheDocument();
    expect(screen.getByText("Offene Aufträge")).toBeInTheDocument();
    expect(screen.getByText("Heute fertig geworden")).toBeInTheDocument();
    expect(screen.getByText("Umsatz laufender Monat")).toBeInTheDocument();
    await waitFor(() => expect(mockedApiFetch).toHaveBeenCalled());
  });

  it("shows neutral skeletons instead of zeros while loading", () => {
    mockedApiFetch.mockImplementation(() => new Promise(() => {}));
    renderDashboard();

    expect(screen.getAllByTestId("stat-skeleton")).toHaveLength(3);
    expect(screen.queryByText("0")).not.toBeInTheDocument();
  });

  it("renders the three figures from the mocked dashboard response", async () => {
    mockApi({ open_orders: 12, completed_today: 3, revenue_month_cents: 1842350 });
    renderDashboard();

    expect(await screen.findByText("12")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByText("18.423,50 €")).toBeInTheDocument();
  });

  it("formats the monthly revenue in German euro notation", () => {
    expect(formatEuro(1842350).replace(/\s/g, " ")).toBe("18.423,50 €");
    expect(formatEuro(0).replace(/\s/g, " ")).toBe("0,00 €");
    expect(formatEuro(1290).replace(/\s/g, " ")).toBe("12,90 €");
    expect(formatEuro(-1290).replace(/\s/g, " ")).toBe("-12,90 €");
  });

  it("lists the most recent orders, newest first, linking to the detail page", async () => {
    const older = order({
      order_number: "AUF-2026-000124",
      created_at: "2026-01-01T08:00:00Z",
      vehicle: { license_plate: "B-CD 5678", brand: "BMW", model: "3er", mileage: 1000 },
      status: "in Arbeit",
    });
    const newer = order({
      order_number: "AUF-2026-000125",
      created_at: "2026-01-03T08:00:00Z",
      vehicle: { license_plate: "M-EF 9012", brand: "Opel", model: "Corsa", mileage: 2000 },
      status: "fertig",
    });
    mockApi(
      { open_orders: 2, completed_today: 1, revenue_month_cents: 0 },
      { orders: [older, newer] },
    );

    renderDashboard();

    expect(await screen.findByText("AUF-2026-000125")).toBeInTheDocument();

    const numberLinks = screen
      .getAllByRole("link")
      .filter((link) => link.textContent?.startsWith("AUF-"));
    expect(numberLinks.map((link) => link.textContent)).toEqual([
      "AUF-2026-000125",
      "AUF-2026-000124",
    ]);
    expect(numberLinks[0]).toHaveAttribute("href", "/werkstatt/auftraege/AUF-2026-000125");

    expect(screen.getByText("B-CD 5678")).toBeInTheDocument();
    expect(screen.getByText("BMW 3er")).toBeInTheDocument();
    expect(screen.getByText("in Arbeit")).toBeInTheDocument();
    expect(screen.getByText("fertig")).toBeInTheDocument();

    expect(screen.getByRole("link", { name: "Alle Aufträge" })).toHaveAttribute(
      "href",
      "/werkstatt/auftraege",
    );
  });

  it("renders a readable error instead of crashing when the dashboard request fails", async () => {
    mockedApiFetch.mockImplementation((path: string) => {
      if (path === "/api/workshop/dashboard") {
        return Promise.reject(new ApiError(503, "internal_error", "Dashboard nicht verfügbar."));
      }
      return Promise.resolve({ orders: [] });
    });

    renderDashboard();

    const alerts = await screen.findAllByRole("alert");
    expect(alerts[0]).toHaveTextContent("Dashboard nicht verfügbar.");
  });

  it("shows an empty state when the order list endpoint is unavailable", async () => {
    mockedApiFetch.mockImplementation((path: string) => {
      if (path === "/api/workshop/dashboard") {
        return Promise.resolve({ open_orders: 0, completed_today: 0, revenue_month_cents: 0 });
      }
      return Promise.reject(new ApiError(501, "not_implemented", "noch nicht verfügbar"));
    });

    renderDashboard();

    const card = await screen.findByRole("heading", { name: "Neueste Aufträge" });
    const section = card.closest("section") as HTMLElement;
    expect(within(section).getAllByRole("alert")[0]).toHaveTextContent(
      "Die neuesten Aufträge konnten nicht geladen werden.",
    );
  });
});
