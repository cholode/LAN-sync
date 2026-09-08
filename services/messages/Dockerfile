ARG DOCKER_HUB_MIRROR=dockerproxy.net
FROM ${DOCKER_HUB_MIRROR}/library/golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 通过构建参数为 API、归档 Worker 和编号器分别生成独立镜像。
ARG MESSAGE_PROCESS=api
# 共用编译缓存并限制并发，避免多个入口重复编译时耗尽开发机内存。
ARG GO_BUILD_PARALLEL=2
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -p ${GO_BUILD_PARALLEL} -trimpath -ldflags="-s -w" -o /message-service ./services/messages/cmd/${MESSAGE_PROCESS}

FROM ${DOCKER_HUB_MIRROR}/library/alpine:3.23
RUN apk --no-cache add tzdata ca-certificates && adduser -S -D messageuser
ENV TZ=UTC
WORKDIR /app
COPY --from=builder /message-service /app/message-service
USER messageuser
EXPOSE 8083 6061 6062
ENTRYPOINT ["/app/message-service"]
