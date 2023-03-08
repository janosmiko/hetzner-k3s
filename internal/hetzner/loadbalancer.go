package hetzner

import (
	"context"
	"fmt"

	"github.com/hetznercloud/hcloud-go/hcloud"

	"hetzner-k3s/internal/cluster"
)

func (c *Client) CreateLoadBalancer(
	ctx context.Context,
	cluster *cluster.Cluster,
	network *hcloud.Network,
	location *hcloud.Location,
) (
	lb *hcloud.LoadBalancer, err error,
) {
	lbName := cluster.ClusterName + "-api"

	// Check and update if LoadBalancer exists.
	{
		lb, _, err = c.Client.LoadBalancer.Get(ctx, lbName)
		if err != nil {
			return nil, fmt.Errorf("cannot get load balancer: %w", err)
		}

		if lb != nil {
			err = c.loadbalancerAddLabelSelector(ctx, lb, cluster)
			if err != nil {
				return nil, err
			}

			c.logger.Sugar().Infof("LoadBalancer exists: %s.", lbName)

			return lb, nil
		}
	}

	// Create new LoadBalancer if it does not exist.
	{
		c.logger.Sugar().Infof("Creating Load Balancer: %s...", lbName)

		lbresults, _, err := c.Client.LoadBalancer.Create(
			ctx, hcloud.LoadBalancerCreateOpts{
				Name:             lbName,
				LoadBalancerType: &hcloud.LoadBalancerType{Name: "lb11"},
				Algorithm:        &hcloud.LoadBalancerAlgorithm{Type: hcloud.LoadBalancerAlgorithmTypeRoundRobin},
				PublicInterface:  hcloud.Bool(true),
				Location:         location,
				Network:          network,
				Services: []hcloud.LoadBalancerCreateOptsService{
					{
						Protocol:        hcloud.LoadBalancerServiceProtocolTCP,
						ListenPort:      hcloud.Int(6443),
						DestinationPort: hcloud.Int(6443),
						Proxyprotocol:   hcloud.Bool(false),
					},
				},
				Targets: []hcloud.LoadBalancerCreateOptsTarget{
					{
						Type: hcloud.LoadBalancerTargetTypeLabelSelector,
						LabelSelector: hcloud.LoadBalancerCreateOptsTargetLabelSelector{
							Selector: fmt.Sprintf(
								"cluster=%s,role=master", cluster.ClusterName,
							),
						},
						UsePrivateIP: hcloud.Bool(true),
					},
				},
			},
		)
		if err != nil {
			return nil, fmt.Errorf("cannot create load balancer: %w", err)
		}

		lb = lbresults.LoadBalancer

		err = c.loadbalancerAddLabelSelector(ctx, lb, cluster)
		if err != nil {
			return nil, err
		}

		c.logger.Sugar().Infof("...LoadBalancer created: %s.", lb.Name)

		return lb, err
	}
}

func (c *Client) loadbalancerAddLabelSelector(
	ctx context.Context, lb *hcloud.LoadBalancer, cluster *cluster.Cluster,
) (err error) {
	labelSelector := fmt.Sprintf("cluster=%s,role=master", cluster.ClusterName)

	for _, v := range lb.Targets {
		if v.Type != hcloud.LoadBalancerTargetTypeLabelSelector {
			continue
		}

		if v.LabelSelector.Selector == labelSelector {
			return nil
		}
	}

	_, _, err = c.Client.LoadBalancer.AddLabelSelectorTarget(
		ctx, lb, hcloud.LoadBalancerAddLabelSelectorTargetOpts{
			Selector:     labelSelector,
			UsePrivateIP: hcloud.Bool(true),
		},
	)
	if err != nil {
		return fmt.Errorf("cannot add label selector to load balancer: %w", err)
	}

	return nil
}

func (c *Client) DeleteLoadBalancer(ctx context.Context, loadbalancer *hcloud.LoadBalancer) (err error) {
	c.logger.Sugar().Infof("Deleting Load Balancer: %s...", loadbalancer.Name)

	_, err = c.Client.LoadBalancer.Delete(ctx, loadbalancer)
	if err != nil {
		return fmt.Errorf("load balancer cannont be deleted: %w", err)
	}

	c.logger.Sugar().Infof("...LoadBalancer deleted: %s.", loadbalancer.Name)

	return nil
}

func (c *Client) GetLoadBalancerIPByName(name string) (string, error) {
	lb, _, err := c.Client.LoadBalancer.Get(context.Background(), name)
	if err != nil {
		return "", fmt.Errorf("cannot get load balancer: %w", err)
	}

	return lb.PublicNet.IPv4.IP.String(), nil
}
