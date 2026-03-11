# Forumline Forum Server

A self-hosted forum server with real-time chat, voice rooms, and optional [Forumline](https://forumline.net) network integration.

**Features:** threads, posts, categories, real-time chat (SSE), voice rooms (LiveKit), user profiles, notifications, bookmarks, avatar uploads, admin tools.

**Stack:** Go + PostgreSQL + GoTrue auth. Frontend: vanilla JS + Vite + TailwindCSS.

## Quick Start

```bash
git clone https://github.com/forumline/forum-server.git
cd forum-server
cp .env.example .env
# Edit .env with your settings (at minimum: POSTGRES_PASSWORD, JWT_SECRET, FORUM_DOMAIN, SITE_URL)
docker compose up -d
```

Your forum is now running at `http://localhost:3000`.

On first run, the `init-db` container creates the database schema and seed data (3 categories, 2 chat channels, 2 voice rooms). This only runs once — Postgres data is persisted in a Docker volume.

## Configuration

See `.env.example` for all available options. The required settings are:

| Variable | Description |
|----------|-------------|
| `POSTGRES_PASSWORD` | Database password |
| `JWT_SECRET` | JWT signing secret (min 32 characters) |
| `GOTRUE_SERVICE_ROLE_KEY` | GoTrue admin key |
| `FORUM_DOMAIN` | Your forum's domain (e.g. `forum.example.com`) |
| `SITE_URL` | Full URL (e.g. `https://forum.example.com`) |

### Optional integrations

- **Email** — Set `SMTP_*` vars to enable email verification. Without SMTP, accounts are auto-confirmed.
- **Forumline** — Set `FORUMLINE_*` vars to join the [Forumline network](https://forumline.net) for cross-forum identity and federation.
- **Voice rooms** — Set `LIVEKIT_*` vars to enable real-time voice chat via [LiveKit](https://livekit.io).
- **Avatar uploads** — Set `R2_*` vars to enable avatar uploads via Cloudflare R2 (or any S3-compatible storage).

## Development

### Frontend

```bash
pnpm install
pnpm dev        # Vite dev server on http://localhost:5174
```

The dev server proxies `/api` and `/auth` to the Go backend (default `http://localhost:3000`).

### Backend

```bash
# Start Postgres + GoTrue
docker compose up -d postgres gotrue init-db

# Run the Go server
export DATABASE_URL="postgres://postgres:changeme@localhost:5432/postgres?sslmode=disable"
export GOTRUE_URL="http://localhost:9999"
export JWT_SECRET="your-jwt-secret-min-32-characters-long"
go run .
```

## Customizing the Frontend

The frontend is vanilla JavaScript — no framework, no build complexity. Edit files in `src/`:

- `src/pages/` — Page components (threads, posts, profile, settings, etc.)
- `src/components/` — Reusable UI components (sidebar, header, avatar, etc.)
- `src/lib/` — API client, auth, SSE streaming, voice, utilities
- `src/style.css` — TailwindCSS styles

## Architecture

```
main.go          Entry point — wires up DB, SSE hub, config, and serves the SPA
forum/           Go API server (handlers, routes, auth, SSE, voice, avatars)
src/             Frontend source (vanilla JS + Vite + TailwindCSS)
init-db/         PostgreSQL schema (tables, triggers for realtime LISTEN/NOTIFY)
```

The server uses PostgreSQL `LISTEN/NOTIFY` for real-time updates — no Redis or message broker needed. The SSE hub listens for database events and pushes them to connected clients.

## License

MIT
