FROM alpine:3.9
# 因为需要访问https网页，需要add ca-certificates

RUN apk add --no-cache bash curl vim ca-certificates tree

WORKDIR /app
# 设置环境变量
ENV LANG='C.UTF-8' PYTHONIOENCODING='UTF-8' TZ='Asia/Shanghai' PORT=9000

EXPOSE ${PORT}

COPY ./app/ /app/

CMD /app/dingding /app/config.yaml