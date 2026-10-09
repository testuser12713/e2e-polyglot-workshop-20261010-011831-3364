const DEFAULT_BASE_URL = "http://localhost:8080";

export interface ApiErrorBody {
  error: {
    code: string;
    message: string;
  };
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

export interface ApiFetchOptions {
  method?: string;
  body?: unknown;
  token?: string | null;
  signal?: AbortSignal;
}

/**
 * Base URL of the workshop API.
 *
 * In a running product VITE_API_BASE_URL is injected by the run contract from
 * the api service origin (${service:api.origin}). The documented local fallback
 * only applies while the api service is not declared yet, so a developer can
 * start the web app on its own without a build-time variable.
 */
export function apiBaseUrl(): string {
  const configured = import.meta.env.VITE_API_BASE_URL as string | undefined;
  const base = configured && configured.length > 0 ? configured : DEFAULT_BASE_URL;
  return base.replace(/\/+$/, "");
}

/**
 * Single entry point through which the web app talks to the API.
 *
 * Attaches the bearer token, parses the contract error body
 * {"error":{"code":...,"message":...}} and rejects with an ApiError.
 */
export async function apiFetch<T>(path: string, options: ApiFetchOptions = {}): Promise<T> {
  const { method = "GET", body, token, signal } = options;

  const headers: Record<string, string> = { Accept: "application/json" };
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  let response: Response;
  try {
    response = await fetch(`${apiBaseUrl()}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal,
    });
  } catch {
    throw new ApiError(0, "network_error", "Die Werkstatt-API ist nicht erreichbar.");
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  let payload: unknown = null;
  if (text.length > 0) {
    try {
      payload = JSON.parse(text);
    } catch {
      payload = null;
    }
  }

  if (!response.ok) {
    const error = (payload as ApiErrorBody | null)?.error;
    throw new ApiError(
      response.status,
      error?.code ?? "internal_error",
      error?.message ?? "Unerwarteter Fehler der Werkstatt-API.",
    );
  }

  return payload as T;
}
