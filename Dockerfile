FROM golang:1.22

WORKDIR /app

COPY . .

RUN wget https://github.com/ethereum/solidity/releases/download/v0.8.27/solc-static-linux && \
    chmod +x solc-static-linux && \
    mv solc-static-linux /usr/local/bin/solc

RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod tidy && go build ./cmd/proxy

EXPOSE 8080

CMD ["./proxy"]