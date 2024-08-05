FROM m.daocloud.io/docker.io/library/golang:1.23rc2-alpine3.20 AS builder
RUN apk add --no-cache make bash
RUN apk add --no-cache git gcc
RUN apk add --no-cache libc-dev btrfs-progs-dev btrfs-progs-libs
RUN go env -w GO111MODULE=on 
RUN go env -w GOPROXY=https://goproxy.cn,direct 
ADD ${CODE_DIR_NAME} /root/{CODE_DIR_NAME}
RUN cd /root/${CODE_DIR_NAME} && go mod tidy && go mod vendor && go build .


FROM m.daocloud.io/docker.io/library/alpine:latest
RUN apk add ip6tables iptables curl tcpdump busybox-extras nftables
COPY --from=builder /root/${CODE_DIR_NAME}/${CODE_DIR_NAME} /usr/local/bin/${CODE_DIR_NAME}
RUN chmod 755 /usr/local/bin/${CODE_DIR_NAME}


CMD ["/usr/local/bin/${CODE_DIR_NAME}"]
