package swagger

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/term"
)

var tracer trace.Tracer

func init() {
	tracer = trace.NewNoopTracerProvider().Tracer("swagger")
}

func SetTracer(t trace.Tracer) {
	tracer = t
}

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

	ctx, span := tracer.Start(ctx, "health")
	defer span.End()

	req, err := http.NewRequest("GET", baseURL+path, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "swagger-cli")
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	_, err = doer.Do(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func getSwagger(ctx context.Context, baseURL, path string) (string, error) {
	ctx, span := tracer.Start(ctx, "swagger")
	defer span.End()

	req, err := http.NewRequest("GET", baseURL+path, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "swagger-cli")
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := doer.Do(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New(resp.Status)
	}

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

func getTerminalWidth() int {
	width, _, err := term.GetSize(1)
	if err != nil {
		return -1
	}
	return width
}
