package k3s

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"text/template"

	"hetzner-k3s/internal/shell"
)

func (c *Client) deployHetznerCloudControllerManager() error {
	err := c.checkKubectl()
	if err != nil {
		return err
	}

	c.logger.Info("Deploying Hetzner Cloud Controller Manager...")

	err = shell.NewClient(c.logger).RunCommand(
		c.hetznerCCMSecretData(), shell.WithEnv("KUBECONFIG="+c.kubeconfigPath()),
	)
	if err != nil {
		return fmt.Errorf("cannot deploy hcloud cloud controller manager secret: %w", err)
	}

	err = shell.NewClient(c.logger).RunCommand(
		c.hetznerCCMData(), shell.WithEnv("KUBECONFIG="+c.kubeconfigPath()),
	)
	if err != nil {
		return fmt.Errorf("cannot deploy hcloud cloud controller manager: %w", err)
	}

	c.logger.Info("...Hetzner Cloud Controller Manager deployed")

	return nil
}

func (c *Client) hetznerCCMSecretData() string {
	var buffer bytes.Buffer

	network := c.cluster.ClusterName
	if c.cluster.ExistingNetwork != "" {
		network = c.cluster.ExistingNetwork
	}

	tpldata := make(map[string]interface{})
	tpldata["NetworkName"] = network
	tpldata["HCloudToken"] = c.cluster.HetznerToken

	tpl := `kubectl apply -f - <<-EOF
apiVersion: "v1"
kind: "Secret"
metadata:
  namespace: 'kube-system'
  name: 'hcloud'
stringData:
  network: "{{ .NetworkName }}"
  token: "{{ .HCloudToken }}"
EOF`

	t := template.Must(template.New("tpl").Parse(tpl))

	err := t.Execute(&buffer, tpldata)
	if err != nil {
		panic(err)
	}

	return buffer.String()
}

func (c *Client) hetznerCCMData() string {
	return `kubectl apply -f https://github.com/hetznercloud/hcloud-cloud-controller-manager/releases/latest/download/ccm-networks.yaml`
}

func (c *Client) deployK3sSystemUpgradeController() error {
	err := c.checkKubectl()
	if err != nil {
		return err
	}

	c.logger.Info("Deploying k3s System Upgrade Controller...")

	err = shell.NewClient(c.logger).RunCommand(
		c.k3sSystemUpgradeControllerData(), shell.WithEnv("KUBECONFIG="+c.kubeconfigPath()),
	)
	if err != nil {
		return fmt.Errorf("cannot deploy system upgrade controller: %w", err)
	}

	c.logger.Info("...k3s System Upgrade Controller deployed")

	return nil
}

func (c *Client) deployHetznerCSIDriver() error {
	err := c.checkKubectl()
	if err != nil {
		return err
	}

	c.logger.Info("Deploying Hetzner CSI Driver...")

	err = shell.NewClient(c.logger).RunCommand(
		c.hetznerCSISecretData(), shell.WithEnv("KUBECONFIG="+c.kubeconfigPath()),
	)
	if err != nil {
		return fmt.Errorf("cannot deploy hetzner csi driver secret: %w", err)
	}

	err = shell.NewClient(c.logger).RunCommand(
		c.hetznerCSIDriverDataDelete(), shell.WithEnv("KUBECONFIG="+c.kubeconfigPath()),
	)
	if err != nil {
		return fmt.Errorf("cannot deploy hetzner csi driver: %w", err)
	}

	err = shell.NewClient(c.logger).RunCommand(
		c.hetznerCSIDriverData(), shell.WithEnv("KUBECONFIG="+c.kubeconfigPath()),
	)
	if err != nil {
		return fmt.Errorf("cannot deploy hetzner csi driver: %w", err)
	}

	c.logger.Info("...CSI Driver deployed")

	if c.cluster.ScheduleCSIControllerOnMaster {
		c.logger.Info("Updating Hetzner CSI Controller...")

		err = shell.NewClient(c.logger).RunCommand(
			c.hetznerCSIControllerUpdateData(), shell.WithEnv("KUBECONFIG="+c.kubeconfigPath()),
		)
		if err != nil {
			return fmt.Errorf("cannot update hetzner csi controller: %w", err)
		}

		c.logger.Info("...CSI Controller updated.")
	}

	return nil
}

func (c *Client) hetznerCSISecretData() string {
	var buffer bytes.Buffer

	tpldata := make(map[string]interface{})
	tpldata["HCloudToken"] = c.cluster.HetznerToken

	tpl := `kubectl apply -f - <<-EOF
apiVersion: "v1"
kind: "Secret"
metadata:
  namespace: 'kube-system'
  name: 'hcloud-csi'
stringData:
  token: "{{ .HCloudToken }}"
EOF`

	t := template.Must(template.New("tpl").Parse(tpl))

	err := t.Execute(&buffer, tpldata)
	if err != nil {
		panic(err)
	}

	return buffer.String()
}

func (c *Client) hetznerCSIDriverData() string {
	resp, err := http.Get(
		`https://raw.githubusercontent.com/hetznercloud/csi-driver/master/deploy/kubernetes/hcloud-csi.yml`,
	)
	if err != nil {
		c.logger.Sugar().Fatal("download failed: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Sugar().Fatal("bad status: %s", resp.Status)
	}

	var buffer bytes.Buffer

	_, err = io.Copy(&buffer, resp.Body)
	if err != nil {
		c.logger.Sugar().Fatal("cannot copy response to buffer: %s", err)
	}

	yaml := buffer.String()

	if !c.cluster.HCloudVolumeIsDefaultStorageClass {
		re := regexp.MustCompile(`storageclass.kubernetes.io/is-default-class.*`)
		yaml = re.ReplaceAllString(yaml, `storageclass.kubernetes.io/is-default-class: "false"`)
	}

	return fmt.Sprintf("kubectl apply -f - <<-EOF\n%s\nEOF", yaml)
}

// Upgraded CSI Controller data to make it possible to run it on master nodes.
func (c *Client) hetznerCSIControllerUpdateData() string {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: hcloud-csi-controller
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: hcloud-csi-controller
  template:
    metadata:
      labels:
        app: hcloud-csi-controller
    spec:
      tolerations:
        - effect: NoSchedule
          key: node-role.kubernetes.io/master
        - key: "CriticalAddonsOnly"
          operator: "Equal"
          value: "true"
          effect: "NoExecute"
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node-role.kubernetes.io/master
                    operator: Exists
      containers:
      - image: k8s.gcr.io/sig-storage/csi-attacher:v3.2.1
        name: csi-attacher
        volumeMounts:
        - mountPath: /run/csi
          name: socket-dir
      - image: k8s.gcr.io/sig-storage/csi-resizer:v1.2.0
        name: csi-resizer
        volumeMounts:
        - mountPath: /run/csi
          name: socket-dir
      - args:
        - --feature-gates=Topology=true
        - --default-fstype=ext4
        image: k8s.gcr.io/sig-storage/csi-provisioner:v2.2.2
        name: csi-provisioner
        volumeMounts:
        - mountPath: /run/csi
          name: socket-dir
      - command:
        - /bin/hcloud-csi-driver-controller
        env:
        - name: CSI_ENDPOINT
          value: unix:///run/csi/socket
        - name: METRICS_ENDPOINT
          value: 0.0.0.0:9189
        - name: ENABLE_METRICS
          value: "true"
        - name: KUBE_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        - name: HCLOUD_TOKEN
          valueFrom:
            secretKeyRef:
              key: token
              name: hcloud
        image: hetznercloud/hcloud-csi-driver:latest
        imagePullPolicy: Always
        livenessProbe:
          failureThreshold: 5
          httpGet:
            path: /healthz
            port: healthz
          initialDelaySeconds: 10
          periodSeconds: 2
          timeoutSeconds: 3
        name: hcloud-csi-driver
        ports:
        - containerPort: 9189
          name: metrics
        - containerPort: 9808
          name: healthz
          protocol: TCP
        volumeMounts:
        - mountPath: /run/csi
          name: socket-dir
      - image: k8s.gcr.io/sig-storage/livenessprobe:v2.3.0
        imagePullPolicy: Always
        name: liveness-probe
        volumeMounts:
        - mountPath: /run/csi
          name: socket-dir
      serviceAccountName: hcloud-csi-controller
      volumes:
      - emptyDir: {}
        name: socket-dir`

	return fmt.Sprintf("kubectl apply -f - <<-EOF\n%s\nEOF", yaml)
}

// It has to be deleted before applying because fsGroupPolicy is an immutable field.
func (c *Client) hetznerCSIDriverDataDelete() string {
	yaml := `apiVersion: storage.k8s.io/v1
kind: CSIDriver
metadata:
  name: csi.hetzner.cloud
spec:
  attachRequired: true
  podInfoOnMount: true
  volumeLifecycleModes:
  - Persistent
  fsGroupPolicy: File`

	return fmt.Sprintf("kubectl delete --ignore-not-found=true -f - <<-EOF\n%s\nEOF", yaml)
}

func (c *Client) deployHetznerClusterAutoscaler() error {
	err := c.checkKubectl()
	if err != nil {
		return err
	}

	c.logger.Info("Deploying Hetzner Autoscaler...")

	err = shell.NewClient(c.logger).RunCommand(
		c.hetznerClusterAutoscalerSecretData(), shell.WithEnv("KUBECONFIG="+c.kubeconfigPath()),
	)
	if err != nil {
		return fmt.Errorf("cannot deploy hetzner autoscaler secret: %w", err)
	}

	err = shell.NewClient(c.logger).RunCommand(
		c.hetznerClusterAutoscalerData(), shell.WithEnv("KUBECONFIG="+c.kubeconfigPath()),
	)
	if err != nil {
		return fmt.Errorf("cannot deploy hetzner autoscalerr: %w", err)
	}

	c.logger.Info("...Autoscaler deployed")

	return nil
}

func (c *Client) hetznerClusterAutoscalerSecretData() string {
	var buffer bytes.Buffer

	tpldata := make(map[string]interface{})
	tpldata["HCloudToken"] = c.cluster.HetznerToken

	tpl := `kubectl apply -f - <<-EOF
apiVersion: "v1"
kind: "Secret"
metadata:
  namespace: 'kube-system'
  name: 'hcloud-cluster-autoscaler'
stringData:
  token: "{{ .HCloudToken }}"
EOF`

	t := template.Must(template.New("tpl").Parse(tpl))

	err := t.Execute(&buffer, tpldata)
	if err != nil {
		panic(err)
	}

	return buffer.String()
}

func (c *Client) hetznerClusterAutoscalerData() string { // nolint: funlen
	var buffer bytes.Buffer

	tpldata := make(map[string]interface{})
	tpldata["NodePoolArgs"] = c.autoscalerNodePoolArgs()
	tpldata["SSHKey"] = c.sshKey().Name
	tpldata["Network"] = c.network().Name
	tpldata["Firewall"] = c.firewall().Name
	tpldata["Image"] = c.image().Name
	tpldata["CloudInit"] = c.autoscalerCloudInit()
	tpldata["ClusterAutoscalerArgs"] = c.clusterAutoscalerArgs()

	tpl := `kubectl apply -f - <<-EOF
---
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    k8s-addon: cluster-autoscaler.addons.k8s.io
    k8s-app: cluster-autoscaler
  name: cluster-autoscaler
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cluster-autoscaler
  labels:
    k8s-addon: cluster-autoscaler.addons.k8s.io
    k8s-app: cluster-autoscaler
rules:
  - apiGroups: [""]
    resources: ["events", "endpoints"]
    verbs: ["create", "patch"]
  - apiGroups: [""]
    resources: ["pods/eviction"]
    verbs: ["create"]
  - apiGroups: [""]
    resources: ["pods/status"]
    verbs: ["update"]
  - apiGroups: [""]
    resources: ["endpoints"]
    resourceNames: ["cluster-autoscaler"]
    verbs: ["get", "update"]
  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["watch", "list", "get", "update"]
  - apiGroups: [""]
    resources:
      - "namespaces"
      - "pods"
      - "services"
      - "replicationcontrollers"
      - "persistentvolumeclaims"
      - "persistentvolumes"
    verbs: ["watch", "list", "get"]
  - apiGroups: ["extensions"]
    resources: ["replicasets", "daemonsets"]
    verbs: ["watch", "list", "get"]
  - apiGroups: ["policy"]
    resources: ["poddisruptionbudgets"]
    verbs: ["watch", "list"]
  - apiGroups: ["apps"]
    resources: ["statefulsets", "replicasets", "daemonsets"]
    verbs: ["watch", "list", "get"]
  - apiGroups: ["storage.k8s.io"]
    resources: ["storageclasses", "csinodes", "csistoragecapacities", "csidrivers"]
    verbs: ["watch", "list", "get"]
  - apiGroups: ["batch", "extensions"]
    resources: ["jobs"]
    verbs: ["get", "list", "watch", "patch"]
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["create"]
  - apiGroups: ["coordination.k8s.io"]
    resourceNames: ["cluster-autoscaler"]
    resources: ["leases"]
    verbs: ["get", "update"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cluster-autoscaler
  namespace: kube-system
  labels:
    k8s-addon: cluster-autoscaler.addons.k8s.io
    k8s-app: cluster-autoscaler
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["create","list","watch"]
  - apiGroups: [""]
    resources: ["configmaps"]
    resourceNames: ["cluster-autoscaler-status", "cluster-autoscaler-priority-expander"]
    verbs: ["delete", "get", "update", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cluster-autoscaler
  labels:
    k8s-addon: cluster-autoscaler.addons.k8s.io
    k8s-app: cluster-autoscaler
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-autoscaler
subjects:
  - kind: ServiceAccount
    name: cluster-autoscaler
    namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: cluster-autoscaler
  namespace: kube-system
  labels:
    k8s-addon: cluster-autoscaler.addons.k8s.io
    k8s-app: cluster-autoscaler
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: cluster-autoscaler
subjects:
  - kind: ServiceAccount
    name: cluster-autoscaler
    namespace: kube-system
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cluster-autoscaler
  namespace: kube-system
  labels:
    app: cluster-autoscaler
spec:
  replicas: 1
  selector:
    matchLabels:
      app: cluster-autoscaler
  template:
    metadata:
      labels:
        app: cluster-autoscaler
      annotations:
        prometheus.io/scrape: 'true'
        prometheus.io/port: '8085'
    spec:
      serviceAccountName: cluster-autoscaler
      tolerations:
        - effect: NoSchedule
          key: node-role.kubernetes.io/master
        - key: "CriticalAddonsOnly"
          operator: "Equal"
          value: "true"
          effect: "NoExecute"
      # Node affinity is used to force cluster-autoscaler to stick
      # to the master node. This allows the cluster to reliably downscale
      # to zero worker nodes when needed.
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node-role.kubernetes.io/master
                    operator: Exists
      containers:
        - image: k8s.gcr.io/autoscaling/cluster-autoscaler:v1.23.0  # or your custom image
          name: cluster-autoscaler
          resources:
            limits:
              cpu: 100m
              memory: 300Mi
            requests:
              cpu: 100m
              memory: 300Mi
          command:
            - ./cluster-autoscaler
            - --cloud-provider=hetzner
{{ range .ClusterAutoscalerArgs }}
            - {{ . }}
{{ end }}
{{ range .NodePoolArgs }}
            - {{ . }}
{{ end }}
          env:
          - name: HCLOUD_TOKEN
            valueFrom:
                secretKeyRef:
                  name: hcloud-cluster-autoscaler
                  key: token
          - name: HCLOUD_CLOUD_INIT
            value: {{ .CloudInit }}
          - name: HCLOUD_IMAGE
            value: "{{ .Image }}"
          - name: HCLOUD_FIREWALL
            value: "{{ .Firewall }}"
          - name: HCLOUD_SSH_KEY
            value: "{{ .SSHKey }}"
          - name: HCLOUD_NETWORK
            value: "{{ .Network }}"
          volumeMounts:
            - name: ssl-certs
              mountPath: /etc/ssl/certs/ca-certificates.crt
              readOnly: true
          imagePullPolicy: "Always"
      imagePullSecrets:
        - name: gitlab-registry
      volumes:
        - name: ssl-certs
          hostPath:
            path: "/etc/ssl/certs/ca-certificates.crt"
EOF`

	t := template.Must(template.New("tpl").Parse(tpl))

	err := t.Execute(&buffer, tpldata)
	if err != nil {
		panic(err)
	}

	return buffer.String()
}

func (c *Client) autoscalerNodePoolArgs() []string {
	pools := make([]string, 0, len(c.cluster.AutoscalingNodePools))

	for _, pool := range c.cluster.AutoscalingNodePools {
		if pool.Location == "" {
			pool.Location = c.masterLocation().Name
		}

		pools = append(
			pools, fmt.Sprintf(
				"--nodes=%d:%d:%s:%s:%s", pool.InstanceMin, pool.InstanceMax,
				strings.ToUpper(pool.InstanceType),
				strings.ToUpper(pool.Location),
				fmt.Sprintf(
					"%s-%s-pool-%s-as", c.clusterName(), pool.InstanceType,
					pool.Name,
				),
			),
		)
	}

	return pools
}
