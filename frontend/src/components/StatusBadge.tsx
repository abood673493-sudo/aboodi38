import type { AccountStatus, CertificateStatus } from "../types";

const ACCOUNT_LABELS: Record<AccountStatus, string> = {
  active: "نشط",
  disabled: "معطّل",
  expired: "منتهي",
  deleted: "محذوف",
};

const CERT_LABELS: Record<CertificateStatus, string> = {
  pending: "قيد الإصدار",
  active: "فعّالة",
  expiring: "قاربت على الانتهاء",
  expired: "منتهية",
  failed: "فشلت",
};

export function AccountStatusBadge({ status }: { status: AccountStatus }) {
  return <span className={`badge badge-${status}`}>{ACCOUNT_LABELS[status] ?? status}</span>;
}

export function CertificateStatusBadge({ status }: { status: CertificateStatus }) {
  return <span className={`badge badge-cert-${status}`}>{CERT_LABELS[status] ?? status}</span>;
}
