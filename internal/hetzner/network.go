package hetzner

import (
	"context"
	"fmt"
	"net"
	"reflect"

	"github.com/hetznercloud/hcloud-go/hcloud"

	"hetzner-k3s/internal/cluster"
)

func (c *Client) CreateNetwork(ctx context.Context, cluster *cluster.Cluster) (network *hcloud.Network, err error) {
	networkName := ""
	if cluster.ExistingNetwork != "" {
		networkName = cluster.ExistingNetwork
	} else {
		networkName = cluster.ClusterName
	}

	// Check and update if network exists.
	{
		network, _, err = c.Client.Network.Get(ctx, networkName)
		if err != nil {
			return nil, fmt.Errorf("cannot get network: %w", err)
		}

		if network != nil {
			c.logger.Sugar().Infof("Network exists: %s.", networkName)

			err = c.addSubnet(ctx, network, cluster)
			if err != nil {
				return nil, fmt.Errorf("cannot add subnet: %w", err)
			}

			return network, nil
		}
	}

	// Create network if it does not exist.
	{
		c.logger.Sugar().Infof("Creating network %s...", networkName)

		network, _, err = c.Client.Network.Create(
			ctx, hcloud.NetworkCreateOpts{
				Name:    networkName,
				IPRange: iprange(cluster),
			},
		)
		if err != nil {
			return nil, fmt.Errorf("cannot create network: %w", err)
		}

		err = c.addSubnet(ctx, network, cluster)
		if err != nil {
			return nil, fmt.Errorf("cannot add subnet: %w", err)
		}

		c.logger.Sugar().Infof("...network created: %s.", networkName)

		return network, nil
	}
}

func iprange(cluster *cluster.Cluster) *net.IPNet {
	var iprange *net.IPNet
	if cluster.ClusterNetworkIPRange == nil {
		_, iprange, _ = net.ParseCIDR(cluster.NetworkIPRange)
	} else {
		iprange = cluster.ClusterNetworkIPRange
	}

	return iprange
}

func (c *Client) addSubnet(ctx context.Context, network *hcloud.Network, cluster *cluster.Cluster) (err error) {
	if cluster.ClusterNetworkIPRange == nil {
		cluster.ClusterNetworkIPRange = network.IPRange
	}

	for _, v := range network.Subnets {
		if reflect.DeepEqual(v.IPRange, cluster.ClusterNetworkIPRange) {
			return nil
		}
	}

	opts := hcloud.NetworkAddSubnetOpts{
		Subnet: hcloud.NetworkSubnet{
			Type:        hcloud.NetworkSubnetTypeCloud,
			IPRange:     cluster.ClusterNetworkIPRange,
			NetworkZone: hcloud.NetworkZone(cluster.ClusterNetworkZone),
		},
	}

	_, _, err = c.Client.Network.AddSubnet(ctx, network, opts)
	if err != nil {
		return fmt.Errorf("subnet cannot be attached: %w", err)
	}

	return nil
}

func (c *Client) DeleteNetwork(ctx context.Context, network *hcloud.Network) (err error) {
	c.logger.Sugar().Infof("Deleting network: %s...", network.Name)

	_, err = c.Client.Network.Delete(ctx, network)
	if err != nil {
		return fmt.Errorf("cannot create network: %w", err)
	}

	c.logger.Sugar().Infof("...network deleted: %s.", network.Name)

	return nil
}
