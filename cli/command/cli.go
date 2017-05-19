package command

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"

	cliconfig "github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/docker/docker/dockerversion"
	dopts "github.com/docker/docker/opts"
	"github.com/docker/go-connections/sockets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

// Streams is an interface which exposes the standard input and output streams
type Streams interface {
	In() *InStream
	Out() *OutStream
	Err() io.Writer
}

// Cli represents the docker command line client.
type Cli interface {
	Client() client.APIClient
	Out() *OutStream
	Err() io.Writer
	In() *InStream
	SetIn(in *InStream)
	ConfigFile() *configfile.ConfigFile
}

// DockerCli is an instance the docker command line client.
// Instances of the client can be returned from NewDockerCli.
type DockerCli struct {
	configFile     *configfile.ConfigFile
	in             *InStream
	out            *OutStream
	err            io.Writer
	client         client.APIClient
	defaultVersion string
	server         ServerInfo
}

// DefaultVersion returns api.defaultVersion or DOCKER_API_VERSION if specified.
func (cli *DockerCli) DefaultVersion() string {
	return cli.defaultVersion
}

// Client returns the APIClient
func (cli *DockerCli) Client() client.APIClient {
	return cli.client
}

// Out returns the writer used for stdout
func (cli *DockerCli) Out() *OutStream {
	return cli.out
}

// Err returns the writer used for stderr
func (cli *DockerCli) Err() io.Writer {
	return cli.err
}

// SetIn sets the reader used for stdin
func (cli *DockerCli) SetIn(in *InStream) {
	cli.in = in
}

// In returns the reader used for stdin
func (cli *DockerCli) In() *InStream {
	return cli.in
}

// ShowHelp shows the command help.
func ShowHelp(err io.Writer) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cmd.SetOutput(err)
		cmd.HelpFunc()(cmd, args)
		return nil
	}
}

// ConfigFile returns the ConfigFile
func (cli *DockerCli) ConfigFile() *configfile.ConfigFile {
	return cli.configFile
}

// ServerInfo returns the server version details for the host this client is
// connected to
func (cli *DockerCli) ServerInfo() ServerInfo {
	return cli.server
}

// Initialize the dockerCli runs initialization that must happen after command
// line flags are parsed.
func (cli *DockerCli) Initialize(opts *cliflags.ClientOptions) error {
	cli.configFile = LoadDefaultConfigFile(cli.err)

	var err error
	cli.client, err = NewAPIClientFromFlags(opts.Common, cli.configFile)

	if err != nil {
		return err
	}

	cli.defaultVersion = cli.client.ClientVersion()

	if ping, err := cli.client.Ping(context.Background()); err == nil {
		cli.server = ServerInfo{
			HasExperimental: ping.Experimental,
			OSType:          ping.OSType,
		}

		// since the new header was added in 1.25, assume server is 1.24 if header is not present.
		if ping.APIVersion == "" {
			ping.APIVersion = "1.24"
		}

		// if server version is lower than the current cli, downgrade
		if versions.LessThan(ping.APIVersion, cli.client.ClientVersion()) {
			cli.client.UpdateClientVersion(ping.APIVersion)
		}
	}

	return nil
}

// ServerInfo stores details about the supported features and platform of the
// server
type ServerInfo struct {
	HasExperimental bool
	OSType          string
}

// NewDockerCli returns a DockerCli instance with IO output and error streams set by in, out and err.
func NewDockerCli(in io.ReadCloser, out, err io.Writer) *DockerCli {
	return &DockerCli{in: NewInStream(in), out: NewOutStream(out), err: err}
}

// LoadDefaultConfigFile attempts to load the default config file and returns
// an initialized ConfigFile struct if none is found.
func LoadDefaultConfigFile(err io.Writer) *configfile.ConfigFile {
	configFile, e := cliconfig.Load(cliconfig.Dir())
	if e != nil {
		fmt.Fprintf(err, "WARNING: Error loading config file:%v\n", e)
	}
	if !configFile.ContainsAuth() {
        fmt.Fprintf(err, "Please login first using dupper login.")
	}
	return configFile
}

// NewAPIClientFromFlags creates a new APIClient from command line flags
func NewAPIClientFromFlags(opts *cliflags.CommonOptions, configFile *configfile.ConfigFile) (client.APIClient, error) {
	host, err := getServerHost(opts.Hosts)
	if err != nil {
		return &client.Client{}, err
	}

	customHeaders := configFile.HTTPHeaders
	if customHeaders == nil {
		customHeaders = map[string]string{}
	}
	customHeaders["User-Agent"] = UserAgent()
    

	verStr := api.DefaultVersion
	if tmpStr := os.Getenv("DOCKER_API_VERSION"); tmpStr != "" {
		verStr = tmpStr
	}

	httpClient, err := newHTTPClient(host)
	if err != nil {
		return &client.Client{}, err
	}

	return client.NewClient(host, verStr, httpClient, customHeaders)
}

func getServerHost(hosts []string) (host string, err error) {
	switch len(hosts) {
	case 0:
		host = os.Getenv("DOCKER_HOST")
	case 1:
		host = hosts[0]
	default:
		return "", errors.New("Please specify only one -H")
	}

	host, err = dopts.ParseHost(false, host)
	return
}

func newHTTPClient(host string) (*http.Client, error) {

    proto, addr, _, err := client.ParseHost(host)
	if err != nil {
		return nil, err
	}
    
    tr := &http.Transport{}
    
    sockets.ConfigureTransport(tr, proto, addr)

	return &http.Client{
        Transport: tr,
	}, nil
}

// UserAgent returns the user agent string used for making API requests
func UserAgent() string {
	return "Docker-Client/" + dockerversion.Version + " (" + runtime.GOOS + ")"
}
