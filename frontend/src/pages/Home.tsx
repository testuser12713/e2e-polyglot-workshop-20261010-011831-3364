import { Link } from "react-router-dom";

export default function Home() {
  return (
    <div className="container">
      <section className="page-header">
        <h1 className="page-title">Willkommen im Werkstattportal</h1>
        <p className="page-lead">
          Hier fragen Sie Termine an, verfolgen den Status Ihres Fahrzeugs und
          sehen Ihre Rechnungen. Der Werkstattbereich steht unseren Mitarbeitern
          nach der Anmeldung zur Verfügung.
        </p>
      </section>

      <div className="entry-grid">
        <section className="card entry-card">
          <h2 className="card-title">Kundenbereich</h2>
          <p className="card-text">
            Termin anfragen und den Bearbeitungsstand Ihres Auftrags jederzeit
            einsehen.
          </p>
          <div className="entry-card__actions">
            <Link className="btn btn-primary" to="/terminanfrage">
              Zur Terminanfrage
            </Link>
            <Link className="btn btn-secondary" to="/auftragsstatus">
              Zum Auftragsstatus
            </Link>
          </div>
        </section>

        <section className="card entry-card">
          <h2 className="card-title">Werkstattbereich</h2>
          <p className="card-text">
            Aufträge, Positionen und Status verwalten — nur für angemeldete
            Mitarbeiter.
          </p>
          <div className="entry-card__actions">
            <Link className="btn btn-secondary" to="/werkstatt/anmeldung">
              Zur Werkstattanmeldung
            </Link>
          </div>
        </section>
      </div>
    </div>
  );
}
