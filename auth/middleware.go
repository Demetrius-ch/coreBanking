package auth

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"gestionbancaire/models"
	"gestionbancaire/storage"
)

type cleContexte struct{}

// Utilisateur renvoie l'utilisateur authentifié placé dans le contexte par Exiger
func Utilisateur(r *http.Request) models.User {
	u, _ := r.Context().Value(cleContexte{}).(models.User)
	return u
}

// Exiger protège un handler : jeton Bearer valide obligatoire, et rôle parmi
// ceux listés (aucun rôle listé = tout utilisateur connecté).
// L'utilisateur est relu dans le store à chaque requête : un changement de rôle,
// une suppression ou une déconnexion (version de jeton) prend effet immédiatement.
func Exiger(next http.HandlerFunc, roles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			refuser(w)
			return
		}
		userID, version, err := VerifierToken(strings.TrimSpace(token))
		if err != nil {
			refuser(w)
			return
		}

		storage.Store.RLock()
		u := storage.TrouverUtilisateur(userID)
		var copie models.User
		if u != nil {
			copie = *u
		}
		storage.Store.RUnlock()

		if u == nil || copie.VersionJeton != version {
			refuser(w)
			return
		}
		if len(roles) > 0 && !slices.Contains(roles, copie.Role) {
			http.Error(w, "Accès refusé", http.StatusForbidden)
			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), cleContexte{}, copie)))
	}
}

// ExigerAdmin protège un handler réservé aux administrateurs
func ExigerAdmin(next http.HandlerFunc) http.HandlerFunc {
	return Exiger(next, models.RoleAdmin)
}

// PeutAcceder indique si l'utilisateur a le droit d'agir sur les données du client donné
func PeutAcceder(u models.User, customerID int) bool {
	return u.Role == models.RoleAdmin || (u.Role == models.RoleClient && u.CustomerID == customerID)
}

func refuser(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="core-banking"`)
	http.Error(w, "Authentification requise", http.StatusUnauthorized)
}
