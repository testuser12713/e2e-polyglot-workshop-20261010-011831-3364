export default function WorkshopLogin() {
  return (
    <div className="container container--login">
      <section className="card login-card">
        <h1 className="card-title">Anmeldung</h1>
        <p className="card-text">
          Der Werkstattbereich ist nur für angemeldete Mitarbeiter zugänglich.
        </p>
        <button type="button" className="btn btn-primary btn-block" disabled>
          Anmelden (bald verfügbar)
        </button>
      </section>
    </div>
  );
}
