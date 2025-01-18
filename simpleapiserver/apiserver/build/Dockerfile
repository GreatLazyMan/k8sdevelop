FROM m.daocloud.io/docker.io/library/debian:stretch-slim

# fuse-device-plugin binary based on architecture
ARG build_arch
COPY apiserver-${build_arch} /usr/bin/apiserver

# replace with your desire device count
CMD ["apiserver"]
