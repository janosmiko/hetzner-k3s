package createcluster

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"hetzner-k3s/internal/k3s"
)

var Cmd = &cobra.Command{
	Use:   "create-cluster [flags]",
	Short: "Create Cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		return Run()
	},
}

func Run() error {
	err := k3s.NewClient().CreateCluster()
	if err != nil {
		return fmt.Errorf("cannot create cluster: %w", err)
	}

	return nil
}

func init() {
	Cmd.Flags().Bool(
		"force", false,
		`Forcefully run kubernetes commands.`,
	)

	_ = viper.BindPFlag("force", Cmd.Flags().Lookup("force"))
}
