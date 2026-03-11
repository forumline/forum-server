# ---- Go build stage ----
FROM golang:1.26-alpine AS go-build

WORKDIR /app
COPY go.mod go.sum ./
COPY forum/ forum/
COPY main.go .

# In the monorepo, go.mod uses a replace directive and placeholder version for
# shared-go. The split repo workflow rewrites these before publish. Strip the
# replace and resolve to the latest published tag so the build works standalone.
ARG SHARED_GO_VERSION=v0.1.0
RUN sed -i '/^replace /d' go.mod && \
    sed -i "s|github.com/forumline/forumline/shared-go v0.0.0-00010101000000-000000000000|github.com/forumline/forumline/shared-go ${SHARED_GO_VERSION}|" go.mod && \
    go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/forum .

# ---- Frontend build stage ----
FROM node:20-slim AS frontend-build

RUN corepack enable && corepack prepare pnpm@10.6.5 --activate

WORKDIR /app

COPY package.json pnpm-lock.yaml* ./
RUN pnpm install --frozen-lockfile || pnpm install

COPY index.html vite.config.js eslint.config.mjs ./
COPY src/ src/
COPY public/ public/
RUN pnpm run build

# ---- Production stage ----
FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=go-build /app/bin/forum .
COPY --from=frontend-build /app/dist ./dist/

ENV PORT=3000
EXPOSE 3000

CMD ["./forum"]
