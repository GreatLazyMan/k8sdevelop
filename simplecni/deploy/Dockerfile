FROM m.daocloud.io/docker.io/library/alpine:latest
RUN mkdir -p /cni/bin &&  wget https://github.com/containernetworking/plugins/releases/download/v1.5.1/cni-plugins-linux-amd64-v1.5.1.tgz && \
      tar zxf cni-plugins-linux-amd64-v1.5.1.tgz -C /cni/bin/ && rm -f cni-plugins-linux-amd64-v1.5.1.tgz  
RUN rm -f /cni/bin/LICENSE && rm -f /cni/bin/README.md
RUN mkdir /etc/simplecni
COPY ../simplecni /usr/local/bin/
RUN chmod 755 /usr/local/bin/simplecni
COPY ../cniplugin/cniplugin /cni/bin
