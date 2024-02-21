package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"sync"

	"github.com/gorilla/mux"
)

type Weather struct {
	TempC float64 `json:"tempC"`
	TempF float64 `json:"tempF"`
	TempK float64 `json:"tempK"`
}

type ViaCEPResponse struct {
	Localidade string `json:"localidade"`
	Erro       bool   `json:"erro"`
}

type WeatherAPIResponse struct {
	Current struct {
		TempC float64 `json:"temp_c"`
	} `json:"current"`
}

func setupRouter() *http.ServeMux {
	r := http.NewServeMux()
	r.HandleFunc("/{cep}", weatherHandler)
	return r
}

func main() {
	r := setupRouter()

	http.Handle("/", r)

	fmt.Println("Servidor rodando na porta 8080 url: http://localhost:8080/01001000")
	http.ListenAndServe(":8080", nil)
}

func weatherHandler(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	var viaCEPResponse ViaCEPResponse
	var weatherAPIResponse WeatherAPIResponse

	vars := mux.Vars(r)
	cep := vars["cep"]

	matched, err := regexp.MatchString(`^\d{8}$`, cep)
	if err != nil || !matched {
		http.Error(w, "invalid zipcode - format eg.: 00111222", http.StatusUnprocessableEntity)
		return
	}

	errc := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := http.Get("http://viacep.com.br/ws/" + cep + "/json/")
		if err != nil {
			errc <- err
			return
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			errc <- err
			return
		}

		if len(body) == 0 {
			errc <- fmt.Errorf("empty response from API")
			return
		}

		err = json.Unmarshal(body, &viaCEPResponse)
		if err != nil || viaCEPResponse.Erro {
			errc <- err
			return
		}

	}()

	wg.Wait()

	select {
	case err := <-errc:
		http.Error(w, "can not find zipcode", http.StatusNotFound)
		log.Println(err)
		return
	default:
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		cityEscaped := url.QueryEscape(viaCEPResponse.Localidade)

		url := fmt.Sprintf("http://api.weatherapi.com/v1/current.json?q=%s&key=3841b81037a5427eb51191826241702", cityEscaped)

		resp, err := http.Get(url)
		if err != nil {
			errc <- err
			return
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			errc <- err
			return
		}

		if len(body) == 0 {
			errc <- fmt.Errorf("empty response from API")
			return
		}

		err = json.Unmarshal(body, &weatherAPIResponse)
		if err != nil {
			errc <- err
			return
		}

	}()

	wg.Wait()

	select {
	case err := <-errc:
		log.Println(err)
		http.Error(w, "can not find weather", http.StatusInternalServerError)
		return
	default:
	}

	tempC, err := strconv.ParseFloat(fmt.Sprintf("%.1f", weatherAPIResponse.Current.TempC), 64)

	tempF, err := strconv.ParseFloat(fmt.Sprintf("%.1f", tempC*1.8+32), 64)

	tempK, err := strconv.ParseFloat(fmt.Sprintf("%.1f", tempC+273.15), 64)

	if err != nil {
		http.Error(w, "can not parse temperature", http.StatusInternalServerError)
		return
	}

	// Crie a resposta
	weather := Weather{TempC: tempC, TempF: tempF, TempK: tempK}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(weather)
}
