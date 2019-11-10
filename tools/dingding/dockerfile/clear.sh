#!/bin/bash

# 第1步：准备变量
NAME=dingding
TAG=v1

# 第2步：删除容器
docker ps -a | grep "$NAME:$TAG" | awk '{print $1}' | xargs docker rm --force $i

# 第3步：删除镜像
docker rmi "$NAME:$TAG"
docker rmi "codelieche/$NAME:$TAG"