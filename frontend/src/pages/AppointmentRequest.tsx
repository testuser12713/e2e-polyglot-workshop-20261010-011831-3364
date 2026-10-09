export default function AppointmentRequest() {
  return (
    <div className="container container--narrow">
      <section className="page-header">
        <h1 className="page-title">Termin anfragen</h1>
        <p className="page-lead">
          Erfassen Sie hier Kontakt-, Fahrzeug- und Anliegensdaten, um einen
          Werkstatttermin anzufragen.
        </p>
      </section>

      <section className="card">
        <p className="card-text">
          Das Anfrageformular wird in einem kommenden Schritt freigeschaltet.
        </p>
        <button type="button" className="btn btn-primary" disabled>
          Termin anfragen (bald verfügbar)
        </button>
      </section>
    </div>
  );
}
