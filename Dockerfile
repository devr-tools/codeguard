FROM golang:1.25 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /out/codeguard ./cmd/codeguard

FROM alpine:3.24

WORKDIR /workspace

RUN apk add --no-cache git

COPY --from=build /out/codeguard /usr/local/bin/codeguard

ENTRYPOINT ["codeguard"]
CMD ["help"]
