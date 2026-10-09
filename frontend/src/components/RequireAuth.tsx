import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { useSession } from "../auth/session";

export function RequireAuth({ children }: { children: ReactNode }) {
  const { token } = useSession();
  const location = useLocation();

  if (!token) {
    return (
      <Navigate
        to="/werkstatt/anmeldung"
        replace
        state={{ from: location.pathname }}
      />
    );
  }

  return <>{children}</>;
}
