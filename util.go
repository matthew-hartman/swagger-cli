package swagger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/opentracing/opentracing-go"
	"github.com/tidwall/gjson"
)

type HTTP interface {
	Do(ctx context.Context, req *http.Request) (*http.Response, error)
}

type DefaultDoer struct{}

func (d *DefaultDoer) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	return http.DefaultClient.Do(req)
}

var doer HTTP = &DefaultDoer{}

func SetHTTP(d HTTP) {
	doer = d
}

func getHealth(ctx context.Context, baseURL, path string) error {
	if path == "" {
		return nil
	}

	span, ctx := opentracing.StartSpanFromContext(ctx, "swagger")
	defer span.Finish()

	req, err := http.NewRequest("GET", baseURL+path, nil)
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

	req.Header.Set("User-Agent", "swagger-cli")

	_, err = doer.Do(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func getSwagger(ctx context.Context, baseURL, path string) (string, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "swagger")
	defer span.Finish()

	req, err := http.NewRequest("GET", baseURL+path, nil)
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

	resp, err := doer.Do(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	s, err := processSwaggerOverrides(string(b))
	if err != nil {
		return "", err
	}
	return s, nil
}

func processSwaggerOverrides(swag string) (string, error) {
	overrides := gjson.Get(swag, "x-swagger-override")
	if !overrides.Exists() {
		return swag, nil
	}

	var mp map[string]interface{}
	err := json.Unmarshal([]byte(swag), &mp)
	if err != nil {
		return "", err
	}

	delete(mp, "x-swagger-override")

	d, err := json.Marshal(mp)
	if err != nil {
		return "", err
	}

	result, err := MergeBytes(d, []byte(overrides.Raw))
	if err != nil {
		return "", err
	}

	return string(result), nil
}

func toStringSlice(a []gjson.Result) []string {
	ret := []string{}
	for _, v := range a {
		ret = append(ret, v.String())
	}
	return ret
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func ToKebabCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}-${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}-${2}")
	return strings.ToLower(snake)
}
