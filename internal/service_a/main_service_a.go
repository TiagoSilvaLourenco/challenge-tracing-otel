package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Zipcode struct {
	Zipcode interface{} `json:"cep"`
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

	resp, err := http.Post("http://service_b:8081/cep", "application/json", bytes.NewBuffer(jsonData))
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

	// Write the formatted response from Service B to the response of Service A
	w.Write(formattedBody)
}
