package k3s

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"github.com/google/go-github/v45/github"
	"github.com/hashicorp/go-version"
	"golang.org/x/oauth2"
)

func (c *Client) newGithubClient(token string) *github.Client {
	ctx := context.Background()

	if token == "" {
		c.logger.Debug("Creating a new Github client without Token.")

		return github.NewClient(nil)
	}

	c.logger.Sugar().Debugf("Creating a new Github client with Token: %s.", token)

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return github.NewClient(tc)
}

func (c *Client) AvailableReleases(token, filter string, latest bool) ([]string, error) {
	c.logger.Info("Fetching available K3s releases...")

	releases, err := c.fetchReleases(token)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch releases: %w", err)
	}

	var (
		filterRegexp *regexp.Regexp
		releaseNames = make([]string, 0, len(releases))
	)

	if filter != "" {
		filterRegexp, err = regexp.Compile(filter)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp: %w", err)
		}
	}

	for _, v := range releases {
		name := v.GetName()

		if name == "" {
			continue
		}

		if filter != "" {
			if filterRegexp.MatchString(name) {
				releaseNames = append(releaseNames, name)
			}

			continue
		}

		releaseNames = append(releaseNames, name)
	}

	sorted := c.sortVersions(releaseNames)

	if latest {
		return []string{sorted[len(sorted)-1]}, nil
	}

	c.logger.Info("...fetching done.")

	return sorted, nil
}

func (c *Client) fetchReleases(token string) ([]*github.RepositoryRelease, error) {
	var (
		allReleases []*github.RepositoryRelease
		opt         = &github.ListOptions{}
	)

	for {
		releases, resp, err := c.newGithubClient(token).Repositories.ListReleases(
			context.Background(), "k3s-io", "k3s", opt,
		)
		if err != nil {
			return nil, fmt.Errorf("cannot list releases: %w", err)
		}

		allReleases = append(allReleases, releases...)

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return allReleases, nil
}

func (c *Client) sortVersions(unsorted []string) []string {
	versions := []*version.Version{}

	for _, raw := range unsorted {
		v, err := version.NewVersion(raw)
		if err != nil {
			continue
		}

		versions = append(versions, v)
	}

	// After this, the versions are properly sorted
	sort.Sort(version.Collection(versions))

	sorted := make([]string, len(versions))

	for i, v := range versions {
		sorted[i] = "v" + v.String()
	}

	return sorted
}

func (c *Client) latestVersion(token string) string {
	release, _, err := c.newGithubClient(token).Repositories.GetLatestRelease(
		context.Background(), "k3s-io", "k3s",
	)
	if err != nil {
		c.logger.Sugar().Fatal(err)
	}

	return release.GetName()
}
