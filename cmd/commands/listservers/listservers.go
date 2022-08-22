package listservers

import (
	"fmt"

	"github.com/spf13/cobra"

	"hetzner-k3s/internal/k3s"
)

var Cmd = &cobra.Command{
	Use:   "list-servers [flags]",
	Short: "List Servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		return Run()
	},
}

func Run() error {
	err := k3s.NewClient().ListServers()
	if err != nil {
		return fmt.Errorf("cannot delete cluster: %w", err)
	}

	return nil
}
