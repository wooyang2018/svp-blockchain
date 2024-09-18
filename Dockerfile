FROM ubuntu:20.04

# 设置工作目录
WORKDIR /app

# 安装必要的软件包，包括 add-apt-repository 命令
RUN apt-get update && apt-get install -y software-properties-common

# 添加 Go 语言的 PPA 仓库并安装 Go
RUN add-apt-repository ppa:longsleep/golang-backports && apt-get update && apt-get install -y golang-go

# 设置 Go 环境变量
#ENV GOPATH /app
#ENV PATH /usr/lib/go-1.16/bin:$PATH

# 复制源代码到工作目录
COPY . .

# 编译 Go 程序
RUN go build -o cmd/proxy

RUN add-apt-repository ppa:ethereum/ethereum && RUN apt-get update && RUN apt-get install solc

# 运行编译后的可执行文件
CMD ["./proxy"]