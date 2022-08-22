package hetzner

import (
	"context"
	"fmt"
	"strings"

	"github.com/hetznercloud/hcloud-go/hcloud"
)

func (c *Client) CreateServer(ctx context.Context, opts hcloud.ServerCreateOpts) (server *hcloud.Server, err error) {
	server, _, err = c.Server.Get(ctx, opts.Name)
	if err != nil {
		return nil, fmt.Errorf("cannot get server: %w", err)
	}

	if server != nil {
		c.logger.Sugar().Infof("Server exist: %s.", opts.Name)

		return server, nil
	}

	c.logger.Sugar().Infof("Creating server: %s...", opts.Name)

	serverResponse, _, err := c.Client.Server.Create(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("cannot create server: %w", err)
	}

	c.logger.Sugar().Infof("...server created: %s.", opts.Name)

	return serverResponse.Server, nil
}

func (c *Client) DeleteServer(ctx context.Context, server *hcloud.Server) (err error) {
	c.logger.Sugar().Infof("Deleting server: %s...", server.Name)

	_, err = c.Client.Server.Delete(ctx, server)
	if err != nil {
		return fmt.Errorf("cannot delete server: %w", err)
	}

	c.logger.Sugar().Infof("...server deleted: %s.", server.Name)

	return nil
}

func (c *Client) GetServer(ctx context.Context, server *hcloud.Server) (*hcloud.Server, error) {
	server, _, err := c.Server.Get(ctx, server.Name)
	if err != nil {
		return nil, fmt.Errorf("cannot get server: %w", err)
	}

	return server, nil
}

func (c *Client) belongsToCluster(server *hcloud.Server, clusterName string) bool {
	for key, label := range server.Labels {
		if key == "hcloud/node-group" {
			if strings.HasPrefix(label, clusterName) {
				return true
			}
		}

		if key == "cluster" {
			if label == clusterName {
				return true
			}
		}
	}

	return false
}
