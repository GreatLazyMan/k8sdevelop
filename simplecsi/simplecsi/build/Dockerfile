FROM m.daocloud.io/docker.io/library/golang:1.23rc2-alpine3.20 AS builder
#RUN apk add --no-cache make bash
#RUN apk add --no-cache git gcc
#RUN apk add --no-cache libc-dev btrfs-progs-dev btrfs-progs-libs
RUN go env -w GO111MODULE=on 
RUN go env -w GOPROXY=https://goproxy.cn,direct 
ADD simplecsi /root/simplecsi
RUN cd /root/simplecsi && go mod tidy && go mod vendor && go build .


FROM m.daocloud.io/docker.io/library/alpine:latest
RUN apk add ip6tables iptables curl tcpdump busybox-extras nftables
RUN mkdir /etc/simplecni
COPY --from=builder /root/simplecsi/simplecsi /usr/local/bin/simplecsi
RUN chmod 755 /usr/local/bin/simplecsi
