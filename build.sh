#!/bin/bash

IMAGE_TAG=v1

# 构建apiserver
cd apiserver
docker build -t codelieche/cronjob-apiserver:${IMAGE_TAG} .

# 构建usercenter
cd ../usercenter
docker build -t codelieche/usercenter:${IMAGE_TAG} .

# 构建todolist
cd ../todolist
docker build -t codelieche/todolist:${IMAGE_TAG} .

# 构建worker
cd ../worker
docker build -t codelieche/worker:${IMAGE_TAG} . 
 