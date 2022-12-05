package swagger

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
)

type SubCmd struct {
	Name      string
	Path      string
	Method    string
	ServerURL string
	Default   bool

	ParsedFlags map[string]*flag
	*cobra.Command

	ctx context.Context
}

func (s *SubCmd) Run() error {
	span, ctx := opentracing.StartSpanFromContext(s.ctx, "cli-run")
	defer span.Finish()

	params := map[string]string{}

	s.Command.Flags().VisitAll(func(f *pflag.Flag) {
		for _, v := range []string{"help", "swagger-path", "base-url", "health-path"} {
			if v == f.Name {
				return
			}
		}
		fg := s.ParsedFlags[f.Name]
		switch fg.In {
		case "path":
			s.Path = strings.ReplaceAll(s.Path, fmt.Sprintf("{%s}", f.Name), f.Value.String())
		case "query":
			if f.Changed || (fg.Env && f.Value.String() != "") {
				if strings.Contains(f.Value.Type(), "Slice") {
					params[f.Name] = strings.Trim(f.Value.String(), "[]")
				} else {
					params[f.Name] = f.Value.String()
				}
			}
		}
	})

	req, err := http.NewRequest(s.Method, s.ServerURL+s.Path, nil)
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
	req.Header.Set("X-Raw-Args", strings.Join(os.Args, " "))

	q := req.URL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := doer.Do(ctx, req)
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
