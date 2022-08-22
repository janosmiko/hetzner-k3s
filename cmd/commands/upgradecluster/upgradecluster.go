package upgradecluster

import (
	"fmt"

	"github.com/spf13/cobra"

	"hetzner-k3s/internal/k3s"
)

var Cmd = &cobra.Command{
	Use:   "upgrade-cluster [flags]",
	Short: "Upgrade Cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		return Run()
	},
}

func Run() error {
	err := k3s.NewClient().UpgradeCluster()
	if err != nil {
		return fmt.Errorf("cannot upgrade cluster: %w", err)
	}

	return nil
}
