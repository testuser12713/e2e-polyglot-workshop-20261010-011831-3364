export default function OrderStatus() {
  return (
    <div className="container container--narrow">
      <section className="page-header">
        <h1 className="page-title">Status abrufen</h1>
        <p className="page-lead">
          Sehen Sie den Bearbeitungsstand Ihres Fahrzeugs über Auftragsnummer und
          Kennzeichen ein.
        </p>
      </section>

      <section className="card">
        <p className="card-text">
          Die Statusabfrage wird in einem kommenden Schritt freigeschaltet.
        </p>
        <button type="button" className="btn btn-primary" disabled>
          Status abrufen (bald verfügbar)
        </button>
      </section>
    </div>
  );
}
