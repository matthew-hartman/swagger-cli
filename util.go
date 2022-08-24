package client

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/tidwall/gjson"
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

func toStringSlice(a []gjson.Result) []string {
	ret := []string{}
	for _, v := range a {
		ret = append(ret, v.String())
	}
	return ret
}