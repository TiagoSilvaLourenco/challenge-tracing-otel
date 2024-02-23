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

type errorResponse struct {
	Error string `json:"error"`
}

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

	http.HandleFunc("/cep", handleServiceB)

	fmt.Println("Service B running on port 8081")

	http.ListenAndServe(":8081", nil)
}

func handleServiceB(w http.ResponseWriter, r *http.Request) {

	ctx := otel.GetTextMapPropagator().Extract(context.Background(), propagation.HeaderCarrier(r.Header))
	_, spanHandleServiceB := otel.GetTracerProvider().Tracer("service_b").Start(ctx, "handleServiceB")
	defer spanHandleServiceB.End()

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

	_, spanLocalidade := otel.GetTracerProvider().Tracer("service_b").Start(ctx, "Return from getLocalidadeFromViaCep")
	spanLocalidade.SetAttributes(attribute.String("localidade", localidade))
	defer spanLocalidade.End()

	if err != nil {
		response := errorResponse{
			Error: "Error getting localidade from ViaCEP API",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(response)
		return
	}

	weather, err := getWeather(ctx, localidade)
	if err != nil {
		response := errorResponse{
			Error: "Error getting weather from WeatherAPI",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(response)
		return
	}

	_, spanWeather := otel.GetTracerProvider().Tracer("service_b").Start(ctx, "Return from getWeather from weatherapi")

	weatherReturn := fmt.Sprintf("Weather in %s: %.1fC, %.1fF, %.1fK", weather.City, weather.TempC, weather.TempF, weather.TempK)

	spanWeather.SetAttributes(attribute.String("weather", weatherReturn))

	defer spanWeather.End()

	// Serializar a resposta da API para JSON
	jsonData, err := json.Marshal(weather)
	if err != nil {
		response := errorResponse{
			Error: "Error encoding weather data",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Definir o cabeçalho de conteúdo para application/json
	w.Header().Set("Content-Type", "application/json")

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
