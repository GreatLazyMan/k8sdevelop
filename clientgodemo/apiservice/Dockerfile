FROM m.daocloud.io/docker.io/library/debian:stretch-slim

# fuse-device-plugin binary based on architecture
ARG build_arch
COPY demo-${build_arch} /usr/bin/demo

# replace with your desire device count
CMD ["demo"]
