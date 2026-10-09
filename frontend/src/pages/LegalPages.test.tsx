import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router-dom";
import "@testing-library/jest-dom/vitest";
import App from "../App";

afterEach(() => {
  cleanup();
  window.localStorage.clear();
});

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

const LEGAL_ROUTES = ["/impressum", "/datenschutz"];

describe("legal pages", () => {
  it("renders the Impressum heading and all of its sections at /impressum", () => {
    renderAt("/impressum");

    expect(
      screen.getByRole("heading", { level: 1, name: "Impressum" }),
    ).toBeInTheDocument();

    for (const section of [
      "Diensteanbieter",
      "Kontakt",
      "Registereintrag",
      "Verantwortlich für den Inhalt",
      "Haftungshinweis",
      "Streitbeilegung",
    ]) {
      expect(
        screen.getByRole("heading", { level: 2, name: section }),
      ).toBeInTheDocument();
    }
  });

  it("renders the Datenschutzerklärung heading and all of its sections at /datenschutz", () => {
    renderAt("/datenschutz");

    expect(
      screen.getByRole("heading", { level: 1, name: "Datenschutzerklärung" }),
    ).toBeInTheDocument();

    for (const section of [
      "1. Verantwortlicher",
      "2. Verarbeitete Daten",
      "3. Zwecke und Rechtsgrundlagen",
      "4. Speicherdauer",
      "5. Empfänger und keine Weitergabe",
      "6. Ihre Rechte",
      "7. Kontakt",
    ]) {
      expect(
        screen.getByRole("heading", { level: 2, name: section }),
      ).toBeInTheDocument();
    }
  });

  it("exposes footer links to both legal pages on every customer and workshop page", () => {
    for (const path of [
      "/",
      "/terminanfrage",
      "/auftragsstatus",
      "/werkstatt/anmeldung",
      ...LEGAL_ROUTES,
    ]) {
      const { unmount } = renderAt(path);

      expect(screen.getByRole("link", { name: "Impressum" })).toHaveAttribute(
        "href",
        "/impressum",
      );
      expect(
        screen.getByRole("link", { name: "Datenschutzerklärung" }),
      ).toHaveAttribute("href", "/datenschutz");

      unmount();
    }
  });

  it("navigates from a footer link to the Impressum page", async () => {
    const user = userEvent.setup();
    renderAt("/");

    await user.click(screen.getByRole("link", { name: "Impressum" }));

    expect(screen.getByTestId("location")).toHaveTextContent("/impressum");
    expect(
      screen.getByRole("heading", { level: 1, name: "Impressum" }),
    ).toBeInTheDocument();
  });

  it("navigates from a footer link to the Datenschutzerklärung page", async () => {
    const user = userEvent.setup();
    renderAt("/werkstatt/anmeldung");

    await user.click(
      screen.getByRole("link", { name: "Datenschutzerklärung" }),
    );

    expect(screen.getByTestId("location")).toHaveTextContent("/datenschutz");
    expect(
      screen.getByRole("heading", { level: 1, name: "Datenschutzerklärung" }),
    ).toBeInTheDocument();
  });
});
