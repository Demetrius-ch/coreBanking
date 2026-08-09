package handlers

import (
	"encoding/json"
	"net/http"

	"gestionbancaire/models"
	"gestionbancaire/storage"
)

func CreateLoan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	var req models.Loan
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON invalide", http.StatusBadRequest)
		return
	}
	storage.Store.Lock()
	defer storage.Store.Unlock()

	var accIdx = -1
	for i, acc := range storage.Store.Accounts {
		if acc.ID == req.AccountID {
			accIdx = i
			break
		}
	}

	if accIdx == -1 {
		http.Error(w, "Compte associé introuvable", http.StatusNotFound)
		return
	}

	// Octroi du prêt : Crédit direct sur le compte
	storage.Store.Accounts[accIdx].Solde += req.MontantEmprunte

	storage.DernierLoanID++
	req.ID = storage.DernierLoanID
	req.MontantRestant = req.MontantEmprunte
	storage.Store.Loans = append(storage.Store.Loans, req)

	go storage.SauvegarderFichier()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}
