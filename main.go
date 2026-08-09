// main.go
package main

import (
	"fmt"
	"net/http"
	"os"

	"gestionbancaire/handlers"
	"gestionbancaire/middleware"
	"gestionbancaire/storage"
)

func main() {
	// 1. Initialisation du stock
	storage.ChargerFichier()
	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	// 2. Initialisation du routeur HTTP natif
	mux := http.NewServeMux()

	// 3. Déclaration des routes
	mux.HandleFunc("POST /customers", handlers.CreateCustomer)
	mux.HandleFunc("POST /accounts", handlers.CreateAccount)
	mux.HandleFunc("POST /loans", handlers.CreateLoan)
	mux.HandleFunc("POST /transfers", handlers.CreateTransfer)

	// Consultation
	mux.HandleFunc("GET /audit", handlers.GetAuditLogs)

	// 4. Encapsulation par le Middleware (CORS + Logger)
	handlerChaine := middleware.EnableCORS(middleware.Logger(mux))

	// 5. Démarrage
	fmt.Printf(" Serveur Core Banking opérationnel sur le port %s\n", port)
	if err := http.ListenAndServe(":"+port, handlerChaine); err != nil {
		fmt.Println("Erreur au démarrage du serveur :", err)
	}
}
