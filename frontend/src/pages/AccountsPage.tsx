import { useCallback, useEffect, useState } from "react";
import * as api from "../api/client";
import type { SSHAccount } from "../types";
import { AccountStatusBadge } from "../components/StatusBadge";
import { AccountFormModal } from "../components/AccountFormModal";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { useAuth } from "../context/AuthContext";
import { errorMessage, useToast } from "../context/ToastContext";
import { daysUntil, formatDateTime } from "../utils/format";

export function AccountsPage() {
  const { hasRole } = useAuth();
  const { notify } = useToast();
  const canManage = hasRole("admin", "operator");
  const canDelete = hasRole("admin");

  const [accounts, setAccounts] = useState<SSHAccount[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState<SSHAccount | null>(null);
  const [creating, setCreating] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<SSHAccount | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const rows = await api.listAccounts();
      setAccounts(rows.sort((a, b) => b.created_at.localeCompare(a.created_at)));
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    const interval = window.setInterval(load, 30_000);
    return () => window.clearInterval(interval);
  }, [load]);

  async function handleToggle(account: SSHAccount) {
    setBusyId(account.id);
    try {
      if (account.status === "active") {
        await api.disableAccount(account.id);
        notify(`تم تعطيل ${account.username}`, "success");
      } else {
        await api.enableAccount(account.id);
        notify(`تم تفعيل ${account.username}`, "success");
      }
      await load();
    } catch (err) {
      notify(errorMessage(err), "error");
    } finally {
      setBusyId(null);
    }
  }

  async function handleDelete() {
    if (!pendingDelete) return;
    setBusyId(pendingDelete.id);
    try {
      await api.deleteAccount(pendingDelete.id);
      notify(`تم حذف ${pendingDelete.username} نهائيًا`, "success");
      setPendingDelete(null);
      await load();
    } catch (err) {
      notify(errorMessage(err), "error");
    } finally {
      setBusyId(null);
    }
  }

  return (
    <div>
      <div className="page-header page-header-row">
        <div>
          <h1>حسابات SSH</h1>
          <p className="page-subtitle">إنشاء وإدارة حسابات الأنفاق على السيرفر</p>
        </div>
        {canManage && (
          <button type="button" className="btn btn-primary" onClick={() => setCreating(true)}>
            + حساب جديد
          </button>
        )}
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {loading ? (
        <div className="page-loading">جارٍ التحميل...</div>
      ) : accounts.length === 0 ? (
        <p className="empty-state">لا توجد حسابات بعد.</p>
      ) : (
        <table className="data-table">
          <thead>
            <tr>
              <th>اسم المستخدم</th>
              <th>الحالة</th>
              <th>الاتصالات النشطة</th>
              <th>الحد الأقصى</th>
              <th>تاريخ الانتهاء</th>
              <th>ملاحظة</th>
              {canManage && <th>إجراءات</th>}
            </tr>
          </thead>
          <tbody>
            {accounts.map((a) => {
              const expiryDays = daysUntil(a.expires_at);
              const expiringSoon = expiryDays !== null && expiryDays <= 3 && expiryDays >= 0 && a.status === "active";
              return (
                <tr key={a.id}>
                  <td dir="ltr">{a.username}</td>
                  <td>
                    <AccountStatusBadge status={a.status} />
                  </td>
                  <td>{a.active_connections}</td>
                  <td>{a.max_connections}</td>
                  <td className={expiringSoon ? "text-warning" : undefined}>
                    {formatDateTime(a.expires_at)}
                    {expiringSoon && ` (بعد ${expiryDays} يوم)`}
                  </td>
                  <td>{a.note || "—"}</td>
                  {canManage && (
                    <td className="actions-cell">
                      <button type="button" className="btn btn-sm btn-ghost" onClick={() => setEditing(a)} disabled={busyId === a.id}>
                        تعديل
                      </button>
                      <button
                        type="button"
                        className="btn btn-sm btn-ghost"
                        onClick={() => handleToggle(a)}
                        disabled={busyId === a.id || a.status === "deleted"}
                        title={a.status === "expired" ? "عدّل تاريخ الانتهاء أولاً إن أردت تفعيله مجددًا" : undefined}
                      >
                        {a.status === "active" ? "تعطيل" : "تفعيل"}
                      </button>
                      {canDelete && (
                        <button
                          type="button"
                          className="btn btn-sm btn-danger-ghost"
                          onClick={() => setPendingDelete(a)}
                          disabled={busyId === a.id}
                        >
                          حذف
                        </button>
                      )}
                    </td>
                  )}
                </tr>
              );
            })}
          </tbody>
        </table>
      )}

      {creating && (
        <AccountFormModal
          onClose={() => setCreating(false)}
          onSaved={() => {
            setCreating(false);
            load();
          }}
        />
      )}

      {editing && (
        <AccountFormModal
          account={editing}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            load();
          }}
        />
      )}

      {pendingDelete && (
        <ConfirmDialog
          title="حذف الحساب نهائيًا"
          message={`سيتم حذف حساب "${pendingDelete.username}" وحساب Linux الحقيقي المرتبط به نهائيًا. هذا الإجراء لا يمكن التراجع عنه.`}
          confirmLabel="حذف نهائي"
          danger
          onConfirm={handleDelete}
          onCancel={() => setPendingDelete(null)}
        />
      )}
    </div>
  );
}
