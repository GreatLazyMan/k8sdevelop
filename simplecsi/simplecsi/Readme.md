# 说明

simplecsi插件的开发是基于 git@github.com:kubernetes-csi/csi-driver-host-path.git ( 做的，改动不大，就是重新梳理了一下代码结构而已。

优化的话，在原来的基础上使用job的形式，可以在集群内任意节点创建目录，使用configmap存储volume的状态信息，很粗糙，有些错误或者异常也没考虑或者处理。

## 开发思路

CSI插件开发基本思路如下：

1. 创建一个grpc server

2. 分别实现`IdentityServer` `NodeServer` 和 `ControllerServer`

3. 注册上面的的服务并启动
   
   > ```go
   > csi.RegisterIdentityServer(server, identityServer)
   > csi.RegisterNodeServer(server, nodeServer)  
   > csi.RegisterControllerServer(server, ctlServer) 
   > server.Serve(listener) //listener 是一个unix端口
   > ```

K8s 中的 Pod 在挂载pvc需经历 Provision（创建卷） Attach（将卷挂载到宿主机） Mount （挂载到目标目录）三个的阶段

## Provision部分

1. 集群管理员创建 StorageClass 资源，该 StorageClass 中包含 CSI 插件名称；

2. 用户创建 PVC 资源，PVC 指定存储大小及 StorageClass；

3. 卷控制器（PV Controller）观察到集群中新创建的 PVC 没有与之匹配的 PV，且其使用的存储类型为 out-of-tree，于是为 PVC 打 annotation：`volume.beta.kubernetes.io/storage-provisioner=[out-of-tree CSI 插件名称]`

4. External Provisioner 组件观察到 PVC 的 annotation 中包含 `volume.beta.kubernetes.io/storage-provisioner`且其 value 是自己，于是开始创盘流程：
   
   1. 获取相关 StorageClass 资源并从中获取参数，用于后面 CSI 函数调用
   
   2. 通过 unix domain socket **调用外部 CSI 插件的**`CreateVolume` **函数**

5. 外部 CSI 插件返回成功后表示盘创建完成，**此时External Provisioner 组件会在集群创建一个 PersistentVolume 资源。**

6. 卷控制器会将 PV 与 PVC 进行绑定。

这里，我们的核心是实现 `CreateVolume` 函数，主要的工作是创建一个目录，并记录volume相关的信息，反馈`csinode`信息

### Attach部分

1. AD 控制器（AttachDetachController）观察到使用 CSI 类型 PV 的 Pod 被调度到某一节点，此时AD 控制器会调用内部 in-tree CSI 插件（csiAttacher）的 Attach 函数；

2. 内部 in-tree CSI 插件（csiAttacher）会创建一个 VolumeAttachment 对象到集群中；

3. External Attacher 观察到该 VolumeAttachment 对象，**并调用外部 CSI插件的** `ControllerPublish` **函数以将卷挂接到对应节点上**。当外部 CSI 插件挂载成功后，External Attacher会更新相关 VolumeAttachment 对象的 .Status.Attached 为 true；

4. AD 控制器内部 in-tree CSI 插件（csiAttacher）观察到 VolumeAttachment 对象的 .Status.Attached 设置为 true，于是更新AD 控制器内部状态（ActualStateOfWorld），该状态会显示在 Node 资源的 .Status.VolumesAttached 上；

hostpath不涉及这一部分操作，没有关注

### Mount部分

1. Volume Manager（Kubelet 组件）观察到有新的使用 CSI 类型 PV 的 Pod 调度到本节点上，于是调用内部 in-tree CSI 插件（csiAttacher）的 WaitForAttach 函数；

2. 内部 in-tree CSI 插件（csiAttacher）等待集群中 VolumeAttachment 对象状态 .Status.Attached 变为 true；

3. in-tree CSI 插件（csiAttacher）调用 MountDevice 函数，该函数内部通过 unix domain socket 调用外部 CSI 插件的 `NodeStageVolume` 函数；之后插件（csiAttacher）调用内部 in-tree CSI 插件（csiMountMgr）的 SetUp 函数，该函数内部会通过 unix domain socket 调用外部 CSI 插件的 `NodePublishVolume` 函数；

`NodeStageVolume` 函数校验下attach，hostpath也不涉及。我们这里主要实现 `NodePublishVolume` 函数，完成的工作是：创建一个pod，进入nodeshell在pv所在节点执行 `mount -o bind src dst` 的操作

# 未学习的部分

1. groupcontrollerserver

2. snapshot

3. ControllerServiceCapability

# 参考：

[【K8s概念】理解容器存储接口 CSI - Varden - 博客园](https://www.cnblogs.com/varden/p/15139819.html)

[GitHub - kubernetes-csi/csi-driver-host-path: A sample (non-production) CSI Driver that creates a local directory as a volume on a single node](https://github.com/kubernetes-csi/csi-driver-host-path)
