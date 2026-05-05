FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -o weather-exporter .

# ---- runtime ----
FROM alpine:3.21

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /app/weather-exporter .

USER app

EXPOSE 8080

ENTRYPOINT ["./weather-exporter"]
