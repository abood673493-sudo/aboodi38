import { useCallback, useEffect, useState, type FormEvent } from "react";
import * as api from "../api/client";
import type { Certificate } from "../types";
import { CertificateStatusBadge } from "../components/StatusBadge";
import { errorMessage, useToast } from "../context/ToastContext";
import { formatDateTime } from "../utils/format";

const DOMAIN_RE = /^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i;

export function CertificatesPage() {
  const { notify } = useToast();
  const [certs, setCerts] = useState<Certificate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [domain, setDomain] = useState("");
  const [email, setEmail] = useState("");
  const [issuing, setIssuing] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setCerts(await api.listCertificates());
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function handleIssue(e: FormEvent) {
    e.preventDefault();
    setFormError(null);
    if (!DOMAIN_RE.test(domain)) {
      setFormError("أدخل نطاقًا فرعيًا صحيحًا، مثل ssh.example.com");
      return;
    }
    setIssuing(true);
    try {
      await api.issueCertificate(domain, email);
      notify(`تم إصدار الشهادة لنطاق ${domain}`, "success");
      setDomain("");
      setEmail("");
      await load();
    } catch (err) {
      setFormError(errorMessage(err));
    } finally {
      setIssuing(false);
    }
  }

  return (
    <div>
      <div className="page-header">
        <h1>الشهادات والنطاق الفرعي</h1>
        <p className="page-subtitle">
          يجب أن يشير سجل DNS (A/AAAA) للنطاق إلى عنوان هذا السيرفر، وأن يكون المنفذ 80 مفتوحًا، قبل طلب الإصدار - اللوحة
          لا تستطيع تعديل سجلات DNS نيابة عنك.
        </p>
      </div>

      <div className="panel">
        <h2>إصدار شهادة جديدة</h2>
        <form className="inline-form" onSubmit={handleIssue}>
          <label className="field">
            <span>النطاق الفرعي</span>
            <input
              type="text"
              placeholder="ssh.example.com"
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              dir="ltr"
              required
            />
          </label>
          <label className="field">
            <span>البريد الإلكتروني (اختياري، لتنبيهات انتهاء الشهادة)</span>
            <input type="email" placeholder="admin@example.com" value={email} onChange={(e) => setEmail(e.target.value)} dir="ltr" />
          </label>
          <button type="submit" className="btn btn-primary" disabled={issuing}>
            {issuing ? "جارٍ الإصدار..." : "إصدار الشهادة"}
          </button>
        </form>
        {formError && (
          <div className="alert alert-error" role="alert">
            {formError}
          </div>
        )}
      </div>

      <div className="panel">
        <h2>الشهادات الحالية</h2>
        {error && <div className="alert alert-error">{error}</div>}
        {loading ? (
          <div className="page-loading">جارٍ التحميل...</div>
        ) : certs.length === 0 ? (
          <p className="empty-state">لا توجد شهادات بعد.</p>
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>النطاق</th>
                <th>الحالة</th>
                <th>تاريخ الإصدار</th>
                <th>تاريخ الانتهاء</th>
                <th>آخر خطأ</th>
              </tr>
            </thead>
            <tbody>
              {certs.map((c) => (
                <tr key={c.id}>
                  <td dir="ltr">{c.domain}</td>
                  <td>
                    <CertificateStatusBadge status={c.status} />
                  </td>
                  <td>{formatDateTime(c.issued_at)}</td>
                  <td>{formatDateTime(c.expires_at)}</td>
                  <td className="text-danger">{c.last_error || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="panel panel-muted">
        <h2>معلومات وسيط النقل (WebSocket/TLS)</h2>
        <p>
          بعد إصدار الشهادة، فعّل الوسيط المتوافق مع HTTP Injector بضبط <code>BRIDGE_TLS_CERT</code> و
          <code>BRIDGE_TLS_KEY</code> و<code>BRIDGE_DOMAIN</code> في ملف البيئة ثم إعادة تشغيل الخدمة. التفاصيل في ملف
          README الخاص بالنشر.
        </p>
      </div>
    </div>
  );
}
