package handlers

import (
	"net/http"
	"slices"

	"gestionbancaire/auth"
	"gestionbancaire/models"
	"gestionbancaire/storage"
)

// Les handlers de consultation copient les données sous verrou de lecture,
// puis répondent une fois le verrou relâché (un client lent ne bloque pas les écritures).
// Un client qui demande une ressource d'un autre client reçoit 404 (pas de fuite d'existence).

type MeResponse struct {
	Utilisateur UserResponse     `json:"utilisateur"`
	Client      *models.Customer `json:"client,omitempty"`
	Comptes     []models.Account `json:"comptes"`
}

// Me renvoie l'utilisateur connecté, sa fiche client et ses comptes
func Me(w http.ResponseWriter, r *http.Request) {
	utilisateur := auth.Utilisateur(r)
	rep := MeResponse{Utilisateur: versUserResponse(utilisateur), Comptes: []models.Account{}}

	storage.Store.RLock()
	if c := storage.TrouverClient(utilisateur.CustomerID); c != nil {
		copie := *c
		rep.Client = &copie
		rep.Comptes = comptesDuClient(c.ID)
	}
	storage.Store.RUnlock()

	repondreJSON(w, http.StatusOK, rep)
}

// ListCustomers renvoie tous les clients (réservé admin)
func ListCustomers(w http.ResponseWriter, r *http.Request) {
	storage.Store.RLock()
	clients := slices.Clone(storage.Store.Customers)
	storage.Store.RUnlock()

	repondreJSON(w, http.StatusOK, nonNil(clients))
}

func GetCustomer(w http.ResponseWriter, r *http.Request) {
	id, ok := idDepuisChemin(r, "id")
	if !ok {
		http.Error(w, "ID client invalide", http.StatusBadRequest)
		return
	}

	storage.Store.RLock()
	c := storage.TrouverClient(id)
	var client models.Customer
	if c != nil {
		client = *c
	}
	storage.Store.RUnlock()

	if c == nil || !auth.PeutAcceder(auth.Utilisateur(r), client.ID) {
		http.Error(w, "Client introuvable", http.StatusNotFound)
		return
	}
	repondreJSON(w, http.StatusOK, client)
}

func ListCustomerAccounts(w http.ResponseWriter, r *http.Request) {
	id, ok := idDepuisChemin(r, "id")
	if !ok {
		http.Error(w, "ID client invalide", http.StatusBadRequest)
		return
	}

	storage.Store.RLock()
	existe := storage.TrouverClient(id) != nil
	comptes := comptesDuClient(id)
	storage.Store.RUnlock()

	if !existe || !auth.PeutAcceder(auth.Utilisateur(r), id) {
		http.Error(w, "Client introuvable", http.StatusNotFound)
		return
	}
	repondreJSON(w, http.StatusOK, comptes)
}

func GetAccount(w http.ResponseWriter, r *http.Request) {
	compte, ok := compteAutorise(w, r)
	if !ok {
		return
	}
	repondreJSON(w, http.StatusOK, compte)
}

// ListAccountTransfers renvoie les virements émis et reçus par le compte
func ListAccountTransfers(w http.ResponseWriter, r *http.Request) {
	compte, ok := compteAutorise(w, r)
	if !ok {
		return
	}

	virements := []models.Transfer{}
	storage.Store.RLock()
	for _, v := range storage.Store.Transfers {
		if v.AccountSourceID == compte.ID || v.AccountDestinationID == compte.ID {
			virements = append(virements, v)
		}
	}
	storage.Store.RUnlock()

	repondreJSON(w, http.StatusOK, virements)
}

func ListAccountLoans(w http.ResponseWriter, r *http.Request) {
	compte, ok := compteAutorise(w, r)
	if !ok {
		return
	}

	prets := []models.Loan{}
	storage.Store.RLock()
	for _, p := range storage.Store.Loans {
		if p.AccountID == compte.ID {
			prets = append(prets, p)
		}
	}
	storage.Store.RUnlock()

	repondreJSON(w, http.StatusOK, prets)
}

// GetAuditLogs renvoie le journal d'audit complet (réservé admin)
func GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	storage.Store.RLock()
	logs := slices.Clone(storage.Store.AuditLogs)
	storage.Store.RUnlock()

	repondreJSON(w, http.StatusOK, nonNil(logs))
}

// compteAutorise lit le compte {id} et vérifie que l'utilisateur y a accès.
// En cas d'échec, la réponse d'erreur est déjà écrite.
func compteAutorise(w http.ResponseWriter, r *http.Request) (models.Account, bool) {
	id, ok := idDepuisChemin(r, "id")
	if !ok {
		http.Error(w, "ID compte invalide", http.StatusBadRequest)
		return models.Account{}, false
	}

	storage.Store.RLock()
	c := storage.TrouverCompte(id)
	var compte models.Account
	if c != nil {
		compte = *c
	}
	storage.Store.RUnlock()

	if c == nil || !auth.PeutAcceder(auth.Utilisateur(r), compte.CustomerID) {
		http.Error(w, "Compte introuvable", http.StatusNotFound)
		return models.Account{}, false
	}
	return compte, true
}

// comptesDuClient renvoie une copie des comptes d'un client (doit être appelée sous verrou)
func comptesDuClient(customerID int) []models.Account {
	comptes := []models.Account{}
	for _, a := range storage.Store.Accounts {
		if a.CustomerID == customerID {
			comptes = append(comptes, a)
		}
	}
	return comptes
}

// nonNil garantit qu'une liste vide est encodée [] et non null
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
