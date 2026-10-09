import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import "@testing-library/jest-dom/vitest";
import App from "./App";

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

describe("app shell", () => {
  it("renders the navigation items with their real routes", () => {
    renderAt("/");

    expect(screen.getByRole("link", { name: "Termin anfragen" })).toHaveAttribute(
      "href",
      "/terminanfrage",
    );
    expect(screen.getByRole("link", { name: "Status abrufen" })).toHaveAttribute(
      "href",
      "/auftragsstatus",
    );
  });

  it("renders the footer links to Impressum and Datenschutzerklärung on every page", () => {
    for (const path of ["/", "/terminanfrage", "/auftragsstatus", "/impressum"]) {
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

  it("redirects /werkstatt/auftraege to the login page without a session", () => {
    renderAt("/werkstatt/auftraege");
    expect(screen.getByTestId("location")).toHaveTextContent("/werkstatt/anmeldung");
  });

  it("redirects /werkstatt/auftraege/:orderNumber to the login page without a session", () => {
    renderAt("/werkstatt/auftraege/AUF-2025-000123");
    expect(screen.getByTestId("location")).toHaveTextContent("/werkstatt/anmeldung");
  });

  it("redirects /werkstatt/dashboard to the login page without a session", () => {
    renderAt("/werkstatt/dashboard");
    expect(screen.getByTestId("location")).toHaveTextContent("/werkstatt/anmeldung");
  });
});
