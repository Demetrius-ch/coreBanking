package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// Seuil au-delà duquel un client est jugé trop risqué (virements et prêts bloqués)
const SeuilRisqueCritique = 80

// Montant maximal d'une opération : 1 milliard d'unités (en centimes).
// Garde une marge très large avant tout débordement d'int64 sur les soldes.
const MontantMaxCentimes int64 = 1_000_000_000_00

// Taux maximal d'un prêt en points de base (100 % = 10 000 pb)
const TauxMaxPB = 10_000

// Taille maximale d'un corps de requête
const tailleMaxCorps = 1 << 20

// DTO de requête : seuls ces champs sont acceptés du client,
// les champs sensibles (solde, KYC, score...) sont fixés côté serveur.
// Tous les montants sont en centimes.

type RegisterRequest struct {
	Nom        string `json:"nom"`
	Email      string `json:"email"`
	MotDePasse string `json:"mot_de_passe"`
}

type LoginRequest struct {
	Email      string `json:"email"`
	MotDePasse string `json:"mot_de_passe"`
}

type CreateCustomerRequest struct {
	Nom string `json:"nom"`
}

type UpdateKycRequest struct {
	KycValide   *bool `json:"kyc_valide"`
	ScoreRisque *int  `json:"score_risque"`
}

type CreateAccountRequest struct {
	CustomerID int    `json:"customer_id"`
	Devise     string `json:"devise"`
}

type UpdateDecouvertRequest struct {
	DecouvertCentimes *int64 `json:"decouvert_centimes"`
}

type CreateLoanRequest struct {
	AccountID               int   `json:"account_id"`
	MontantEmprunteCentimes int64 `json:"montant_emprunte_centimes"`
	TauxPB                  int   `json:"taux_pb"`
}

type CreateTransferRequest struct {
	AccountSourceID      int   `json:"account_source_id"`
	AccountDestinationID int   `json:"account_destination_id"`
	MontantCentimes      int64 `json:"montant_centimes"`
}

// decoderJSON décode le corps en refusant les champs inconnus et les corps trop gros
func decoderJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, tailleMaxCorps))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("un seul objet JSON attendu")
	}
	return nil
}

func repondreJSON(w http.ResponseWriter, statut int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statut)
	json.NewEncoder(w).Encode(v)
}

// idDepuisChemin lit un paramètre de route entier positif ({id})
func idDepuisChemin(r *http.Request, nom string) (int, bool) {
	id, err := strconv.Atoi(r.PathValue(nom))
	return id, err == nil && id > 0
}

// montantValide vérifie qu'un montant en centimes est strictement positif et borné
func montantValide(centimes int64) bool {
	return centimes > 0 && centimes <= MontantMaxCentimes
}

// formaterMontant affiche des centimes sous forme décimale (ex : 123456 -> "1234.56")
func formaterMontant(centimes int64) string {
	signe := ""
	if centimes < 0 {
		signe = "-"
		centimes = -centimes
	}
	return fmt.Sprintf("%s%d.%02d", signe, centimes/100, centimes%100)
}
