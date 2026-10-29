package auth

import (
	"errors"
	"fmt"
	"strings"

	"gestionbancaire/models"
	"gestionbancaire/storage"
)

// AssurerAdmin crée le compte administrateur initial s'il n'existe pas encore.
// Un email déjà utilisé par un compte non-admin est une erreur.
func AssurerAdmin(email, motDePasse string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(motDePasse) < 12 {
		return errors.New("email admin vide ou mot de passe admin trop court (12 caractères minimum)")
	}

	// Hachage hors verrou : PBKDF2 est volontairement lent
	hash, err := HacherMotDePasse(motDePasse)
	if err != nil {
		return err
	}

	storage.Store.Lock()
	defer storage.Store.Unlock()

	if u := storage.TrouverUtilisateurParEmail(email); u != nil {
		if u.Role != models.RoleAdmin {
			return fmt.Errorf("l'email %s appartient déjà à un compte client", email)
		}
		return nil
	}

	storage.DernierUserID++
	storage.Store.Users = append(storage.Store.Users, models.User{
		ID:             storage.DernierUserID,
		Email:          email,
		MotDePasseHash: hash,
		Role:           models.RoleAdmin,
	})
	storage.EnregistrerAudit("ADMIN_CREATED", fmt.Sprintf("Administrateur %d créé (%s)", storage.DernierUserID, email))
	storage.SauvegarderAsync()
	return nil
}
