// Mirrors backend/internal/models/models.go field-for-field (JSON tags).
// Keep these two in sync by hand - there is no shared schema generator.

export type AdminRole = "admin" | "operator" | "viewer";

export type AccountStatus = "active" | "disabled" | "expired" | "deleted";

export interface SSHAccount {
  id: string;
  username: string;
  linux_uid?: number;
  status: AccountStatus;
  expires_at?: string; // RFC3339
  max_connections: number;
  note?: string;
  created_by?: string;
  created_at: string;
  updated_at: string;
  active_connections: number;
  bytes_in_recent: number;
  bytes_out_recent: number;
}

export interface CreateAccountRequest {
  username: string;
  password: string;
  expires_at?: string | null;
  max_connections: number;
  note?: string;
}

export interface UpdateAccountRequest {
  new_username?: string;
  new_password?: string;
  expires_at?: string | null;
  max_connections?: number;
  note?: string;
}

export interface ServerStatus {
  hostname: string;
  uptime_seconds: number;
  load_avg_1: number;
  load_avg_5: number;
  load_avg_15: number;
  mem_total_bytes: number;
  mem_used_bytes: number;
  disk_total_bytes: number;
  disk_used_bytes: number;
  total_accounts: number;
  active_accounts: number;
  active_sessions: number;
}

export interface UsageRow {
  account_id: string;
  username: string;
  bytes_in_delta: number | null;
  bytes_out_delta: number | null;
  sampled_at?: string;
}

export type CertificateStatus = "pending" | "active" | "expiring" | "expired" | "failed";

export interface Certificate {
  id: string;
  domain: string;
  provider: string;
  status: CertificateStatus;
  issued_at?: string;
  expires_at?: string;
  last_error?: string;
}

export interface LoginResponse {
  access_token: string;
  refresh_token: string;
  username: string;
  role: AdminRole;
}

export interface ApiErrorBody {
  error: string;
}
