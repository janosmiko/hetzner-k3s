package deletecluster

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"hetzner-k3s/internal/k3s"
)

var Cmd = &cobra.Command{
	Use:   "delete-cluster [flags]",
	Short: "Delete Cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		return Run()
	},
}

func Run() error {
	if !confirm("Are you sure you want to delete the cluster?") {
		return nil
	}

	err := k3s.NewClient().DeleteCluster()
	if err != nil {
		return fmt.Errorf("cannot delete cluster: %w", err)
	}

	return nil
}

func confirm(msg string) bool {
	if viper.GetBool("auto_approve") {
		return true
	}

	fmt.Printf("%s (y)es, (n)o ", msg)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()

	switch strings.TrimSpace(strings.ToLower(scanner.Text())) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		fmt.Println("I'm sorry but I didn't get what you meant, please type (y)es or (n)o and then press enter:")

		return confirm(msg)
	}
}
