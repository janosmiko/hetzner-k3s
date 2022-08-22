package cluster

import (
	"net"

	"github.com/hetznercloud/hcloud-go/hcloud"

	"hetzner-k3s/internal/config"
)

type Cluster struct {
	K3sToken                string
	UserData                string
	ClusterNetworkIPRange   *net.IPNet
	ClusterNetworkZone      string
	PrivateSSHKeyPassphrase string
	Network                 *hcloud.Network
	SSHKey                  *hcloud.SSHKey
	Firewall                *hcloud.Firewall
	LoadBalancer            *hcloud.LoadBalancer
	PlacementGroups         []*hcloud.PlacementGroup
	Instances               []*hcloud.Server
	MasterInstances         []*hcloud.Server
	WorkerInstances         []*hcloud.Server
	*config.Config
}

func NewCluster(config *config.Config) *Cluster {
	cluster := Cluster{
		Config: config,
	}

	_ = cluster.Validate()

	return &cluster
}

func (c *Cluster) Validate() error {
	return nil
}
