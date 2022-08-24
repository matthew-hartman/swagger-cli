package client

import (
	"fmt"
	"io"

	ot "github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jConfig "github.com/uber/jaeger-client-go/config"
)

func NewTracer(name string) io.Closer {
	cfgEnv, err := jConfig.FromEnv()
	if err != nil {
		fmt.Printf("Failed to setup tracing: %v", err)
		return &io.PipeReader{}
	}

	cfg := jConfig.Configuration{
		Sampler: &jConfig.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jConfig.ReporterConfig{
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

	ot.SetGlobalTracer(tracer)
	return closer
}
