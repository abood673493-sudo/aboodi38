import { useEffect, useState } from "react";
import * as api from "../api/client";
import type { ServerStatus, UsageRow } from "../types";
import { formatBytes, formatUptime, formatDateTime } from "../utils/format";
import { errorMessage } from "../context/ToastContext";

const POLL_MS = 30_000;

export function DashboardPage() {
  const [status, setStatus] = useState<ServerStatus | null>(null);
  const [usage, setUsage] = useState<UsageRow[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const [s, u] = await Promise.all([api.getServerStatus(), api.getUsage()]);
        if (!cancelled) {
          setStatus(s);
          setUsage(u);
          setError(null);
        }
      } catch (err) {
        if (!cancelled) setError(errorMessage(err));
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    load();
    const interval = window.setInterval(load, POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, []);

  if (loading) return <div className="page-loading">جارٍ التحميل...</div>;
  if (error) return <div className="alert alert-error">{error}</div>;
  if (!status) return null;

  const memPct = status.mem_total_bytes > 0 ? Math.round((status.mem_used_bytes / status.mem_total_bytes) * 100) : 0;
  const diskPct = status.disk_total_bytes > 0 ? Math.round((status.disk_used_bytes / status.disk_total_bytes) * 100) : 0;

  return (
    <div>
      <div className="page-header">
        <h1>لوحة التحكم</h1>
        <p className="page-subtitle">{status.hostname} · تحديث تلقائي كل 30 ثانية</p>
      </div>

      <div className="stat-grid">
        <StatCard label="وقت التشغيل" value={formatUptime(status.uptime_seconds)} />
        <StatCard label="معدل الحمل (1 دقيقة)" value={status.load_avg_1.toFixed(2)} hint={`5د: ${status.load_avg_5.toFixed(2)} · 15د: ${status.load_avg_15.toFixed(2)}`} />
        <StatCard label="الذاكرة" value={`${memPct}%`} hint={`${formatBytes(status.mem_used_bytes)} / ${formatBytes(status.mem_total_bytes)}`} />
        <StatCard label="التخزين" value={`${diskPct}%`} hint={`${formatBytes(status.disk_used_bytes)} / ${formatBytes(status.disk_total_bytes)}`} />
        <StatCard label="إجمالي الحسابات" value={String(status.total_accounts)} hint={`${status.active_accounts} نشط`} />
        <StatCard label="الاتصالات النشطة الآن" value={String(status.active_sessions)} />
      </div>

      <div className="panel">
        <h2>استهلاك البيانات لكل حساب</h2>
        <p className="panel-note">
          الأرقام مأخوذة من عدّادات حقيقية على السيرفر (systemd IPAccounting). الحساب الذي لا يملك قياسين متتاليين بعد
          يظهر بدون بيانات، وليس صفرًا.
        </p>
        {usage.length === 0 ? (
          <p className="empty-state">لا توجد بيانات استهلاك بعد.</p>
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>الحساب</th>
                <th>تنزيل</th>
                <th>رفع</th>
                <th>آخر قياس</th>
              </tr>
            </thead>
            <tbody>
              {usage.map((row) => (
                <tr key={row.account_id}>
                  <td>{row.username}</td>
                  <td>{formatBytes(row.bytes_in_delta)}</td>
                  <td>{formatBytes(row.bytes_out_delta)}</td>
                  <td>{formatDateTime(row.sampled_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function StatCard({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="stat-card">
      <div className="stat-label">{label}</div>
      <div className="stat-value">{value}</div>
      {hint && <div className="stat-hint">{hint}</div>}
    </div>
  );
}
