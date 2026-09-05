ARG DOCKER_HUB_MIRROR=dockerproxy.net
FROM ${DOCKER_HUB_MIRROR}/library/golang:1.26-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct \
    GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o admin-server ./services/admin/cmd

FROM ${DOCKER_HUB_MIRROR}/library/alpine:3.18
RUN apk --no-cache add tzdata ca-certificates su-exec && \
	cp /usr/share/zoneinfo/UTC /etc/localtime && \
	echo "UTC" > /etc/timezone

ENV TZ=UTC

WORKDIR /app
COPY --from=builder /build/admin-server /app/admin-server
RUN adduser -S -D -H -h /app imuser && \
    chown -R imuser:nobody /app && \
    chmod +x /app/admin-server

USER imuser
EXPOSE 8081
ENTRYPOINT ["/app/admin-server"]
