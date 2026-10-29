package handlers

import (
	"fmt"
	"math"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"sync"

	"gestionbancaire/auth"
	"gestionbancaire/models"
	"gestionbancaire/storage"
)

// Utilisateur tel que renvoyé par l'API (sans le hash du mot de passe)
type UserResponse struct {
	ID         int    `json:"id"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	CustomerID int    `json:"customer_id,omitempty"`
}

type AuthResponse struct {
	Token       string       `json:"token"`
	Utilisateur UserResponse `json:"utilisateur"`
}

func versUserResponse(u models.User) UserResponse {
	return UserResponse{ID: u.ID, Email: u.Email, Role: u.Role, CustomerID: u.CustomerID}
}

// Hash factice calculé une fois : un login sur un email inconnu coûte autant
// qu'un login sur un email existant (pas d'énumération par le temps de réponse).
var hashFactice = sync.OnceValue(func() string {
	h, _ := auth.HacherMotDePasse("mot-de-passe-factice")
	return h
})

// Register crée un client (KYC en attente) et son compte utilisateur
func Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := decoderJSON(w, r, &req); err != nil {
		http.Error(w, "JSON invalide : "+err.Error(), http.StatusBadRequest)
		return
	}

	nom := strings.TrimSpace(req.Nom)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if nom == "" {
		http.Error(w, "Le nom est obligatoire", http.StatusBadRequest)
		return
	}
	if adresse, err := mail.ParseAddress(email); err != nil || adresse.Address != email {
		http.Error(w, "Email invalide", http.StatusBadRequest)
		return
	}
	if len(req.MotDePasse) < 8 || len(req.MotDePasse) > 128 {
		http.Error(w, "Le mot de passe doit contenir entre 8 et 128 caractères", http.StatusBadRequest)
		return
	}

	// Hachage hors verrou : PBKDF2 est volontairement lent
	hash, err := auth.HacherMotDePasse(req.MotDePasse)
	if err != nil {
		http.Error(w, "Erreur interne", http.StatusInternalServerError)
		return
	}

	storage.Store.Lock()
	if storage.TrouverUtilisateurParEmail(email) != nil {
		storage.Store.Unlock()
		http.Error(w, "Email déjà utilisé", http.StatusConflict)
		return
	}

	client := creerClient(nom)

	storage.DernierUserID++
	utilisateur := models.User{
		ID:             storage.DernierUserID,
		Email:          email,
		MotDePasseHash: hash,
		Role:           models.RoleClient,
		CustomerID:     client.ID,
	}
	storage.Store.Users = append(storage.Store.Users, utilisateur)

	storage.EnregistrerAudit("USER_REGISTERED", fmt.Sprintf("Utilisateur %d inscrit pour le client %d", utilisateur.ID, client.ID))
	storage.SauvegarderAsync()
	storage.Store.Unlock()

	token, err := auth.GenererToken(utilisateur.ID, utilisateur.VersionJeton)
	if err != nil {
		http.Error(w, "Erreur interne", http.StatusInternalServerError)
		return
	}
	repondreJSON(w, http.StatusCreated, AuthResponse{Token: token, Utilisateur: versUserResponse(utilisateur)})
}

// Login échange email + mot de passe contre un jeton d'accès.
// Le limiteur bloque temporairement un email après trop d'échecs.
func Login(limiteur *auth.Limiteur) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		login(limiteur, w, r)
	}
}

func login(limiteur *auth.Limiteur, w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := decoderJSON(w, r, &req); err != nil {
		http.Error(w, "JSON invalide : "+err.Error(), http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Vérifié avant le hachage : un email bloqué ne coûte pas de calcul PBKDF2
	if ok, attente := limiteur.Tenter(email); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(attente.Seconds()))))
		http.Error(w, "Trop de tentatives de connexion, réessayez plus tard", http.StatusTooManyRequests)
		return
	}

	storage.Store.RLock()
	var utilisateur models.User
	u := storage.TrouverUtilisateurParEmail(email)
	if u != nil {
		utilisateur = *u
	}
	storage.Store.RUnlock()

	hash := hashFactice()
	if u != nil {
		hash = utilisateur.MotDePasseHash
	}
	ok, err := auth.VerifierMotDePasse(req.MotDePasse, hash)

	if u == nil || err != nil || !ok {
		storage.Store.Lock()
		storage.EnregistrerAudit("LOGIN_FAILED", fmt.Sprintf("Échec de connexion pour %q", email))
		storage.SauvegarderAsync()
		storage.Store.Unlock()
		http.Error(w, "Identifiants invalides", http.StatusUnauthorized)
		return
	}

	limiteur.Succes(email)

	token, err := auth.GenererToken(utilisateur.ID, utilisateur.VersionJeton)
	if err != nil {
		http.Error(w, "Erreur interne", http.StatusInternalServerError)
		return
	}
	repondreJSON(w, http.StatusOK, AuthResponse{Token: token, Utilisateur: versUserResponse(utilisateur)})
}

// Logout révoque tous les jetons de l'utilisateur connecté (toutes ses sessions)
func Logout(w http.ResponseWriter, r *http.Request) {
	storage.Store.Lock()
	defer storage.Store.Unlock()

	u := storage.TrouverUtilisateur(auth.Utilisateur(r).ID)
	if u == nil {
		http.Error(w, "Authentification requise", http.StatusUnauthorized)
		return
	}
	u.VersionJeton++

	storage.EnregistrerAudit("USER_LOGOUT", fmt.Sprintf("Utilisateur %d : jetons révoqués", u.ID))
	storage.SauvegarderAsync()

	w.WriteHeader(http.StatusNoContent)
}
