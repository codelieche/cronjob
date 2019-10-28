#!/bin/bash

# 第1步：准备变量
NAME=master
TAG=v1

# 第2步：打包程序
# 2-1: 进入程序入口目录
cd ../../cmd/master
# 2-2：执行构建命令
GOOS=linux GOARCH=amd64 go build -o master ./master.go && echo "$(date +"%F %T"): 构建成功" \
|| (echo "$(date %"%F %T"): 构建失败！！！" && exit 1)
tree

# 第3步：打包镜像
# 3-1: 移动程序
mkdir ../../dockerfile/master/app/ ../../dockerfile/master/app/static ../../dockerfile/master/app/templates

mv ./master ../../dockerfile/master/app/
# 复配配置文件和静态文件
cp ../../master/config.yaml ../../dockerfile/master/app/
cp  -rf ../../master/static/* ../../dockerfile/master/app/static/
cp  -rf ../../master/templates/* ../../dockerfile/master/app/templates/

# 3-1：进入Dockerfile目录
cd ../../dockerfile/master
tree

# 3-3: 执行docker build
docker build . -t "$NAME:$TAG" && rm -rf ./app || (echo "$(date +"%F %T"): 构建失败！！！" && exit 1)

# 第4步：推送镜像到registry
# 4-1: 打标签
docker tag "$NAME:$TAG" "codelieche/$NAME:$TAG"


# 4-2:查看镜像
docker images | grep $NAME

# 4-3：推送【请手动推送吧，不自动执行】
# docker push "codelieche/$NAME:$TAG"

# 第5步：创建测试容器
# docker run -itd -v "${PWD}/config.yaml:/app/config.yaml" -p 9000:9000 --name master-t1 master:v1

# 创建容器,进入容器查看文件:
# docker run -it --rm -v "${PWD}/config.yaml:/app/config.yaml" -p 9000:9000 --name master-t1 master:v1 /bin/bash
