package main

import (
	"context"
	"os"
	"strings"

	swagger "github.com/matthew-hartman/swagger-cli"
	"github.com/opentracing/opentracing-go"

	"github.com/spf13/cobra"
)

func Execute() {

	name := "swagger-cmd"

	rootCmd := &cobra.Command{
		Use: name,
	}

	c := swagger.Client{
		Name:               name,
		BaseURLDefault:     "http://localhost:8010",
		SwaggerPathDefault: "/swagger.json",
	}

	close := swagger.SetupTracer(name)
	defer close.Close()

	span, ctx := opentracing.StartSpanFromContext(context.Background(), "cli")
	defer span.Finish()

	span.LogKV("args", strings.Join(os.Args, " "))

	cobra.CheckErr(c.Bind(ctx, rootCmd))
	cobra.CheckErr(rootCmd.Execute())
}

func main() {
	Execute()
}
