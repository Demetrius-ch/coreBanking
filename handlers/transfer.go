package handlers

import (
	"fmt"
	"net/http"
	"time"

	"gestionbancaire/auth"
	"gestionbancaire/models"
	"gestionbancaire/storage"
)

func CreateTransfer(w http.ResponseWriter, r *http.Request) {
	var req CreateTransferRequest
	if err := decoderJSON(w, r, &req); err != nil {
		http.Error(w, "JSON invalide : "+err.Error(), http.StatusBadRequest)
		return
	}

	if !montantValide(req.MontantCentimes) {
		http.Error(w, "Le montant doit être strictement positif et inférieur au plafond", http.StatusBadRequest)
		return
	}
	if req.AccountSourceID == req.AccountDestinationID {
		http.Error(w, "Les comptes source et destination doivent être différents", http.StatusBadRequest)
		return
	}

	storage.Store.Lock()
	defer storage.Store.Unlock()

	// Recherche des comptes source et destination
	source := storage.TrouverCompte(req.AccountSourceID)
	destination := storage.TrouverCompte(req.AccountDestinationID)
	if source == nil || destination == nil {
		http.Error(w, "Compte source ou destination introuvable", http.StatusNotFound)
		return
	}

	// Un client ne peut débiter que ses propres comptes (404 pour ne pas révéler leur existence)
	if !auth.PeutAcceder(auth.Utilisateur(r), source.CustomerID) {
		http.Error(w, "Compte source ou destination introuvable", http.StatusNotFound)
		return
	}

	if source.Devise != destination.Devise {
		http.Error(w, "Virement refusé : devises différentes", http.StatusBadRequest)
		return
	}

	// Règle Métier 2 : Vérification du score de risque du client source
	client := storage.TrouverClient(source.CustomerID)
	if client != nil && client.ScoreRisque > SeuilRisqueCritique {
		storage.EnregistrerAudit("TRANSFER_BLOCKED", fmt.Sprintf("Virement %d -> %d bloqué : score de risque critique (%d)", source.ID, destination.ID, client.ScoreRisque))
		storage.SauvegarderAsync()
		http.Error(w, "Transfert refusé : Risque client trop élevé", http.StatusForbidden)
		return
	}

	// Règle Métier 3 : Vérification du découvert
	if source.SoldeCentimes-req.MontantCentimes < -source.DecouvertCentimes {
		storage.EnregistrerAudit("TRANSFER_REFUSED", fmt.Sprintf("Virement %d -> %d de %s refusé : découvert dépassé", source.ID, destination.ID, formaterMontant(req.MontantCentimes)))
		storage.SauvegarderAsync()
		http.Error(w, "Solde insuffisant (découvert dépassé)", http.StatusBadRequest)
		return
	}

	// Exécution atomique (sous verrou exclusif)
	source.SoldeCentimes -= req.MontantCentimes
	destination.SoldeCentimes += req.MontantCentimes

	storage.DernierTransferID++
	virement := models.Transfer{
		ID:                   storage.DernierTransferID,
		AccountSourceID:      source.ID,
		AccountDestinationID: destination.ID,
		MontantCentimes:      req.MontantCentimes,
		Date:                 time.Now(),
	}
	storage.Store.Transfers = append(storage.Store.Transfers, virement)

	storage.EnregistrerAudit("TRANSFER_SUCCESS", fmt.Sprintf("Virement %d : %s %s du compte %d vers %d", virement.ID, formaterMontant(virement.MontantCentimes), source.Devise, source.ID, destination.ID))
	storage.SauvegarderAsync()

	repondreJSON(w, http.StatusCreated, virement)
}
