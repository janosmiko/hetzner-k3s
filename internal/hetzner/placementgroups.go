package hetzner

import (
	"context"
	"fmt"

	"github.com/hetznercloud/hcloud-go/hcloud"
)

func (c *Client) CreatePlacementGroup(ctx context.Context, pgName string) (
	pg *hcloud.PlacementGroup, err error,
) {
	pg, _, err = c.Client.PlacementGroup.Get(ctx, pgName)
	if err != nil {
		return nil, fmt.Errorf("cannot get placement group: %w", err)
	}

	if pg != nil {
		c.logger.Sugar().Infof("Placement group exists: %s.", pgName)

		return pg, nil
	}

	c.logger.Sugar().Infof("Creating placement group: %s...", pgName)

	pgResults, _, err := c.Client.PlacementGroup.Create(
		ctx, hcloud.PlacementGroupCreateOpts{
			Name: pgName,
			Type: hcloud.PlacementGroupTypeSpread,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create placement group: %w", err)
	}

	pg = pgResults.PlacementGroup

	c.logger.Sugar().Infof("...placement group created: %s.", pg.Name)

	return pg, nil
}

func (c *Client) DeletePlacementGroup(ctx context.Context, pg *hcloud.PlacementGroup) (err error) {
	c.logger.Sugar().Infof("Deleting placement group: %s...", pg.Name)

	_, err = c.Client.PlacementGroup.Delete(ctx, pg)
	if err != nil {
		return fmt.Errorf("cannot delete placement group: %w", err)
	}

	c.logger.Sugar().Infof("...placement group deleted: %s.", pg.Name)

	return nil
}
