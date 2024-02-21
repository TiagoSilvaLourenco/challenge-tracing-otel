package main

import (
	"fmt"
	"net/http"
)

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

	cep := r.FormValue("cep")
	fmt.Fprintf(w, "CEP received in Service B: %s", cep)
}
