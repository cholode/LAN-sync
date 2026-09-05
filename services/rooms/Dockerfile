ARG DOCKER_HUB_MIRROR=dockerproxy.net

FROM ${DOCKER_HUB_MIRROR}/library/golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o room-server ./services/rooms/cmd

FROM ${DOCKER_HUB_MIRROR}/library/alpine:3.23
RUN apk --no-cache add tzdata ca-certificates && \
    cp /usr/share/zoneinfo/UTC /etc/localtime && \
    echo "UTC" > /etc/timezone
ENV TZ=UTC
WORKDIR /app
COPY --from=builder /build/room-server /app/room-server
EXPOSE 8082
ENTRYPOINT ["/app/room-server"]
