// Mirrors backend/internal/auth/password.go's ValidatePasswordStrength
// exactly, so the UI can reject a weak password before a round trip - the
// backend re-checks this itself regardless, this is purely for fast
// feedback, never the actual enforcement.
export const MIN_PASSWORD_LENGTH = 12;

export function passwordStrengthError(pw: string): string | null {
  if (pw.length < MIN_PASSWORD_LENGTH) {
    return `يجب ألا تقل كلمة المرور عن ${MIN_PASSWORD_LENGTH} حرفًا`;
  }
  const hasUpper = /[A-Z]/.test(pw);
  const hasLower = /[a-z]/.test(pw);
  const hasDigit = /[0-9]/.test(pw);
  const hasSymbol = /[^A-Za-z0-9]/.test(pw);
  if (!(hasUpper && hasLower && hasDigit && hasSymbol)) {
    return "يجب أن تحتوي كلمة المرور على حرف كبير وحرف صغير ورقم ورمز خاص";
  }
  return null;
}

// Mirrors backend/internal/sshmgr/validate.go's ValidateUsername.
const USERNAME_RE = /^[a-z_][a-z0-9_-]{2,31}$/;
const RESERVED_USERNAMES = new Set([
  "root", "daemon", "bin", "sys", "sync", "games", "man", "lp", "mail",
  "news", "uucp", "proxy", "www-data", "backup", "list", "irc", "gnats",
  "nobody", "systemd-network", "systemd-resolve", "messagebus", "sshd",
  "postgres", "sshpanel",
]);

export function usernameError(username: string): string | null {
  if (!USERNAME_RE.test(username)) {
    return "اسم المستخدم يجب أن يبدأ بحرف صغير أو شرطة سفلية، ويتكون من 3-32 حرفًا صغيرًا/رقمًا/- فقط";
  }
  if (RESERVED_USERNAMES.has(username)) {
    return "اسم المستخدم محجوز ولا يمكن استخدامه";
  }
  return null;
}
