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

	log.Printf("Received request with CEP: %s\n", req.Cep)

	localidade, err := getLocalidadeFromViaCep(req.Cep)
	if err != nil {
		http.Error(w, "Error getting localidade from ViaCEP API", http.StatusInternalServerError)
		return
	}

	log.Printf("Localidade for CEP %s: %s\n", req.Cep, localidade)
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
