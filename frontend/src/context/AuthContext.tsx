import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";
import * as api from "../api/client";
import type { AdminRole } from "../types";

interface AuthState {
  username: string | null;
  role: AdminRole | null;
  isAuthenticated: boolean;
}

interface AuthContextValue extends AuthState {
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
  /** Role hierarchy check - matches backend middleware.RequireRole call sites. */
  hasRole: (...roles: AdminRole[]) => boolean;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const stored = api.getStoredSession();
  const [state, setState] = useState<AuthState>({
    username: stored.username,
    role: (stored.role as AdminRole | null) ?? null,
    isAuthenticated: Boolean(stored.accessToken && stored.username),
  });

  const login = useCallback(async (username: string, password: string) => {
    const res = await api.login(username, password);
    setState({ username: res.username, role: res.role, isAuthenticated: true });
  }, []);

  const logout = useCallback(() => {
    api.logout();
    setState({ username: null, role: null, isAuthenticated: false });
  }, []);

  const hasRole = useCallback((...roles: AdminRole[]) => Boolean(state.role && roles.includes(state.role)), [state.role]);

  const value = useMemo<AuthContextValue>(
    () => ({ ...state, login, logout, hasRole }),
    [state, login, logout, hasRole],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
