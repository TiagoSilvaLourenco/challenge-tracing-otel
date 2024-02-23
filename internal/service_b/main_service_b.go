package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type TemplateData struct {
	ResponseTime time.Duration
	OTELTracer   trace.Tracer
}

type ServiceAHandler struct {
	templateData *TemplateData
}

type WeatherResult struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

type WeatherAPIResponse struct {
	Location struct {
		Name string `json:"name"`
	} `json:"location"`
	Current struct {
		TempC float64 `json:"temp_c"`
	} `json:"current"`
}

type ViaCepResponse struct {
	Localidade string `json:"localidade"`
	Erro       bool   `json:"erro"`
}

type Request struct {
	Cep string `json:"cep"`
}

// func (h *ServiceAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

// 	carrier := propagation.HeaderCarrier(r.Header)
// 	ctx := r.Context()
// 	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)

// 	ctx, span := h.templateData.OTELTracer.Start(ctx, "Service B Request")
// 	defer span.End()

// 	time.Sleep(time.Millisecond * h.templateData.ResponseTime)

// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	body, err := io.ReadAll(r.Body)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	var req Request
// 	if err := json.Unmarshal(body, &req); err != nil {
// 		http.Error(w, "Error decoding request body", http.StatusBadRequest)
// 		return
// 	}

// 	if len(req.Cep) != 8 {
// 		http.Error(w, "Invalid CEP", http.StatusUnprocessableEntity)
// 		return
// 	}

// 	// Span para busca de localidade no ViaCEP
// 	ctx, viaCepSpan := h.templateData.OTELTracer.Start(ctx, "ViaCEP API Call")
// 	viaCepSpan.SetAttributes(semconv.HTTPMethod(http.MethodGet), semconv.HTTPURL("https://viacep.com.br/ws/"+req.Cep+"/json/"))
// 	defer viaCepSpan.End()

// 	localidade, err := getLocalidadeFromViaCep(ctx, req.Cep)
// 	if err != nil {
// 		http.Error(w, "Error getting localidade from ViaCEP API", http.StatusInternalServerError)
// 		return
// 	}

// 	// Span para busca de temperatura no WeatherAPI
// 	ctx, weatherSpan := h.templateData.OTELTracer.Start(ctx, "WeatherAPI Call")
// 	weatherSpan.SetAttributes(semconv.HTTPMethod(http.MethodGet), semconv.HTTPURL("https://api.weatherapi.com/v1/current.json?q="+localidade+"&key=3841b81037a5427eb51191826241702"))
// 	defer weatherSpan.End()

// 	weather, err := getWeather(ctx, localidade)
// 	if err != nil {
// 		http.Error(w, "Error getting weather from WeatherAPI", http.StatusInternalServerError)
// 		return
// 	}

// 	// Serializar a resposta da API para JSON
// 	jsonData, err := json.Marshal(weather)
// 	if err != nil {
// 		http.Error(w, "Error encoding weather data", http.StatusInternalServerError)
// 		return
// 	}

// 	// Definir o cabeçalho de conteúdo para application/json
// 	w.Header().Set("Content-Type", "application/json")

// 	// Escrever a resposta JSON
// 	w.Write(jsonData)
// }

func main() {
	// Configure o exportador para enviar os spans de tracing para o Zipkin.
	exporter, err := zipkin.New(
		"http://zipkin:9411/api/v2/spans",
	)
	if err != nil {
		log.Fatal(err)
	}

	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Configure o propagador global para usar o formato B3 (usado pelo Zipkin).
	// b3Propagator := b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader))
	// otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}, b3Propagator))

	http.HandleFunc("/cep", handleServiceB)

	fmt.Println("Service B running on port 8081")

	http.ListenAndServe(":8081", nil)
}

func handleServiceB(w http.ResponseWriter, r *http.Request) {

	ctx := otel.GetTextMapPropagator().Extract(context.Background(), propagation.HeaderCarrier(r.Header))
	_, spanHandleServiceB := otel.GetTracerProvider().Tracer("service_b").Start(ctx, "handleServiceB")
	defer spanHandleServiceB.End()

	// // Extraia o contexto do span do cabeçalho HTTP.
	// ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

	// // Agora você pode começar a criar spans de tracing.
	// tracer := otel.Tracer("service_b")

	// // Crie um novo span.
	// _, span := tracer.Start(ctx, "handleServiceB")
	// defer span.End()

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Error decoding request body", http.StatusBadRequest)
		return
	}

	if len(req.Cep) != 8 {
		http.Error(w, "Invalid CEP", http.StatusUnprocessableEntity)
		return
	}

	localidade, err := getLocalidadeFromViaCep(ctx, req.Cep)
	if err != nil {
		http.Error(w, "Error getting localidade from ViaCEP API", http.StatusInternalServerError)
		return
	}
	_, spanLocalidade := otel.GetTracerProvider().Tracer("service_b").Start(ctx, "Return from getLocalidadeFromViaCep")
	spanLocalidade.SetAttributes(attribute.String("localidade", localidade))
	defer spanLocalidade.End()

	// tracer = otel.Tracer("service_b")
	// ctx, span = tracer.Start(r.Context(), "busca de temperatura")
	// defer span.End()
	weather, err := getWeather(ctx, localidade)
	if err != nil {
		http.Error(w, "Error getting weather from WeatherAPI", http.StatusInternalServerError)
		return
	}

	_, spanWeather := otel.GetTracerProvider().Tracer("service_b").Start(ctx, "Return from getWeather from weatherapi")

	weatherReturn := fmt.Sprintf("Weather in %s: %.1fC, %.1fF, %.1fK", weather.City, weather.TempC, weather.TempF, weather.TempK)

	spanWeather.SetAttributes(attribute.String("weather", weatherReturn))

	defer spanWeather.End()

	// Serializar a resposta da API para JSON
	jsonData, err := json.Marshal(weather)
	if err != nil {
		http.Error(w, "Error encoding weather data", http.StatusInternalServerError)
		return
	}

	// Definir o cabeçalho de conteúdo para application/json
	w.Header().Set("Content-Type", "application/json")

	// fmt.Println(string(jsonData))
	// log.Println(w.Header().Get("Content"))
	// Escrever a resposta JSON
	_, spanJsonData := otel.GetTracerProvider().Tracer("service_b").Start(ctx, "Return jsonData to service A")
	defer spanJsonData.End()

	w.Write(jsonData)
}

func getLocalidadeFromViaCep(ctx context.Context, cep string) (string, error) {

	_, span := otel.GetTracerProvider().Tracer("service_b").Start(ctx, "getLocalidadeFromViaCep-request-to-ViaCEP")
	defer span.End()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://viacep.com.br/ws/"+cep+"/json/", nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Error in ViaCEP API response: %s", resp.Status)
	}

	var viaCepResponse ViaCepResponse
	if err := json.NewDecoder(resp.Body).Decode(&viaCepResponse); err != nil {
		return "", err
	}

	if viaCepResponse.Erro {
		return "", fmt.Errorf("Error in ViaCEP API response: %s", viaCepResponse.Erro)
	}

	return viaCepResponse.Localidade, nil
}

func getWeather(ctx context.Context, localidade string) (WeatherResult, error) {
	ctx, span := otel.GetTracerProvider().Tracer("service_b").Start(ctx, "getWeather-request-to-WeatherAPI")
	defer span.End()

	// scape the URL
	urlWeatherAPI := fmt.Sprintf("https://api.weatherapi.com/v1/current.json?q=%s&key=3841b81037a5427eb51191826241702", url.QueryEscape(localidade))

	fmt.Println(urlWeatherAPI)

	//
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlWeatherAPI, nil)
	if err != nil {
		return WeatherResult{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return WeatherResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return WeatherResult{}, fmt.Errorf("Error in WeatherAPI response: %s", resp.Status)
	}

	var weatherResponse WeatherAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&weatherResponse); err != nil {
		return WeatherResult{}, err
	}

	tempC, _ := strconv.ParseFloat(fmt.Sprintf("%.1f", weatherResponse.Current.TempC), 64)
	tempF, _ := strconv.ParseFloat(fmt.Sprintf("%.1f", weatherResponse.Current.TempC*1.8+32), 64)
	tempK, _ := strconv.ParseFloat(fmt.Sprintf("%.1f", weatherResponse.Current.TempC+273), 64)

	weatherResult := WeatherResult{
		City:  weatherResponse.Location.Name,
		TempC: tempC,
		TempF: tempF,
		TempK: tempK,
	}

	return weatherResult, nil
}
