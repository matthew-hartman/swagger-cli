package swagger

import (
	"context"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed example.json
var testSwagger string

func TestParseSwagger(t *testing.T) {
	c := &Client{
		Name: "test",
	}
	swag, err := processSwaggerOverrides(testSwagger)
	if err != nil {
		t.Fatal(err)
	}

	cmd, err := c.parseSwagger(context.Background(), swag)
	if err != nil {
		t.Fatal(err)
	}

	if len(cmd.Cmds) != 1 {
		t.Fatal("unexpected number of commands returned")
	}

	gen := cmd.Cmds[0]
	assert.Equal(t, "GET", gen.Method)
	assert.Equal(t, "test-cmd", gen.Name)
	assert.Equal(t, "/test", gen.Path)
	assert.Equal(t, "short cmd description", gen.Command.Short)
	assert.Equal(t, "Long description\n", gen.Command.Long)
	assert.ElementsMatch(t, []string{"test", "alias", "override"}, gen.Command.Aliases)
}
