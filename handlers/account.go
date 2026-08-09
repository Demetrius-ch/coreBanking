package handlers

import (
	"encoding/json"
	"net/http"

	"gestionbancaire/models"
	"gestionbancaire/storage"
)

func CreateAccount(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	var req models.Account
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON invalide", http.StatusBadRequest)
		return
	}
	storage.Store.Lock()
	defer storage.Store.Unlock()

	// Règle Métier 1 : Vérification du client et de son statut KYC
	var client *models.Customer
	// for _, c := range storage.Store.Customers {
	// 	if c.ID == req.CustomerID {
	// 		client = &c
	// 		break
	// 	}
	// }
	for i := range storage.Store.Customers {
		if storage.Store.Customers[i].ID == req.CustomerID {
			client = &storage.Store.Customers[i]
			break
		}
	}

	if client == nil {
		http.Error(w, "Client inexistant", http.StatusBadRequest)
		return
	}

	if !client.KycValide {
		http.Error(w, "Création refusée : KYC du client non validé", http.StatusForbidden)
		return
	}

	storage.DernierAccountID++
	req.ID = storage.DernierAccountID
	storage.Store.Accounts = append(storage.Store.Accounts, req)
	go storage.SauvegarderFichier()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}
