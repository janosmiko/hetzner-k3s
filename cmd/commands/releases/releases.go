package releases

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"hetzner-k3s/internal/k3s"
)

var Cmd = &cobra.Command{
	Use:   "releases [flags]",
	Short: "List K3s releases",
	RunE: func(cmd *cobra.Command, args []string) error {
		return Run()
	},
}

func Run() error {
	filter := viper.GetString("filter")
	token := strings.TrimSpace(viper.GetString("github_token"))
	latest := viper.GetBool("latest_version_only")

	releases, err := k3s.NewEmptyClient().AvailableReleases(token, filter, latest)
	if err != nil {
		fmt.Printf("error: %s", err)
	}

	filtermsg := ""
	if filter != "" {
		filtermsg = fmt.Sprintf(" (using filter: %s)", filter)
	}

	fmt.Printf("Available releases%s:\n", filtermsg)

	for _, v := range releases {
		fmt.Println(v)
	}

	return nil
}

func init() {
	Cmd.Flags().String(
		"filter", "",
		`Filter to version using a regex.`,
	)

	_ = viper.BindPFlag("filter", Cmd.Flags().Lookup("filter"))

	Cmd.Flags().Bool(
		"latest", false,
		`Filter to the latest stable version only.`,
	)

	_ = viper.BindPFlag("latest_version_only", Cmd.Flags().Lookup("latest"))
}
