#!/bin/bash

# 第1步：准备变量
NAME=master
TAG=v1

# 删除容器
docker ps -a | grep "$NAME:$TAG" | awk '{print $1}' | xargs docker rm --force $i

# 删除镜像
docker rmi "$NAME:$TAG"
docker rmi "codelieche/$NAME:$TAG"

