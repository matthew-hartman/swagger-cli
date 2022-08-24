package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

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

func getSwagger(ctx context.Context, baseURL, path string) (string, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "swagger")
	defer span.Finish()

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+path, nil)
	if err != nil {
		return "", err
	}

	err = opentracing.GlobalTracer().Inject(
		span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header))
	if err != nil {
		fmt.Printf("Error Injecting span context to http req: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "swagger-cli")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

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

type Command struct {
	flags   map[string]*flag
	baseURL string
	Cmds    []*SubCmd
}

func (c *Command) loadParameter(key, value gjson.Result) bool {
	c.flags[fmt.Sprintf("#/parameters/%s", key.String())] = c.parseFlag(value)
	return true
}

func (c *Command) addCmd(
	ctx context.Context,
	swag string,
	k0, v0 gjson.Result,
) bool {

	name := k0.String()
	method := v0.Get("method").String()
	path := v0.Get("path").String()
	alias := toStringSlice(v0.Get("alias").Array())
	outerParameters := gjson.Get(swag, fmt.Sprintf("paths.%s.parameters", path))
	innerParameters := gjson.Get(swag, fmt.Sprintf("paths.%s.%s.parameters", path, strings.ToLower(method)))
	operation := gjson.Get(swag, fmt.Sprintf("paths.%s.%s", path, strings.ToLower(method)))

	s := &SubCmd{
		ParsedFlags: make(map[string]*flag),
		Path:        path,
		Method:      strings.ToUpper(method),
		ServerURL:   c.baseURL,
		Command: &cobra.Command{
			Use:     name,
			Short:   operation.Get("summary").String(),
			Long:    operation.Get("description").String(),
			Aliases: alias,
		},
		ctx: ctx,
	}
	s.Command.Run = func(cmd *cobra.Command, args []string) {
		cobra.CheckErr(s.Run())
	}
	handleParams := func(k2, v2 gjson.Result) bool {
		if v2.Get("in").String() == "body" {
			return true
		}

		fg := c.parseFlag(v2).Register(s.Command.Flags())
		s.ParsedFlags[fg.Name] = fg

		if fg.Required {
			err := s.Command.MarkFlagRequired(fg.Name)
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"Failed to mark flag as required: %s", err)
			}
		}
		return true
	}

	innerParameters.ForEach(handleParams)
	outerParameters.ForEach(handleParams)

	c.Cmds = append(c.Cmds, s)
	return true
}

func (c *Command) parseFlag(v gjson.Result) *flag {
	if v.Get("$ref").Exists() {
		return c.flags[v.Get("$ref").String()]
	}

	t := v.Get("type").String()
	if t == "array" {
		t = "array." + v.Get("items.type").String()
	}
	req := v.Get("required").Bool()
	def := v.Get("default")
	env := false
	if val := v.Get("x-swagger-cmd-env"); val.Exists() {
		def = gjson.Result{
			Type: gjson.String,
			Str:  os.Getenv(val.String()),
		}
		env = true
		if def.Str != "" {
			req = false
		}
	}

	return &flag{
		Name:        v.Get("name").String(),
		Short:       v.Get("x-swagger-cmd-short").String(),
		Description: v.Get("description").String(),
		Type:        t,
		Default:     def,
		In:          v.Get("in").String(),
		Required:    req,
		Env:         env,
		src:         v,
	}
}

type flag struct {
	Name        string
	Short       string
	Description string
	Type        string
	In          string
	Env         bool
	Required    bool
	Default     gjson.Result

	src gjson.Result
}

func (f *flag) Register(flags *pflag.FlagSet) *flag {
	switch f.Type {
	case "string":
		flags.StringP(f.Name, f.Short, f.Default.String(), f.Description)
	case "integer":
		flags.Int64P(f.Name, f.Short, f.Default.Int(), f.Description)
	case "boolean":
		flags.BoolP(f.Name, f.Short, f.Default.Bool(), f.Description)
	case "array.string":
		flags.StringSliceP(f.Name, f.Short, nil, f.Description)
	case "array.integer":
		flags.Int64SliceP(f.Name, f.Short, nil, f.Description)
	case "array.boolean":
		flags.BoolSliceP(f.Name, f.Short, nil, f.Description)
	default:
		fmt.Printf("unknown type: %v, %v\n", f.Type, f.src)
	}
	return f
}

type SubCmd struct {
	Name      string
	Path      string
	Method    string
	ServerURL string

	ShortDesc string
	LongDesc  string

	ParsedFlags map[string]*flag
	*cobra.Command

	ctx context.Context
}

func (s *SubCmd) Run() error {
	span, ctx := opentracing.StartSpanFromContext(s.ctx, "cli-run")
	defer span.Finish()

	params := map[string]string{}

	s.Command.Flags().VisitAll(func(f *pflag.Flag) {
		for _, v := range []string{"help", "swagger-path", "base-url"} {
			if v == f.Name {
				return
			}
		}
		fg := s.ParsedFlags[f.Name]
		switch fg.In {
		case "path":
			s.Path = strings.ReplaceAll(s.Path, fmt.Sprintf("{%s}", f.Name), f.Value.String())
		case "query":
			if f.Changed || fg.Env {
				if strings.Contains(f.Value.Type(), "Slice") {
					params[f.Name] = strings.Trim(f.Value.String(), "[]")
				} else {
					params[f.Name] = f.Value.String()
				}
			}
		}
	})

	req, err := http.NewRequestWithContext(
		ctx, s.Method,
		s.ServerURL+s.Path, nil,
	)
	if err != nil {
		return err
	}

	err = opentracing.GlobalTracer().Inject(
		span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header))
	if err != nil {
		fmt.Printf("Error Injecting span context to http req: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/plain")
	req.Header.Set("User-Agent", "swagger-cli")

	q := req.URL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("ERR: " + err.Error())
	}

	fmt.Println(strings.Trim(string(b), "\n"))

	return nil
}

func toStringSlice(a []gjson.Result) []string {
	ret := []string{}
	for _, v := range a {
		ret = append(ret, v.String())
	}
	return ret
}
