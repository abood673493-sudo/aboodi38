import { useAuth } from "../context/AuthContext";

const ROLE_LABELS: Record<string, string> = {
  admin: "مدير (كل الصلاحيات)",
  operator: "مشغّل (إنشاء/تعديل/تعطيل الحسابات، بدون حذف أو شهادات)",
  viewer: "مراقب (عرض فقط)",
};

export function SettingsPage() {
  const { username, role } = useAuth();

  return (
    <div>
      <div className="page-header">
        <h1>الإعدادات</h1>
      </div>

      <div className="panel">
        <h2>الحساب الحالي</h2>
        <dl className="kv-list">
          <div>
            <dt>اسم المستخدم</dt>
            <dd dir="ltr">{username}</dd>
          </div>
          <div>
            <dt>الصلاحية</dt>
            <dd>{role ? ROLE_LABELS[role] ?? role : "—"}</dd>
          </div>
        </dl>
      </div>

      <div className="panel panel-muted">
        <h2>إدارة المديرين</h2>
        <p>
          لا توجد واجهة ويب لإنشاء مديرين جدد أو تغيير الأدوار عمدًا - وهذا إجراء حساس يُنفَّذ فقط من سطر الأوامر على
          السيرفر نفسه عبر الأداة <code>bootstrap-admin</code>، بحيث لا يمكن لأي ثغرة في واجهة الويب أن تُستخدم لإنشاء
          مدير جديد.
        </p>
      </div>

      <div className="panel panel-muted">
        <h2>الأمان</h2>
        <ul className="plain-list">
          <li>يتم قفل الحساب تلقائيًا بعد عدة محاولات دخول فاشلة متتالية.</li>
          <li>كل عملية حساسة (إنشاء/حذف/تعطيل حساب، إصدار شهادة) تُسجَّل في سجل تدقيق على السيرفر.</li>
          <li>لا تُنفَّذ أي أوامر نظام حرة من واجهة الويب - فقط عمليات محددة ومُتحقق منها مسبقًا.</li>
        </ul>
      </div>
    </div>
  );
}
