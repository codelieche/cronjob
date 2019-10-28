FROM alpine:3.9
# 因为需要访问https网页，需要add ca-certificates

RUN apk add --no-cache bash curl vim ca-certificates tree

WORKDIR /app
# 设置环境变量
ENV LANG='C.UTF-8' PYTHONIOENCODING='UTF-8' TZ='Asia/Shanghai' MASTER_PORT=9000

EXPOSE ${MASTER_PORT}

COPY ./app/ /app/

CMD /app/master /app/config.yaml
