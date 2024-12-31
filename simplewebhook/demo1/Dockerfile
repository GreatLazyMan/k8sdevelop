FROM golang:1.23 AS build_base

RUN mkdir -p /opt/src/my-webhook
WORKDIR /opt/src/my-webhook
RUN go env -w GO111MODULE=on
RUN go env -w GOPROXY=https://goproxy.cn,direct 
RUN go env -w GOSUMDB="sum.golang.org" # go env -w GOSUMDB=sum.golang.google.cn
COPY go.mod .
COPY go.sum .
RUN go mod download

# go binary build
FROM build_base AS binary_builder
WORKDIR /opt/src/my-webhook
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /my-webhook main.go


# final image, direct copy binary build result to a simple base image
FROM alpine:3.20.1
COPY --from=binary_builder /my-webhook /bin/

# 使用了ENTRYPOINT就不能像CMD那样运行容器时自定义执行的命令了
ENTRYPOINT [ "my-webhook" ] 
