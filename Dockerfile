FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine@sha256:3f6d04dc61331ee3c2fbbaad62d54412a84680f6a041d269a20a5270a078515b AS builder

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
