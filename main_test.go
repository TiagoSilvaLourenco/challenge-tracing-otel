package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWeatherHandler(t *testing.T) {
	r := setupRouter()

	req, err := http.NewRequest("GET", "/12345678", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusUnprocessableEntity)
	}

	expected := "can not find zipcode"
	if rr.Body.String() == expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}
