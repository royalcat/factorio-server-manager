# syntax=docker/dockerfile:1

# ---- Frontend build (output is architecture-independent) ----
FROM --platform=$BUILDPLATFORM node:26.3.1-slim AS frontend
RUN npm install -g pnpm@11.8.0
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN CI=true pnpm install --frozen-lockfile
COPY vite.config.js tailwind.config.js postcss.config.js ./
COPY ui/ ui/
RUN pnpm run build

# ---- Backend build (cross-compiled for the target architecture) ----
FROM --platform=$BUILDPLATFORM golang:1.26 AS backend
ARG TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY src/ ./src
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/factorio-server-manager ./src/main.go

# ---- Runtime ----
# Glibc is required for Factorio Server binaries to run.
FROM debian:forky-slim
ARG TARGETARCH
ENV RCON_PASS="" \
    FSM_ADMIN_USERNAME=admin

VOLUME /opt/fsm-data /opt/factorio

EXPOSE 80/tcp 34197/udp

# tar + xz-utils extract the Factorio archive (tar -xJf); jq is used by the
# entrypoint to seed conf.json; ca-certificates is required for Factorio
# downloads over HTTPS. On non-amd64 hosts, box64 and multiarch amd64 libc are
# installed so the x86_64-only Factorio server binary can run under emulation.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tar xz-utils jq \
    && if [ "$TARGETARCH" != "amd64" ]; then \
         dpkg --add-architecture amd64; \
         apt-get update; \
         apt-get install -y --no-install-recommends box64 libc6:amd64 libstdc++6:amd64 libgcc-s1:amd64; \
       fi \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/fsm

COPY docker/.box64rc /etc/box64.box64rc
COPY conf.json.example /opt/fsm/conf.json
COPY docker/entrypoint.sh /opt/entrypoint.sh

COPY --from=backend /out/factorio-server-manager /opt/fsm/factorio-server-manager
COPY --from=frontend /src/app/ /opt/fsm/app/


ENTRYPOINT ["/opt/entrypoint.sh"]
