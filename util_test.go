package swagger

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

const testJson = `{
	"x-swagger-override": {
	  "paths": {
		"/abort": {
		  "get": {
			"x-swagger-cmd": "abort"
		  }
		}
	  }
	},
	"paths": {
	  "/override": {
		"get": {
		  "whatever": [ "normally", "goes", "here" ]
		}
	  },
	  "/abort": {
		"get": {
		  "whatever": [ "normally", "goes", "here" ]
		}
	  }
	}
}`

func TestOverrides(t *testing.T) {
	input := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(testJson, "\t", ""), "\n", ""), " ", "")
	res, err := processSwaggerOverrides(input)
	if err != nil {
		t.Error(err)
	}
	resJ := gjson.Parse(res)
	if !resJ.Get("paths./abort.get.whatever").Exists() {
		t.Error("paths./abort.get.whatever not found")
	}
	if !resJ.Get("paths./abort.get.x-swagger-cmd").Exists() {
		t.Error("paths./abort.get.x-swagger-cmd not found")
	}
	if resJ.Get("x-swagger-override").Exists() {
		t.Error("x-swagger-override was not removed from json")
	}
	t.Log(res)
}
