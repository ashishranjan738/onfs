// Copyright © 2017 Ashish Ranjan <ashishranjan738@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"html/template"
	"os"

	"github.com/spf13/cobra"
)

const (
	defaultProvisioneStatefulName    = "openebs-nfs-provisioner"
	defaultNFSClusterRoleName        = "openebs-nfs-provisioner-runner"
	defaultStorageSize               = 5
	defaultNFSClusterRoleBindingName = "openebs-run-nfs-provisioner"
	defaultNFSRoleName               = "openebs-leader-locking-nfs-provisioner"
	defaultProvisionerName           = "openebs.io/nfs"
	defaultOpenebsPVCName            = "openebspvc"
	defaultNFSAccessMode             = "ReadWriteMany"
	defaultNFSStorageClass           = "openebs-nfs"
	defaultNFSPVCName                = "openebs-nfs-pvc"
	defaultOpenebsStorageClass       = "openebs-jiva-default"
)

const openebsNFSTemplate = `
apiVersion: extensions/v1beta1
kind: PodSecurityPolicy
metadata:
  name: {{ .ProvisionerStatefulName }}
spec:
  fsGroup:
    rule: RunAsAny
  allowedCapabilities:
  - DAC_READ_SEARCH
  - SYS_RESOURCE
  runAsUser:
    rule: RunAsAny
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  volumes:
  - configMap
  - downwardAPI
  - emptyDir
  - persistentVolumeClaim
  - secret
  - hostPath
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: {{ .NFSClusterRoleName }}
rules:
  - apiGroups: [""]
    resources: ["persistentvolumes"]
    verbs: ["get", "list", "watch", "create", "delete"]
  - apiGroups: [""]
    resources: ["persistentvolumeclaims"]
    verbs: ["get", "list", "watch", "update"]
  - apiGroups: ["storage.k8s.io"]
    resources: ["storageclasses"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "update", "patch"]
  - apiGroups: [""]
    resources: ["services", "endpoints"]
    verbs: ["get"]
  - apiGroups: ["extensions"]
    resources: ["podsecuritypolicies"]
    resourceNames: ["nfs-provisioner"]
    verbs: ["use"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: {{ .NFSClusterRoleBindingName }}
subjects:
  - kind: ServiceAccount
    name: {{ .ProvisionerStatefulName }}
     # replace with namespace where provisioner is deployed
    namespace: default
roleRef:
  kind: ClusterRole
  name: {{ .NFSClusterRoleName }}
  apiGroup: rbac.authorization.k8s.io
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: {{ .NFSRoleName }}
rules:
  - apiGroups: [""]
    resources: ["endpoints"]
    verbs: ["get", "list", "watch", "create", "update", "patch"]
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: {{ .NFSRoleName }}
subjects:
  - kind: ServiceAccount
    name: {{ .ProvisionerStatefulName }}
    # replace with namespace where provisioner is deployed
roleRef:
  kind: Role
  name: {{ .NFSRoleName }}
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .ProvisionerStatefulName }}
---
kind: Service
apiVersion: v1
metadata:
  name: {{ .ProvisionerStatefulName }}
  labels:
    app: {{ .ProvisionerStatefulName }}
spec:
  ports:
    - name: nfs
      port: 2049
    - name: mountd
      port: 20048
    - name: rpcbind
      port: 111
    - name: rpcbind-udp
      port: 111
      protocol: UDP
  selector:
    app: {{ .ProvisionerStatefulName }}
---
kind: Deployment
apiVersion: apps/v1
metadata:
  name: {{ .ProvisionerStatefulName }}
spec:
  selector:
    matchLabels:
      app: {{ .ProvisionerStatefulName }}
  replicas: 1
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: {{ .ProvisionerStatefulName }}
    spec:
      serviceAccount: {{ .ProvisionerStatefulName }}
      terminationGracePeriodSeconds: 10
      containers:
        - name: {{ .ProvisionerStatefulName }}
          image: quay.io/kubernetes_incubator/nfs-provisioner:latest
          ports:
            - name: nfs
              containerPort: 2049
            - name: mountd
              containerPort: 20048
            - name: rpcbind
              containerPort: 111
            - name: rpcbind-udp
              containerPort: 111
              protocol: UDP
          securityContext:
            capabilities:
              add:
                - DAC_READ_SEARCH
                - SYS_RESOURCE
          args:
            - "-provisioner={{ .ProvisionerName }}"
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: SERVICE_NAME
              value: {{ .ProvisionerStatefulName }}
            - name: POD_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
          imagePullPolicy: "IfNotPresent"
          volumeMounts:
            - name: export-volume
              mountPath: /export
      # volumes:
      #   - name: export-volume
      #     hostPath:
      #       path: /tmp
      volumes:
      - name: export-volume
        persistentVolumeClaim:
          claimName: {{ .OpenebsPVCName }}
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: {{ .OpenebsPVCName }}
spec:
  storageClassName: {{ .OpenebsStorageClass }}
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: "{{ .OpenebsStorageSize }}"
---
kind: StorageClass  
apiVersion: storage.k8s.io/v1
metadata:
  name: {{ .NFSStorageClass }}
provisioner: {{ .ProvisionerName }}
parameters:
  mountOptions: "vers=4.1"  # TODO: reconcile with StorageClass.mountOptions
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: {{ .NFSPVCName }}
  annotations:
    volume.beta.kubernetes.io/storage-class: "{{ .NFSStorageClass }}"
spec:
  accessModes:
    - {{ .NFSAccessMode }}
  resources:
    requests:
      storage: {{ .NFSStorageSize }}
`

type openebsNFS struct {
	ProvisionerStatefulName   string
	ApplicationName           string
	NFSClusterRoleName        string
	NFSStorageSize            string
	OpenebsStorageSize        string
	NFSClusterRoleBindingName string
	NFSRoleName               string
	ProvisionerName           string
	OpenebsPVCName            string
	NFSAccessMode             string
	NFSStorageClass           string
	NFSPVCName                string
	OpenebsStorageClass       string
	StorageSize               float64
}

var openNFS openebsNFS

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "onfs",
	Short: "onfs is a cli tool for generating preconfigured yamls to run nfs server on top of openebs",
	Long:  "onfs is a cli tool for generating preconfigured yamls to run nfs server on top of openebs",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		openebsStorageSize := fmt.Sprintf("%.2f", openNFS.StorageSize+0.1*openNFS.StorageSize) + "G"
		openNFS = openebsNFS{
			ApplicationName:           openNFS.ApplicationName,
			ProvisionerStatefulName:   openNFS.ApplicationName + "-" + defaultProvisioneStatefulName,
			NFSClusterRoleName:        openNFS.ApplicationName + "-" + defaultNFSClusterRoleName,
			StorageSize:               openNFS.StorageSize,
			NFSClusterRoleBindingName: openNFS.ApplicationName + "-" + defaultNFSClusterRoleBindingName,
			NFSRoleName:               openNFS.ApplicationName + "-" + defaultNFSRoleName,
			ProvisionerName:           openNFS.ApplicationName + defaultProvisionerName,
			OpenebsPVCName:            openNFS.ApplicationName + defaultOpenebsPVCName,
			NFSAccessMode:             defaultNFSAccessMode,
			NFSStorageClass:           openNFS.ApplicationName + "-" + defaultNFSStorageClass,
			NFSPVCName:                openNFS.ApplicationName + "-" + defaultNFSPVCName,
			OpenebsStorageClass:       openNFS.OpenebsStorageClass,
			NFSStorageSize:            fmt.Sprintf("%.2f", openNFS.StorageSize) + "G",
			OpenebsStorageSize:        openebsStorageSize,
		}
		generateTemplate()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	openNFS = openebsNFS{}
	RootCmd.PersistentFlags().StringVarP(&openNFS.ApplicationName, "appname", "a", "", "name of the application that has to be deployed on NFS")
	RootCmd.PersistentFlags().Float64VarP(&openNFS.StorageSize, "size", "s", defaultStorageSize, "space size to needed in G")
	RootCmd.PersistentFlags().StringVarP(&openNFS.OpenebsStorageClass, "openebsstorageclass", "c", defaultOpenebsStorageClass, "storageclass of openebs")
	RootCmd.MarkPersistentFlagRequired("appname")
}

func generateTemplate() {
	oNFSTeplate, err := template.New("master").Parse(openebsNFSTemplate)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	file, err := os.Create(openNFS.ApplicationName + "-nfs.yaml")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := oNFSTeplate.Execute(file, openNFS); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
