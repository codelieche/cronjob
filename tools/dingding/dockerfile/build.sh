#!/bin/bash

# 第1步：准备变量
NAME=dingding
TAG=v1

# 第2步：打包程序
# 2-1：进入程序目录
cd ../entry
# 2-2: 执行构建命令
# MacOS下编译，会报错：go-sqlite3/sqlite3_opt_preupdate.go:12:16: undefined: SQLiteConn
# 原因：MacOS上未安装linux的交叉编译器: http://crossgcc.rts-software.org/doku.php?id=compiling_for_linux
# 故可以启动个docker容器来编译程序： docker run -it --rm -v "$GOPATH:/go" golang:1.13

# 故改用MySQL存储数据，不用sqlite3
GOOS=linux GOARCH=amd64 go build -o dingding ./dingding.go && echo "$(date "%F %T"):构建成功！" \
 || (echo "$(date "%F %T"): 构建失败!！" && exit 1;)

#docker run -it --rm -v "$GOPATH:/go" golang:1.13 \
# /bin/bash -c "echo '进入容器了' && cd /go/src/github.com/codelieche/cronjob/tools/dingding/entry && echo $PWD && GOOS=linux GOARCH=amd64 go build -o dingding ./dingding.go && ls -alh"

# 2-3: 查看当前目录文件
tree

# 第3步：打包镜像
# 3-1：移动程序执行文件
mkdir ../dockerfile/app ../dockerfile/app/web ../dockerfile/app/web/templates ../dockerfile/app/web/public
mv ./dingding ../dockerfile/app/

# 3-2: 复制其它文件
cp ../config.yaml ../dockerfile/app/
cp -rf ../web/templates/* ../dockerfile/app/web/templates
#cp -rf ../web/public/* ../dockerfile/app/web/public

# 3-3 进入Dockerfile目录
cd ../dockerfile
tree

# 3-4: 执行docker build
docker build ./ -t ${NAME}:${TAG} && rm -rf ./app || (echo "$(date +"%F %T"): 构建失败！！！" && exit 1)

# 第4步：推送到镜像仓库
# 4-1：打标签
docker tag "$NAME:$TAG" "codelieche/$NAME:$TAG"

# 4-2: 查看镜像
docker images | grep $NAME

# 4-3：推送镜像【推荐手动执行】
# docker push "codelieche/$NAME:$TAG"

# 第5步：创建测试容器
# docker run -itd -p 9000:9000 --name "${NAME}-t1" $NAME:$TAG

# 创建容器 手动去执行程序
# docker run -it --rm  -p 9000:9000 --name dingding-t1 dingding:v1 /bin/bash
