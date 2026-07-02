# Base images are digest-pinned for build-input integrity (OWASP A08). The
# trailing tag comment records the human-readable version; Dependabot bumps the
# digest.
FROM golang:1.26@sha256:f96cc555eb8db430159a3aa6797cd5bae561945b7b0fe7d0e284c63a3b291609 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /out/codeguard ./cmd/codeguard

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

WORKDIR /workspace

RUN apk add --no-cache git \
    && addgroup -S codeguard && adduser -S -G codeguard -H codeguard

COPY --from=build /out/codeguard /usr/local/bin/codeguard

# Run as an unprivileged user; codeguard only needs to read the mounted repo.
USER codeguard

ENTRYPOINT ["codeguard"]
CMD ["help"]
