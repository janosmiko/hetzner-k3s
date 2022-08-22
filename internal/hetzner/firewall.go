package hetzner

import (
	"context"
	"fmt"
	"net"

	"github.com/hetznercloud/hcloud-go/hcloud"

	"hetzner-k3s/internal/cluster"
)

func (c *Client) CreateFirewall(ctx context.Context, cluster *cluster.Cluster) (fw *hcloud.Firewall, err error) {
	firewallName := cluster.ClusterName
	// Check and update if firewall exists.
	{
		fw, _, err = c.Client.Firewall.Get(ctx, firewallName)
		if err != nil {
			return nil, fmt.Errorf("cannot get firewall: %w", err)
		}

		if fw != nil {
			_, _, err = c.Client.Firewall.SetRules(
				ctx, fw, hcloud.FirewallSetRulesOpts{Rules: createFirewallRules(cluster)},
			)
			if err != nil {
				return nil, fmt.Errorf("cannot set firewall rules: %w", err)
			}

			c.logger.Sugar().Infof("Firewall %s exists.", firewallName)

			return fw, nil
		}
	}

	// Create new firewall if it does not exist.
	{
		c.logger.Sugar().Infof("Creating firewall %s...", cluster.ClusterName)

		fwresults, _, err := c.Client.Firewall.Create(
			ctx, hcloud.FirewallCreateOpts{
				Name:  firewallName,
				Rules: createFirewallRules(cluster),
			},
		)
		if err != nil {
			return nil, fmt.Errorf("cannot create firewall: %w", err)
		}
		fw = fwresults.Firewall

		c.logger.Sugar().Infof("...firewall %s created.", firewallName)

		return fw, nil
	}
}

func createFirewallRules(cluster *cluster.Cluster) []hcloud.FirewallRule {
	rules := []hcloud.FirewallRule{}
	ping := hcloud.FirewallRule{
		Description: hcloud.String("Allow ICMP (ping)"),
		Direction:   hcloud.FirewallRuleDirectionIn,
		SourceIPs: []net.IPNet{
			{
				IP:   net.ParseIP("0.0.0.0"),
				Mask: net.CIDRMask(0, 32),
			},
		},
		Protocol: hcloud.FirewallRuleProtocolICMP,
	}
	rules = append(rules, ping)

	internalTCP := hcloud.FirewallRule{
		Description: hcloud.String("Allow all TCP traffic between nodes on the private network"),
		Direction:   hcloud.FirewallRuleDirectionIn,
		SourceIPs: []net.IPNet{
			*cluster.ClusterNetworkIPRange,
		},
		Protocol: hcloud.FirewallRuleProtocolTCP,
		Port:     hcloud.String("any"),
	}
	rules = append(rules, internalTCP)

	internalUDP := hcloud.FirewallRule{
		Description: hcloud.String("Allow all UDP traffic between nodes on the private network"),
		Direction:   hcloud.FirewallRuleDirectionIn,
		SourceIPs: []net.IPNet{
			*cluster.ClusterNetworkIPRange,
		},
		Protocol: hcloud.FirewallRuleProtocolUDP,
		Port:     hcloud.String("any"),
	}
	rules = append(rules, internalUDP)

	if cluster.Masters.InstanceCount == 1 {
		for _, v := range cluster.APIAllowedNetworks {
			_, ipnet, _ := net.ParseCIDR(v)
			customAPI := hcloud.FirewallRule{
				Description: hcloud.String("Allow port 6443 (Kubernetes API server)"),
				Direction:   hcloud.FirewallRuleDirectionIn,
				SourceIPs: []net.IPNet{
					*ipnet,
				},
				Protocol: hcloud.FirewallRuleProtocolTCP,
				Port:     hcloud.String("6443"),
			}
			rules = append(rules, customAPI)
		}
	}

	for _, v := range cluster.SSHAllowedNetworks {
		_, ipnet, _ := net.ParseCIDR(v)
		customSSH := hcloud.FirewallRule{
			Description: hcloud.String("Allow port 22 (SSH)"),
			Direction:   hcloud.FirewallRuleDirectionIn,
			SourceIPs: []net.IPNet{
				*ipnet,
			},
			Protocol: hcloud.FirewallRuleProtocolTCP,
			Port:     hcloud.String("22"),
		}
		rules = append(rules, customSSH)
	}

	return rules
}

func (c *Client) DeleteFirewall(ctx context.Context, firewall *hcloud.Firewall) (err error) {
	c.logger.Sugar().Infof("Deleting firewall %s...", firewall.Name)

	_, err = c.Client.Firewall.Delete(
		ctx, firewall,
	)
	if err != nil {
		return fmt.Errorf("firewall cannot be deleted: %w", err)
	}

	c.logger.Sugar().Infof("...firewall %s deleted.", firewall.Name)

	return nil
}
