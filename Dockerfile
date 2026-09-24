# Base images are digest-pinned for build-input integrity (OWASP A08). The
# trailing tag comment records the human-readable version; Dependabot bumps the
# digest.
FROM golang:1.26@sha256:3aff6657219a4d9c14e27fb1d8976c49c29fddb70ba835014f477e1c70636647 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /out/codeguard ./cmd/codeguard

FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

WORKDIR /workspace

RUN apk add --no-cache git \
    && addgroup -S codeguard && adduser -S -G codeguard -H codeguard

COPY --from=build /out/codeguard /usr/local/bin/codeguard

# Run as an unprivileged user; codeguard only needs to read the mounted repo.
USER codeguard

ENTRYPOINT ["codeguard"]
CMD ["help"]
