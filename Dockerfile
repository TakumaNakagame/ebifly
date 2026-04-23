# syntax=docker/dockerfile:1.7

# --- Stage 1: build frontend ---
FROM node:25-alpine AS frontend
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# --- Stage 2: build Go binary with embedded frontend ---
FROM golang:1.25 AS backend
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Replace placeholder assets with the real built frontend
RUN rm -rf webassets/dist
COPY --from=frontend /app/dist webassets/dist
# CGO is not required — modernc.org/sqlite is pure Go.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server

# --- Stage 3: minimal runtime ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /out/server /app/server
# Persistent state: mount a named volume or host path here.
VOLUME ["/data"]
EXPOSE 8080
ENV ADDR=:8080 \
    DB_PATH=/data/planning-poker.db \
    COOKIE_SECURE=true
ENTRYPOINT ["/app/server"]
