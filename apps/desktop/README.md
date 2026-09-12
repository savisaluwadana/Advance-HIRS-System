# Advance HRIS Desktop

Native HR/admin console for Advance HRIS, built with Wails v2 + React + Go.

## Local-first guarantees

- Employee payloads are encrypted with AES-256-GCM before they are stored in SQLite.
- The encryption key is generated on first launch and stored in the operating-system keyring.
- Local edits commit to SQLite first and are placed in a durable sync queue.
- The desktop stays usable when the cloud API cannot be reached.
- Sync pushes pending operations, then refreshes authoritative cloud records.

## Run

Install the Wails CLI:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
```

Then:

```bash
cd apps/desktop
wails doctor
wails dev
```

The desktop expects the cloud API at `http://localhost:8080`. Override it with:

```bash
ADVANCE_HRIS_API_URL=https://your-api.example.com wails dev
```

## Current sync contract

- `POST /api/v1/sync/push` — push queued desktop mutations
- `GET /api/v1/sync/pull?since=<cursor>` — pull cloud state

The first implementation uses snapshot pulls and server-side version counters. The next sync milestone adds per-entity delta cursors and explicit conflict records for concurrent edits.
