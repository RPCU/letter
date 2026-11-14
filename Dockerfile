# syntax=docker/dockerfile:1
# Same layout as nix/oci.nix: /letter (template, assets and zone database
# are embedded), Claude Code for the AI notes and a shell to sign it in
# (`claude auth login`), user 1001:0 with a writable home, default command
# `serve`.
FROM --platform=$BUILDPLATFORM golang:1.26 AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
WORKDIR /src
COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
      -ldflags "-s -w -X github.com/Banh-Canh/letter/cmd.Version=${VERSION} -X github.com/Banh-Canh/letter/cmd.Commit=${COMMIT}" \
      -o /out/letter .

# Claude Code's native build (glibc only), pinned.
FROM debian:bookworm-slim AS claude
ARG CLAUDE_VERSION=2.1.308
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
    && curl -fsSL https://claude.ai/install.sh | bash -s "${CLAUDE_VERSION}" \
    && cp -L /root/.local/bin/claude /claude

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --uid 1001 --gid 0 --home-dir /home/letter --create-home --shell /bin/bash letter \
    && chmod 0770 /home/letter
COPY --from=build /out/letter /letter
COPY --from=claude /claude /usr/local/bin/claude
# The AI notes' Claude login lives here, apart from any other
# account settings: sign in with `claude auth login` in the container.
ENV HOME=/home/letter \
    CLAUDE_CONFIG_DIR=/home/letter/.claude \
    DISABLE_AUTOUPDATER=1
USER 1001:0
WORKDIR /home/letter
EXPOSE 8080
ENTRYPOINT ["/letter"]
CMD ["serve"]
