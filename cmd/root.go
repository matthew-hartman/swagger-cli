package main

import (
	swagger "github.com/matthew-hartman/swagger-cli"

	"github.com/spf13/cobra"
)

func Execute() {

	rootCmd := &cobra.Command{
		Use: "swagger-cli",
	}

	c := swagger.Client{
		Name:               "swagger-cli",
		BaseURLDefault:     "http://localhost:5000",
		SwaggerPathDefault: "/swagger.json",
	}

	cobra.CheckErr(c.Bind(rootCmd))
	cobra.CheckErr(rootCmd.Execute())
}

func main() {
	Execute()
}
