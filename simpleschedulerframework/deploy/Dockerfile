FROM m.daocloud.io/docker.io/library/golang:1.23rc2-alpine3.20 AS builder
RUN apk add --no-cache make bash
RUN apk add --no-cache git gcc
RUN apk add --no-cache libc-dev btrfs-progs-dev btrfs-progs-libs
RUN go env -w GO111MODULE=on 
RUN go env -w GOPROXY=https://goproxy.cn,direct 
ADD simpleschedulerframework /root/simpleschedulerframework
RUN cd /root/simpleschedulerframework && go mod tidy && go mod vendor && go build .


FROM m.daocloud.io/docker.io/library/alpine:latest
RUN apk add ip6tables iptables curl tcpdump busybox-extras nftables
COPY --from=builder /root/simpleschedulerframework/simplescheduler /usr/local/bin/simplescheduler
RUN chmod 755 /usr/local/bin/simplescheduler


CMD ["/usr/local/bin/simplescheduler"]
