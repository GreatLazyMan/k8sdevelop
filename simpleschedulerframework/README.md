# 说明

学习 scheduler-plugins 的coscheduler 插件
https://github.com/kubernetes-sigs/scheduler-plugins
调度器层面重新组织了一下开源代码，pluginargs的代码组织尤其奇怪，但是只有这样组织才是有效的
并没有做新的功能,主要是学习调度器开发的流程

有一些注意点：

1. 原生调度器是分调度阶段的。`Prefilter`过滤无法调度的pod，`Filter`过滤无法使用的节点，`Postfilter`在pod没有可用节点的时候做一些特殊的处理或者记录。`Reserve`和`Unreserve`用于给pod预留资源和释放预留的资源。`Permit` 扩展用于阻止或者延迟 Pod 与节点的绑定。`Prebind` 扩展用于在 Pod 绑定之前执行某些逻辑。如果任何一个 `Prebind` 扩展返回错误，Pod 将被放回到待调度队列，此时将触发 Unreserve 扩展。`Bind` 扩展用于将 Pod 绑定到节点上

2. 在调度一个pod的各个阶段 `state *framework.CycleState`，state变量可以在各个节点传递信息

3. `f framework.Handle` 可以用来获取clientset，可以在一定程度上处理waiting队列中的pod

其他：

暂时不支持缩容情况下的pod删除，那种情况需要额外写一个控制器
