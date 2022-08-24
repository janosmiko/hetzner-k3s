package k3s

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"text/template"

	"github.com/hashicorp/go-version"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"golang.org/x/sync/errgroup"

	clusterpkg "hetzner-k3s/internal/cluster"
	configpkg "hetzner-k3s/internal/config"
	hetznerpkg "hetzner-k3s/internal/hetzner"
	loggerpkg "hetzner-k3s/internal/logger"
	sshpkg "hetzner-k3s/internal/ssh"
)

type Client struct {
	logger    *loggerpkg.Logger
	cluster   *clusterpkg.Cluster
	hetzner   *hetznerpkg.Client
	sshclient *sshpkg.Client
}

func NewEmptyClient() *Client {
	logger := loggerpkg.InitLogger()

	return &Client{
		logger: logger,
	}
}

func NewClient() *Client {
	config := configpkg.InitConfig()
	logger := loggerpkg.InitLogger()
	cluster := clusterpkg.NewCluster(config)

	c := Client{
		logger:  logger,
		cluster: cluster,
		hetzner: hetznerpkg.NewClient(logger, cluster.HetznerToken),
	}

	// Set ssh client after the k3s client is ready, so we can resolve the privateSSHKeyPath if it contains "~".
	c.sshclient = sshpkg.NewClient(logger, "root", c.privateSSHKeyPath()).
		SetTimeout(30).
		SetPassphrase(cluster.PrivateSSHKeyPassphrase).
		SetPrintOutput(true).
		SetVerifyHostKey(cluster.VerifyHostKey)

	return &c
}

func (c *Client) k3sVersion() string {
	if c.cluster.K3sVersion == "" {
		return c.latestVersion("")
	}

	return c.cluster.K3sVersion
}

func (c *Client) k3sToken() (token string) {
	if c.cluster.K3sToken != "" {
		return c.cluster.K3sToken
	}

	token, err := c.sshclient.SSHCommand(
		c.firstMasterPublicIP(),
		"{ TOKEN=$(< /var/lib/rancher/k3s/server/node-token); } 2> /dev/null; echo $TOKEN",
	)
	if err != nil {
		panic(err)
	}

	token = strings.TrimSpace(token)

	if token == "" {
		token = randomHex(32)
	} else {
		s := strings.Split(token, ":")
		token = s[len(s)-1]
	}

	c.cluster.K3sToken = token

	return c.cluster.K3sToken
}

func randomHex(n int) string {
	bs := make([]byte, n)

	_, err := rand.Read(bs)
	if err != nil {
		panic(err)
	}

	return hex.EncodeToString(bs)[:n]
}

func (c *Client) k3sServerUpgradeData() string {
	var buffer bytes.Buffer

	tpldata := make(map[string]interface{})
	tpldata["K3sVersion"] = c.k3sVersion()

	tpl := `kubectl apply -f - <<-EOF
apiVersion: upgrade.cattle.io/v1
kind: Plan
metadata:
  name: k3s-server
  namespace: system-upgrade
  labels:
    k3s-upgrade: server
spec:
  concurrency: 1
  version: {{ .K3sVersion }}
  nodeSelector:
    matchExpressions:
      - {key: node-role.kubernetes.io/master, operator: In, values: ["true"]}
  serviceAccountName: system-upgrade
  tolerations:
  - key: "CriticalAddonsOnly"
    operator: "Equal"
    value: "true"
    effect: "NoExecute"
  cordon: true
  upgrade:
    image: rancher/k3s-upgrade
EOF`

	// nolint: varnamelen
	t := template.Must(template.New("tpl").Parse(tpl))

	err := t.Execute(&buffer, tpldata)
	if err != nil {
		panic(err)
	}

	return buffer.String()
}

func (c *Client) k3sAgentUpgradeData() string {
	var buffer bytes.Buffer

	workerUpgradeConcurrency := len(c.cluster.WorkerInstances) - 1
	if workerUpgradeConcurrency <= 0 {
		workerUpgradeConcurrency = 1
	}

	tpldata := make(map[string]interface{})
	tpldata["WorkerUpgradeConcurrency"] = workerUpgradeConcurrency
	tpldata["K3sVersion"] = c.k3sVersion()

	tpl := `kubectl apply -f - <<-EOF
apiVersion: upgrade.cattle.io/v1
kind: Plan
metadata:
  name: k3s-agent
  namespace: system-upgrade
  labels:
    k3s-upgrade: agent
spec:
  concurrency: {{ .WorkerUpgradeConcurrency }}
  version: {{ .K3sVersion }}
  nodeSelector:
    matchExpressions:
      - {key: node-role.kubernetes.io/master, operator: NotIn, values: ["true"]}
  serviceAccountName: system-upgrade
  prepare:
    image: rancher/k3s-upgrade
    args: ["prepare", "k3s-server"]
  cordon: true
  upgrade:
    image: rancher/k3s-upgrade
EOF`

	t := template.Must(template.New("tpl").Parse(tpl))

	err := t.Execute(&buffer, tpldata)
	if err != nil {
		panic(err)
	}

	return buffer.String()
}

func (c *Client) deployK3s() error {
	err := c.deployK3sOnMaster(c.cluster.MasterInstances[0], true)
	if err != nil {
		return err
	}

	c.saveKubeconfig()

	if c.mastersSize() > 1 {
		errGroup, _ := errgroup.WithContext(context.Background())

		for i := 1; i < c.mastersSize(); i++ {
			master := c.cluster.MasterInstances[i]

			errGroup.Go(
				func() error {
					err := c.deployK3sOnMaster(master, false)
					if err != nil {
						return err
					}

					return nil
				},
			)
		}

		if err := errGroup.Wait(); err != nil {
			return fmt.Errorf("an error occurred: %w", err)
		}
	}

	errGroup, _ := errgroup.WithContext(context.Background())

	for _, worker := range c.workers() {
		worker := worker

		errGroup.Go(
			func() error {
				err := c.deployK3sOnWorker(worker)
				if err != nil {
					return err
				}

				return nil
			},
		)
	}

	if err := errGroup.Wait(); err != nil {
		return fmt.Errorf("an error occurred: %w", err)
	}

	return nil
}

func (c *Client) masterScript(master *hcloud.Server) string {
	var buffer bytes.Buffer

	server := ""
	if c.isFirstMaster(master) {
		server = "--cluster-init"
	} else {
		server = fmt.Sprintf("--server https://%s:6443", c.apiServerIP())
	}

	tpldata := make(map[string]interface{})
	tpldata["Server"] = server
	tpldata["TLSSans"] = c.tlsSans()
	tpldata["K3sVersion"] = c.k3sVersion()
	tpldata["K3sToken"] = c.k3sToken()
	tpldata["ExtraArgs"] = fmt.Sprintf(
		"%s %s %s %s %s %s",
		c.kubeAPIServerArgsList(),
		c.kubeSchedulerArgsList(),
		c.kubeControllerManagerArgsList(),
		c.kubeCloudControllerManagerArgsList(),
		c.kubeletArgsList(),
		c.kubeProxyArgsList(),
	)

	wireguardNativeMinVersion, _ := version.NewVersion("v1.23.6+k3s1")
	k3sVersion, _ := version.NewVersion(c.cluster.K3sVersion)

	if c.cluster.EnableEncryption {
		if k3sVersion.GreaterThanOrEqual(wireguardNativeMinVersion) {
			tpldata["FlannelWireguard"] = "--flannel-backend=wireguard-native"
		} else {
			tpldata["FlannelWireguard"] = "--flannel-backend=wireguard"
		}
	} else {
		tpldata["FlannelWireguard"] = ""
	}

	if c.scheduleWorkloadsOnMasters() {
		tpldata["Taint"] = ""
	} else {
		tpldata["Taint"] = "--node-taint CriticalAddonsOnly=true:NoExecute"
	}

	tpl := `if lscpu | grep Vendor | grep -q Intel; then export FLANNEL_INTERFACE=ens10 ; else export FLANNEL_INTERFACE=enp7s0 ; fi && \
curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="{{ .K3sVersion }}" K3S_TOKEN="{{ .K3sToken }}" INSTALL_K3S_EXEC="server \
--disable-cloud-controller \
--disable servicelb \
--disable traefik \
--disable local-storage \
--disable metrics-server \
--write-kubeconfig-mode=644 \
--node-name="$(hostname -f)" \
--cluster-cidr=10.244.0.0/16 \
--etcd-expose-metrics=true \
{{ .FlannelWireguard }} \
--kube-controller-manager-arg="bind-address=0.0.0.0" \
--kube-proxy-arg="metrics-bind-address=0.0.0.0" \
--kube-scheduler-arg="bind-address=0.0.0.0" \
{{ .Taint }} {{ .ExtraArgs }} \
--kubelet-arg="cloud-provider=external" \
--advertise-address=$(hostname -I | awk '{print $2}') \
--node-ip=$(hostname -I | awk '{print $2}') \
--node-external-ip=$(hostname -I | awk '{print $1}') \
--flannel-iface=$FLANNEL_INTERFACE \
{{ .Server }} {{ .TLSSans }}" sh -`

	t := template.Must(template.New("tpl").Parse(tpl))

	err := t.Execute(&buffer, tpldata)
	if err != nil {
		panic(err)
	}

	return buffer.String()
}

func (c *Client) workerScript() string {
	var buffer bytes.Buffer

	tpldata := make(map[string]interface{})
	tpldata["K3sVersion"] = c.k3sVersion()
	tpldata["K3sToken"] = c.k3sToken()
	tpldata["FirstMasterPrivateIP"] = c.firstMasterPrivateIP()

	tpl := `if lscpu | grep Vendor | grep -q Intel; then export FLANNEL_INTERFACE=ens10 ; else export FLANNEL_INTERFACE=enp7s0 ; fi && \
curl -sfL https://get.k3s.io | K3S_TOKEN="{{ .K3sToken }}" INSTALL_K3S_VERSION="{{ .K3sVersion }}" K3S_URL=https://{{ .FirstMasterPrivateIP }}:6443 INSTALL_K3S_EXEC="agent \
--node-name="$(hostname -f)" \
--kubelet-arg="cloud-provider=external" \
--node-ip=$(hostname -I | awk '{print $2}') \
--node-external-ip=$(hostname -I | awk '{print $1}') \
--flannel-iface=$FLANNEL_INTERFACE" sh -`

	t := template.Must(template.New("tpl").Parse(tpl))

	err := t.Execute(&buffer, tpldata)
	if err != nil {
		panic(err)
	}

	return buffer.String()
}

func (c *Client) k3sSystemUpgradeControllerData() string {
	return "kubectl apply -f https://github.com/rancher/system-upgrade-controller/releases/download/v0.9.1/system-upgrade-controller.yaml"
}

func (c *Client) deployK3sOnMaster(master *hcloud.Server, isFirst bool) error {
	first := ""
	if isFirst {
		first = "first "
	}

	c.logger.Sugar().Infof("Deploying k3s to %smaster %s...", first, master.Name)

	out, err := c.sshclient.SSHCommand(
		master.PublicNet.IPv4.IP.String(), c.masterScript(master),
	)
	if err != nil {
		return fmt.Errorf("cannot deploy kubernetes on %smaster: error: %w, output: %s", first, err, out)
	}

	c.logger.Sugar().Infof("...k3s has been deployed to %smaster %s.", first, master.Name)

	return nil
}

func (c *Client) deployK3sOnWorker(worker *hcloud.Server) error {
	c.logger.Sugar().Infof("Deploying k3s to worker %s...", worker.Name)

	out, err := c.sshclient.SSHCommand(
		worker.PublicNet.IPv4.IP.String(), c.workerScript(),
	)
	if err != nil {
		return fmt.Errorf("cannot deploy kubernetes on worker: error: %w, output: %s", err, out)
	}

	c.logger.Sugar().Infof("...k3s has been deployed to worker %s.", worker.Name)

	return nil
}
