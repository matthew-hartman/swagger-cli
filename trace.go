package swagger

import (
	"fmt"
	"io"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
)

func SetupTracer(name string) io.Closer {
	cfgEnv, err := config.FromEnv()
	if err != nil {
		fmt.Printf("Failed to setup tracing: %v", err)
		return &io.PipeReader{}
	}

	cfg := config.Configuration{
		Sampler: &config.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans:           true,
			LocalAgentHostPort: cfgEnv.Reporter.LocalAgentHostPort,
		},
		ServiceName: name,
	}

	tracer, closer, err := cfg.NewTracer()
	if err != nil {
		fmt.Printf("Failed to setup tracing: %v", err)
		return &io.PipeReader{}
	}

	opentracing.SetGlobalTracer(tracer)
	return closer
}
