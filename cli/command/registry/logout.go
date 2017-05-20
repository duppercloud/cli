package registry

import (
	"fmt"

	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli"
	"github.com/docker/cli/cli/command"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// NewLogoutCommand creates a new `docker logout` command
func NewLogoutCommand(dockerCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout [SERVER]",
		Short: "Log out from a Docker registry",
		Long:  "Log out from a Docker registry.\nIf no server is specified, the default is defined by the daemon.",
		Args:  cli.RequiresMaxArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var serverAddress string
			if len(args) > 0 {
				serverAddress = args[0]
            } else {
                serverAddress = configfile.DefaultAuthServer
            }
            
			return runLogout(dockerCli, serverAddress)
		},
	}

	return cmd
}

func runLogout(dockerCli command.Cli, serverAddress string) error {

    // check if we're logged in based on the records in the config file
	// which means it couldn't have user/pass cause they may be in the creds store
    if _, ok := dockerCli.ConfigFile().AuthConfigs[serverAddress]; !ok {
		fmt.Fprintf(dockerCli.Out(), "Not logged in to %s\n", serverAddress)
		return nil
    }

    dockerCli.ConfigFile().DelAuthConfig(serverAddress)

    if err := dockerCli.ConfigFile().Save(); err != nil {
		return errors.Errorf("Error removing credentials: %v", err)
	}

	fmt.Fprintf(dockerCli.Out(), "Removed login information for %s\n", serverAddress)
	return nil
}
