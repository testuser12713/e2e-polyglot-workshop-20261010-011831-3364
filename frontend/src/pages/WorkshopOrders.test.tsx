import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router-dom";
import "@testing-library/jest-dom/vitest";
import { AuthProvider } from "../auth/session";
import WorkshopOrders from "./WorkshopOrders";
import { apiFetch } from "../lib/api";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return { ...actual, apiFetch: vi.fn() };
});

const mockedApiFetch = vi.mocked(apiFetch);

const SESSION_KEY = "werkstatt.session";

function seedSession() {
  window.localStorage.setItem(
    SESSION_KEY,
    JSON.stringify({
      employee: { id: 1, name: "Anna Meier", email: "anna@werkstatt.de" },
      token: "test-token",
    }),
  );
}

function sampleOrder(overrides: Record<string, unknown> = {}) {
  return {
    order_number: "AUF-2026-000123",
    status: "in Arbeit",
    requested_date: "2026-10-20",
    problem_description: "Bremsen quietschen",
    created_at: "2026-10-01T08:00:00Z",
    customer: {
      name: "Max Mustermann",
      email: "max@example.com",
      phone: "030-123456",
    },
    vehicle: {
      license_plate: "B-AB 1234",
      brand: "VW",
      model: "Golf 8",
      mileage: 45000,
    },
    items: [
      {
        id: 1,
        kind: "labor",
        description: "Bremsen prüfen",
        hours: 1,
        total_cents: 10000,
      },
    ],
    history: [],
    ...overrides,
  };
}

function LocationProbe() {
  const location = useLocation();
  return <span data-testid="location">{location.pathname}</span>;
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={["/werkstatt/auftraege"]}>
      <AuthProvider>
        <WorkshopOrders />
        <LocationProbe />
      </AuthProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  seedSession();
  mockedApiFetch.mockReset();
});

afterEach(() => {
  cleanup();
  window.localStorage.clear();
});

describe("WorkshopOrders", () => {
  it("renders the orders from the mocked API response", async () => {
    mockedApiFetch.mockResolvedValue({ orders: [sampleOrder()] });

    renderPage();

    expect(await screen.findAllByText("AUF-2026-000123")).not.toHaveLength(0);
    expect(screen.getAllByText("B-AB 1234").length).toBeGreaterThan(0);
    expect(screen.getAllByText("VW Golf 8").length).toBeGreaterThan(0);
    expect(screen.getAllByText("in Arbeit").length).toBeGreaterThan(0);
    expect(screen.getAllByText(/119,00/).length).toBeGreaterThan(0);

    expect(mockedApiFetch).toHaveBeenCalledWith(
      "/api/workshop/orders",
      expect.objectContaining({ token: "test-token" }),
    );
  });

  it("sends status and license plate as query parameters", async () => {
    mockedApiFetch.mockResolvedValue({ orders: [] });

    renderPage();
    await waitFor(() => expect(mockedApiFetch).toHaveBeenCalled());

    await userEvent.selectOptions(
      screen.getByLabelText("Nach Status filtern"),
      "in Arbeit",
    );
    await userEvent.type(
      screen.getByLabelText("Nach Kennzeichen suchen"),
      "b-ab",
    );

    await waitFor(
      () => {
        const lastPath = mockedApiFetch.mock.calls.at(-1)?.[0];
        expect(lastPath).toBe(
          "/api/workshop/orders?status=in+Arbeit&license_plate=B-AB",
        );
      },
      { timeout: 2000 },
    );
  });

  it("shows the filter empty state and resets it", async () => {
    mockedApiFetch.mockResolvedValue({ orders: [] });

    renderPage();

    expect(await screen.findByText("Noch keine Aufträge")).toBeInTheDocument();

    await userEvent.selectOptions(
      screen.getByLabelText("Nach Status filtern"),
      "fertig",
    );

    expect(
      await screen.findByText("Keine Aufträge für diesen Filter"),
    ).toBeInTheDocument();

    const emptyState = screen.getByRole("status");
    await userEvent.click(
      within(emptyState).getByRole("button", { name: "Filter zurücksetzen" }),
    );

    expect(
      screen.queryByText("Keine Aufträge für diesen Filter"),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Noch keine Aufträge")).toBeInTheDocument();
  });

  it("redirects to the login page on a 401", async () => {
    const { ApiError } = await import("../lib/api");
    mockedApiFetch.mockRejectedValue(
      new ApiError(401, "unauthorized", "Nicht angemeldet."),
    );

    renderPage();

    await waitFor(() =>
      expect(screen.getByTestId("location")).toHaveTextContent(
        "/werkstatt/anmeldung",
      ),
    );
  });
});
