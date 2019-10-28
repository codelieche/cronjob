FROM alpine:3.9
# 因为需要访问https网页，需要add ca-certificates

RUN apk add --no-cache bash curl vim ca-certificates tree

WORKDIR /app
# 设置环境变量
ENV LANG='C.UTF-8' PYTHONIOENCODING='UTF-8' TZ='Asia/Shanghai' WORKER_PORT=8080

EXPOSE ${WORKER_PORT}

COPY ./app/* /app/

CMD /app/worker /app/config.yaml
