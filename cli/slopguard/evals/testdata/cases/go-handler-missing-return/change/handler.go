package api

import (
	"encoding/json"
	"net/http"
)

type order struct {
	Item     string `json:"item"`
	Quantity int    `json:"quantity"`
}

func CreateOrder(w http.ResponseWriter, r *http.Request) {
	var request order
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if request.Quantity < 1 {
		http.Error(w, "quantity must be positive", http.StatusBadRequest)
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(request)
}
