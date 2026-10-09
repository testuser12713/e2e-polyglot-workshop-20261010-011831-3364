import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom/vitest";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return { ...actual, apiFetch: vi.fn() };
});

import { ApiError, apiFetch } from "../lib/api";
import OrderStatus from "./OrderStatus";

const apiFetchMock = vi.mocked(apiFetch);

const NOT_FOUND_MESSAGE =
  "Zu dieser Auftragsnummer und diesem Kennzeichen liegt kein Auftrag vor.";
const NO_INVOICE_MESSAGE =
  "Noch keine Rechnung vorhanden — sie erscheint, sobald der Auftrag fertig ist.";

function makeOrder(overrides: Record<string, unknown> = {}) {
  return {
    order_number: "AUF-2026-000124",
    status: "in Arbeit",
    requested_date: "2026-10-12",
    problem_description: "Bremsen quietschen.",
    created_at: "2026-10-09T13:50:00Z",
    customer: { name: "Max Mustermann", email: "max@example.com", phone: "0123" },
    vehicle: {
      license_plate: "B-CD 5678",
      brand: "BMW",
      model: "3er (G20)",
      mileage: 88240,
    },
    items: [],
    history: [
      {
        from_status: null,
        to_status: "angefragt",
        changed_by: "System",
        changed_at: "2026-10-09T13:50:00Z",
      },
      {
        from_status: "angefragt",
        to_status: "bestätigt",
        changed_by: "Anna Meier",
        changed_at: "2026-10-09T16:10:00Z",
      },
      {
        from_status: "bestätigt",
        to_status: "in Arbeit",
        changed_by: "Anna Meier",
        changed_at: "2026-10-10T07:45:00Z",
      },
    ],
    ...overrides,
  };
}

function makeInvoice(overrides: Record<string, unknown> = {}) {
  return {
    invoice_number: "RE-2026-000125",
    order_number: "AUF-2026-000125",
    issued_at: "2026-10-03T15:20:00Z",
    lines: [
      {
        description: "Bremsen vorne erneuert",
        quantity: 2.5,
        unit_price_cents: 7800,
        total_cents: 19500,
      },
      {
        description: "Bremsbelagsatz vorne (ATE)",
        quantity: 2,
        unit_price_cents: 3850,
        total_cents: 7700,
      },
    ],
    labor_cents: 19500,
    parts_cents: 7700,
    net_cents: 27200,
    vat_cents: 5168,
    gross_cents: 32368,
    ...overrides,
  };
}

async function submitLookup(orderNumber: string, plate: string) {
  const user = userEvent.setup();
  await user.type(screen.getByLabelText(/Auftragsnummer/), orderNumber);
  await user.type(screen.getByLabelText(/Kennzeichen/), plate);
  await user.click(screen.getByRole("button", { name: "Status abrufen" }));
}

beforeEach(() => {
  apiFetchMock.mockReset();
});

afterEach(() => {
  cleanup();
});

describe("OrderStatus", () => {
  it("renders the status badge, the full timeline and the vehicle line from a mocked response", async () => {
    apiFetchMock.mockResolvedValueOnce(makeOrder());

    render(<OrderStatus />);
    await submitLookup("AUF-2026-000124", "B-CD 5678");

    expect(await screen.findByRole("region", { name: "Auftragsstatus" })).toBeInTheDocument();

    for (const status of ["angefragt", "bestätigt", "in Arbeit", "fertig", "abgeholt"]) {
      expect(screen.getAllByText(status).length).toBeGreaterThan(0);
    }

    expect(screen.getByText(/von System/)).toBeInTheDocument();
    expect(screen.getAllByText(/von Anna Meier/).length).toBeGreaterThan(0);
    expect(screen.getByText(/88\.240 km/)).toBeInTheDocument();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
  });

  it("shows the quiet empty state while no invoice exists", async () => {
    apiFetchMock
      .mockResolvedValueOnce(
        makeOrder({
          order_number: "AUF-2026-000125",
          status: "fertig",
          vehicle: {
            license_plate: "M-EF 9012",
            brand: "Opel",
            model: "Corsa F",
            mileage: 41350,
          },
        }),
      )
      .mockRejectedValueOnce(new ApiError(404, "not_found", "Noch keine Rechnung."));

    render(<OrderStatus />);
    await submitLookup("AUF-2026-000125", "M-EF 9012");

    expect(await screen.findByText(NO_INVOICE_MESSAGE)).toBeInTheDocument();
    expect(screen.queryByText(/Brutto/)).not.toBeInTheDocument();
  });

  it("renders the invoice for a finished order", async () => {
    apiFetchMock
      .mockResolvedValueOnce(
        makeOrder({
          order_number: "AUF-2026-000125",
          status: "fertig",
          vehicle: {
            license_plate: "M-EF 9012",
            brand: "Opel",
            model: "Corsa F",
            mileage: 41350,
          },
        }),
      )
      .mockResolvedValueOnce(makeInvoice());

    render(<OrderStatus />);
    await submitLookup("AUF-2026-000125", "M-EF 9012");

    expect(
      await screen.findByText("Rechnung zu Auftrag AUF-2026-000125"),
    ).toBeInTheDocument();
    expect(screen.getByText("Bremsen vorne erneuert")).toBeInTheDocument();
    expect(screen.getByText("Netto")).toBeInTheDocument();
    expect(screen.getByText(/Mehrwertsteuer/)).toBeInTheDocument();
    expect(screen.getByText("Brutto")).toBeInTheDocument();
    expect(screen.queryByText(NO_INVOICE_MESSAGE)).not.toBeInTheDocument();
  });

  it("shows the inline not-found message for a wrong pair", async () => {
    apiFetchMock.mockRejectedValueOnce(
      new ApiError(404, "not_found", "Auftrag nicht gefunden."),
    );

    render(<OrderStatus />);
    await submitLookup("AUF-2026-999999", "X-Y 0000");

    expect(await screen.findByText(NOT_FOUND_MESSAGE)).toBeInTheDocument();
    expect(screen.queryByText(/Rechnung zu Auftrag/)).not.toBeInTheDocument();

    const alert = screen.getByText(NOT_FOUND_MESSAGE);
    expect(alert.closest(".alert-danger")).not.toBeNull();
  });
});
