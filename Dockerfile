FROM --platform=$BUILDPLATFORM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS builder

ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache make git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify
COPY . .

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH make

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
RUN apk add --no-cache git openssh-client ca-certificates
COPY --from=builder /app/minifleet /usr/local/bin/minifleet
ENTRYPOINT ["/usr/local/bin/minifleet"]
