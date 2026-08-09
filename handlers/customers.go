package handlers

import (
	"encoding/json"
	"net/http"

	"gestionbancaire/models"
	"gestionbancaire/storage"
)

func CreateCustomer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	var req models.Customer
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON invalide", http.StatusBadRequest)
		return
	}
	storage.Store.Lock()
	defer storage.Store.Unlock()

	storage.DernierCustomerID++
	req.ID = storage.DernierCustomerID
	storage.Store.Customers = append(storage.Store.Customers, req)
	go storage.SauvegarderFichier()

	// if err := storage.SauvegarderFichier(); err != nil {
	// 	http.Error(w, "Erreur de sauvegarde", http.StatusInternalServerError)
	// 	return
	// }

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}
