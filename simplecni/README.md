# 说明

主控逻辑、架构的书写方式基本是参考flannel

CSI开发主要包含两部分

1. 主控逻辑的代码，用于构建节点之间的容器网络互通，主要是增加到其他节点podcidr的路由

2. cni插件的开发，用于创建容器网络命名空间、vethpair等

我们在flannel架构基础上，实现一个简易的基于用户空间wireguard的cni，主要是用于学习用途，比较粗糙，也没考虑删除节点的操作，也顺带了解一些wireguard在用户空间如何启动，虽然在内核态性能好，但是并不是虽有版本的内核都支持，对于低版本内核使用用户态wireguard可以解决一些特殊的问题，比如通过公网纳管集群。



