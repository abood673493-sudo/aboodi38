// Thin typed wrapper around the backend's JSON API. Every function here
// corresponds 1:1 to a route wired up in backend/cmd/server/main.go - there
// is no endpoint called from the UI that doesn't exist on the backend, and
// no endpoint on the backend that performs a raw/arbitrary command: every
// one of these maps to a specific, validated operation.
import type {
  Certificate,
  CreateAccountRequest,
  LoginResponse,
  ServerStatus,
  SSHAccount,
  UpdateAccountRequest,
  UsageRow,
} from "../types";

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? "";

const ACCESS_TOKEN_KEY = "sshpanel_access_token";
const REFRESH_TOKEN_KEY = "sshpanel_refresh_token";
const USERNAME_KEY = "sshpanel_username";
const ROLE_KEY = "sshpanel_role";

export function getStoredSession() {
  return {
    accessToken: localStorage.getItem(ACCESS_TOKEN_KEY),
    refreshToken: localStorage.getItem(REFRESH_TOKEN_KEY),
    username: localStorage.getItem(USERNAME_KEY),
    role: localStorage.getItem(ROLE_KEY),
  };
}

function storeSession(res: LoginResponse) {
  localStorage.setItem(ACCESS_TOKEN_KEY, res.access_token);
  localStorage.setItem(REFRESH_TOKEN_KEY, res.refresh_token);
  localStorage.setItem(USERNAME_KEY, res.username);
  localStorage.setItem(ROLE_KEY, res.role);
}

export function clearSession() {
  localStorage.removeItem(ACCESS_TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
  localStorage.removeItem(USERNAME_KEY);
  localStorage.removeItem(ROLE_KEY);
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

/**
 * request() is the single place that attaches the bearer token, retries
 * exactly once after a silent token refresh on 401, and turns a non-2xx
 * response into a thrown ApiError with the backend's own message.
 */
async function request<T>(
  path: string,
  options: RequestInit = {},
  { skipAuth = false, isRetry = false }: { skipAuth?: boolean; isRetry?: boolean } = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set("Content-Type", "application/json");

  if (!skipAuth) {
    const { accessToken } = getStoredSession();
    if (accessToken) headers.set("Authorization", `Bearer ${accessToken}`);
  }

  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });

  if (res.status === 401 && !skipAuth && !isRetry) {
    const refreshed = await tryRefresh();
    if (refreshed) {
      return request<T>(path, options, { skipAuth, isRetry: true });
    }
    clearSession();
    window.location.assign("/login");
    throw new ApiError(401, "الجلسة منتهية، يرجى تسجيل الدخول مرة أخرى");
  }

  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    try {
      const body = (await res.json()) as { error?: string };
      if (body?.error) message = body.error;
    } catch {
      // response had no JSON body - keep the generic status message
    }
    throw new ApiError(res.status, message);
  }

  if (res.status === 204 || res.headers.get("Content-Length") === "0") {
    return undefined as T;
  }
  return (await res.json()) as T;
}

async function tryRefresh(): Promise<boolean> {
  const { refreshToken } = getStoredSession();
  if (!refreshToken) return false;
  try {
    const res = await fetch(`${API_BASE}/api/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    if (!res.ok) return false;
    const body = (await res.json()) as { access_token: string };
    localStorage.setItem(ACCESS_TOKEN_KEY, body.access_token);
    return true;
  } catch {
    return false;
  }
}

// --- auth -------------------------------------------------------------

export async function login(username: string, password: string): Promise<LoginResponse> {
  const res = await request<LoginResponse>(
    "/api/auth/login",
    { method: "POST", body: JSON.stringify({ username, password }) },
    { skipAuth: true },
  );
  storeSession(res);
  return res;
}

export function logout() {
  clearSession();
}

// --- dashboard ----------------------------------------------------------

export function getServerStatus(): Promise<ServerStatus> {
  return request<ServerStatus>("/api/dashboard/status");
}

export function getUsage(): Promise<UsageRow[]> {
  return request<UsageRow[]>("/api/dashboard/usage").then((rows) => rows ?? []);
}

// --- SSH accounts ---------------------------------------------------------

export function listAccounts(): Promise<SSHAccount[]> {
  return request<SSHAccount[]>("/api/accounts").then((rows) => rows ?? []);
}

export function createAccount(body: CreateAccountRequest): Promise<SSHAccount> {
  return request<SSHAccount>("/api/accounts", { method: "POST", body: JSON.stringify(body) });
}

export function updateAccount(id: string, body: UpdateAccountRequest): Promise<SSHAccount> {
  return request<SSHAccount>(`/api/accounts/${id}`, { method: "PATCH", body: JSON.stringify(body) });
}

export function disableAccount(id: string): Promise<void> {
  return request<void>(`/api/accounts/${id}/disable`, { method: "POST" });
}

export function enableAccount(id: string): Promise<void> {
  return request<void>(`/api/accounts/${id}/enable`, { method: "POST" });
}

export function deleteAccount(id: string): Promise<void> {
  return request<void>(`/api/accounts/${id}`, { method: "DELETE" });
}

// --- certificates ---------------------------------------------------------

export function listCertificates(): Promise<Certificate[]> {
  return request<Certificate[]>("/api/certificates").then((rows) => rows ?? []);
}

export function issueCertificate(domain: string, email: string): Promise<{ status: string; domain: string }> {
  return request("/api/certificates", { method: "POST", body: JSON.stringify({ domain, email: email || undefined }) });
}
