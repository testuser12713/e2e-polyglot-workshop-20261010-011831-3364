import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { AuthProvider, useSession } from "./session";
import { apiFetch } from "../lib/api";

vi.mock("../lib/api", () => ({ apiFetch: vi.fn() }));

const mockedApiFetch = vi.mocked(apiFetch);

function Probe() {
  const { employee, token, login, logout } = useSession();
  return (
    <div>
      <span data-testid="token">{token ?? "none"}</span>
      <span data-testid="employee">{employee?.email ?? "none"}</span>
      <button
        type="button"
        onClick={() => {
          void login("anna@werkstatt.de", "geheim");
        }}
      >
        login
      </button>
      <button type="button" onClick={logout}>
        logout
      </button>
    </div>
  );
}

function renderProbe() {
  return render(
    <AuthProvider>
      <Probe />
    </AuthProvider>,
  );
}

beforeEach(() => {
  window.sessionStorage.clear();
  mockedApiFetch.mockReset();
});

describe("useSession", () => {
  it("keeps a successful login in memory and in sessionStorage", async () => {
    mockedApiFetch.mockResolvedValueOnce({
      token: "test-token",
      employee: { id: 7, name: "Anna Meier", email: "anna@werkstatt.de" },
    });

    const user = userEvent.setup();
    renderProbe();

    await user.click(screen.getByRole("button", { name: "login" }));

    await waitFor(() =>
      expect(screen.getByTestId("token")).toHaveTextContent("test-token"),
    );
    expect(screen.getByTestId("employee")).toHaveTextContent("anna@werkstatt.de");
    expect(mockedApiFetch).toHaveBeenCalledWith(
      "/api/workshop/login",
      expect.objectContaining({
        method: "POST",
        body: { email: "anna@werkstatt.de", password: "geheim" },
      }),
    );
    expect(window.sessionStorage.getItem("werkstatt.session")).toContain(
      "test-token",
    );
  });

  it("restores an existing session from sessionStorage on reload", () => {
    window.sessionStorage.setItem(
      "werkstatt.session",
      JSON.stringify({
        token: "restored-token",
        employee: { id: 3, name: "Bea Schmidt", email: "bea@werkstatt.de" },
      }),
    );

    renderProbe();

    expect(screen.getByTestId("token")).toHaveTextContent("restored-token");
    expect(screen.getByTestId("employee")).toHaveTextContent("bea@werkstatt.de");
  });

  it("clears the session on logout", async () => {
    window.sessionStorage.setItem(
      "werkstatt.session",
      JSON.stringify({
        token: "restored-token",
        employee: { id: 3, name: "Bea Schmidt", email: "bea@werkstatt.de" },
      }),
    );

    const user = userEvent.setup();
    renderProbe();

    await user.click(screen.getByRole("button", { name: "logout" }));

    expect(screen.getByTestId("token")).toHaveTextContent("none");
    expect(screen.getByTestId("employee")).toHaveTextContent("none");
    expect(window.sessionStorage.getItem("werkstatt.session")).toBeNull();
  });
});
