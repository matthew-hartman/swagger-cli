package swagger

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/tidwall/gjson"
)

const (
	BaseURLFlag         = "base-url"
	BaseSwaggerPathFlag = "swagger-path"
	BaseHealthPathFlag  = "health-path"
)

type Client struct {
	Name                  string
	BaseURLDefault        string
	SwaggerPathDefault    string
	HealthPathDefault     string
	HealthCheckFailedTmpl string
	FlagOutput            io.Writer
}

func (c *Client) Flags() *pflag.FlagSet {
	baseFlags := pflag.NewFlagSet(c.Name, pflag.ContinueOnError)
	baseFlags.String(BaseURLFlag, c.BaseURLDefault,
		"base url of "+c.Name)
	baseFlags.String(BaseSwaggerPathFlag, c.SwaggerPathDefault,
		"default path of swagger.json on remote")
	baseFlags.String(BaseHealthPathFlag, c.HealthPathDefault,
		"default path of health check on remote (set to empty string to bypass)",
	)
	if c.FlagOutput != nil {
		baseFlags.SetOutput(c.FlagOutput)
	} else {
		baseFlags.SetOutput(os.Stdout)
	}

	return baseFlags
}

func (c *Client) Bind(ctx context.Context, cmd *cobra.Command) error {
	cFlags := c.Flags()
	cmd.PersistentFlags().AddFlagSet(cFlags)
	err := viper.BindPFlags(cFlags)
	if err != nil {
		return err
	}
	_ = cFlags.Parse(os.Args)

	generatedCmd, err := c.CreateSubCommands(ctx)
	if err != nil {
		return err
	}

	defaultCmd := ""
	for _, v := range generatedCmd.Cmds {
		cmd.AddCommand(v.Command)
		err = viper.BindPFlags(v.Command.Flags())
		if err != nil {
			return err
		}
		if v.Default {
			defaultCmd = v.Name
		}
		v.ctx = ctx
	}

	// if help is in args dont set default command
	for _, v := range os.Args {
		if v == "help" || v == "-h" || v == "--help" {
			defaultCmd = ""
		}
	}
	if defaultCmd != "" {
		subCmd, _, err := cmd.Find(os.Args[1:])
		if err != nil || cmd.Use == subCmd.Use {
			args := append([]string{defaultCmd}, os.Args[1:]...)
			cmd.SetArgs(args)
		}
	}

	return nil
}

func (c *Client) CreateSubCommands(ctx context.Context) (*Command, error) {

	baseURL := viper.GetString(BaseURLFlag)
	swaggerPath := viper.GetString(BaseSwaggerPathFlag)
	healthPath := viper.GetString(BaseHealthPathFlag)

	ctx, span := tracer.Start(ctx, "health")
	defer span.End()

	err := getHealth(ctx, baseURL, healthPath)
	if err != nil {
		fmt.Printf(c.HealthCheckFailedTmpl, err)
		return nil, err
	}

	swag, err := getSwagger(ctx, baseURL, swaggerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get swagger: %s", err)
	}
	return c.parseSwagger(ctx, swag)
}

func (c *Client) parseSwagger(ctx context.Context, swag string) (*Command, error) {
	cmd := &Command{
		flags:   map[string]*flag{},
		baseURL: viper.GetString(BaseURLFlag),
	}

	gjson.Get(swag, "parameters").ForEach(func(k0, v0 gjson.Result) bool {
		if v0.Get("in").String() != "body" {
			return cmd.loadParameter(k0, v0)
		}
		return true
	})

	errG := []string{}

	gjson.Get(swag, "paths").ForEach(func(k0, v0 gjson.Result) bool {
		v0.ForEach(func(k1, v1 gjson.Result) bool {
			err := cmd.addCmd(k0.String(), k1.String(), v0, v1)
			if err != nil {
				errG = append(errG, err.Error())
			}
			return true
		})
		return true
	})
	if len(errG) != 0 {
		return nil, fmt.Errorf("failed to generate commands: (%v)", strings.Join(errG, ", "))
	}

	return cmd, nil
}
