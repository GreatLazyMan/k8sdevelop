FROM m.daocloud.io/docker.io/library/golang:1.23rc2-alpine3.20 AS builder
RUN apk add --no-cache make bash
RUN apk add --no-cache git gcc
RUN apk add --no-cache libc-dev btrfs-progs-dev btrfs-progs-libs
RUN go env -w GO111MODULE=on 
RUN go env -w GOPROXY=https://goproxy.cn,direct 
ADD simplecni /root/simplecni
RUN cd /root/simplecni && go mod tidy && go mod vendor && go build .


FROM m.daocloud.io/docker.io/library/alpine:latest
RUN mkdir -p /cni/bin &&  wget https://github.com/containernetworking/plugins/releases/download/v1.5.1/cni-plugins-linux-amd64-v1.5.1.tgz && \
      tar zxf cni-plugins-linux-amd64-v1.5.1.tgz -C /cni/bin/ && rm -f cni-plugins-linux-amd64-v1.5.1.tgz  
RUN rm -f /cni/bin/LICENSE && rm -f /cni/bin/README.md
RUN apk add ip6tables iptables curl tcpdump busybox-extras nftables
RUN mkdir /etc/simplecni
COPY --from=builder /root/simplecni/simplecni /usr/local/bin/simplecni
RUN chmod 755 /usr/local/bin/simplecni
COPY cniplugin /cni/bin
