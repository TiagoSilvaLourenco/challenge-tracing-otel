package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

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

	resp, err := http.Get("https://viacep.com.br/ws/" + req.Cep + "/json/")
	if err != nil {
		http.Error(w, "Error making request to ViaCEP API", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Decode the response body into the ViaCepResponse struct.
	var viaCepResponse ViaCepResponse
	if err := json.NewDecoder(resp.Body).Decode(&viaCepResponse); err != nil {
		http.Error(w, "Error decoding ViaCEP API response", http.StatusInternalServerError)
		return
	}

	// Check if there was an error in the ViaCEP API response.
	if viaCepResponse.Erro {
		http.Error(w, "Error in ViaCEP API response, don't found CEP", http.StatusInternalServerError)
		return
	}

	log.Printf("Localidade for CEP %s: %s\n", req.Cep, viaCepResponse.Localidade)
}
