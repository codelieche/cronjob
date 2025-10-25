# 第一阶段：构建Go应用
FROM golang:1.25-alpine AS builder

WORKDIR /data

# 设置代理
ENV GOPROXY=https://goproxy.cn,direct

# 复制项目文件
COPY . .

# 安装swag工具并生成swagger文档
RUN go install github.com/swaggo/swag/cmd/swag@latest && swag init

# 下载依赖并构建（使用静态链接）
RUN go mod download && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o apiserver

# 第二阶段：运行时镜像
FROM alpine:3.22.1

# 安装必要的工具
RUN apk update && apk add --no-cache curl bash tree \
    && rm -rf /var/cache/apk/*

# 创建非 root 用户
RUN addgroup -g 1000 appuser && \
adduser -D -u 1000 -G appuser appuser

WORKDIR /data

# 从builder阶段复制编译好的二进制文件
COPY --from=builder  --chown=appuser:appuser /data/apiserver ./

# 复制 Swagger 文档文件（从builder阶段生成的文件）
RUN mkdir -p ./docs
COPY --from=builder  --chown=appuser:appuser /data/docs/swagger.* ./docs/

# 设置可执行权限
RUN chown -R appuser:appuser /data && \
    chmod +x ./apiserver

# 切换到非 root 用户
USER appuser

EXPOSE 80

CMD ["./apiserver"]
