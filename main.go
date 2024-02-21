package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Zipcode struct {
	Zipcode interface{} `json:"cep"`
}
type ViaCepResponse struct {
	Localidade string `json:"localidade"`
	Erro       bool   `json:"erro"`
}

func main() {
	http.HandleFunc("/cep", ServiceA)

	fmt.Println("Server run in the port 8080")

	http.ListenAndServe(":8080", nil)
}

func ServiceA(w http.ResponseWriter, r *http.Request) {
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

	// Send to Service B
	jsonData, err := json.Marshal(Zipcode{Zipcode: cepStr})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := http.Post("http://localhost:8081/cep", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	fmt.Fprintf(w, "CEP enviado para o Serviço B: %s", cepStr)
}
