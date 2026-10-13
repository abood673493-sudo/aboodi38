# Migrations

Plain SQL, applied in order, no migration framework required:

```bash
psql "$DATABASE_URL" -f 0001_init.sql
```

There is **no seed data** here on purpose — no demo admin, no sample SSH
accounts, no fake usage numbers. The first real administrator is created with
the `bootstrap-admin` CLI (see `backend/cmd/bootstrap-admin`), which prompts
for a real username and password and inserts exactly one row.
