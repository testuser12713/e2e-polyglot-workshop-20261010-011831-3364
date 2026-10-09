import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";

export interface Employee {
  id: number;
  name: string;
  email: string;
}

export interface Session {
  employee: Employee | null;
  token: string | null;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
}

const STORAGE_KEY = "werkstatt.session";

interface StoredSession {
  employee: Employee | null;
  token: string | null;
}

function readStoredSession(): StoredSession {
  if (typeof window === "undefined") {
    return { employee: null, token: null };
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return { employee: null, token: null };
    }
    const parsed = JSON.parse(raw) as Partial<StoredSession>;
    return {
      employee: parsed.employee ?? null,
      token: typeof parsed.token === "string" ? parsed.token : null,
    };
  } catch {
    return { employee: null, token: null };
  }
}

function writeStoredSession(session: StoredSession): void {
  if (typeof window === "undefined") {
    return;
  }
  try {
    if (session.token || session.employee) {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(session));
    } else {
      window.localStorage.removeItem(STORAGE_KEY);
    }
  } catch {
    // A full or unavailable storage must never break the running app.
  }
}

const SessionContext = createContext<Session | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [initial] = useState(readStoredSession);
  const [employee, setEmployee] = useState<Employee | null>(initial.employee);
  const [token, setToken] = useState<string | null>(initial.token);

  const login = useCallback(
    async (_email: string, _password: string): Promise<void> => {
      throw new Error("Die Anmeldung ist noch nicht implementiert.");
    },
    [],
  );

  const logout = useCallback((): void => {
    setEmployee(null);
    setToken(null);
    writeStoredSession({ employee: null, token: null });
  }, []);

  const value = useMemo<Session>(
    () => ({ employee, token, login, logout }),
    [employee, token, login, logout],
  );

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession(): Session {
  const context = useContext(SessionContext);
  if (!context) {
    throw new Error("useSession must be used inside an AuthProvider");
  }
  return context;
}
