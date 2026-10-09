export default function LegalNotice() {
  return (
    <div className="container container--text">
      <div className="page-header">
        <h1 className="page-title">Impressum</h1>
        <p className="legal-meta">Angaben gemäß § 5 DDG</p>
      </div>

      <div className="prose">
        <section>
          <h2>Diensteanbieter</h2>
          <p>
            Kfz-Service Berger GmbH
            <br />
            Musterstraße 12
            <br />
            10115 Berlin
          </p>
          <p>Vertreten durch die Geschäftsführung: Daniel Berger</p>
        </section>

        <section>
          <h2>Kontakt</h2>
          <p>
            Telefon: +49 30 12345678
            <br />
            E-Mail: service@kfz-berger.de
          </p>
        </section>

        <section>
          <h2>Registereintrag</h2>
          <p>
            Handelsregister: Amtsgericht Charlottenburg
            <br />
            Registernummer: HRB 123456
            <br />
            Umsatzsteuer-Identifikationsnummer gemäß § 27a UStG: DE 123 456 789
          </p>
        </section>

        <section>
          <h2>Verantwortlich für den Inhalt</h2>
          <p>Daniel Berger (Anschrift wie oben)</p>
        </section>

        <section>
          <h2>Haftungshinweis</h2>
          <p>
            Wir prüfen Inhalte externer Verweise mit Sorgfalt, übernehmen für
            deren Inhalte jedoch keine Haftung. Für die Inhalte der verlinkten
            Seiten sind ausschließlich deren Betreiber verantwortlich.
          </p>
        </section>

        <section>
          <h2>Streitbeilegung</h2>
          <p>
            Zur Teilnahme an einem Streitbeilegungsverfahren vor einer
            Verbraucherschlichtungsstelle sind wir nicht verpflichtet und nicht
            bereit.
          </p>
        </section>
      </div>
    </div>
  );
}
