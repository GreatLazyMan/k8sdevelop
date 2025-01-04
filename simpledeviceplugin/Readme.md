开发流程：
1. 启动一个grpcServer，侦听sock
2. 注册一个pluginServer，主要是实现ListAndWatch和Allocate，Allocate实现资源的挂载和环境变量的配置，ListAndWatch接受终止信号也可以持续检查plugin的状态
3. 向kubelet中注册对应资源


参考：
https://zhuanlan.zhihu.com/p/83015668
https://github.com/kuberenetes-learning-group/fuse-device-plugin
https://github.com/lixd/i-device-plugin.git
