FROM golang:alpine AS builder

RUN apk add --no-cache --update git build-base
ENV CGO_ENABLED=0

WORKDIR /app

COPY . /app

WORKDIR /app/src
ARG BUILD_VERSION=0.0.0-dev
RUN go mod download
RUN go build -trimpath -ldflags="-s -w -X github.com/GMWalletApp/epusdt/config.BuildVersion=${BUILD_VERSION}" -o /app/epusdt .

FROM alpine:latest AS runner
ENV TZ=Asia/Shanghai
RUN apk --no-cache add ca-certificates tzdata
ARG API_RATE_URL=""
ENV EPUSDT_CONFIG=/app/conf/.env
ENV EPUSDT_DOCKER=1

WORKDIR /app
RUN mkdir -p /app/conf /app/runtime/logs
COPY --from=builder /app/src/.env.example /app/.env.example
COPY --from=builder /app/epusdt .

VOLUME ["/app/conf", "/app/runtime"]
ENTRYPOINT ["./epusdt", "http", "start"]
