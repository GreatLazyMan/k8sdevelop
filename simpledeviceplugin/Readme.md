开发流程：
1. 启动一个grpcServer，侦听sock
2. 注册一个pluginServer，主要是实现ListAndWatch和Allocate，Allocate实现资源的挂载和环境变量的配置，ListAndWatch接受终止信号也可以持续检查plugin的状态
3. 向kubelet中注册对应资源
4. 注意监控 kubelet 的重启，一般是使用 fsnotify 类似的库监控 kubelet.sock 的重新创建事件。如果 kubelet.sock 重新创建了，则认为 kubelet 是重启了，那么需要重新注册


参考：
https://zhuanlan.zhihu.com/p/83015668
https://github.com/kuberenetes-learning-group/fuse-device-plugin
https://github.com/lixd/i-device-plugin.git
