package hetzner

import (
	"context"
	"fmt"
	"os"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"golang.org/x/crypto/ssh"

	"hetzner-k3s/internal/cluster"
)

func (c *Client) CreateSSHKey(ctx context.Context, cluster *cluster.Cluster) (key *hcloud.SSHKey, err error) {
	sshkeybytes, err := os.ReadFile(cluster.PublicSSHKeyPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}

	_, _, _, _, err = ssh.ParseAuthorizedKey(sshkeybytes)
	if err != nil {
		return nil, fmt.Errorf("cannot parse ssh public key: %w", err)
	}

	key, _, err = c.Client.SSHKey.Get(ctx, cluster.ClusterName)
	if err != nil {
		return nil, fmt.Errorf("cannot get ssh public key: %w", err)
	}

	if key != nil {
		c.logger.Sugar().Infof("SSH public key exists.")

		return key, nil
	}

	c.logger.Sugar().Infof("Creating SSH public key...")

	key, _, err = c.Client.SSHKey.Create(
		ctx, hcloud.SSHKeyCreateOpts{
			Name:      cluster.ClusterName,
			PublicKey: string(sshkeybytes),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create ssh public key: %w", err)
	}

	c.logger.Sugar().Infof("...SSH public key uploaded.")

	return key, nil
}

func (c *Client) DeleteSSHKey(ctx context.Context, sshkey *hcloud.SSHKey) (err error) {
	c.logger.Sugar().Infof("Deleting SSH public key...")

	_, err = c.Client.SSHKey.Delete(ctx, sshkey)
	if err != nil {
		return fmt.Errorf("cannot delete ssh public key: %w", err)
	}

	c.logger.Sugar().Infof("...SSH public key deleted.")

	return nil
}
