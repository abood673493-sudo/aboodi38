import type { ReactNode } from "react";
import { Navigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import type { AdminRole } from "../types";

/**
 * Client-side gating only - purely for a clean UI (hide/redirect away from
 * pages a role can't use). The backend's middleware.RequireRole is the real
 * enforcement; every mutating request is re-checked there regardless of
 * what this component allowed the user to click.
 */
export function ProtectedRoute({ children, requireRole }: { children: ReactNode; requireRole?: AdminRole[] }) {
  const { isAuthenticated, hasRole } = useAuth();

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }
  if (requireRole && !hasRole(...requireRole)) {
    return <Navigate to="/" replace />;
  }
  return <>{children}</>;
}
