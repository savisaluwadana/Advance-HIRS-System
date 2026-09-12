# Advance HRIS Desktop

Native HR/admin console built with Wails, React and Go.

## Security model

- Workforce records are cached in SQLite as AES-256-GCM encrypted JSON payloads.
- The local data-encryption key lives in the operating-system keychain.
- The cloud bearer token also lives in the OS keychain; it is never stored in SQLite or browser local storage.
- The React layer receives only user/session metadata, not the token.
- Local workforce data stays locked from the UI until an online session has been validated.
- Employee edits are written locally first, then queued for tenant-scoped cloud sync.

## Development

Start PostgreSQL and the API from the repository root:

```bash
docker compose up postgres api
```

Then run the desktop app:

```bash
cd apps/desktop
wails dev
```

By default the desktop connects to `http://localhost:8080`. Override it with:

```bash
ADVANCE_HRIS_API_URL=https://your-api.example.com wails dev
```

The local Docker stack bootstraps this development account:

```text
Organization: northstar
Email:        admin@advancehris.local
Password:     local-admin-change-me
```

## Sync behavior

1. HR/admin signs in.
2. API returns a role- and tenant-scoped bearer token.
3. Desktop stores the token in the OS keychain.
4. Edits are committed to encrypted SQLite immediately.
5. Repeated unsynced edits to the same employee are coalesced.
6. The sync engine pushes queued mutations with the bearer token.
7. The API checks organization scope, role and server version before writing PostgreSQL.
8. Cloud pulls never overwrite an employee with a pending local edit.

The next sync milestone is explicit conflict-resolution UX and delta cursors rather than full-snapshot pulls.
