FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-w -s -X github.com/unkmonster/tmd/internal/api.Version=$VERSION" -o /out/tmd .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

COPY --from=build /out/tmd /usr/local/bin/tmd

ENV TMD_HOME=/config \
    TMD_ROOT_PATH=/data \
    TMD_PORT=25556

VOLUME ["/config", "/data"]
EXPOSE 25556

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
  CMD wget -qO- "http://127.0.0.1:${TMD_PORT}/api/v1/health" >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/tmd"]
CMD ["-server"]
