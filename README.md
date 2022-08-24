# swagger-cli
Generate updating cli's for services that provide swagger specs

```
rootCmd := &cobra.Command{
  Use: "swagger-cli",
}

c := swagger.Client{
	Name:               "swagger-cli",
	BaseURLDefault:     "http://localhost:8010",
	SwaggerPathDefault: "/swagger.json",
}

cobra.CheckErr(c.Bind(context.Background(), rootCmd))
cobra.CheckErr(rootCmd.Execute())

```
