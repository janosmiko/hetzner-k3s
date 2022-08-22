package k3s

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"golang.org/x/exp/slices"

	sshpkg "hetzner-k3s/internal/ssh"
)

func (c *Client) validate(action string) error {
	switch action {
	case "create":
		return c.validateCreate()
	case "delete":
		return c.validateDelete()
	case "upgrade":
		return c.validateUpgrade()
	}

	return nil
}

func (c *Client) validateCreate() error {
	if err := c.validatePublicSSHKey(); err != nil {
		return err
	}

	if err := c.validatePrivateSSHKeyAndPassphrase(); err != nil {
		return err
	}

	if err := c.validateAllowedSSHNetworks(); err != nil {
		return err
	}

	if err := c.validateAllowedAPINetworks(); err != nil {
		return err
	}

	if err := c.validateToken(); err != nil {
		return err
	}

	if err := c.validateK3sVersion(); err != nil {
		return err
	}

	c.clusterNetworkZone()
	c.clusterNetworkIPRange()

	if err := c.validateCluster(); err != nil {
		return err
	}

	if err := c.validateMasterLocation(); err != nil {
		return err
	}

	if err := c.validateExistingNetwork(); err != nil {
		return err
	}

	return nil
}

func (c *Client) validateDelete() error {
	if err := c.validateToken(); err != nil {
		return err
	}

	c.clusterNetworkZone()
	c.clusterNetworkIPRange()

	if err := c.validateCluster(); err != nil {
		return err
	}

	return nil
}

func (c *Client) validateUpgrade() error {
	if err := c.validateCluster(); err != nil {
		return err
	}

	if err := c.validateKubeconfigPath(); err != nil {
		return err
	}

	if err := c.validateK3sVersion(); err != nil {
		return err
	}

	return nil
}

func (c *Client) validateToken() error {
	_, err := c.hetzner.GetLocations()
	if err != nil {
		return fmt.Errorf("cannot validate token: %w", err)
	}

	return nil
}

func (c *Client) validateK3sVersion() error {
	releases, err := c.AvailableReleases(c.cluster.GithubToken, "", false)
	if err != nil {
		return err
	}

	currentVersion := c.k3sVersion()

	if !slices.Contains(releases, currentVersion) {
		return errors.New("k3s version does not exist")
	}

	return nil
}

func (c *Client) validateCluster() error {
	err := c.cluster.Validate()
	if err != nil {
		return fmt.Errorf("cannot validate cluster: %w", err)
	}

	return nil
}

func (c *Client) validateKubeconfigPath() error {
	if isDir, err := isDirectory(c.kubeconfigPath()); err != nil {
		return fmt.Errorf("cannot read kubeconfig path: %w", err)
	} else if isDir {
		return errors.New("kubeconfig path is a directory")
	}

	return nil
}

func (c *Client) validatePrivateSSHKeyAndPassphrase() error {
	if isDir, err := isDirectory(c.privateSSHKeyPath()); err != nil {
		return fmt.Errorf("cannot read private key path: %w", err)
	} else if isDir {
		return errors.New("private key path is a directory")
	}

	passphrase, err := sshpkg.ParseSSHPasshprase(c.privateSSHKeyPath())
	if err != nil {
		fmt.Printf("Error: %s\n", err)

		os.Exit(1)
	}

	c.cluster.PrivateSSHKeyPassphrase = passphrase
	c.sshclient.SetPassphrase(c.cluster.PrivateSSHKeyPassphrase)

	return nil
}

func (c *Client) validatePublicSSHKey() error {
	if isDir, err := isDirectory(c.publicSSHKeyPath()); err != nil {
		return fmt.Errorf("cannot read public key path: %w", err)
	} else if isDir {
		return errors.New("public key path is a directory")
	}

	return nil
}

func isDirectory(path string) (bool, error) {
	fi, err := os.Stat(resolvePath(path))
	if err != nil {
		return false, fmt.Errorf("cannot read path: %w", err)
	}

	return fi.IsDir(), err
}

func (c *Client) validateAllowedSSHNetworks() error {
	currentAddress, err := getPublicIP()
	if err != nil {
		return err
	}

	contains := false

	for _, network := range c.cluster.SSHAllowedNetworks {
		_, iprange, err := net.ParseCIDR(network)
		if err != nil {
			return fmt.Errorf("cannot parse address: %w", err)
		}

		if iprange.Contains(currentAddress.IP) {
			contains = true
		}
	}

	if !contains {
		return fmt.Errorf(
			"your current ip %s is not included in any of the allowed ssh networks", currentAddress.IP,
		)
	}

	return nil
}

func (c *Client) validateAllowedAPINetworks() error {
	currentAddress, err := getPublicIP()
	if err != nil {
		return err
	}

	contains := false

	for _, network := range c.cluster.APIAllowedNetworks {
		_, iprange, err := net.ParseCIDR(network)
		if err != nil {
			return fmt.Errorf("cannot parse address: %w", err)
		}

		if iprange.Contains(currentAddress.IP) {
			contains = true
		}
	}

	if !contains {
		return fmt.Errorf(
			"your current ip %s is not included in any of the allowed api networks", currentAddress.IP,
		)
	}

	return nil
}

func getPublicIP() (*net.IPNet, error) {
	resp, err := http.Get("http://whatismyip.akamai.com")
	if err != nil {
		return nil, fmt.Errorf("cannot fetch public ip address: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	var buffer bytes.Buffer

	_, err = io.Copy(&buffer, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot get public ip address: %w", err)
	}

	_, currentAddress, err := net.ParseCIDR(buffer.String() + "/32")
	if err != nil {
		return nil, fmt.Errorf("cannot parse public ip address: %w", err)
	}

	return currentAddress, nil
}

func (c *Client) validateMasterLocation() error {
	err := c.validateLocation(c.masterLocation().Name)
	if err != nil {
		return fmt.Errorf(
			"invalid location for master nodes - "+
				"valid locations: nbg1 (Nuremberg, Germany), "+
				"fsn1 (Falkenstein, Germany), hel1 (Helsinki, "+
				"Finland) or ash (Ashburn, Virginia, USA): %w",
			err,
		)
	}

	return nil
}

func (c *Client) validateLocation(location string) error {
	locations, err := c.hetzner.GetLocations()
	if err != nil {
		return fmt.Errorf("cannot fetch locations: %w", err)
	}

	valid := false

	for _, loc := range locations {
		if location == loc.Name {
			valid = true
		}
	}

	if !valid {
		return errors.New("invalid location")
	}

	return nil
}

func (c *Client) validateExistingNetwork() error {
	_, _, err := c.hetzner.Network.Get(context.Background(), c.cluster.ExistingNetwork)
	if err != nil {
		return fmt.Errorf("cannot find existing network: %w", err)
	}

	return nil
}
