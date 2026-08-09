package models

import "time"

// Gestion des clients
type Customer struct {
	ID          int    `json:"id"`
	Nom         string `json:"nom"`
	KycValide   bool   `json:"kyc_valide"`   // true = approuvé, false = en attente/bloqué
	ScoreRisque int    `json:"score_risque"` // Ex: 0 à 100
}

// Gestion de Compte
type Account struct {
	ID         int     `json:"id"`
	CustomerID int     `json:"customer_id"` // Clé étrangère vers Customer
	Solde      float64 `json:"solde"`
	Decouvert  float64 `json:"decouvert"` // Montant de découvert autorisé
	Devise     string  `json:"devise"`    // Ex: "EUR", "USD"
}

//Module Transaction & Virements

type Transfer struct {
	ID                   int       `json:"id"`
	AccountSourceID      int       `json:"account_source_id"`
	AccountDestinationID int       `json:"account_destination_id"`
	Montant              float64   `json:"montant"`
	Date                 time.Time `json:"date"`
}

// Module Prêt & Crédits
type Loan struct {
	ID              int     `json:"id"`
	AccountID       int     `json:"account_id"` // Clé étrangère vers Account
	MontantEmprunte float64 `json:"montant_emprunte"`
	MontantRestant  float64 `json:"montant_restant"`
	Taux            float64 `json:"taux"`
}

// Audit & Conformité
type AuditLog struct {
	ID         int       `json:"id"`
	Action     string    `json:"action"` // Ex: "PROT_GRANTED", "TRANSFER_BLOCKED"
	Details    string    `json:"details"`
	Timestamp  string    `json:"timestamp"`
	Horodatage time.Time `json:"horodatage"`
}
