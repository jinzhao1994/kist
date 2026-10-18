# syntax=docker/dockerfile:1
FROM node:24-bookworm-slim AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26.1-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/web/static/build ./internal/web/static/build
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X github.com/jinzhao1994/kist/internal/buildinfo.version=${VERSION}" \
    -o /out/kist ./cmd/kist

FROM debian:trixie-slim AS runtime
RUN apt-get update \
    && apt-get upgrade -y --no-install-recommends \
    && apt-get install -y --no-install-recommends poppler-utils ffmpeg ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 kist \
    && useradd --uid 10001 --gid kist --no-create-home --home-dir /library kist \
    && mkdir /library && chown kist:kist /library
RUN apt-get update \
    && apt-get install -y --no-install-recommends libheif-examples libreoffice-writer libreoffice-impress \
        fonts-noto-cjk fonts-crosextra-carlito fonts-crosextra-caladea \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/kist /usr/local/bin/kist
COPY LICENSE THIRD_PARTY_NOTICES.md SECURITY.md /usr/share/doc/kist/
ENV TZ=UTC
USER 10001:10001
WORKDIR /library
EXPOSE 8473
ENTRYPOINT ["kist"]
CMD ["serve", "--library", "/library", "--listen", "0.0.0.0:8473", "--allow-origin", "http://127.0.0.1:8473", "--allow-origin", "http://localhost:8473"]
