package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.23.1"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Zipcode struct {
	Zipcode interface{} `json:"cep"`
}

type TemplateData struct {
	ResponseTime       time.Duration
	ExternalCallURL    string
	ExternalCallMethod string
	RequestNameOTEL    string
	Content            string
	OTELTracer         trace.Tracer
}

type ServiceAHandler struct {
	templateData *TemplateData
}

func (h *ServiceAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Crie um novo contexto de rastreamento.
	ctx := context.Background()
	ctx, span := otel.GetTracerProvider().Tracer("service_a").Start(ctx, "ServeHTTP")
	defer span.End()

	// // Crie um novo tracer.
	// tracer := otel.Tracer("service_a")

	// // Inicie um novo span.
	// ctx, span := tracer.Start(ctx, "ServeHTTP")
	// defer span.End()

	time.Sleep(time.Millisecond * h.templateData.ResponseTime)

	if h.templateData.ExternalCallURL != "" {

		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var cep Zipcode
		err := json.NewDecoder(r.Body).Decode(&cep)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		cepStr, ok := cep.Zipcode.(string)
		if !ok {
			http.Error(w, "Invalid zipcode", http.StatusUnprocessableEntity)
			return
		}

		if len(cepStr) != 8 {
			http.Error(w, "Zipcode must have 8 digits", http.StatusUnprocessableEntity)
			return
		}

		span.SetAttributes(attribute.String("cep", cepStr))

		// Send to Service B
		jsonData, err := json.Marshal(Zipcode{Zipcode: cepStr})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", h.templateData.ExternalCallURL, bytes.NewBuffer(jsonData))

		// resp, err := http.Post(h.templateData.ExternalCallURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

		_, span := otel.GetTracerProvider().Tracer("service_a").Start(ctx, "handle request to service B")
		defer span.End()

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		// read the response from Service B
		body, err := io.ReadAll(resp.Body)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_, spanBody := otel.GetTracerProvider().Tracer("service_b").Start(ctx, "Return coming from Service B")
		spanBody.SetAttributes(attribute.String("responseData", string(body)))
		defer spanBody.End()

		// //

		// Unmarshal the JSON response
		var result interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Marshal the result with indentation
		formattedBody, err := json.MarshalIndent(result, "", "  ")

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		h.templateData.Content = string(formattedBody)

		// Definir o cabeçalho de conteúdo para application/json
		w.Header().Set("Content-Type", "application/json")

		// Write the formatted response from Service B to the response of Service A
		w.Write(formattedBody)
	}
}
func initProvider(serviceName, collectorURL string) (func(context.Context) error, error) {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, collectorURL, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)

	// Configurar o manipulador de erros
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		log.Println("caught an error:", err)
	}))

	// Configurar o propagador
	otel.SetTextMapPropagator(propagation.TraceContext{}) // Configurar manipulador de erros

	return traceExporter.Shutdown, nil

}

func init() {
	viper.AutomaticEnv()
}

func main() {
	exporter, err := zipkin.New(
		"http://localhost:9411/api/v2/spans",
	)
	if err != nil {
		log.Fatal(err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	shutdown, err := initProvider(viper.GetString("OTEL_SERVICE_NAME"), viper.GetString("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatal("Failed to shutdown provider: %w", err)
		}
	}()

	tracer := otel.Tracer("service-a")

	templateData := &TemplateData{
		ResponseTime:       time.Duration(viper.GetInt("RESPONSE_TIME")),
		ExternalCallURL:    viper.GetString("EXTERNAL_CALL_URL"),
		ExternalCallMethod: viper.GetString("EXTERNAL_CALL_METHOD"),
		RequestNameOTEL:    viper.GetString("OTEL_REQUEST_NAME"),
		OTELTracer:         tracer,
	}

	// server := web.NewServer(templateData)
	// router := server.CreateServer()

	handler := &ServiceAHandler{
		templateData: templateData,
	}

	go func() {
		log.Println("Starting server on port :" + viper.GetString("HTTP_PORT"))

		http.Handle("/cep", handler)

		if err := http.ListenAndServe(":"+viper.GetString("HTTP_PORT"), nil); err != nil {
			log.Fatal(err)
		}
	}()

	select {
	case <-sigCh:
		log.Println("Shutting down graceully, CTRL+C pressed...")
	case <-ctx.Done():
		log.Println("Shutting down due to other reason...")
	}

	_, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

}
