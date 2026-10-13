import { useState, type FormEvent } from "react";
import * as api from "../api/client";
import type { SSHAccount } from "../types";
import { passwordStrengthError, usernameError } from "../utils/password";
import { errorMessage, useToast } from "../context/ToastContext";

interface Props {
  account?: SSHAccount; // present => edit mode, absent => create mode
  onClose: () => void;
  onSaved: () => void;
}

/** Converts a <input type="date"> value (or empty) to an RFC3339 UTC timestamp, or null to clear. */
function dateInputToISO(value: string): string | null {
  if (!value) return null;
  return new Date(`${value}T00:00:00Z`).toISOString();
}

function isoToDateInput(iso: string | undefined): string {
  if (!iso) return "";
  return iso.slice(0, 10);
}

export function AccountFormModal({ account, onClose, onSaved }: Props) {
  const { notify } = useToast();
  const isEdit = Boolean(account);

  const [username, setUsername] = useState(account?.username ?? "");
  const [password, setPassword] = useState("");
  const [expiresAt, setExpiresAt] = useState(isoToDateInput(account?.expires_at));
  const [maxConnections, setMaxConnections] = useState(account?.max_connections ?? 2);
  const [note, setNote] = useState(account?.note ?? "");
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setFieldError(null);

    if (!isEdit) {
      const uErr = usernameError(username);
      if (uErr) return setFieldError(uErr);
      const pErr = passwordStrengthError(password);
      if (pErr) return setFieldError(pErr);
    } else if (password) {
      const pErr = passwordStrengthError(password);
      if (pErr) return setFieldError(pErr);
    } else if (username !== account!.username) {
      const uErr = usernameError(username);
      if (uErr) return setFieldError(uErr);
    }

    setSubmitting(true);
    try {
      if (isEdit) {
        await api.updateAccount(account!.id, {
          new_username: username !== account!.username ? username : undefined,
          new_password: password || undefined,
          expires_at: dateInputToISO(expiresAt),
          max_connections: maxConnections,
          note,
        });
        notify("تم تحديث الحساب بنجاح", "success");
      } else {
        await api.createAccount({
          username,
          password,
          expires_at: dateInputToISO(expiresAt),
          max_connections: maxConnections,
          note,
        });
        notify("تم إنشاء الحساب بنجاح", "success");
      }
      onSaved();
    } catch (err) {
      setFieldError(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="modal-backdrop" role="presentation" onClick={onClose}>
      <form className="modal modal-form" role="dialog" aria-modal="true" onClick={(e) => e.stopPropagation()} onSubmit={handleSubmit}>
        <h3>{isEdit ? `تعديل الحساب: ${account!.username}` : "إنشاء حساب SSH جديد"}</h3>

        <label className="field">
          <span>اسم المستخدم</span>
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="مثال: user01"
            required
            dir="ltr"
          />
        </label>

        <label className="field">
          <span>{isEdit ? "كلمة مرور جديدة (اتركها فارغة لعدم التغيير)" : "كلمة المرور"}</span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required={!isEdit}
            dir="ltr"
          />
        </label>

        <div className="field-row">
          <label className="field">
            <span>تاريخ انتهاء الحساب (اختياري)</span>
            <input type="date" value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} />
          </label>
          <label className="field">
            <span>الحد الأقصى للاتصالات المتزامنة</span>
            <input
              type="number"
              min={1}
              max={50}
              value={maxConnections}
              onChange={(e) => setMaxConnections(Number(e.target.value))}
            />
          </label>
        </div>

        <label className="field">
          <span>ملاحظة (اختياري)</span>
          <input type="text" value={note} onChange={(e) => setNote(e.target.value)} />
        </label>

        {fieldError && (
          <div className="alert alert-error" role="alert">
            {fieldError}
          </div>
        )}

        <div className="modal-actions">
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            إلغاء
          </button>
          <button type="submit" className="btn btn-primary" disabled={submitting}>
            {submitting ? "جارٍ الحفظ..." : isEdit ? "حفظ التغييرات" : "إنشاء الحساب"}
          </button>
        </div>
      </form>
    </div>
  );
}
