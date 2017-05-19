package registry

import (
	"fmt"
    "net/http"
    "bytes"
    "encoding/json"
    "io/ioutil"
    
	"github.com/docker/cli/cli"
	"github.com/docker/cli/cli/command"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type loginOptions struct {
	serverAddress string
	user          string
	password      string
	email         string
}

type loginData struct{
    AuthToken  string `json:"authToken"`
    UserId     string `json:"userId"`
}

type loginResp struct {
	Status string     `json:"status"`
	Data   loginData  `json:"data"`
    Message string    `json:"message"`
}


// NewLoginCommand creates a new `docker login` command
func NewLoginCommand(dockerCli command.Cli) *cobra.Command {
	var opts loginOptions

	cmd := &cobra.Command{
		Use:   "login [OPTIONS] [SERVER]",
		Short: "Log in to a Docker registry",
		Long:  "Log in to a Docker registry.\nIf no server is specified, the default is defined by the daemon.",
		Args:  cli.RequiresMaxArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.serverAddress = args[0]
			}
			return runLogin(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(&opts.user, "email", "e", "", "Email")
	flags.StringVarP(&opts.password, "password", "p", "", "Password")
    
	return cmd
}

func runLogin(dockerCli command.Cli, opts loginOptions) error {
    var serverAddress string

    if opts.serverAddress != "" {
		serverAddress = opts.serverAddress
	}

	authConfig, err := command.ConfigureAuth(dockerCli, opts.user, opts.password, serverAddress)
	if err != nil {
		return err
	}
    
    var data = []byte("email=" + authConfig.Username + "&" + "password=" + authConfig.Password )
    
    authClient := &http.Client{}
    resp, err := authClient.Post(serverAddress, "application/x-www-form-urlencoded", bytes.NewBuffer(data))
    if err != nil {
        return errors.Errorf("Error logging in: %v", err)
    }
    defer resp.Body.Close()

    var jsonResp loginResp
    body, _ := ioutil.ReadAll(resp.Body)
    if err := json.Unmarshal(body, &jsonResp); err != nil {
        fmt.Println(err)
        return errors.Errorf("Error in parsinng response: %v", err)
    }
        
    if jsonResp.Status == "success" {
		authConfig.Password = ""
		authConfig.IdentityToken = jsonResp.Data.AuthToken
    }
    
    dockerCli.ConfigFile().AddAuthConfig(serverAddress, authConfig)
    
    if err := dockerCli.ConfigFile().Save(); err != nil {
		return errors.Errorf("Error saving credentials: %v", err)
	}

	if jsonResp.Status != "" {
        fmt.Fprintln(dockerCli.Out(), string(body))
	}
	return nil
}
