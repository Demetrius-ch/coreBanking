package handlers

import (
	"fmt"
	"net/http"

	"gestionbancaire/models"
	"gestionbancaire/storage"
)

// CreateLoan accorde un prêt sur un compte (réservé admin : décision du conseiller)
func CreateLoan(w http.ResponseWriter, r *http.Request) {
	var req CreateLoanRequest
	if err := decoderJSON(w, r, &req); err != nil {
		http.Error(w, "JSON invalide : "+err.Error(), http.StatusBadRequest)
		return
	}

	if !montantValide(req.MontantEmprunteCentimes) {
		http.Error(w, "Le montant emprunté doit être strictement positif et inférieur au plafond", http.StatusBadRequest)
		return
	}
	if req.TauxPB < 0 || req.TauxPB > TauxMaxPB {
		http.Error(w, "Le taux doit être compris entre 0 et 10000 points de base", http.StatusBadRequest)
		return
	}

	storage.Store.Lock()
	defer storage.Store.Unlock()

	compte := storage.TrouverCompte(req.AccountID)
	if compte == nil {
		http.Error(w, "Compte associé introuvable", http.StatusNotFound)
		return
	}

	// Règle Métier : prêt réservé aux clients KYC validés et au risque acceptable
	client := storage.TrouverClient(compte.CustomerID)
	if client == nil || !client.KycValide {
		storage.EnregistrerAudit("LOAN_REFUSED", fmt.Sprintf("Compte %d : KYC non validé", compte.ID))
		storage.SauvegarderAsync()
		http.Error(w, "Prêt refusé : KYC du client non validé", http.StatusForbidden)
		return
	}
	if client.ScoreRisque > SeuilRisqueCritique {
		storage.EnregistrerAudit("LOAN_REFUSED", fmt.Sprintf("Compte %d : score de risque critique (%d)", compte.ID, client.ScoreRisque))
		storage.SauvegarderAsync()
		http.Error(w, "Prêt refusé : Risque client trop élevé", http.StatusForbidden)
		return
	}

	// Octroi du prêt : Crédit direct sur le compte
	compte.SoldeCentimes += req.MontantEmprunteCentimes

	storage.DernierLoanID++
	pret := models.Loan{
		ID:                      storage.DernierLoanID,
		AccountID:               compte.ID,
		MontantEmprunteCentimes: req.MontantEmprunteCentimes,
		MontantRestantCentimes:  req.MontantEmprunteCentimes,
		TauxPB:                  req.TauxPB,
	}
	storage.Store.Loans = append(storage.Store.Loans, pret)

	storage.EnregistrerAudit("LOAN_GRANTED", fmt.Sprintf("Prêt %d de %s %s accordé sur le compte %d", pret.ID, formaterMontant(pret.MontantEmprunteCentimes), compte.Devise, compte.ID))
	storage.SauvegarderAsync()

	repondreJSON(w, http.StatusCreated, pret)
}
