package server

type PodStruct struct {
	PvName       string
	NodeName     string
	LocalPath    string
	Image        string
	SaName       string
	Namespace    string
	CommandSlice string
}

const InitPodTemplate = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: {{ .PvName }}
  name: init-pvc-{{ .PvName }}
  namespace: {{ .Namespace }}
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: kubernetes.io/hostname
            operator: In
            values:
            - {{ .NodeName }}
  containers:
  - command: [ {{ .CommandSlice }} ]
    image: {{ .Image }}
    imagePullPolicy: IfNotPresent
    name: local-path-init
    resources: {}
    securityContext:
      privileged: true
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    volumeMounts:
    - mountPath: {{ .LocalPath }}
      name: data
  dnsPolicy: ClusterFirst
  enableServiceLinks: false
  nodeName: {{ .NodeName }}
  preemptionPolicy: PreemptLowerPriority
  priority: 0
  restartPolicy: Never
  schedulerName: default-scheduler
  terminationGracePeriodSeconds: 30
  tolerations:
  - effect: NoExecute
    key: node.kubernetes.io/not-ready
    operator: Exists
    tolerationSeconds: 300
  - effect: NoExecute
    key: node.kubernetes.io/unreachable
    operator: Exists
    tolerationSeconds: 300
  volumes:
  - hostPath:
      path: {{ .LocalPath }}
      type: ""
    name: data
`

type PvStruct struct {
	PvName    string
	PvcCap    string
	PvcName   string
	PvcRV     string
	PvcUID    string
	Namespace string

	BasePath string
	NodeName string
	ScName   string
}

const PvTemplate = `
apiVersion: v1
kind: PersistentVolume
metadata:
  labels:
    simplecsi/cas-type: hostpath
  name: {{ .PvName }}
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: {{ .PvcCap }}
  claimRef:
    apiVersion: v1
    kind: PersistentVolumeClaim
    name: {{ .PvcName }}
    namespace: {{ .Namespace }}
    resourceVersion: "{{ .PvcRV }}"
    uid: {{ .PvcUID }}
  local:
    fsType: ""
    path: {{ .BasePath }}/{{.PvName}}
  nodeAffinity:
    required:
      nodeSelectorTerms:
      - matchExpressions:
        - key: kubernetes.io/hostname
          operator: In
          values:
          - {{ .NodeName }}
  persistentVolumeReclaimPolicy: Delete
  storageClassName: {{ .ScName }}
  volumeMode: Filesystem
  `
