#!/bin/bash

IMAGE_TAG=v1

# 构建apiserver
cd apiserver
echo "构建apiserver"
docker build -t codelieche/apiserver:${IMAGE_TAG} .

# 构建usercenter
cd ../usercenter
echo "构建usercenter"
docker build -t codelieche/usercenter:${IMAGE_TAG} .

# 构建todolist
cd ../todolist
echo "构建todolist"
docker build -t codelieche/todolist:${IMAGE_TAG} .

# 构建worker
cd ../worker
echo "构建worker"
docker build -t codelieche/worker:${IMAGE_TAG} . 

# 构建chatbot
cd ../chatbot
echo "构建chatbot"
docker build -t codelieche/chatbot:${IMAGE_TAG} . 
 
