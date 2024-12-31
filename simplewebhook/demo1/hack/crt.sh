# 生成CA的证书和私钥
openssl genrsa -out /tmp/ca.key 2048
openssl req -x509 -new -nodes -key /tmp/ca.key -subj "/CN=ca" -days 3650 -out /tmp/ca.crt

# 再生成一个私钥和以及它的证书签名请求（CSR）
openssl genrsa -out /tmp/webhook.key 2048
openssl req -new -key /tmp/webhook.key -subj "/CN=my-webhook.${NAMESPACE}.svc" \
    -reqexts v3_req \
    -config <(cat <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req

[req_distinguished_name]

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = my-webhook.${NAMESPACE}.svc
DNS.2 = my-webhook.${NAMESPACE}.svc.cluster.local
DNS.3 = *.${NAMESPACE}.svc
EOF
) -out /tmp/webhook.csr

# 使用CA的key和证书签署CSR `webhook.csr`，得到由CA签署的和私钥对应的证书
openssl x509 -req -in /tmp/webhook.csr -CA /tmp/ca.crt -CAkey /tmp/ca.key -CAcreateserial \
    -out /tmp/webhook.crt -days 365 -extensions v3_req \
    -extfile <(cat <<EOF
[req]
distinguished_name = req_distinguished_name

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = my-webhook.${NAMESPACE}.svc
DNS.2 = my-webhook.${NAMESPACE}.svc.cluster.local
EOF
)
