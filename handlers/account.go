package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"gestionbancaire/auth"
	"gestionbancaire/models"
	"gestionbancaire/storage"
)

// Devises acceptées à l'ouverture d'un compte
var devisesAutorisees = map[string]bool{"EUR": true, "USD": true, "XOF": true, "XAF": true}

func CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req CreateAccountRequest
	if err := decoderJSON(w, r, &req); err != nil {
		http.Error(w, "JSON invalide : "+err.Error(), http.StatusBadRequest)
		return
	}

	// Un client ouvre toujours un compte pour lui-même
	utilisateur := auth.Utilisateur(r)
	if utilisateur.Role == models.RoleClient {
		if req.CustomerID != 0 && req.CustomerID != utilisateur.CustomerID {
			http.Error(w, "Accès refusé", http.StatusForbidden)
			return
		}
		req.CustomerID = utilisateur.CustomerID
	}

	devise := strings.ToUpper(strings.TrimSpace(req.Devise))
	if !devisesAutorisees[devise] {
		http.Error(w, "Devise non supportée", http.StatusBadRequest)
		return
	}

	storage.Store.Lock()
	defer storage.Store.Unlock()

	// Règle Métier 1 : Vérification du client et de son statut KYC
	client := storage.TrouverClient(req.CustomerID)
	if client == nil {
		http.Error(w, "Client inexistant", http.StatusBadRequest)
		return
	}

	if !client.KycValide {
		storage.EnregistrerAudit("ACCOUNT_REFUSED", fmt.Sprintf("Client %d : KYC non validé", client.ID))
		storage.SauvegarderAsync()
		http.Error(w, "Création refusée : KYC du client non validé", http.StatusForbidden)
		return
	}

	// Un compte s'ouvre toujours à solde nul et sans découvert autorisé
	storage.DernierAccountID++
	compte := models.Account{
		ID:                storage.DernierAccountID,
		CustomerID:        client.ID,
		SoldeCentimes:     0,
		DecouvertCentimes: 0,
		Devise:            devise,
	}
	storage.Store.Accounts = append(storage.Store.Accounts, compte)

	storage.EnregistrerAudit("ACCOUNT_CREATED", fmt.Sprintf("Compte %d (%s) ouvert pour le client %d", compte.ID, devise, client.ID))
	storage.SauvegarderAsync()

	repondreJSON(w, http.StatusCreated, compte)
}

// UpdateDecouvert fixe le découvert autorisé d'un compte (réservé admin).
// Une baisse sous la dette actuelle est permise : le compte ne pourra simplement
// plus être débité tant qu'il n'est pas revenu dans la limite.
func UpdateDecouvert(w http.ResponseWriter, r *http.Request) {
	id, ok := idDepuisChemin(r, "id")
	if !ok {
		http.Error(w, "ID compte invalide", http.StatusBadRequest)
		return
	}

	var req UpdateDecouvertRequest
	if err := decoderJSON(w, r, &req); err != nil {
		http.Error(w, "JSON invalide : "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.DecouvertCentimes == nil {
		http.Error(w, "Le champ decouvert_centimes est obligatoire", http.StatusBadRequest)
		return
	}
	decouvert := *req.DecouvertCentimes
	if decouvert < 0 || decouvert > MontantMaxCentimes {
		http.Error(w, "Le découvert doit être positif ou nul et inférieur au plafond", http.StatusBadRequest)
		return
	}

	storage.Store.Lock()
	defer storage.Store.Unlock()

	compte := storage.TrouverCompte(id)
	if compte == nil {
		http.Error(w, "Compte introuvable", http.StatusNotFound)
		return
	}

	// Règle Métier : pas de découvert pour un client non validé ou trop risqué
	if decouvert > 0 {
		client := storage.TrouverClient(compte.CustomerID)
		if client == nil || !client.KycValide || client.ScoreRisque > SeuilRisqueCritique {
			storage.EnregistrerAudit("OVERDRAFT_REFUSED", fmt.Sprintf("Compte %d : client non validé ou trop risqué", compte.ID))
			storage.SauvegarderAsync()
			http.Error(w, "Découvert refusé : KYC non validé ou risque client trop élevé", http.StatusForbidden)
			return
		}
	}

	ancien := compte.DecouvertCentimes
	compte.DecouvertCentimes = decouvert

	storage.EnregistrerAudit("OVERDRAFT_UPDATED", fmt.Sprintf("Compte %d : découvert %s -> %s %s", compte.ID, formaterMontant(ancien), formaterMontant(decouvert), compte.Devise))
	storage.SauvegarderAsync()

	repondreJSON(w, http.StatusOK, compte)
}
