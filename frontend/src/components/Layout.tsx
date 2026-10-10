import { useEffect, useState } from "react";
import { Link, NavLink, Outlet, useLocation } from "react-router-dom";
import { useSession } from "../auth/session";
import { Footer } from "./Footer";

interface NavItem {
  to: string;
  label: string;
}

const CUSTOMER_ITEMS: NavItem[] = [
  { to: "/terminanfrage", label: "Termin anfragen" },
  { to: "/auftragsstatus", label: "Status abrufen" },
];

const WORKSHOP_ITEMS: NavItem[] = [
  { to: "/werkstatt/dashboard", label: "Dashboard" },
  { to: "/werkstatt/auftraege", label: "Auftragsliste" },
];

export function Layout() {
  const { pathname } = useLocation();
  const { employee, token, logout } = useSession();
  const [menuOpen, setMenuOpen] = useState(false);

  const isWorkshopArea = pathname.startsWith("/werkstatt");
  const items = isWorkshopArea ? WORKSHOP_ITEMS : CUSTOMER_ITEMS;

  useEffect(() => {
    setMenuOpen(false);
  }, [pathname]);

  return (
    <div className="app-shell">
      <header className="site-header">
        <div className="site-header__inner">
          {pathname === "/" ? (
            <span className="brand">Werkstattportal</span>
          ) : (
            <Link className="brand" to="/">
              Werkstattportal
            </Link>
          )}

          <button
            type="button"
            className="menu-toggle"
            aria-expanded={menuOpen}
            aria-controls="site-nav"
            aria-label={menuOpen ? "Menü schließen" : "Menü öffnen"}
            onClick={() => setMenuOpen((open) => !open)}
          >
            <span className="menu-toggle__bars" aria-hidden="true" />
            <span className="menu-toggle__label">Menü</span>
          </button>

          <nav
            id="site-nav"
            className={`site-nav${menuOpen ? " site-nav--open" : ""}`}
            aria-label={isWorkshopArea ? "Werkstattbereich" : "Kundenbereich"}
          >
            {items.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) =>
                  `site-nav__link${isActive ? " site-nav__link--active" : ""}`
                }
              >
                {item.label}
              </NavLink>
            ))}

            {isWorkshopArea && token && (
              <div className="site-nav__session">
                {employee && <span className="site-nav__user">{employee.email}</span>}
                <button type="button" className="link-button" onClick={logout}>
                  Abmelden
                </button>
              </div>
            )}
          </nav>
        </div>
      </header>

      <main className="site-main">
        <Outlet />
      </main>

      <Footer />
    </div>
  );
}
