安装kubebuilder

```
# download kubebuilder and install locally.
curl -L -o kubebuilder "https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)"
chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/
```

初始化

```
kubebuilder init --domain github.com --repo github.com/greatlazyman/simplewebhook
kubebuilder create api --group simple.io --version v1 --kind App # 一直按y
make generate 
make manifests
kubebuilder create webhook --group simple.io  --version v1 --kind App   --conversion   --defaulting   --programmatic-validation
make manifests
```

更新

```
go mod tidy
make generate 
make manifests
make docker-build # 制作镜像
```

`internal/webhook/v1/app_webhook.go`

```
// SetupAppWebhookWithManager registers the webhook for App in the manager.
func SetupAppWebhookWithManager(mgr ctrl.Manager) error {
        return ctrl.NewWebhookManagedBy(mgr).For(&simpleiov1.App{}).
                WithValidator(&AppCustomValidator{}).
                WithDefaulter(&AppCustomDefaulter{}).
                Complete()
}
```

这两个变量分别表示 MutatingWebhookServer 和 ValidatingWebhookServer，在程序启动的时候，这两个 Server 会 run 起来。

对于我们希望 Webhook 在资源发生什么样的变化时触发，可以通过这条注释修改：

```
// +kubebuilder:webhook:path=/mutate-app-o0w0o-cn-v1-app,mutating=true,failurePolicy=fail,groups=app.o0w0o.cn,resources=apps,verbs=create;update,versions=v1,name=mapp.kb.io
```

对应的参数为：

- failurePolicy：表示 ApiServer 无法与 webhook server 通信时的失败策略，取值为 "ignore" 或 "fail"；

- groups：表示这个 webhook 在哪个 Api Group 下会收到请求；

- mutating：这个参数是个 bool 型，表示是否是 mutating 类型；

- name：webhook 的名字，需要与 configuration 中对应；

- path：webhook 的 path；

- resources：表示这个 webhook 在哪个资源发生变化时会收到请求；

- verbs：表示这个 webhook 在资源发生哪种变化时会收到请求，取值为 “create“, "update", "delete", "connect", 或 "*" (即所有)；

- versions：表示这个 webhook 在资源的哪个 version 发生变化时会收到请求；

部署cert-manager

```
#kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.crds.yaml
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml
```

编辑文件 config/default/kustomization.yaml

打开注释 `- ../certmanager` 

编辑文件 config/certmanager/certificate.yaml，修改 `SERVICE_NAME.SERVICE_NAMESPACE.svc` `SERVICE_NAME.SERVICE_NAMESPACE.svc.cluster.local`

编辑文件 ./config/webhook/manifests.yaml，修改为真实的svc

```
  clientConfig:
    service:
      name: webhook-service
      namespace: system
      path: /mutate-simple-io-github-com-v1-app
  failurePolicy: Fail
```

编辑文件 config/webhook/manifests.yaml，添加注释

```
  annotations:
    cert-manager.io/inject-ca-from: simplewebhook-system/simplewebhook-serving-cert
```

添加了注释，启动成功后webhook中会被注入 clientConfig

```
kubectl get mutatingwebhookconfigurations simplewebhook-mutating-webhook-configuration -oyaml
```

安装

```
wget https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv5.5.0/kustomize_v5.5.0_linux_amd64.tar.gz
make install  
make deploy IMG=xxxx # kustomize build config/default | kubectl  apply -f -
```

测试

```
kustomize  build config/samples/ | kubectl  apply -f -
```

最终完整部署的yaml如下

```
apiVersion: v1
kind: Namespace
metadata:
  labels:
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: simplewebhook
    control-plane: controller-manager
  name: simplewebhook-system
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.16.4
  name: apps.simple.io.github.com
spec:
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        service:
          name: simplewebhook-webhook-service
          namespace: simplewebhook-system
          path: /convert
      conversionReviewVersions:
      - v1
  group: simple.io.github.com
  names:
    kind: App
    listKind: AppList
    plural: apps
    singular: app
  scope: Namespaced
  versions:
  - name: v1
    schema:
      openAPIV3Schema:
        description: App is the Schema for the apps API.
        properties:
          apiVersion:
            description: |-
              APIVersion defines the versioned schema of this representation of an object.
              Servers should convert recognized schemas to the latest internal value, and
              may reject unrecognized values.
              More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
            type: string
          kind:
            description: |-
              Kind is a string value representing the REST resource this object represents.
              Servers may infer this from the endpoint the client submits requests to.
              Cannot be updated.
              In CamelCase.
              More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
            type: string
          metadata:
            type: object
          spec:
            description: AppSpec defines the desired state of App.
            properties:
              foo:
                description: Foo is an example field of App. Edit app_types.go to
                  remove/update
                type: string
            type: object
          status:
            description: AppStatus defines the observed state of App.
            type: object
        type: object
    served: true
    storage: true
    subresources:
      status: {}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: simplewebhook
  name: simplewebhook-controller-manager
  namespace: simplewebhook-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  labels:
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: simplewebhook
  name: simplewebhook-leader-election-role
  namespace: simplewebhook-system
rules:
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: simplewebhook
  name: simplewebhook-app-editor-role
rules:
- apiGroups:
  - simple.io.github.com
  resources:
  - apps
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - simple.io.github.com
  resources:
  - apps/status
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: simplewebhook
  name: simplewebhook-app-viewer-role
rules:
- apiGroups:
  - simple.io.github.com
  resources:
  - apps
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - simple.io.github.com
  resources:
  - apps/status
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: simplewebhook-manager-role
rules:
- apiGroups:
  - simple.io.github.com
  resources:
  - apps
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - simple.io.github.com
  resources:
  - apps/finalizers
  verbs:
  - update
- apiGroups:
  - simple.io.github.com
  resources:
  - apps/status
  verbs:
  - get
  - patch
  - update
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: simplewebhook-metrics-auth-role
rules:
- apiGroups:
  - authentication.k8s.io
  resources:
  - tokenreviews
  verbs:
  - create
- apiGroups:
  - authorization.k8s.io
  resources:
  - subjectaccessreviews
  verbs:
  - create
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: simplewebhook-metrics-reader
rules:
- nonResourceURLs:
  - /metrics
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: simplewebhook
  name: simplewebhook-leader-election-rolebinding
  namespace: simplewebhook-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: simplewebhook-leader-election-role
subjects:
- kind: ServiceAccount
  name: simplewebhook-controller-manager
  namespace: simplewebhook-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: simplewebhook
  name: simplewebhook-manager-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: simplewebhook-manager-role
subjects:
- kind: ServiceAccount
  name: simplewebhook-controller-manager
  namespace: simplewebhook-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: simplewebhook-metrics-auth-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: simplewebhook-metrics-auth-role
subjects:
- kind: ServiceAccount
  name: simplewebhook-controller-manager
  namespace: simplewebhook-system
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: simplewebhook
    control-plane: controller-manager
  name: simplewebhook-controller-manager-metrics-service
  namespace: simplewebhook-system
spec:
  ports:
  - name: https
    port: 8443
    protocol: TCP
    targetPort: 8443
  selector:
    control-plane: controller-manager
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: simplewebhook
  name: simplewebhook-webhook-service
  namespace: simplewebhook-system
spec:
  ports:
  - port: 443
    protocol: TCP
    targetPort: 9443
  selector:
    control-plane: controller-manager
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: simplewebhook
    control-plane: controller-manager
  name: simplewebhook-controller-manager
  namespace: simplewebhook-system
spec:
  replicas: 1
  selector:
    matchLabels:
      control-plane: controller-manager
  template:
    metadata:
      annotations:
        kubectl.kubernetes.io/default-container: manager
      labels:
        control-plane: controller-manager
    spec:
      containers:
      - args:
        - --metrics-bind-address=:8443
        - --leader-elect
        - --health-probe-bind-address=:8081
        command:
        - /manager
        image: controller:latest
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 20
        name: manager
        ports:
        - containerPort: 9443
          name: webhook-server
          protocol: TCP
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          limits:
            cpu: 500m
            memory: 128Mi
          requests:
            cpu: 10m
            memory: 64Mi
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
        volumeMounts:
        - mountPath: /tmp/k8s-webhook-server/serving-certs
          name: cert
          readOnly: true
      securityContext:
        runAsNonRoot: true
      serviceAccountName: simplewebhook-controller-manager
      terminationGracePeriodSeconds: 10
      volumes:
      - name: cert
        secret:
          defaultMode: 420
          secretName: webhook-server-cert
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  labels:
    app.kubernetes.io/component: certificate
    app.kubernetes.io/created-by: simplewebhook
    app.kubernetes.io/instance: serving-cert
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: certificate
    app.kubernetes.io/part-of: simplewebhook
  name: simplewebhook-serving-cert
  namespace: simplewebhook-system
spec:
  dnsNames:
  - SERVICE_NAME.SERVICE_NAMESPACE.svc
  - SERVICE_NAME.SERVICE_NAMESPACE.svc.cluster.local
  issuerRef:
    kind: Issuer
    name: simplewebhook-selfsigned-issuer
  secretName: webhook-server-cert
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  labels:
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: simplewebhook
  name: simplewebhook-selfsigned-issuer
  namespace: simplewebhook-system
spec:
  selfSigned: {}
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: simplewebhook-mutating-webhook-configuration
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    service:
      name: simplewebhook-webhook-service
      namespace: simplewebhook-system
      path: /mutate-simple-io-github-com-v1-app
  failurePolicy: Fail
  name: mapp-v1.kb.io
  rules:
  - apiGroups:
    - simple.io.github.com
    apiVersions:
    - v1
    operations:
    - CREATE
    - UPDATE
    resources:
    - apps
  sideEffects: None
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: simplewebhook-validating-webhook-configuration
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    service:
      name: simplewebhook-webhook-service
      namespace: simplewebhook-system
      path: /validate-simple-io-github-com-v1-app
  failurePolicy: Fail
  name: vapp-v1.kb.io
  rules:
  - apiGroups:
    - simple.io.github.com
    apiVersions:
    - v1
    operations:
    - CREATE
    - UPDATE
    resources:
    - apps
  sideEffects: None
```

[GitHub - zwwhdls/KubeAdmissionWebhookDemo: ������A demo about Kubernetes AdmissionWebhook by kubebuilder.](https://github.com/zwwhdls/KubeAdmissionWebhookDemo)
