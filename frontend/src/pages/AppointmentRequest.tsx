import { useState, type ChangeEvent, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { ApiError, apiFetch } from "../lib/api";

type FieldKey =
  | "name"
  | "email"
  | "phone"
  | "licensePlate"
  | "brand"
  | "model"
  | "mileage"
  | "requestedDate"
  | "problemDescription";

interface FormValues {
  name: string;
  email: string;
  phone: string;
  licensePlate: string;
  brand: string;
  model: string;
  mileage: string;
  requestedDate: string;
  problemDescription: string;
}

interface FieldConfig {
  key: FieldKey;
  label: string;
  type: "text" | "email" | "tel" | "date" | "textarea";
  inputMode?: "text" | "email" | "tel" | "numeric";
  placeholder?: string;
  numeric?: boolean;
  hint?: string;
  maxLength?: number;
}

interface FieldGroupConfig {
  title: string;
  fields: FieldConfig[];
}

/**
 * Minimal shape of the OrderDetail answered by POST /api/customer/appointments
 * (201). Only the fields this page renders are typed here.
 */
interface AppointmentDetail {
  order_number: string;
  vehicle?: {
    license_plate?: string;
  };
}

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const MILEAGE_PATTERN = /^\d{1,7}$/;

const EMPTY_VALUES: FormValues = {
  name: "",
  email: "",
  phone: "",
  licensePlate: "",
  brand: "",
  model: "",
  mileage: "",
  requestedDate: "",
  problemDescription: "",
};

const FIELDS: FieldGroupConfig[] = [
  {
    title: "Kontakt",
    fields: [
      { key: "name", label: "Name", type: "text", inputMode: "text" },
      { key: "email", label: "E-Mail", type: "email", inputMode: "email" },
      { key: "phone", label: "Telefon", type: "tel", inputMode: "tel" },
    ],
  },
  {
    title: "Fahrzeug",
    fields: [
      {
        key: "licensePlate",
        label: "Kennzeichen",
        type: "text",
        inputMode: "text",
        placeholder: "z. B. B-AB 1234",
      },
      { key: "brand", label: "Marke", type: "text", inputMode: "text" },
      { key: "model", label: "Modell", type: "text", inputMode: "text" },
      {
        key: "mileage",
        label: "Kilometerstand",
        type: "text",
        inputMode: "numeric",
        numeric: true,
      },
    ],
  },
  {
    title: "Anliegen",
    fields: [
      {
        key: "requestedDate",
        label: "Wunschtermin",
        type: "date",
        hint: "Wunschtermin, kein fester Termin",
      },
      {
        key: "problemDescription",
        label: "Problembeschreibung",
        type: "textarea",
        maxLength: 500,
      },
    ],
  },
];

const REQUIRED_MESSAGES: Record<FieldKey, string> = {
  name: "Bitte den Namen angeben.",
  email: "Bitte eine gültige E-Mail-Adresse angeben.",
  phone: "Bitte eine Telefonnummer angeben.",
  licensePlate: "Bitte das Kennzeichen angeben.",
  brand: "Bitte die Marke angeben.",
  model: "Bitte das Modell angeben.",
  mileage: "Bitte den Kilometerstand angeben.",
  requestedDate: "Bitte einen Wunschtermin wählen.",
  problemDescription: "Bitte das Problem kurz beschreiben.",
};

const GENERIC_ERROR = "Die Anfrage konnte nicht übermittelt werden. Bitte erneut versuchen.";

/** Returns the validation message for a field, or "" when it is valid. */
function validateField(key: FieldKey, value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    return REQUIRED_MESSAGES[key];
  }
  if (key === "email" && !EMAIL_PATTERN.test(trimmed)) {
    return REQUIRED_MESSAGES.email;
  }
  if (key === "mileage" && !MILEAGE_PATTERN.test(trimmed)) {
    return "Bitte einen gültigen Kilometerstand angeben.";
  }
  return "";
}

export default function AppointmentRequest() {
  const [values, setValues] = useState<FormValues>(EMPTY_VALUES);
  const [touched, setTouched] = useState<Partial<Record<FieldKey, boolean>>>({});
  const [submitted, setSubmitted] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [confirmation, setConfirmation] = useState<{
    orderNumber: string;
    licensePlate: string;
  } | null>(null);

  function handleChange(event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>): void {
    const { name, value } = event.target;
    setValues((current) => ({ ...current, [name]: value }));
  }

  function handleBlur(event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>): void {
    const { name } = event.target;
    setTouched((current) => ({ ...current, [name]: true }));
  }

  /** Error shown for a field: only after entry (blur with content) or a submit. */
  function fieldError(key: FieldKey): string {
    const showForTouched = Boolean(touched[key]) && values[key].trim() !== "";
    if (!showForTouched && !submitted) {
      return "";
    }
    return validateField(key, values[key]);
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setSubmitted(true);
    setErrorMessage(null);

    const invalid = FIELDS.some((group) =>
      group.fields.some((field) => validateField(field.key, values[field.key]) !== ""),
    );
    if (invalid) {
      return;
    }

    setSubmitting(true);
    try {
      const detail = await apiFetch<AppointmentDetail>("/api/customer/appointments", {
        method: "POST",
        body: {
          customer: {
            name: values.name.trim(),
            email: values.email.trim(),
            phone: values.phone.trim(),
          },
          vehicle: {
            license_plate: values.licensePlate.trim(),
            brand: values.brand.trim(),
            model: values.model.trim(),
            mileage: Number.parseInt(values.mileage, 10),
          },
          requested_date: values.requestedDate,
          problem_description: values.problemDescription.trim(),
        },
      });
      setConfirmation({
        orderNumber: detail.order_number,
        licensePlate: detail.vehicle?.license_plate ?? values.licensePlate.trim(),
      });
    } catch (error) {
      if (error instanceof ApiError) {
        setErrorMessage(error.message || GENERIC_ERROR);
      } else {
        setErrorMessage(GENERIC_ERROR);
      }
    } finally {
      setSubmitting(false);
    }
  }

  function resetForm(): void {
    setValues(EMPTY_VALUES);
    setTouched({});
    setSubmitted(false);
    setErrorMessage(null);
    setConfirmation(null);
  }

  if (confirmation) {
    return (
      <div className="container container--narrow">
        <section className="page-header">
          <h1 className="page-title">Termin anfragen</h1>
        </section>

        <section className="card form-card">
          <div className="alert alert-success" role="status">
            <span aria-hidden="true">✓</span>
            <div>
              <div>Termin angefragt.</div>
              <div className="alert-sub">
                Ihre Anfrage wurde als Werkstattauftrag angelegt.
              </div>
            </div>
          </div>

          <div className="card-stack" style={{ marginTop: "var(--space-3)" }}>
            <p style={{ margin: 0 }}>Ihre Auftragsnummer:</p>
            <p className="confirm-num">{confirmation.orderNumber}</p>
            <p style={{ margin: 0 }} className="alert-sub">
              Der Status des Auftrags lässt sich jederzeit über die Auftragsnummer und
              das Kennzeichen abrufen.
            </p>
            <div className="confirm-actions">
              <Link className="btn btn-secondary" to="/auftragsstatus">
                Status abrufen
              </Link>
              <button type="button" className="btn btn-primary" onClick={resetForm}>
                Weitere Anfrage stellen
              </button>
            </div>
          </div>
        </section>
      </div>
    );
  }

  return (
    <div className="container container--narrow">
      <section className="page-header">
        <h1 className="page-title">Termin anfragen</h1>
      </section>

      <section className="card form-card">
        <form noValidate onSubmit={handleSubmit}>
          {FIELDS.map((group) => (
            <fieldset className="field-group" key={group.title} style={{ border: 0, padding: 0 }}>
              <legend className="group-title">{group.title}</legend>
              {group.fields.map((field) => {
                const message = fieldError(field.key);
                const inputId = `appointment-${field.key}`;
                const helpId = field.hint ? `${inputId}-help` : undefined;
                const errorId = message ? `${inputId}-error` : undefined;
                const describedBy = [helpId, errorId].filter(Boolean).join(" ") || undefined;
                const invalid = message !== "";

                const sharedProps = {
                  id: inputId,
                  name: field.key,
                  value: values[field.key],
                  onChange: handleChange,
                  onBlur: handleBlur,
                  "aria-invalid": invalid || undefined,
                  "aria-describedby": describedBy,
                  className: `${field.numeric ? "is-num" : ""}${invalid ? " is-invalid" : ""}`.trim() || undefined,
                };

                return (
                  <div className={`field${invalid ? " field--invalid" : ""}`} key={field.key}>
                    <label htmlFor={inputId}>
                      {field.label}
                      <span className="required-marker">erforderlich</span>
                    </label>

                    {field.type === "textarea" ? (
                      <textarea {...sharedProps} rows={4} maxLength={field.maxLength} />
                    ) : (
                      <input
                        {...sharedProps}
                        type={field.type}
                        inputMode={field.inputMode}
                        placeholder={field.placeholder}
                        autoComplete={field.key === "email" ? "email" : field.key === "phone" ? "tel" : "off"}
                      />
                    )}

                    {field.type === "textarea" && field.maxLength ? (
                      <p className="char-counter">
                        {values[field.key].length}/{field.maxLength}
                      </p>
                    ) : null}

                    {field.hint ? (
                      <p className="field-help" id={helpId}>
                        {field.hint}
                      </p>
                    ) : null}

                    {message ? (
                      <p className="field-error" id={errorId} role="alert">
                        {message}
                      </p>
                    ) : null}
                  </div>
                );
              })}
            </fieldset>
          ))}

          {errorMessage ? (
            <div className="alert alert-danger" role="alert" style={{ marginBottom: "var(--space-3)" }}>
              <span aria-hidden="true">!</span>
              <div>{errorMessage}</div>
            </div>
          ) : null}

          <div className="field-group">
            <button
              type="submit"
              className="btn btn-primary btn-lg"
              disabled={submitting}
              aria-busy={submitting || undefined}
            >
              {submitting ? (
                <>
                  <span className="sr-only">Wird gesendet …</span>
                  <span className="dots" aria-hidden="true">
                    <i />
                    <i />
                    <i />
                  </span>
                </>
              ) : (
                "Termin anfragen"
              )}
            </button>
          </div>
        </form>
      </section>
    </div>
  );
}
