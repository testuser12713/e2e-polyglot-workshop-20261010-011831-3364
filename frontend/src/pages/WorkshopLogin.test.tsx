import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import App from "../App";
import { ApiError, apiFetch } from "../lib/api";

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return { ...actual, apiFetch: vi.fn() };
});

const mockedApiFetch = vi.mocked(apiFetch);

function LocationProbe() {
  const location = useLocation();
  return <span data-testid="location">{location.pathname}</span>;
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
      <LocationProbe />
    </MemoryRouter>,
  );
}

async function fillAndSubmit(email: string, password: string) {
  const user = userEvent.setup();
  if (email) {
    await user.type(screen.getByLabelText(/E-Mail/), email);
  }
  if (password) {
    await user.type(screen.getByLabelText(/Passwort/), password);
  }
  await user.click(screen.getByRole("button", { name: "Anmelden" }));
}

beforeEach(() => {
  window.sessionStorage.clear();
  mockedApiFetch.mockReset();
});

afterEach(() => {
  cleanup();
});

describe("WorkshopLogin", () => {
  it("shows the login card built from the shell and DESIGN tokens", () => {
    renderAt("/werkstatt/anmeldung");

    expect(
      screen.getByRole("heading", { name: "Werkstattbereich" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/E-Mail/)).toBeInTheDocument();
    expect(screen.getByLabelText(/Passwort/)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Anmelden" }),
    ).toBeInTheDocument();
  });

  it("stays neutral until submit and then reports missing fields", async () => {
    renderAt("/werkstatt/anmeldung");

    expect(
      screen.queryByText("Bitte eine gültige E-Mail-Adresse angeben."),
    ).not.toBeInTheDocument();

    await fillAndSubmit("", "");

    expect(
      await screen.findByText("Bitte eine gültige E-Mail-Adresse angeben."),
    ).toBeInTheDocument();
    expect(screen.getByText("Bitte das Passwort angeben.")).toBeInTheDocument();
    expect(mockedApiFetch).not.toHaveBeenCalled();
  });

  it("logs in, stores the session and opens the workshop area", async () => {
    mockedApiFetch.mockResolvedValueOnce({
      token: "test-token",
      employee: { id: 1, name: "Anna Meier", email: "anna@werkstatt.de" },
    });

    renderAt("/werkstatt/anmeldung");
    await fillAndSubmit("anna@werkstatt.de", "geheim");

    await waitFor(() =>
      expect(screen.getByTestId("location")).toHaveTextContent(
        "/werkstatt/auftraege",
      ),
    );
    expect(screen.getByText("anna@werkstatt.de")).toBeInTheDocument();
    expect(mockedApiFetch).toHaveBeenCalledWith(
      "/api/workshop/login",
      expect.objectContaining({
        method: "POST",
        body: { email: "anna@werkstatt.de", password: "geheim" },
      }),
    );
  });

  it("shows the API message on a wrong credential (401) and stays on the page", async () => {
    mockedApiFetch.mockRejectedValueOnce(
      new ApiError(401, "unauthorized", "Ungültige Zugangsdaten."),
    );

    renderAt("/werkstatt/anmeldung");
    await fillAndSubmit("anna@werkstatt.de", "falsch");

    const alert = await screen.findByTestId("login-alert");
    expect(alert).toHaveTextContent("Ungültige Zugangsdaten.");
    expect(screen.getByTestId("location")).toHaveTextContent(
      "/werkstatt/anmeldung",
    );
  });

  it("names the rate limit on a 429 and shows its own message", async () => {
    mockedApiFetch.mockRejectedValueOnce(
      new ApiError(429, "rate_limited", "too many login attempts"),
    );

    renderAt("/werkstatt/anmeldung");
    await fillAndSubmit("anna@werkstatt.de", "geheim");

    const alert = await screen.findByTestId("login-alert");
    expect(alert).toHaveTextContent(
      "Zu viele Anmeldeversuche, bitte in einer Minute erneut versuchen.",
    );
    expect(screen.getByTestId("location")).toHaveTextContent(
      "/werkstatt/anmeldung",
    );
  });
});
