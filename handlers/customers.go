package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"gestionbancaire/models"
	"gestionbancaire/storage"
)

// creerClient ajoute un client au store (doit être appelée sous verrou).
// Règle Métier : un nouveau client n'est jamais validé d'office.
// Risque maximal tant que le KYC n'a pas été évalué.
func creerClient(nom string) models.Customer {
	storage.DernierCustomerID++
	client := models.Customer{
		ID:          storage.DernierCustomerID,
		Nom:         nom,
		KycValide:   false,
		ScoreRisque: 100,
	}
	storage.Store.Customers = append(storage.Store.Customers, client)
	storage.EnregistrerAudit("CUSTOMER_CREATED", fmt.Sprintf("Client %d créé (KYC en attente)", client.ID))
	return client
}

// CreateCustomer crée une fiche client sans accès en ligne (réservé admin)
func CreateCustomer(w http.ResponseWriter, r *http.Request) {
	var req CreateCustomerRequest
	if err := decoderJSON(w, r, &req); err != nil {
		http.Error(w, "JSON invalide : "+err.Error(), http.StatusBadRequest)
		return
	}

	nom := strings.TrimSpace(req.Nom)
	if nom == "" {
		http.Error(w, "Le nom est obligatoire", http.StatusBadRequest)
		return
	}

	storage.Store.Lock()
	defer storage.Store.Unlock()

	client := creerClient(nom)
	storage.SauvegarderAsync()

	repondreJSON(w, http.StatusCreated, client)
}

// UpdateKyc enregistre le résultat de l'évaluation KYC d'un client (réservé admin)
func UpdateKyc(w http.ResponseWriter, r *http.Request) {
	id, ok := idDepuisChemin(r, "id")
	if !ok {
		http.Error(w, "ID client invalide", http.StatusBadRequest)
		return
	}

	var req UpdateKycRequest
	if err := decoderJSON(w, r, &req); err != nil {
		http.Error(w, "JSON invalide : "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.KycValide == nil || req.ScoreRisque == nil {
		http.Error(w, "Les champs kyc_valide et score_risque sont obligatoires", http.StatusBadRequest)
		return
	}
	if *req.ScoreRisque < 0 || *req.ScoreRisque > 100 {
		http.Error(w, "Le score de risque doit être compris entre 0 et 100", http.StatusBadRequest)
		return
	}

	storage.Store.Lock()
	defer storage.Store.Unlock()

	client := storage.TrouverClient(id)
	if client == nil {
		http.Error(w, "Client inexistant", http.StatusNotFound)
		return
	}

	client.KycValide = *req.KycValide
	client.ScoreRisque = *req.ScoreRisque

	storage.EnregistrerAudit("KYC_UPDATED", fmt.Sprintf("Client %d : KYC=%t, score=%d", client.ID, client.KycValide, client.ScoreRisque))
	storage.SauvegarderAsync()

	repondreJSON(w, http.StatusOK, client)
}
