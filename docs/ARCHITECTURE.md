# المعمارية

## مخطط عام

```
                         ┌─────────────────────────┐
  المتصفح  ──HTTPS(443)──▶│  nginx                  │
                         │  - يخدم ملفات الواجهة   │
                         │  - يعكس /api/* إلى :8080│
                         └───────────┬─────────────┘
                                     │ HTTP محلي
                                     ▼
                         ┌─────────────────────────┐
                         │  sshpanel-api (Go)       │   يعمل بحساب نظام
                         │  - REST API              │   غير مميز، بلا root
                         │  - وسيط WS/TLS :8443     │   أبدًا
                         └───┬───────────────┬──────┘
                             │ SQL           │ sudo <helper> <فعل محدد> ...
                             ▼               ▼
                     ┌───────────────┐  ┌─────────────────────┐
                     │  PostgreSQL   │  │  sshpanel-helper      │ ◀── الثنائي
                     └───────────────┘  │  (يعمل كـ root فقط    │     الوحيد
                                        │   أثناء هذا الاستدعاء) │     المميّز
                                        └──────────┬───────────┘
                                                   │ useradd / usermod /
                                                   │ chage / chpasswd /
                                                   │ certbot / userdel
                                                   ▼
                                        ┌─────────────────────┐
                                        │  نظام Linux الفعلي    │
                                        │  (حسابات SSH الحقيقية)│
                                        └──────────┬───────────┘
                                                   │
  عميل SSH / HTTP Injector ──TCP:22 أو WSS:8443──▶ sshd المحلي
```

## تدفّق إنشاء حساب SSH (مثال يوضّح كل الطبقات)

1. الواجهة ترسل `POST /api/accounts` مع JWT في ترويسة `Authorization`.
2. `middleware.RequireAuth` يتحقق من JWT، ثم `middleware.RequireRole("admin","operator")`.
3. `handlers.AccountHandlers.Create` يفكّ JSON ويستدعي `AccountService.CreateAccount`.
4. `AccountService` يتحقق من اسم المستخدم (`sshmgr.ValidateUsername`) وقوة
   كلمة المرور (`auth.ValidatePasswordStrength`)، يتأكد من عدم تكرار الاسم،
   **يُدرج سجل DB أولًا**.
5. يستدعي `sshmgr.Manager.CreateAccount` والذي يبني
   `sudo /usr/local/bin/sshpanel-helper create <user> <group> <password>`
   (argv، لا shell).
6. `sshpanel-helper` (يعمل الآن كـ root عبر sudo) يعيد التحقق من كل شيء
   بنفسه، ثم `useradd --shell /usr/sbin/nologin ...` و`chpasswd` عبر stdin.
7. فشل الخطوة 6 ↦ `AccountService` يُرجع السجل في DB إلى `deleted` (لا يُترك
   سجل "نشط" يشير إلى مستخدم Linux غير موجود فعليًا).
8. نجاح ↦ `LookupUID` يقرأ الـ UID الحقيقي ويُخزَّن في DB (يُستخدم لاحقًا
   لربط عدّادات استهلاك البيانات الحقيقية بهذا الحساب).
9. سجل تدقيق يُضاف في `audit_log`.

## من أين تأتي كل بيانات اللوحة

| البيانات المعروضة | المصدر الفعلي |
|---|---|
| حالة السيرفر (حمل، ذاكرة، تخزين، uptime) | `/proc/loadavg`, `/proc/meminfo`, `statfs(2)`, `/proc/uptime` |
| الاتصالات النشطة لكل حساب | تحليل مخرجات `ps -eo user=,args=` لعمليات `sshd:` بعد المصادقة |
| استهلاك البيانات لكل حساب | عدّادات `systemd` الحقيقية (`IPAccounting=yes` على user slice)، يُسجَّل الفرق بين قياسين دوريين |
| حالة الحسابات/الشهادات | PostgreSQL، تُحدَّث بواسطة نفس العمليات التي تنفّذ التغيير الفعلي على النظام |

لا مصدر بيانات هنا "مُحاكى" - إن تعذّر الحصول على قيمة حقيقية (مثلاً
IPAccounting غير مفعّل)، تُعرض القيمة كـ "لا توجد بيانات" صراحة.

## لماذا وسيط WebSocket/TLS منفصل عن nginx

الوسيط (`internal/bridge`) ينهي TLS بنفسه على منفذ مستقل (افتراضي 8443)
بدل المرور عبر nginx، حتى يمكن توجيه عملاء مثل HTTP Injector إلى منفذ
مخصص بشكل مستقل عن واجهة الويب، ويستخدم **نفس شهادة** النطاق (المنسوخة إلى
`/etc/sshpanel/certs/`). هذا خيار معماري وليس ضرورة - يمكن بدلاً منه تمرير
WebSocket عبر nginx على نفس المنفذ 443 إن أردت توحيد المنفذ؛ هذا يتطلب تعديل
بسيطًا في nginx (`proxy_pass` مع ترويسات `Upgrade`/`Connection`) وتشغيل
الوسيط بدون TLS خاص به محليًا.
