import { useState, type FormEvent } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useSession } from "../auth/session";
import { ApiError } from "../lib/api";
import "./WorkshopLogin.css";

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

const RATE_LIMIT_MESSAGE =
  "Zu viele Anmeldeversuche, bitte in einer Minute erneut versuchen.";

type AlertTone = "danger" | "warning";

interface FormAlert {
  tone: AlertTone;
  message: string;
}

function emailErrorFor(value: string): string | undefined {
  const trimmed = value.trim();
  if (trimmed === "" || !EMAIL_PATTERN.test(trimmed)) {
    return "Bitte eine gültige E-Mail-Adresse angeben.";
  }
  return undefined;
}

function passwordErrorFor(value: string): string | undefined {
  return value === "" ? "Bitte das Passwort angeben." : undefined;
}

export default function WorkshopLogin() {
  const { login } = useSession();
  const navigate = useNavigate();
  const location = useLocation();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [touched, setTouched] = useState({ email: false, password: false });
  const [submitted, setSubmitted] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [alert, setAlert] = useState<FormAlert | null>(null);

  const emailError =
    submitted || touched.email ? emailErrorFor(email) : undefined;
  const passwordError =
    submitted || touched.password ? passwordErrorFor(password) : undefined;

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitted(true);
    setAlert(null);

    if (emailErrorFor(email) || passwordErrorFor(password)) {
      return;
    }

    setSubmitting(true);
    try {
      await login(email.trim(), password);
      const state = location.state as { from?: string } | null;
      navigate(state?.from ?? "/werkstatt/auftraege", { replace: true });
    } catch (error) {
      if (error instanceof ApiError && error.code === "rate_limited") {
        setAlert({ tone: "warning", message: RATE_LIMIT_MESSAGE });
      } else {
        setAlert({
          tone: "danger",
          message:
            error instanceof Error && error.message
              ? error.message
              : "Die Anmeldung ist fehlgeschlagen.",
        });
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="container container--login">
      <section className="card login-card">
        <h1 className="card-title login-card__title">Werkstattbereich</h1>

        <form className="login-form" onSubmit={handleSubmit} noValidate>
          <div className="field">
            <label htmlFor="login-email">
              E-Mail
              <span className="required-marker">erforderlich</span>
            </label>
            <input
              id="login-email"
              name="email"
              type="email"
              inputMode="email"
              autoComplete="email"
              value={email}
              aria-invalid={emailError ? true : undefined}
              aria-describedby={emailError ? "login-email-error" : undefined}
              onChange={(event) => {
                setEmail(event.target.value);
                setAlert(null);
              }}
              onBlur={() => setTouched((state) => ({ ...state, email: true }))}
            />
            {emailError && (
              <p className="field-error" id="login-email-error">
                {emailError}
              </p>
            )}
          </div>

          <div className="field">
            <label htmlFor="login-password">
              Passwort
              <span className="required-marker">erforderlich</span>
            </label>
            <input
              id="login-password"
              name="password"
              type="password"
              autoComplete="current-password"
              value={password}
              aria-invalid={passwordError ? true : undefined}
              aria-describedby={passwordError ? "login-password-error" : undefined}
              onChange={(event) => {
                setPassword(event.target.value);
                setAlert(null);
              }}
              onBlur={() => setTouched((state) => ({ ...state, password: true }))}
            />
            {passwordError && (
              <p className="field-error" id="login-password-error">
                {passwordError}
              </p>
            )}
          </div>

          {alert && (
            <div
              className={`alert alert--${alert.tone}`}
              role="alert"
              data-testid="login-alert"
            >
              {alert.message}
            </div>
          )}

          <button
            type="submit"
            className="btn btn-primary btn-block"
            disabled={submitting}
            aria-busy={submitting}
          >
            {submitting ? (
              <span className="loading-dots" aria-hidden="true">
                <i />
                <i />
                <i />
              </span>
            ) : (
              <span>Anmelden</span>
            )}
          </button>
        </form>
      </section>
    </div>
  );
}
