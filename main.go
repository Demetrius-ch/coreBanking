// main.go
package main

import (
	"fmt"
	"net/http"

	"gestionbancaire/handlers"
	"gestionbancaire/middleware"
	"gestionbancaire/storage"
)

func main() {
	// 1. Initialisation du stock
	storage.ChargerFichier()

	// 2. Initialisation du routeur HTTP natif
	mux := http.NewServeMux()

	// 3. Déclaration des routes
	mux.HandleFunc("POST /customers", handlers.CreateCustomer)
	mux.HandleFunc("POST /accounts", handlers.CreateAccount)
	mux.HandleFunc("POST /loans", handlers.CreateLoan)
	mux.HandleFunc("POST /transfers", handlers.CreateTransfer)

	// Consultation
	mux.HandleFunc("GET /audit", handlers.GetAuditLogs)

	// 4. Encapsulation par le Middleware
	handlerChaine := middleware.Logger(mux)

	// 5. Démarrage
	fmt.Println("🚀 Serveur Core Banking opérationnel sur http://localhost:8080")
	if err := http.ListenAndServe(":8080", handlerChaine); err != nil {
		fmt.Println("Erreur au démarrage du serveur :", err)
	}
}
