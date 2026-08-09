package handlers

import (
	"encoding/json"
	"net/http"

	"time"

	"gestionbancaire/models"
	"gestionbancaire/storage"
)

func CreateTransfer(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	var req models.Transfer
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON invalide", http.StatusBadRequest)
		return
	}

	if req.Montant <= 0 {
		http.Error(w, "Le montant doit être supérieur à zéro", http.StatusBadRequest)
		return
	}
	storage.Store.Lock()
	defer storage.Store.Unlock()

	// Recherche des comptes source et destination
	var sourceIdx, destIdx = -1, -1
	for i, acc := range storage.Store.Accounts {
		if acc.ID == req.AccountSourceID {
			sourceIdx = i
		}
		if acc.ID == req.AccountDestinationID {
			destIdx = i
		}
	}

	if sourceIdx == -1 || destIdx == -1 {
		http.Error(w, "Compte source ou destination introuvable", http.StatusNotFound)
		return
	}

	// Règle Métier 2 : Vérification du score de risque du client source
	sourceAcc := &storage.Store.Accounts[sourceIdx]
	for _, c := range storage.Store.Customers {
		if c.ID == sourceAcc.CustomerID && c.ScoreRisque > 80 {
			// Enregistrement d'audit pour tentative bloquée
			enregistrerAudit("TRANSFER_BLOCKED", "Virement bloqué : Score de risque critique")
			http.Error(w, "Transfert refusé : Risque client trop élevé", http.StatusForbidden)
			return
		}
	}

	// Règle Métier 3 : Vérification du découvert
	if sourceAcc.Solde-req.Montant < -sourceAcc.Decouvert {
		http.Error(w, "Solde insuffisant (découvert dépassé)", http.StatusBadRequest)
		return
	}

	// Exécution atomique
	sourceAcc.Solde -= req.Montant
	storage.Store.Accounts[destIdx].Solde += req.Montant

	storage.DernierTransferID++
	req.ID = storage.DernierTransferID
	req.Date = time.Now()
	storage.Store.Transfers = append(storage.Store.Transfers, req)

	enregistrerAudit("TRANSFER_SUCCESS", "Virement réussi")
	go storage.SauvegarderFichier()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}

func enregistrerAudit(action, details string) {
	storage.DernierAuditID++
	audit := models.AuditLog{
		ID:         storage.DernierAuditID,
		Action:     action,
		Details:    details,
		Horodatage: time.Now(),
	}
	storage.Store.AuditLogs = append(storage.Store.AuditLogs, audit)
}
