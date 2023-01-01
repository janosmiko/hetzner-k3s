package k3s

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/avast/retry-go"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"golang.org/x/sync/errgroup"

	configpkg "hetzner-k3s/internal/config"
	"hetzner-k3s/internal/shell"
)

func (c *Client) CreateCluster() error {
	c.logger.Info("Creating cluster...")

	err := c.validate("create")
	if err != nil {
		return err
	}

	err = c.createResources()
	if err != nil {
		return err
	}

	err = c.deployK3s()
	if err != nil {
		return err
	}

	err = c.deployHetznerCloudControllerManager()
	if err != nil {
		return err
	}

	err = c.deployHetznerCSIDriver()
	if err != nil {
		return err
	}

	err = c.deployK3sSystemUpgradeController()
	if err != nil {
		return err
	}

	if len(c.cluster.AutoscalingNodePools) > 0 {
		err = c.deployHetznerClusterAutoscaler()
		if err != nil {
			return err
		}
	}

	c.logger.Info("...cluster created.")

	return nil
}

func (c *Client) UpgradeCluster() error {
	c.logger.Info("Upgrading cluster...")

	err := c.validate("upgrade")
	if err != nil {
		return err
	}

	err = shell.NewClient(c.logger).RunCommand(c.k3sServerUpgradeData())
	if err != nil {
		return fmt.Errorf("cannot upgrade k3server: %w", err)
	}

	err = shell.NewClient(c.logger).RunCommand(c.k3sAgentUpgradeData())
	if err != nil {
		return fmt.Errorf("cannot upgrade k3server: %w", err)
	}

	c.logger.Info("Upgrade will now start. Run `watch kubectl get nodes` to see the nodes being upgraded. This should take a few minutes for a small cluster.")
	c.logger.Info("The API server may be briefly unavailable during the upgrade of the controlplane.")

	return nil
}

func (c *Client) DeleteCluster() error {
	c.logger.Info("Deleting cluster...")

	err := c.validate("delete")
	if err != nil {
		return err
	}

	ctx := context.Background()

	err = c.hetzner.DeleteLoadBalancer(ctx, c.loadBalancer())
	if err != nil {
		c.logger.Sugar().Error(err)
	}

	err = c.hetzner.DeleteSSHKey(ctx, c.sshKey())
	if err != nil {
		c.logger.Sugar().Error(err)
	}

	err = c.deleteServers(ctx, c.allServers())
	if err != nil {
		c.logger.Sugar().Error(err)
	}

	err = c.deletePlacementGroups(ctx, c.placementGroups())
	if err != nil {
		c.logger.Sugar().Error(err)
	}

	time.Sleep(time.Second * 1)

	err = c.hetzner.DeleteNetwork(ctx, c.network())
	if err != nil {
		c.logger.Sugar().Error(err)
	}

	time.Sleep(time.Second * 1)

	err = c.hetzner.DeleteFirewall(ctx, c.firewall())
	if err != nil {
		c.logger.Sugar().Error(err)
	}

	c.logger.Info("...cluster deleted.")

	return nil
}

func (c *Client) ListServers() error {
	// nolint: wrapcheck
	servers, err := c.hetzner.GetAllServers(c.cluster)
	if err != nil {
		return fmt.Errorf("cannot get all servers: %w", err)
	}

	for _, server := range servers {
		c.logger.Sugar().Info(server.Name)
	}

	return nil
}

func (c *Client) createResources() error {
	ctx := context.Background()

	err := c.createServers(ctx)
	if err != nil {
		return fmt.Errorf("cannot create servers: %w", err)
	}

	if c.mastersSize() > 1 {
		_ = c.loadBalancer()
	}

	return nil
}

func (c *Client) createServers(ctx context.Context) error {
	var (
		mu          = &sync.Mutex{} // nolint: varnamelen
		configs     = c.serverConfigs()
		servers     = make([]*hcloud.Server, 0, len(configs))
		errGroup, _ = errgroup.WithContext(ctx)
	)

	for _, server := range configs {
		server := server

		errGroup.Go(
			func() error {
				currentServer, err := c.hetzner.CreateServer(ctx, server)
				if err != nil {
					return fmt.Errorf("cannot create server: %w", err)
				}

				mu.Lock()
				servers = append(servers, currentServer)
				mu.Unlock()

				return nil
			},
		)
	}

	if err := errGroup.Wait(); err != nil {
		return fmt.Errorf("an error occurred: %w", err)
	}

	for _, server := range servers {
		server := server

		errGroup.Go(
			func() error {
				err := c.waitForSSH(server)
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

	sort.Slice(
		servers, func(i, j int) bool {
			return servers[i].Name < servers[j].Name
		},
	)

	c.cluster.Instances = servers
	_ = c.masters()
	_ = c.workers()

	return nil
}

func (c *Client) deleteServers(ctx context.Context, servers []*hcloud.Server) error {
	for _, server := range servers {
		err := c.hetzner.DeleteServer(ctx, server)
		if err != nil {
			return fmt.Errorf("cannot delete server: %w", err)
		}
	}

	return nil
}

func (c *Client) deletePlacementGroups(ctx context.Context, placementGroups []*hcloud.PlacementGroup) error {
	for _, pg := range placementGroups {
		err := c.hetzner.DeletePlacementGroup(ctx, pg)
		if err != nil {
			return fmt.Errorf("cannot delete placement group: %w", err)
		}
	}

	return nil
}

func (c *Client) serverConfigs() []hcloud.ServerCreateOpts {
	serverConfigs := c.masterDefinitionsForCreate()

	for _, pool := range c.cluster.WorkerNodePools {
		pool := pool
		serverConfigs = append(serverConfigs, c.workerNodePoolDefinitions(&pool)...)
	}

	return serverConfigs
}

func (c *Client) masterDefinitionsForCreate() []hcloud.ServerCreateOpts {
	masterDefinitions := make([]hcloud.ServerCreateOpts, 0, c.mastersSize())

	for i := 0; i < c.mastersSize(); i++ {
		masterDefinitions = append(
			masterDefinitions, hcloud.ServerCreateOpts{
				Name: fmt.Sprintf(
					"%s-%s-master%d", c.clusterName(), c.masterInstanceType(), i+1,
				),
				ServerType:     &hcloud.ServerType{Name: c.masterInstanceType()},
				SSHKeys:        []*hcloud.SSHKey{c.sshKey()},
				Networks:       []*hcloud.Network{c.network()},
				Firewalls:      []*hcloud.ServerCreateFirewall{{Firewall: *c.firewall()}},
				PlacementGroup: c.placementGroup(c.cluster.ClusterName),
				Image:          c.image(),
				Location:       c.masterLocation(),
				UserData:       c.userData(),
				Labels: map[string]string{
					"cluster": c.clusterName(),
					"role":    "master",
				},
			},
		)
	}

	return masterDefinitions
}

func (c *Client) workerNodePoolDefinitions(workerNodePool *configpkg.WorkerConfig) []hcloud.ServerCreateOpts {
	if workerNodePool.Location == "" {
		workerNodePool.Location = c.masterLocation().Name
	}

	workerDefinitions := make([]hcloud.ServerCreateOpts, 0, workerNodePool.InstanceCount)

	for i := 0; i < workerNodePool.InstanceCount; i++ {
		workerDefinitions = append(
			workerDefinitions, hcloud.ServerCreateOpts{
				Name: fmt.Sprintf(
					"%s-%s-pool-%s-worker%d", c.clusterName(), workerNodePool.InstanceType,
					workerNodePool.Name, i+1,
				),
				ServerType: &hcloud.ServerType{Name: c.masterInstanceType()},
				SSHKeys:    []*hcloud.SSHKey{c.sshKey()},
				Networks:   []*hcloud.Network{c.network()},
				Firewalls:  []*hcloud.ServerCreateFirewall{{Firewall: *c.firewall()}},
				PlacementGroup: c.placementGroup(
					fmt.Sprintf(
						"%s-%s", c.cluster.ClusterName, workerNodePool.Name,
					),
				),
				Image:    c.image(),
				Location: c.mustLocation(workerNodePool.Location),
				UserData: c.userData(),
				Labels: map[string]string{
					"cluster": c.clusterName(),
					"role":    "master",
				},
			},
		)
	}

	return workerDefinitions
}

func (c *Client) kubeAPIServerArgsList() string {
	if len(c.cluster.KubeAPIServerArgs) == 0 {
		return ``
	}

	res := ``
	for _, v := range c.cluster.KubeAPIServerArgs {
		res += fmt.Sprintf(`--kube-apiserver-arg="%s" `, v)
	}

	return res
}

func (c *Client) kubeSchedulerArgsList() string {
	if len(c.cluster.KubeSchedulerArgs) == 0 {
		return ``
	}

	res := ``
	for _, v := range c.cluster.KubeSchedulerArgs {
		res += fmt.Sprintf(`--kube-scheduler-arg="%s" `, v)
	}

	return res
}

func (c *Client) kubeControllerManagerArgsList() string {
	if len(c.cluster.KubeControllerManagerArgs) == 0 {
		return ``
	}

	res := ``
	for _, v := range c.cluster.KubeControllerManagerArgs {
		res += fmt.Sprintf(`--kube-controller-manager-arg="%s" `, v)
	}

	return res
}

func (c *Client) kubeCloudControllerManagerArgsList() string {
	if len(c.cluster.KubeCloudControllerManagerArgs) == 0 {
		return ``
	}

	res := ``
	for _, v := range c.cluster.KubeCloudControllerManagerArgs {
		res += fmt.Sprintf(`--kube-cloud-controller-manager-arg="%s" `, v)
	}

	return res
}

func (c *Client) kubeletArgsList() string {
	if len(c.cluster.KubeletArgs) == 0 {
		return ``
	}

	res := ``
	for _, v := range c.cluster.KubeletArgs {
		res += fmt.Sprintf(`--kubelet-arg="%s" `, v)
	}

	return res
}

func (c *Client) kubeProxyArgsList() string {
	if len(c.cluster.KubeProxyArgs) == 0 {
		return ``
	}

	res := ``
	for _, v := range c.cluster.KubeProxyArgs {
		res += fmt.Sprintf(`--kube-proxy-arg="%s" `, v)
	}

	return res
}

func (c *Client) clusterAutoscalerArgs() []string {
	stderrThresholdIsDefined := false

	for _, v := range c.cluster.ClusterAutoscalerArgs {
		if strings.Contains(v, "--stderrthreshold") {
			stderrThresholdIsDefined = true
		}
	}

	if !stderrThresholdIsDefined {
		c.cluster.ClusterAutoscalerArgs = append(c.cluster.ClusterAutoscalerArgs, `--stderrthreshold=info`)
	}

	return c.cluster.ClusterAutoscalerArgs
}

func (c *Client) apiServerIP() string {
	if c.cluster.Masters.InstanceCount > 1 {
		return c.loadBalancer().PublicNet.IPv4.IP.String()
	}

	return c.firstMasterPublicIP()
}

func (c *Client) tlsSans() string {
	sans := fmt.Sprintf("--tls-san=%s", c.apiServerIP())

	for _, server := range c.cluster.MasterInstances {
		sans += fmt.Sprintf(" --tls-san=%s", c.privateIP(server))
	}

	return sans
}

func (c *Client) firstMasterPrivateIP() string {
	return c.privateIP(c.cluster.MasterInstances[0])
}

func (c *Client) firstMasterPublicIP() string {
	return c.cluster.MasterInstances[0].PublicNet.IPv4.IP.String()
}

func (c *Client) scheduleWorkloadsOnMasters() bool {
	return c.cluster.ScheduleWorkloadsOnMasters
}

func (c *Client) clusterName() string {
	return c.cluster.ClusterName
}

func (c *Client) saveKubeconfig() {
	kubeconfig, err := c.sshclient.SSHCommand(c.firstMasterPublicIP(), "cat /etc/rancher/k3s/k3s.yaml")
	if err != nil {
		c.logger.Sugar().Fatal(err)
	}

	kubeconfig = strings.ReplaceAll(kubeconfig, "127.0.0.1", c.apiServerIP())
	kubeconfig = strings.ReplaceAll(kubeconfig, "default", c.clusterName())

	err = os.MkdirAll(filepath.Dir(c.kubeconfigPath()), 0750)
	if err != nil {
		c.logger.Sugar().Fatal(err)
	}

	err = os.WriteFile(c.kubeconfigPath(), []byte(kubeconfig), 0600)
	if err != nil {
		c.logger.Sugar().Fatal(err)
	}
}

func (c *Client) checkKubectl() error {
	_, err := exec.LookPath("kubectl")
	if err != nil {
		return fmt.Errorf("cannot find kubectl in the PATH: %w", err)
	}

	return nil
}

func (c *Client) isFirstMaster(server *hcloud.Server) bool {
	return reflect.DeepEqual(c.cluster.MasterInstances[0], server)
}

func (c *Client) mastersSize() int {
	return c.cluster.Masters.InstanceCount
}

func (c *Client) masterInstanceType() string {
	return c.cluster.Masters.InstanceType
}

func (c *Client) mustLocation(name string) *hcloud.Location {
	location, _, err := c.hetzner.Location.Get(context.Background(), name)
	if err != nil {
		c.logger.Sugar().Fatal(err)
	}

	return location
}

func (c *Client) masterLocation() *hcloud.Location {
	return c.mustLocation(c.location())
}

func (c *Client) image() *hcloud.Image {
	return &hcloud.Image{Name: c.cluster.Image}
}

func (c *Client) waitForSSH(server *hcloud.Server) (err error) {
	c.logger.Sugar().Infof("Waiting for server to be ready: %s...", server.Name)

	sshclient := *c.sshclient.SetPrintOutput(false).SetTimeout(5)

	try := 1
	o := func() error { // nolint: varnamelen
		if try > 1 {
			c.logger.Sugar().Infof("Waiting for server to be ready: %s... retrying: %d", server.Name, try)
		}
		try++

		out, err := sshclient.SSHCommand(server.PublicNet.IPv4.IP.String(), "cat /etc/ready")
		if err != nil {
			c.logger.Sugar().Debugf("ssh command failed: %s, command output: %s", err, out)

			return fmt.Errorf("command failed: %w", err)
		}

		if strings.TrimSpace(out) != "true" {
			return errors.New("server is not ready")
		}

		return nil
	}

	err = retry.Do(o, retry.Attempts(15), retry.Delay(5*time.Second))
	if err != nil {
		return fmt.Errorf("failed after 15 tries. error: %w", err)
	}

	c.logger.Sugar().Infof("...server is ready: %s.", server.Name)

	return nil
}

func (c *Client) allServers() []*hcloud.Server {
	if len(c.cluster.Instances) > 0 {
		return c.cluster.Instances
	}

	servers, err := c.hetzner.GetAllServers(c.cluster)
	if err != nil {
		c.logger.Sugar().Fatal(err)
	}

	c.cluster.Instances = servers

	return c.cluster.Instances
}

func (c *Client) masters() []*hcloud.Server {
	if len(c.cluster.MasterInstances) > 0 {
		return c.cluster.MasterInstances
	}

	masters := make([]*hcloud.Server, 0)

	re := regexp.MustCompile(`master\d$`)
	for _, server := range c.allServers() {
		if re.MatchString(server.Name) {
			masters = append(masters, server)
		}
	}

	c.cluster.MasterInstances = masters

	return masters
}

func (c *Client) workers() []*hcloud.Server {
	if len(c.cluster.WorkerInstances) > 0 {
		return c.cluster.WorkerInstances
	}

	workers := make([]*hcloud.Server, 0)

	re := regexp.MustCompile(`worker\d$`)
	for _, server := range c.allServers() {
		if re.MatchString(server.Name) {
			workers = append(workers, server)
		}
	}

	c.cluster.WorkerInstances = workers

	return workers
}

func (c *Client) sshKey() *hcloud.SSHKey {
	if c.cluster.SSHKey != nil {
		return c.cluster.SSHKey
	}

	sshkey, err := c.hetzner.CreateSSHKey(context.Background(), c.cluster)
	if err != nil {
		c.logger.Sugar().Fatal(err)
	}

	c.cluster.SSHKey = sshkey

	return c.cluster.SSHKey
}

func (c *Client) network() *hcloud.Network {
	if c.cluster.Network != nil {
		return c.cluster.Network
	}

	network, err := c.hetzner.CreateNetwork(context.Background(), c.cluster)
	if err != nil {
		c.logger.Sugar().Fatal(err)
	}

	c.cluster.Network = network

	return c.cluster.Network
}

func (c *Client) firewall() *hcloud.Firewall {
	if c.cluster.Firewall != nil {
		return c.cluster.Firewall
	}

	firewall, err := c.hetzner.CreateFirewall(context.Background(), c.cluster)
	if err != nil {
		c.logger.Sugar().Fatal(err)
	}

	c.cluster.Firewall = firewall

	return c.cluster.Firewall
}

func (c *Client) placementGroup(name string) *hcloud.PlacementGroup {
	for _, pg := range c.cluster.PlacementGroups {
		if pg.Name == name {
			return pg
		}
	}

	placementGroup, err := c.hetzner.CreatePlacementGroup(context.Background(), name)
	if err != nil {
		c.logger.Sugar().Fatal(err)
	}

	c.cluster.PlacementGroups = append(c.cluster.PlacementGroups, placementGroup)

	return placementGroup
}

func (c *Client) placementGroups() []*hcloud.PlacementGroup {
	if c.cluster.PlacementGroups != nil {
		return c.cluster.PlacementGroups
	}

	placementGroups := []*hcloud.PlacementGroup{c.placementGroup(c.clusterName())}
	for _, pool := range c.cluster.WorkerNodePools {
		placementGroups = append(
			placementGroups, c.placementGroup(
				fmt.Sprintf(
					"%s-%s", c.cluster.ClusterName, pool.Name,
				),
			),
		)
	}

	c.cluster.PlacementGroups = placementGroups

	return c.cluster.PlacementGroups
}

func (c *Client) loadBalancer() *hcloud.LoadBalancer {
	// The Public IP address is not returned during the creation of the Load Balancer.
	// So if it's empty fetch the Load Balancer from Hetzner API again.
	if c.cluster.LoadBalancer != nil && c.cluster.LoadBalancer.PublicNet.IPv4.IP != nil {
		return c.cluster.LoadBalancer
	}

	lb, err := c.hetzner.CreateLoadBalancer(context.Background(), c.cluster, c.network())
	if err != nil {
		c.logger.Sugar().Fatal("cannot create load balancer: %w", err)
	}

	c.cluster.LoadBalancer = lb

	return c.cluster.LoadBalancer
}

func (c *Client) autoscalerCloudInit() string {
	cloudInit := c.userData()
	cloudInit = strings.ReplaceAll(cloudInit, "  - shutdown -r now\n", "")
	cloudInit += "  - |\n"
	cloudInit += "    " + strings.ReplaceAll(c.workerScript(), "\n", "\n    ")
	cloudInit += "\n  - shutdown -r now"

	return base64.StdEncoding.EncodeToString([]byte(cloudInit))
}

func (c *Client) privateIP(server *hcloud.Server) string {
	if server.PrivateNet == nil {
		newSrv, err := c.hetzner.GetServer(context.Background(), server)
		if err != nil {
			c.logger.Sugar().Fatal(err)
		}

		server = newSrv

		return server.PrivateNet[0].IP.String()
	}

	return server.PrivateNet[0].IP.String()
}

func (c *Client) userData() string {
	if c.cluster.UserData != "" {
		return c.cluster.UserData
	}

	packages := []string{"fail2ban", "wireguard"}
	packages = append(packages, c.cluster.AdditionalPackages...)
	postCreateCommands := []string{
		"crontab -l > /etc/cron_bkp",
		"echo '@reboot echo true > /etc/ready' >> /etc/cron_bkp",
		"crontab /etc/cron_bkp",
		"sed -i 's/[#]*PermitRootLogin yes/PermitRootLogin prohibit-password/g' /etc/ssh/sshd_config",
		"sed -i 's/[#]*PasswordAuthentication yes/PasswordAuthentication no/g' /etc/ssh/sshd_config",
		"systemctl restart sshd",
		"systemctl stop systemd-resolved",
		"systemctl disable systemd-resolved",
		"rm -f /etc/resolv.conf",
	}

	for i, nameserver := range c.cluster.DefaultNameservers {
		if i == 0 {
			postCreateCommands = append(
				postCreateCommands, fmt.Sprintf("echo 'nameserver %s' > /etc/resolv.conf", nameserver),
			)
		} else {
			postCreateCommands = append(
				postCreateCommands, fmt.Sprintf("echo 'nameserver %s' >> /etc/resolv.conf", nameserver),
			)
		}
	}

	postCreateCommands = append(postCreateCommands, c.cluster.PostCreateCommands...)

	if c.cluster.FixMultipath {
		postCreateCommands = append(
			postCreateCommands,
			`if ! grep -q blacklist /etc/multipath.conf; then printf 'blacklist {\n    devnode "^sd[a-z0-9]+"\n}' >> /etc/multipath.conf; fi`,
			`systemctl restart multipathd.service`,
		)
	}

	re := regexp.MustCompile(`shutdown|[^@]reboot`)
	if !re.MatchString(strings.Join(postCreateCommands, " ")) {
		postCreateCommands = append(postCreateCommands, "shutdown -r now")
	}

	var buffer bytes.Buffer

	tpldata := make(map[string]interface{})
	tpldata["Packages"] = packages
	tpldata["PostCreateCommands"] = postCreateCommands

	tpl := `#cloud-config
packages: {{- range .Packages }}
  - {{ . }}{{ end }}
runcmd: {{- range .PostCreateCommands }}
  - {{ . }}{{ end }}
`
	t := template.Must(template.New("tpl").Parse(tpl))

	err := t.Execute(&buffer, tpldata)
	if err != nil {
		panic(err)
	}

	c.cluster.UserData = buffer.String()

	return c.cluster.UserData
}

func (c *Client) clusterNetworkIPRange() *net.IPNet {
	if c.cluster.ClusterNetworkIPRange != nil {
		return c.cluster.ClusterNetworkIPRange
	}

	c.cluster.ClusterNetworkIPRange = c.network().IPRange

	return c.cluster.ClusterNetworkIPRange
}

func (c *Client) clusterNetworkZone() string {
	if c.cluster.ClusterNetworkZone != "" {
		return c.cluster.ClusterNetworkZone
	}

	if c.location() == "ash" {
		c.cluster.ClusterNetworkZone = "us-east"
	} else if c.location() == "hil" {
		c.cluster.ClusterNetworkZone = "us-west"
	} else {
		c.cluster.ClusterNetworkZone = "eu-central"
	}

	return c.cluster.ClusterNetworkZone
}

func (c *Client) location() string {
	return c.cluster.Location
}

// If the path contains ~, translate that to the User's home directory.
func resolvePath(filepath string) string {
	usr, _ := user.Current()
	dir := usr.HomeDir

	if filepath == "~" {
		filepath = dir
	} else if strings.HasPrefix(filepath, "~/") {
		filepath = path.Join(dir, strings.TrimPrefix(filepath, "~/"))
	}

	return filepath
}

func (c *Client) publicSSHKeyPath() string {
	c.cluster.PublicSSHKeyPath = resolvePath(c.cluster.PublicSSHKeyPath)

	return c.cluster.PublicSSHKeyPath
}

func (c *Client) privateSSHKeyPath() string {
	c.cluster.PrivateSSHKeyPath = resolvePath(c.cluster.PrivateSSHKeyPath)

	return c.cluster.PrivateSSHKeyPath
}

func (c *Client) kubeconfigPath() string {
	c.cluster.KubeconfigPath = resolvePath(c.cluster.KubeconfigPath)

	return c.cluster.KubeconfigPath
}
