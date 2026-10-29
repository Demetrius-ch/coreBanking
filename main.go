// main.go
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gestionbancaire/auth"
	"gestionbancaire/handlers"
	"gestionbancaire/middleware"
	"gestionbancaire/storage"
)

func main() {
	// 1. Initialisation du stock (refus de démarrer si le fichier est corrompu)
	if chemin := os.Getenv("DATA_FILE"); chemin != "" {
		storage.ConfigurerFichier(chemin)
	}
	if err := storage.ChargerFichier(); err != nil {
		log.Fatal("Impossible de charger les données : ", err)
	}
	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	// 2. Secret de signature des jetons
	secret := []byte(os.Getenv("JWT_SECRET"))
	if len(secret) == 0 {
		log.Println("ATTENTION : JWT_SECRET absent, secret aléatoire généré (les jetons expirent au redémarrage)")
		secret = make([]byte, 32)
		rand.Read(secret)
	}
	if err := auth.ConfigurerSecret(secret); err != nil {
		log.Fatal(err)
	}

	// 3. Administrateur initial
	if email := os.Getenv("ADMIN_EMAIL"); email != "" {
		if err := auth.AssurerAdmin(email, os.Getenv("ADMIN_PASSWORD")); err != nil {
			log.Fatal("Création de l'administrateur : ", err)
		}
	} else {
		log.Println("ATTENTION : ADMIN_EMAIL absent, aucun administrateur créé au démarrage")
	}

	// 4. Routes + Middleware (CORS + Logger)
	handlerChaine := middleware.EnableCORS(middleware.Logger(handlers.Routes()))

	// 5. Serveur avec timeouts (protège contre les clients lents type Slowloris)
	serveur := &http.Server{
		Addr:              ":" + port,
		Handler:           handlerChaine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 6. Démarrage, puis arrêt propre sur SIGINT / SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	erreurServeur := make(chan error, 1)
	go func() {
		fmt.Printf(" Serveur Core Banking opérationnel sur le port %s\n", port)
		erreurServeur <- serveur.ListenAndServe()
	}()

	select {
	case err := <-erreurServeur:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("Erreur au démarrage du serveur : ", err)
		}
	case <-ctx.Done():
		log.Println("Signal d'arrêt reçu, fin des requêtes en cours...")
	}

	ctxArret, annuler := context.WithTimeout(context.Background(), 10*time.Second)
	defer annuler()
	if err := serveur.Shutdown(ctxArret); err != nil {
		log.Println("Arrêt forcé du serveur : ", err)
	}

	// Plus aucune requête : on attend les sauvegardes en vol puis on écrit l'état final
	storage.AttendreSauvegardes()
	if err := storage.SauvegarderFichier(); err != nil {
		log.Println("ERREUR sauvegarde finale : ", err)
		os.Exit(1)
	}
	log.Println("Données sauvegardées, arrêt terminé")
}
