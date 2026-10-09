import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import "@testing-library/jest-dom/vitest";

const { apiFetchMock } = vi.hoisted(() => ({ apiFetchMock: vi.fn() }));

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return { ...actual, apiFetch: apiFetchMock };
});

import AppointmentRequest from "./AppointmentRequest";
import { ApiError } from "../lib/api";

afterEach(() => {
  cleanup();
  apiFetchMock.mockReset();
});

function renderPage() {
  return render(
    <MemoryRouter>
      <AppointmentRequest />
    </MemoryRouter>,
  );
}

async function fillValidForm(user: UserEvent): Promise<void> {
  await user.type(screen.getByLabelText(/^Name/), "Anna Meier");
  await user.type(screen.getByLabelText(/E-Mail/), "anna@example.com");
  await user.type(screen.getByLabelText(/Telefon/), "030 123456");
  await user.type(screen.getByLabelText(/Kennzeichen/), "B-AB 1234");
  await user.type(screen.getByLabelText(/Marke/), "VW");
  await user.type(screen.getByLabelText(/Modell/), "Golf");
  await user.type(screen.getByLabelText(/Kilometerstand/), "12345");
  fireEvent.change(screen.getByLabelText(/Wunschtermin/), {
    target: { value: "2026-10-20" },
  });
  await user.type(screen.getByLabelText(/Problembeschreibung/), "Bremsen quietschen.");
}

describe("AppointmentRequest", () => {
  it("starts neutral without any reported errors", () => {
    renderPage();

    expect(screen.getByRole("button", { name: "Termin anfragen" })).toBeEnabled();
    expect(screen.getByLabelText(/E-Mail/)).not.toHaveAttribute("aria-invalid");
    expect(screen.queryByText("Bitte den Namen angeben.")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Bitte eine gültige E-Mail-Adresse angeben."),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("reports an invalid e-mail only after it was entered and left", async () => {
    const user = userEvent.setup();
    renderPage();

    const email = screen.getByLabelText(/E-Mail/);
    await user.type(email, "keine-mail");
    expect(
      screen.queryByText("Bitte eine gültige E-Mail-Adresse angeben."),
    ).not.toBeInTheDocument();

    await user.tab();

    expect(
      screen.getByText("Bitte eine gültige E-Mail-Adresse angeben."),
    ).toBeInTheDocument();
    expect(email).toHaveAttribute("aria-invalid", "true");
  });

  it("shows the new order number after a successful submit", async () => {
    const user = userEvent.setup();
    apiFetchMock.mockResolvedValue({
      order_number: "AUF-2026-000123",
      status: "angefragt",
      vehicle: { license_plate: "B-AB 1234" },
    });
    renderPage();

    await fillValidForm(user);
    await user.click(screen.getByRole("button", { name: "Termin anfragen" }));

    await waitFor(() => {
      expect(screen.getByText("AUF-2026-000123")).toBeInTheDocument();
    });

    expect(apiFetchMock).toHaveBeenCalledWith(
      "/api/customer/appointments",
      expect.objectContaining({ method: "POST" }),
    );
    const [, options] = apiFetchMock.mock.calls[0] as [string, { body: Record<string, unknown> }];
    expect(options.body).toMatchObject({
      customer: { name: "Anna Meier", email: "anna@example.com", phone: "030 123456" },
      vehicle: { license_plate: "B-AB 1234", brand: "VW", model: "Golf", mileage: 12345 },
      requested_date: "2026-10-20",
      problem_description: "Bremsen quietschen.",
    });

    expect(
      screen.getByText(/über die Auftragsnummer und das Kennzeichen abrufen/),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Termin anfragen" })).not.toBeInTheDocument();
  });

  it("shows the returned error on a 400 response and keeps the form", async () => {
    const user = userEvent.setup();
    apiFetchMock.mockRejectedValue(
      new ApiError(400, "validation_error", "Validierung fehlgeschlagen."),
    );
    renderPage();

    await fillValidForm(user);
    await user.click(screen.getByRole("button", { name: "Termin anfragen" }));

    await waitFor(() => {
      expect(screen.getByText("Validierung fehlgeschlagen.")).toBeInTheDocument();
    });

    expect(screen.getByRole("button", { name: "Termin anfragen" })).toBeEnabled();
    expect(screen.getByLabelText(/^Name/)).toHaveValue("Anna Meier");
  });
});
