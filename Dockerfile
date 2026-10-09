FROM --platform=$BUILDPLATFORM golang:1.27.2-alpine@sha256:85dc1069ac644ea3c527b177303a406eb3358192816cd7f9e5848eb658851673 AS builder

ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache make git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify
COPY . .

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH make

FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
RUN apk add --no-cache git openssh-client ca-certificates
COPY --from=builder /app/minifleet /usr/local/bin/minifleet
ENTRYPOINT ["/usr/local/bin/minifleet"]
