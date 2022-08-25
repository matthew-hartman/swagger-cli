package swagger

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tidwall/gjson"
)

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
	path, method string,
	pathJSON, methodJSON gjson.Result,
) error {

	if !methodJSON.Get("x-swagger-cmd").Exists() {
		return nil
	}

	name := methodJSON.Get("x-swagger-cmd").String()
	alias := toStringSlice(methodJSON.Get("x-swagger-cmd-alias").Array())

	s := &SubCmd{
		Name:        name,
		ParsedFlags: make(map[string]*flag),
		Path:        path,
		Method:      strings.ToUpper(method),
		ServerURL:   c.baseURL,
		Command: &cobra.Command{
			Use:     name,
			Short:   methodJSON.Get("summary").String(),
			Long:    methodJSON.Get("description").String(),
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

		if fg.Required && fg.In != "path" {
			err := s.Command.MarkFlagRequired(fg.Name)
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"Failed to mark flag as required: %s", err)
			}
		}
		return true
	}

	methodJSON.Get("parameters").ForEach(handleParams)
	pathJSON.Get("parameters").ForEach(handleParams)

	c.Cmds = append(c.Cmds, s)
	return nil
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
		Name:        ToKebabCase(v.Get("name").String()),
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
