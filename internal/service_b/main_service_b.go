package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

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
	http.HandleFunc("/cep", handleServiceB)

	fmt.Println("Service B running on port 8081")

	http.ListenAndServe(":8081", nil)
}

func handleServiceB(w http.ResponseWriter, r *http.Request) {
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

	localidade, err := getLocalidadeFromViaCep(req.Cep)
	if err != nil {
		http.Error(w, "Error getting localidade from ViaCEP API", http.StatusInternalServerError)
		return
	}

	weather, err := getWeather(localidade)
	if err != nil {
		http.Error(w, "Error getting weather from WeatherAPI", http.StatusInternalServerError)
		return
	}

	// Serializar a resposta da API para JSON
	jsonData, err := json.Marshal(weather)
	if err != nil {
		http.Error(w, "Error encoding weather data", http.StatusInternalServerError)
		return
	}

	// Definir o cabeçalho de conteúdo para application/json
	w.Header().Set("Content-Type", "application/json")

	// Escrever a resposta JSON
	w.Write(jsonData)
}

func getLocalidadeFromViaCep(cep string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var viaCepResponse ViaCepResponse
	if err := json.NewDecoder(resp.Body).Decode(&viaCepResponse); err != nil {
		return "", err
	}

	if viaCepResponse.Erro {
		return "", fmt.Errorf("Error in ViaCEP API response")
	}

	return viaCepResponse.Localidade, nil
}

func getWeather(localidade string) (WeatherResult, error) {
	resp, err := http.Get(fmt.Sprintf("https://api.weatherapi.com/v1/current.json?q=%s&key=3841b81037a5427eb51191826241702", localidade))
	if err != nil {
		return WeatherResult{}, err
	}
	defer resp.Body.Close()

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
