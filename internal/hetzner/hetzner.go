package hetzner

import (
	"context"
	"fmt"

	"github.com/hetznercloud/hcloud-go/hcloud"

	clusterpkg "hetzner-k3s/internal/cluster"
	"hetzner-k3s/internal/logger"
)

type Client struct {
	logger *logger.Logger
	*hcloud.Client
}

func NewClient(logger *logger.Logger, token string) *Client {
	return &Client{
		logger: logger,
		Client: hcloud.NewClient(hcloud.WithToken(token), hcloud.WithDebugWriter(logger.DebugWriter())),
	}
}

func (c *Client) GetAllServers(cluster *clusterpkg.Cluster) ([]*hcloud.Server, error) {
	allServers, err := c.fetchServers()
	if err != nil {
		return nil, fmt.Errorf("cannot fetch servers: %w", err)
	}

	var clusterServers []*hcloud.Server

	for _, server := range allServers {
		if c.belongsToCluster(server, cluster.ClusterName) {
			clusterServers = append(clusterServers, server)
		}
	}

	return clusterServers, nil
}

func (c *Client) fetchServers() ([]*hcloud.Server, error) {
	var (
		ctx        = context.Background()
		allServers []*hcloud.Server
		opt        = hcloud.ServerListOpts{
			ListOpts: hcloud.ListOpts{
				Page:    0,
				PerPage: 0,
			},
		}
	)

	for {
		servers, resp, err := c.Server.List(
			ctx, opt,
		)
		if err != nil {
			return nil, fmt.Errorf("cannot list servers: %w", err)
		}

		allServers = append(allServers, servers...)

		if resp.Meta.Pagination.NextPage == 0 {
			break
		}

		opt.Page = resp.Meta.Pagination.NextPage
	}

	return allServers, nil
}
