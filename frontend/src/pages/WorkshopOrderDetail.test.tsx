import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import "@testing-library/jest-dom/vitest";
import WorkshopOrderDetail from "./WorkshopOrderDetail";
import { apiFetch } from "../lib/api";

vi.mock("../auth/session", () => ({
  useSession: () => ({
    employee: { id: 1, name: "Anna Meier", email: "anna@werkstatt.de" },
    token: "test-token",
    login: vi.fn(),
    logout: vi.fn(),
  }),
}));

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return { ...actual, apiFetch: vi.fn() };
});

const mockedApiFetch = apiFetch as unknown as Mock;
const ORDER_NUMBER = "AUF-2026-000123";

interface MockOrder {
  order_number: string;
  status: string;
  requested_date: string;
  problem_description: string;
  created_at: string;
  customer: { name: string; email: string; phone: string };
  vehicle: {
    license_plate: string;
    brand: string;
    model: string;
    mileage: number;
  };
  items: Array<Record<string, unknown>>;
  history: Array<Record<string, unknown>>;
}

function makeOrder(overrides: Partial<MockOrder> = {}): MockOrder {
  return {
    order_number: ORDER_NUMBER,
    status: "bestätigt",
    requested_date: "2026-10-20",
    problem_description:
      "Bremsen quietschen beim Anfahren, ABS-Lampe leuchtet zeitweise auf.",
    created_at: "2026-10-05T09:20:00Z",
    customer: {
      name: "Marco Brandt",
      email: "m.brandt@example.com",
      phone: "0176 1234 5678",
    },
    vehicle: {
      license_plate: "B-AB 1234",
      brand: "VW",
      model: "Golf 8",
      mileage: 123456,
    },
    items: [
      {
        id: 1,
        kind: "labor",
        description: "Voruntersuchung Bremse vorne",
        hours: 0.5,
        unit_price_cents: 7800,
        total_cents: 3900,
      },
      {
        id: 2,
        kind: "part",
        description: "Bremsbelagsatz vorne (ATE)",
        quantity: 1,
        unit_price_cents: 5280,
        total_cents: 5280,
      },
    ],
    history: [
      {
        from_status: null,
        to_status: "angefragt",
        changed_by: "System",
        changed_at: "2026-10-05T09:20:00Z",
      },
      {
        from_status: "angefragt",
        to_status: "bestätigt",
        changed_by: "Anna Meier",
        changed_at: "2026-10-05T13:45:00Z",
      },
    ],
    ...overrides,
  };
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={[`/werkstatt/auftraege/${ORDER_NUMBER}`]}>
      <Routes>
        <Route
          path="/werkstatt/auftraege/:orderNumber"
          element={<WorkshopOrderDetail />}
        />
      </Routes>
    </MemoryRouter>,
  );
}

async function findLoadedTitle() {
  return screen.findByRole("heading", {
    level: 1,
    name: `Auftrag ${ORDER_NUMBER}`,
  });
}

beforeEach(() => {
  mockedApiFetch.mockReset();
});

afterEach(() => {
  cleanup();
});

describe("WorkshopOrderDetail", () => {
  it("renders a loaded order with vehicle, customer and positions", async () => {
    mockedApiFetch.mockResolvedValue(makeOrder());

    renderPage();
    await findLoadedTitle();

    expect(
      screen.getByRole("link", { name: "← Zur Auftragsliste" }),
    ).toHaveAttribute("href", "/werkstatt/auftraege");
    expect(screen.getByText("Marco Brandt")).toBeInTheDocument();
    expect(screen.getByText("m.brandt@example.com")).toBeInTheDocument();
    expect(screen.getByText("B-AB 1234")).toBeInTheDocument();
    expect(screen.getByText("VW Golf 8")).toBeInTheDocument();
    expect(
      screen.getByText(/Bremsen quietschen beim Anfahren/),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Voruntersuchung Bremse vorne"),
    ).toBeInTheDocument();
    expect(screen.getByText("Bremsbelagsatz vorne (ATE)")).toBeInTheDocument();
    expect(mockedApiFetch).toHaveBeenCalledWith(
      `/api/workshop/orders/${ORDER_NUMBER}`,
      expect.objectContaining({ token: "test-token" }),
    );
  });

  it("adds a position through the modal and posts it to the items endpoint", async () => {
    const user = userEvent.setup();
    mockedApiFetch.mockResolvedValueOnce(makeOrder());
    mockedApiFetch.mockResolvedValueOnce(
      makeOrder({
        items: [
          ...makeOrder().items,
          {
            id: 3,
            kind: "labor",
            description: "Testarbeit",
            hours: 1.5,
            unit_price_cents: 7800,
            total_cents: 11700,
          },
        ],
      }),
    );

    renderPage();
    await findLoadedTitle();

    await user.click(
      screen.getByRole("button", { name: "Position erfassen" }),
    );
    expect(
      screen.getByRole("heading", { name: "Position erfassen" }),
    ).toBeInTheDocument();

    await user.type(
      screen.getByLabelText(/Beschreibung/),
      "Testarbeit",
    );
    await user.type(
      screen.getByLabelText(/Arbeitszeit \(Stunden\)/),
      "1,5",
    );
    await user.type(screen.getByLabelText(/Einzelpreis/), "78,00");

    await user.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mockedApiFetch).toHaveBeenCalledWith(
        `/api/workshop/orders/${ORDER_NUMBER}/items`,
        {
          method: "POST",
          body: {
            kind: "labor",
            description: "Testarbeit",
            hours: 1.5,
            unit_price_cents: 7800,
          },
          token: "test-token",
        },
      );
    });

    expect(await screen.findByText("Testarbeit")).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Position erfassen" }),
    ).not.toBeInTheDocument();
  });

  it("offers only the allowed next status as an enabled option", async () => {
    mockedApiFetch.mockResolvedValue(makeOrder({ status: "bestätigt" }));

    renderPage();
    await findLoadedTitle();

    const allowed = screen.getByRole("option", {
      name: "in Arbeit",
    }) as HTMLOptionElement;
    const same = screen.getByRole("option", {
      name: "bestätigt",
    }) as HTMLOptionElement;
    const skipped = screen.getByRole("option", {
      name: "fertig",
    }) as HTMLOptionElement;
    const earlier = screen.getByRole("option", {
      name: "angefragt",
    }) as HTMLOptionElement;

    expect(allowed.disabled).toBe(false);
    expect(same.disabled).toBe(true);
    expect(skipped.disabled).toBe(true);
    expect(earlier.disabled).toBe(true);
  });

  it("shows the API message when a status change is rejected with 409", async () => {
    const { ApiError } = await import("../lib/api");
    const user = userEvent.setup();
    mockedApiFetch.mockResolvedValueOnce(makeOrder({ status: "bestätigt" }));
    mockedApiFetch.mockRejectedValueOnce(
      new ApiError(409, "invalid_transition", "Dieser Statuswechsel ist nicht erlaubt."),
    );

    renderPage();
    await findLoadedTitle();

    await user.click(screen.getByRole("button", { name: "Status setzen" }));

    expect(
      await screen.findByText("Dieser Statuswechsel ist nicht erlaubt."),
    ).toBeInTheDocument();
  });
});
