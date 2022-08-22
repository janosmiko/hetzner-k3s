package commands

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"hetzner-k3s/cmd/commands/createcluster"
	"hetzner-k3s/cmd/commands/deletecluster"
	"hetzner-k3s/cmd/commands/listservers"
	"hetzner-k3s/cmd/commands/releases"
	"hetzner-k3s/cmd/commands/upgradecluster"
	"hetzner-k3s/internal/config"
)

var Version string

var cmd = &cobra.Command{
	Use:          "hetzner-k3s",
	Short:        "",
	SilenceUsage: true,
	Version:      Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		return Cmd(cmd)
	},
}

func init() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

		fmt.Println("received signal ", <-c)
		os.Exit(1)
	}()

	cobra.OnInitialize(config.InitViper)

	cmd.AddCommand(createcluster.Cmd)
	cmd.AddCommand(deletecluster.Cmd)
	cmd.AddCommand(upgradecluster.Cmd)
	cmd.AddCommand(releases.Cmd)
	cmd.AddCommand(listservers.Cmd)

	cmd.PersistentFlags().Bool(
		"debug", false,
		`Print debug messages.`,
	)

	_ = viper.BindPFlag("debug", cmd.PersistentFlags().Lookup("debug"))

	cmd.PersistentFlags().StringP(
		"config-file", "c", "",
		`Specify the config file to use.`,
	)

	_ = viper.BindPFlag("config_file", cmd.PersistentFlags().Lookup("config-file"))

	cmd.PersistentFlags().BoolP(
		"auto-approve", "y", false,
		`Automatic yes to prompts. Assume "yes" as answer to all prompts and run non-interactively.`,
	)

	_ = viper.BindPFlag("auto_approve", cmd.PersistentFlags().Lookup("auto-approve"))

	cmd.PersistentFlags().String(
		"github-token", "",
		`You can pass your github token if you reach the rate limit while querying K3s version.`,
	)

	_ = viper.BindPFlag("github_token", cmd.PersistentFlags().Lookup("github-token"))

	cmd.Flags().Bool(
		"print-environment", false, "Print all environment variables.",
	)

	_ = viper.BindPFlag("print_environment", cmd.Flags().Lookup("print-environment"))
}

func Execute() {
	if err := cmd.Execute(); err != nil {
		zap.S().Error(err)
	}
}

func Cmd(cmd *cobra.Command) error {
	if viper.GetBool("print_environment") {
		for i, v := range viper.AllSettings() {
			log.Printf("%v=%v", strings.ToUpper(i), v)
		}

		os.Exit(0)
	}

	// nolint: wrapcheck
	return cmd.Help()
}
