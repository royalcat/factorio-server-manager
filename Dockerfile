# syntax=docker/dockerfile:1

# ---- Frontend build ----
FROM node:26.3.1-slim AS frontend
RUN npm install -g pnpm@11.8.0
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN CI=true pnpm install --frozen-lockfile
COPY vite.config.js tailwind.config.js postcss.config.js ./
COPY ui/ ui/
RUN pnpm run build

# ---- Backend build ----
FROM golang:1.26.4 AS backend
WORKDIR /src
COPY src/go.mod src/go.sum ./
RUN go mod download
COPY src/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/factorio-server-manager .

# ---- Runtime ----
# Glibc is required for Factorio Server binaries to run.
FROM debian:bookworm-slim
ENV RCON_PASS="" \
    FSM_ADMIN_USERNAME=admin

VOLUME /opt/fsm-data /opt/factorio

EXPOSE 80/tcp 34197/udp

# tar + xz-utils extract the Factorio archive (tar -xJf); jq is used by the
# entrypoint to seed conf.json; ca-certificates is required for Factorio
# downloads over HTTPS.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tar xz-utils jq \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/fsm

COPY --from=backend /out/factorio-server-manager /opt/fsm/factorio-server-manager
COPY --from=frontend /src/app/ /opt/fsm/app/
COPY conf.json.example /opt/fsm/conf.json
COPY docker/entrypoint.sh /opt/entrypoint.sh

ENTRYPOINT ["/opt/entrypoint.sh"]
