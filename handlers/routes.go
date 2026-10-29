package handlers

import (
	"net/http"
	"time"

	"gestionbancaire/auth"
)

// Limite de connexion : 5 échecs par email sur 15 min, puis blocage de 15 min
const (
	maxEchecsConnexion     = 5
	fenetreEchecsConnexion = 15 * time.Minute
)

// Routes déclare toutes les routes de l'API et leur niveau d'accès
func Routes() *http.ServeMux {
	mux := http.NewServeMux()

	// Public
	mux.HandleFunc("POST /auth/register", Register)
	mux.HandleFunc("POST /auth/login", Login(auth.NouveauLimiteur(maxEchecsConnexion, fenetreEchecsConnexion)))

	// Utilisateur connecté (un client n'accède qu'à ses propres données)
	mux.HandleFunc("POST /auth/logout", auth.Exiger(Logout))
	mux.HandleFunc("GET /me", auth.Exiger(Me))
	mux.HandleFunc("GET /customers/{id}", auth.Exiger(GetCustomer))
	mux.HandleFunc("GET /customers/{id}/accounts", auth.Exiger(ListCustomerAccounts))
	mux.HandleFunc("POST /accounts", auth.Exiger(CreateAccount))
	mux.HandleFunc("GET /accounts/{id}", auth.Exiger(GetAccount))
	mux.HandleFunc("GET /accounts/{id}/transfers", auth.Exiger(ListAccountTransfers))
	mux.HandleFunc("GET /accounts/{id}/loans", auth.Exiger(ListAccountLoans))
	mux.HandleFunc("POST /transfers", auth.Exiger(CreateTransfer))

	// Administrateur
	mux.HandleFunc("GET /customers", auth.ExigerAdmin(ListCustomers))
	mux.HandleFunc("POST /customers", auth.ExigerAdmin(CreateCustomer))
	mux.HandleFunc("PATCH /customers/{id}/kyc", auth.ExigerAdmin(UpdateKyc))
	mux.HandleFunc("PATCH /accounts/{id}/decouvert", auth.ExigerAdmin(UpdateDecouvert))
	mux.HandleFunc("POST /loans", auth.ExigerAdmin(CreateLoan))
	mux.HandleFunc("GET /audit", auth.ExigerAdmin(GetAuditLogs))

	return mux
}
