package swagger

import (
	"context"
	"fmt"
	"os"

	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/tidwall/gjson"
)

const (
	BaseURLFlag         = "base-url"
	BaseSwaggerPathFlag = "swagger-path"
)

type Client struct {
	Name               string
	BaseURLDefault     string
	SwaggerPathDefault string
}

func (c *Client) Flags() *pflag.FlagSet {
	baseFlags := pflag.NewFlagSet(c.Name, pflag.ContinueOnError)
	baseFlags.String(BaseURLFlag, c.BaseURLDefault,
		"base url of "+c.Name)
	baseFlags.String(BaseSwaggerPathFlag, c.SwaggerPathDefault,
		"default path of swagger.json on remote")

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
	for _, v := range generatedCmd.Cmds {
		cmd.AddCommand(v.Command)
		err = viper.BindPFlags(v.Command.Flags())
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) CreateSubCommands(ctx context.Context) (*Command, error) {

	baseURL := viper.GetString(BaseURLFlag)
	swaggerPath := viper.GetString(BaseSwaggerPathFlag)

	span, ctx := opentracing.StartSpanFromContext(ctx, "cli-build")
	defer span.Finish()

	swag, err := getSwagger(ctx, baseURL, swaggerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get swagger: %s", err)
	}

	cmd := &Command{
		flags:   map[string]*flag{},
		baseURL: baseURL,
	}

	gjson.Get(swag, "parameters").ForEach(func(k0, v0 gjson.Result) bool {
		if v0.Get("in").String() != "body" {
			return cmd.loadParameter(k0, v0)
		}
		return true
	})

	gjson.Get(swag, "x-swagger-cmds").ForEach(func(k0, v0 gjson.Result) bool {
		if v0.IsArray() {
			return true
		}
		return cmd.addCmd(ctx, swag, k0, v0)
	})

	return cmd, nil
}
