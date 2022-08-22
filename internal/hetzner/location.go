package hetzner

import (
	"context"
	"fmt"

	"github.com/hetznercloud/hcloud-go/hcloud"
)

func (c *Client) GetLocations() ([]*hcloud.Location, error) {
	locations, _, err := c.Client.Location.List(context.Background(), hcloud.LocationListOpts{})
	if err != nil {
		return nil, fmt.Errorf("cannot fetch locations: %w", err)
	}

	return locations, nil
}
