package models

import "time"

// Tous les montants sont exprimés en centimes (int64) pour éviter les erreurs d'arrondi
// des nombres flottants. Ex : 1234 = 12,34 EUR.

// Gestion des clients
type Customer struct {
	ID          int    `json:"id"`
	Nom         string `json:"nom"`
	KycValide   bool   `json:"kyc_valide"`   // true = approuvé, false = en attente/bloqué
	ScoreRisque int    `json:"score_risque"` // Ex: 0 à 100
}

// Gestion de Compte
type Account struct {
	ID                int    `json:"id"`
	CustomerID        int    `json:"customer_id"` // Clé étrangère vers Customer
	SoldeCentimes     int64  `json:"solde_centimes"`
	DecouvertCentimes int64  `json:"decouvert_centimes"` // Montant de découvert autorisé
	Devise            string `json:"devise"`             // Ex: "EUR", "USD"
}

//Module Transaction & Virements

type Transfer struct {
	ID                   int       `json:"id"`
	AccountSourceID      int       `json:"account_source_id"`
	AccountDestinationID int       `json:"account_destination_id"`
	MontantCentimes      int64     `json:"montant_centimes"`
	Date                 time.Time `json:"date"`
}

// Module Prêt & Crédits
type Loan struct {
	ID                      int   `json:"id"`
	AccountID               int   `json:"account_id"` // Clé étrangère vers Account
	MontantEmprunteCentimes int64 `json:"montant_emprunte_centimes"`
	MontantRestantCentimes  int64 `json:"montant_restant_centimes"`
	TauxPB                  int   `json:"taux_pb"` // Taux en points de base : 350 = 3,50 %
}

// Audit & Conformité
type AuditLog struct {
	ID         int       `json:"id"`
	Action     string    `json:"action"` // Ex: "LOAN_GRANTED", "TRANSFER_BLOCKED"
	Details    string    `json:"details"`
	Horodatage time.Time `json:"horodatage"`
}

// Rôles d'accès à l'API
const (
	RoleClient = "client"
	RoleAdmin  = "admin"
)

// Utilisateur de l'API (identifiants de connexion)
type User struct {
	ID             int    `json:"id"`
	Email          string `json:"email"`
	MotDePasseHash string `json:"mot_de_passe_hash"` // Jamais renvoyé par l'API
	Role           string `json:"role"`              // RoleClient ou RoleAdmin
	CustomerID     int    `json:"customer_id"`       // 0 pour un admin
	VersionJeton   int    `json:"version_jeton"`     // Incrémentée à la déconnexion : invalide tous les jetons émis avant
}
