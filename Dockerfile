FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -o weather-exporter .

# ---- runtime ----
FROM alpine:3.24

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /app/weather-exporter .

# Numeric, not the name - the kubelet cannot verify runAsNonRoot for a user it
# cannot resolve, and fails the container closed. Same uid the account already has.
USER 100

EXPOSE 8080

ENTRYPOINT ["./weather-exporter"]
