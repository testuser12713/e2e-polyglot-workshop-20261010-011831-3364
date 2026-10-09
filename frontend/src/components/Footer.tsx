import { Link } from "react-router-dom";

const WORKSHOP_NAME = "Kfz-Werkstatt";

export function Footer() {
  const year = new Date().getFullYear();

  return (
    <footer className="site-footer">
      <div className="site-footer__inner">
        <p className="site-footer__meta">
          © {year} {WORKSHOP_NAME}
        </p>
        <nav className="site-footer__nav" aria-label="Rechtliches">
          <Link className="site-footer__link" to="/impressum">
            Impressum
          </Link>
          <Link className="site-footer__link" to="/datenschutz">
            Datenschutzerklärung
          </Link>
        </nav>
      </div>
    </footer>
  );
}
