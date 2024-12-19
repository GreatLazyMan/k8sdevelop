学习 scheduler-plugins 的coscheduler 插件
https://github.com/kubernetes-sigs/scheduler-plugins
重新组织了一下开源代码，并没有做新的功能
暂时不支持缩容情况下的pod删除，那种情况需要额外写一个控制器
在调度一个pod的各个阶段 state *framework.CycleState，state变量可以在各个节点传递信息
f framework.Handle 可以用来获取clientset，可以在一定程度上处理waiting队列中的pod
