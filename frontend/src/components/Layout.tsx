import type { ReactNode } from "react";
import { NavLink, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";

const ROLE_LABELS: Record<string, string> = {
  admin: "مدير",
  operator: "مشغّل",
  viewer: "مراقب",
};

export function Layout({ children }: { children: ReactNode }) {
  const { username, role, logout } = useAuth();
  const navigate = useNavigate();

  function handleLogout() {
    logout();
    navigate("/login", { replace: true });
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-brand">SSH Manager Panel</div>
        <nav className="sidebar-nav">
          <NavLink to="/" end className={({ isActive }) => (isActive ? "active" : "")}>
            لوحة التحكم
          </NavLink>
          <NavLink to="/accounts" className={({ isActive }) => (isActive ? "active" : "")}>
            حسابات SSH
          </NavLink>
          <NavLink to="/certificates" className={({ isActive }) => (isActive ? "active" : "")}>
            الشهادات والنطاق
          </NavLink>
          <NavLink to="/settings" className={({ isActive }) => (isActive ? "active" : "")}>
            الإعدادات
          </NavLink>
        </nav>
        <div className="sidebar-footer">
          <div className="sidebar-user">
            <div className="sidebar-user-name">{username}</div>
            <div className="sidebar-user-role">{role ? ROLE_LABELS[role] ?? role : ""}</div>
          </div>
          <button type="button" className="btn btn-ghost btn-sm" onClick={handleLogout}>
            تسجيل الخروج
          </button>
        </div>
      </aside>
      <main className="main-content">{children}</main>
    </div>
  );
}
