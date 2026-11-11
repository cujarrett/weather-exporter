FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -o weather-exporter .

# ---- runtime ----
FROM ghcr.io/cujarrett/base-go:static-debian13@sha256:d1f5d2a974de81dffc620668df57baf021f1dab8d8b90968428dd00d20a3a090

WORKDIR /app

COPY --from=builder /app/weather-exporter .

EXPOSE 8080

ENTRYPOINT ["./weather-exporter"]
