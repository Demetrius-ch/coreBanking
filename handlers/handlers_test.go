package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gestionbancaire/auth"
	"gestionbancaire/models"
	"gestionbancaire/storage"
)

const (
	emailAdmin = "admin@banque.test"
	mdpAdmin   = "mot-de-passe-admin"
)

func TestMain(m *testing.M) {
	auth.IterationsPBKDF2 = 1000 // hachage rapide pour les tests
	if err := auth.ConfigurerSecret([]byte(strings.Repeat("s", 32))); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// nouveauServeur réinitialise le store sur un fichier temporaire
// (client 1 KYC validé, compte 1 en EUR : 5000,00 de solde et 500,00 de découvert)
// et renvoie le routeur avec un jeton administrateur.
func nouveauServeur(t *testing.T) (*http.ServeMux, string) {
	t.Helper()
	storage.ConfigurerFichier(filepath.Join(t.TempDir(), "banking.json"))
	t.Cleanup(storage.AttendreSauvegardes)
	if err := storage.ChargerFichier(); err != nil {
		t.Fatal(err)
	}
	if err := auth.AssurerAdmin(emailAdmin, mdpAdmin); err != nil {
		t.Fatal(err)
	}

	mux := Routes()
	return mux, connecter(t, mux, emailAdmin, mdpAdmin)
}

func appeler(t *testing.T, mux *http.ServeMux, methode, chemin, token, corps string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(methode, chemin, strings.NewReader(corps))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func verifierStatut(t *testing.T, rec *httptest.ResponseRecorder, attendu int) {
	t.Helper()
	if rec.Code != attendu {
		t.Fatalf("statut attendu %d, obtenu %d : %s", attendu, rec.Code, rec.Body.String())
	}
}

func decoder[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func connecter(t *testing.T, mux *http.ServeMux, email, mdp string) string {
	t.Helper()
	rec := appeler(t, mux, "POST", "/auth/login", "", fmt.Sprintf(`{"email":%q,"mot_de_passe":%q}`, email, mdp))
	verifierStatut(t, rec, http.StatusOK)
	return decoder[AuthResponse](t, rec).Token
}

// inscrire crée un client, valide son KYC (score 20) et renvoie son jeton et son ID client
func inscrire(t *testing.T, mux *http.ServeMux, admin, email string) (string, int) {
	t.Helper()
	rec := appeler(t, mux, "POST", "/auth/register", "", fmt.Sprintf(`{"nom":"Client","email":%q,"mot_de_passe":"motdepasse"}`, email))
	verifierStatut(t, rec, http.StatusCreated)
	r := decoder[AuthResponse](t, rec)

	verifierStatut(t, appeler(t, mux, "PATCH", fmt.Sprintf("/customers/%d/kyc", r.Utilisateur.CustomerID), admin, `{"kyc_valide":true,"score_risque":20}`), http.StatusOK)
	return r.Token, r.Utilisateur.CustomerID
}

func ouvrirCompte(t *testing.T, mux *http.ServeMux, token, corps string) models.Account {
	t.Helper()
	rec := appeler(t, mux, "POST", "/accounts", token, corps)
	verifierStatut(t, rec, http.StatusCreated)
	return decoder[models.Account](t, rec)
}

func dernierAudit(t *testing.T) string {
	t.Helper()
	storage.Store.RLock()
	defer storage.Store.RUnlock()
	logs := storage.Store.AuditLogs
	if len(logs) == 0 {
		t.Fatal("aucune entrée d'audit")
	}
	return logs[len(logs)-1].Action
}

func solde(t *testing.T, id int) int64 {
	t.Helper()
	storage.Store.RLock()
	defer storage.Store.RUnlock()
	return storage.TrouverCompte(id).SoldeCentimes
}

// --- Authentification ---

func TestRoutesProtegees_SansJeton(t *testing.T) {
	mux, _ := nouveauServeur(t)
	routes := [][2]string{
		{"GET", "/me"}, {"GET", "/customers"}, {"GET", "/customers/1"}, {"GET", "/customers/1/accounts"},
		{"POST", "/customers"}, {"PATCH", "/customers/1/kyc"}, {"POST", "/accounts"}, {"GET", "/accounts/1"},
		{"GET", "/accounts/1/transfers"}, {"GET", "/accounts/1/loans"}, {"POST", "/transfers"},
		{"POST", "/loans"}, {"GET", "/audit"}, {"POST", "/auth/logout"}, {"PATCH", "/accounts/1/decouvert"},
	}
	for _, r := range routes {
		verifierStatut(t, appeler(t, mux, r[0], r[1], "", `{}`), http.StatusUnauthorized)
		verifierStatut(t, appeler(t, mux, r[0], r[1], "jeton.bidon.invalide", `{}`), http.StatusUnauthorized)
	}
}

func TestRoutesAdmin_InterditesAuxClients(t *testing.T) {
	mux, admin := nouveauServeur(t)
	client, customerID := inscrire(t, mux, admin, "alice@test.fr")

	verifierStatut(t, appeler(t, mux, "GET", "/customers", client, ""), http.StatusForbidden)
	verifierStatut(t, appeler(t, mux, "GET", "/audit", client, ""), http.StatusForbidden)
	verifierStatut(t, appeler(t, mux, "POST", "/customers", client, `{"nom":"X"}`), http.StatusForbidden)
	verifierStatut(t, appeler(t, mux, "POST", "/loans", client, `{"account_id":1,"montant_emprunte_centimes":100,"taux_pb":0}`), http.StatusForbidden)
	// Un client ne peut pas valider son propre KYC
	verifierStatut(t, appeler(t, mux, "PATCH", fmt.Sprintf("/customers/%d/kyc", customerID), client, `{"kyc_valide":true,"score_risque":0}`), http.StatusForbidden)

	verifierStatut(t, appeler(t, mux, "GET", "/customers", admin, ""), http.StatusOK)
	verifierStatut(t, appeler(t, mux, "GET", "/audit", admin, ""), http.StatusOK)
}

func TestRegister_Validation(t *testing.T) {
	mux, _ := nouveauServeur(t)
	verifierStatut(t, appeler(t, mux, "POST", "/auth/register", "", `{"nom":"A","email":"pas-un-email","mot_de_passe":"motdepasse"}`), http.StatusBadRequest)
	verifierStatut(t, appeler(t, mux, "POST", "/auth/register", "", `{"nom":"A","email":"a@test.fr","mot_de_passe":"court"}`), http.StatusBadRequest)
	verifierStatut(t, appeler(t, mux, "POST", "/auth/register", "", `{"nom":" ","email":"a@test.fr","mot_de_passe":"motdepasse"}`), http.StatusBadRequest)
	verifierStatut(t, appeler(t, mux, "POST", "/auth/register", "", `{"nom":"A","email":"a@test.fr","mot_de_passe":"motdepasse","role":"admin"}`), http.StatusBadRequest)

	rec := appeler(t, mux, "POST", "/auth/register", "", `{"nom":"A","email":"A@Test.fr","mot_de_passe":"motdepasse"}`)
	verifierStatut(t, rec, http.StatusCreated)
	if strings.Contains(rec.Body.String(), "pbkdf2") || strings.Contains(rec.Body.String(), "hash") {
		t.Fatalf("le hash du mot de passe ne doit jamais être renvoyé : %s", rec.Body.String())
	}
	r := decoder[AuthResponse](t, rec)
	if r.Utilisateur.Role != models.RoleClient || r.Utilisateur.Email != "a@test.fr" || r.Token == "" {
		t.Fatalf("inscription inattendue : %+v", r)
	}

	// Email déjà pris (insensible à la casse)
	verifierStatut(t, appeler(t, mux, "POST", "/auth/register", "", `{"nom":"B","email":"a@TEST.fr","mot_de_passe":"motdepasse"}`), http.StatusConflict)
}

func TestLogin(t *testing.T) {
	mux, _ := nouveauServeur(t)
	verifierStatut(t, appeler(t, mux, "POST", "/auth/register", "", `{"nom":"A","email":"a@test.fr","mot_de_passe":"motdepasse"}`), http.StatusCreated)

	verifierStatut(t, appeler(t, mux, "POST", "/auth/login", "", `{"email":"a@test.fr","mot_de_passe":"mauvais"}`), http.StatusUnauthorized)
	if dernierAudit(t) != "LOGIN_FAILED" {
		t.Fatalf("audit attendu LOGIN_FAILED, obtenu %s", dernierAudit(t))
	}
	verifierStatut(t, appeler(t, mux, "POST", "/auth/login", "", `{"email":"inconnu@test.fr","mot_de_passe":"motdepasse"}`), http.StatusUnauthorized)

	token := connecter(t, mux, "A@test.fr", "motdepasse")
	verifierStatut(t, appeler(t, mux, "GET", "/me", token, ""), http.StatusOK)
}

func TestLogin_BlocageForceBrute(t *testing.T) {
	mux, _ := nouveauServeur(t)
	verifierStatut(t, appeler(t, mux, "POST", "/auth/register", "", `{"nom":"A","email":"a@test.fr","mot_de_passe":"motdepasse"}`), http.StatusCreated)

	for i := 0; i < maxEchecsConnexion; i++ {
		verifierStatut(t, appeler(t, mux, "POST", "/auth/login", "", `{"email":"a@test.fr","mot_de_passe":"mauvais"}`), http.StatusUnauthorized)
	}
	// Bloqué, même avec le bon mot de passe
	rec := appeler(t, mux, "POST", "/auth/login", "", `{"email":"a@test.fr","mot_de_passe":"motdepasse"}`)
	verifierStatut(t, rec, http.StatusTooManyRequests)
	if rec.Header().Get("Retry-After") != "900" {
		t.Fatalf("Retry-After attendu 900, obtenu %q", rec.Header().Get("Retry-After"))
	}
	// Le blocage est propre à l'email (insensible à la casse)
	verifierStatut(t, appeler(t, mux, "POST", "/auth/login", "", `{"email":"A@TEST.FR","mot_de_passe":"motdepasse"}`), http.StatusTooManyRequests)
	verifierStatut(t, appeler(t, mux, "POST", "/auth/login", "", fmt.Sprintf(`{"email":%q,"mot_de_passe":%q}`, emailAdmin, mdpAdmin)), http.StatusOK)
}

func TestLogout_RevoqueTousLesJetons(t *testing.T) {
	mux, admin := nouveauServeur(t)
	session1, _ := inscrire(t, mux, admin, "alice@test.fr")
	session2 := connecter(t, mux, "alice@test.fr", "motdepasse")

	verifierStatut(t, appeler(t, mux, "POST", "/auth/logout", session1, ""), http.StatusNoContent)
	verifierStatut(t, appeler(t, mux, "GET", "/me", session1, ""), http.StatusUnauthorized)
	verifierStatut(t, appeler(t, mux, "GET", "/me", session2, ""), http.StatusUnauthorized)
	if dernierAudit(t) != "USER_LOGOUT" {
		t.Fatalf("audit attendu USER_LOGOUT, obtenu %s", dernierAudit(t))
	}

	// Une nouvelle connexion fonctionne, et les autres utilisateurs ne sont pas touchés
	verifierStatut(t, appeler(t, mux, "GET", "/me", connecter(t, mux, "alice@test.fr", "motdepasse"), ""), http.StatusOK)
	verifierStatut(t, appeler(t, mux, "GET", "/me", admin, ""), http.StatusOK)
}

// --- Clients et KYC ---

func TestCreateCustomer_ChampsSensiblesRefuses(t *testing.T) {
	mux, admin := nouveauServeur(t)
	verifierStatut(t, appeler(t, mux, "POST", "/customers", admin, `{"nom":"Alice","kyc_valide":true,"score_risque":0}`), http.StatusBadRequest)
	verifierStatut(t, appeler(t, mux, "POST", "/customers", admin, `{"nom":"  "}`), http.StatusBadRequest)

	rec := appeler(t, mux, "POST", "/customers", admin, `{"nom":"Alice"}`)
	verifierStatut(t, rec, http.StatusCreated)
	if c := decoder[models.Customer](t, rec); c.KycValide || c.ScoreRisque != 100 {
		t.Fatalf("nouveau client doit être non validé et à risque maximal : %+v", c)
	}
}

func TestKyc_ConditionneOuvertureCompte(t *testing.T) {
	mux, admin := nouveauServeur(t)
	rec := appeler(t, mux, "POST", "/auth/register", "", `{"nom":"A","email":"a@test.fr","mot_de_passe":"motdepasse"}`)
	verifierStatut(t, rec, http.StatusCreated)
	r := decoder[AuthResponse](t, rec)

	verifierStatut(t, appeler(t, mux, "POST", "/accounts", r.Token, `{"devise":"EUR"}`), http.StatusForbidden)
	verifierStatut(t, appeler(t, mux, "PATCH", fmt.Sprintf("/customers/%d/kyc", r.Utilisateur.CustomerID), admin, `{"kyc_valide":true,"score_risque":20}`), http.StatusOK)

	compte := ouvrirCompte(t, mux, r.Token, `{"devise":"eur"}`)
	if compte.CustomerID != r.Utilisateur.CustomerID || compte.SoldeCentimes != 0 || compte.Devise != "EUR" {
		t.Fatalf("compte inattendu : %+v", compte)
	}
}

func TestUpdateKyc_Validation(t *testing.T) {
	mux, admin := nouveauServeur(t)
	verifierStatut(t, appeler(t, mux, "PATCH", "/customers/1/kyc", admin, `{"kyc_valide":true}`), http.StatusBadRequest)
	verifierStatut(t, appeler(t, mux, "PATCH", "/customers/1/kyc", admin, `{"kyc_valide":true,"score_risque":101}`), http.StatusBadRequest)
	verifierStatut(t, appeler(t, mux, "PATCH", "/customers/99/kyc", admin, `{"kyc_valide":true,"score_risque":10}`), http.StatusNotFound)
	verifierStatut(t, appeler(t, mux, "PATCH", "/customers/abc/kyc", admin, `{"kyc_valide":true,"score_risque":10}`), http.StatusBadRequest)
}

// --- Comptes ---

func TestCreateAccount_Regles(t *testing.T) {
	mux, admin := nouveauServeur(t)
	client, _ := inscrire(t, mux, admin, "alice@test.fr")

	verifierStatut(t, appeler(t, mux, "POST", "/accounts", client, `{"devise":"EUR","solde_centimes":100000000}`), http.StatusBadRequest)
	verifierStatut(t, appeler(t, mux, "POST", "/accounts", client, `{"devise":"BTC"}`), http.StatusBadRequest)
	// Un client ne peut pas ouvrir de compte au nom d'un autre
	verifierStatut(t, appeler(t, mux, "POST", "/accounts", client, `{"customer_id":1,"devise":"EUR"}`), http.StatusForbidden)
	// L'admin peut ouvrir un compte pour n'importe quel client
	if c := ouvrirCompte(t, mux, admin, `{"customer_id":1,"devise":"USD"}`); c.CustomerID != 1 {
		t.Fatalf("compte inattendu : %+v", c)
	}
}

// --- Isolation des données entre clients ---

func TestConsultation_Isolation(t *testing.T) {
	mux, admin := nouveauServeur(t)
	alice, aliceID := inscrire(t, mux, admin, "alice@test.fr")
	bob, _ := inscrire(t, mux, admin, "bob@test.fr")
	compte := ouvrirCompte(t, mux, alice, `{"devise":"EUR"}`)

	chemins := []string{
		fmt.Sprintf("/customers/%d", aliceID),
		fmt.Sprintf("/customers/%d/accounts", aliceID),
		fmt.Sprintf("/accounts/%d", compte.ID),
		fmt.Sprintf("/accounts/%d/transfers", compte.ID),
		fmt.Sprintf("/accounts/%d/loans", compte.ID),
	}
	for _, c := range chemins {
		verifierStatut(t, appeler(t, mux, "GET", c, alice, ""), http.StatusOK)
		verifierStatut(t, appeler(t, mux, "GET", c, admin, ""), http.StatusOK)
		verifierStatut(t, appeler(t, mux, "GET", c, bob, ""), http.StatusNotFound)
	}
	verifierStatut(t, appeler(t, mux, "GET", "/accounts/999", admin, ""), http.StatusNotFound)
	verifierStatut(t, appeler(t, mux, "GET", "/accounts/abc", admin, ""), http.StatusBadRequest)

	// Bob ne peut pas débiter le compte d'Alice
	verifierStatut(t, appeler(t, mux, "POST", "/transfers", bob, fmt.Sprintf(`{"account_source_id":%d,"account_destination_id":1,"montant_centimes":1}`, compte.ID)), http.StatusNotFound)
}

func TestParcoursComplet(t *testing.T) {
	mux, admin := nouveauServeur(t)
	alice, _ := inscrire(t, mux, admin, "alice@test.fr")
	compte := ouvrirCompte(t, mux, alice, `{"devise":"EUR"}`)

	// Solde nul et pas de découvert : virement impossible
	virement := fmt.Sprintf(`{"account_source_id":%d,"account_destination_id":1,"montant_centimes":2550}`, compte.ID)
	verifierStatut(t, appeler(t, mux, "POST", "/transfers", alice, virement), http.StatusBadRequest)

	// Prêt accordé par l'admin, puis virement de 25,50 EUR
	verifierStatut(t, appeler(t, mux, "POST", "/loans", admin, fmt.Sprintf(`{"account_id":%d,"montant_emprunte_centimes":100000,"taux_pb":350}`, compte.ID)), http.StatusCreated)
	verifierStatut(t, appeler(t, mux, "POST", "/transfers", alice, virement), http.StatusCreated)

	if solde(t, compte.ID) != 97450 || solde(t, 1) != 502550 {
		t.Fatalf("soldes inattendus : %d / %d", solde(t, compte.ID), solde(t, 1))
	}

	me := decoder[MeResponse](t, appeler(t, mux, "GET", "/me", alice, ""))
	if me.Client == nil || len(me.Comptes) != 1 || me.Comptes[0].SoldeCentimes != 97450 {
		t.Fatalf("/me inattendu : %+v", me)
	}
	virements := decoder[[]models.Transfer](t, appeler(t, mux, "GET", fmt.Sprintf("/accounts/%d/transfers", compte.ID), alice, ""))
	if len(virements) != 1 || virements[0].MontantCentimes != 2550 {
		t.Fatalf("virements inattendus : %+v", virements)
	}
	prets := decoder[[]models.Loan](t, appeler(t, mux, "GET", fmt.Sprintf("/accounts/%d/loans", compte.ID), alice, ""))
	if len(prets) != 1 || prets[0].TauxPB != 350 {
		t.Fatalf("prêts inattendus : %+v", prets)
	}
}

// --- Découvert ---

func TestUpdateDecouvert(t *testing.T) {
	mux, admin := nouveauServeur(t)
	alice, aliceID := inscrire(t, mux, admin, "alice@test.fr")
	compte := ouvrirCompte(t, mux, alice, `{"devise":"EUR"}`)
	chemin := fmt.Sprintf("/accounts/%d/decouvert", compte.ID)

	verifierStatut(t, appeler(t, mux, "PATCH", chemin, alice, `{"decouvert_centimes":100000}`), http.StatusForbidden)
	verifierStatut(t, appeler(t, mux, "PATCH", chemin, admin, `{}`), http.StatusBadRequest)
	verifierStatut(t, appeler(t, mux, "PATCH", chemin, admin, `{"decouvert_centimes":-1}`), http.StatusBadRequest)
	verifierStatut(t, appeler(t, mux, "PATCH", "/accounts/999/decouvert", admin, `{"decouvert_centimes":1}`), http.StatusNotFound)

	// Découvert de 200,00 : Alice peut descendre jusqu'à -200,00, pas un centime de plus
	rec := appeler(t, mux, "PATCH", chemin, admin, `{"decouvert_centimes":20000}`)
	verifierStatut(t, rec, http.StatusOK)
	if c := decoder[models.Account](t, rec); c.DecouvertCentimes != 20000 {
		t.Fatalf("découvert inattendu : %+v", c)
	}
	virement := func(centimes int) string {
		return fmt.Sprintf(`{"account_source_id":%d,"account_destination_id":1,"montant_centimes":%d}`, compte.ID, centimes)
	}
	verifierStatut(t, appeler(t, mux, "POST", "/transfers", alice, virement(20001)), http.StatusBadRequest)
	verifierStatut(t, appeler(t, mux, "POST", "/transfers", alice, virement(20000)), http.StatusCreated)

	// Client devenu trop risqué : nouveau découvert refusé, suppression toujours possible
	verifierStatut(t, appeler(t, mux, "PATCH", fmt.Sprintf("/customers/%d/kyc", aliceID), admin, `{"kyc_valide":true,"score_risque":95}`), http.StatusOK)
	verifierStatut(t, appeler(t, mux, "PATCH", chemin, admin, `{"decouvert_centimes":50000}`), http.StatusForbidden)
	if dernierAudit(t) != "OVERDRAFT_REFUSED" {
		t.Fatalf("audit attendu OVERDRAFT_REFUSED, obtenu %s", dernierAudit(t))
	}
	verifierStatut(t, appeler(t, mux, "PATCH", chemin, admin, `{"decouvert_centimes":0}`), http.StatusOK)
}

// --- Prêts ---

func TestCreateLoan(t *testing.T) {
	mux, admin := nouveauServeur(t)

	verifierStatut(t, appeler(t, mux, "POST", "/loans", admin, `{"account_id":1,"montant_emprunte_centimes":-1000,"taux_pb":300}`), http.StatusBadRequest)
	verifierStatut(t, appeler(t, mux, "POST", "/loans", admin, fmt.Sprintf(`{"account_id":1,"montant_emprunte_centimes":%d,"taux_pb":300}`, MontantMaxCentimes+1)), http.StatusBadRequest)
	verifierStatut(t, appeler(t, mux, "POST", "/loans", admin, `{"account_id":1,"montant_emprunte_centimes":1000,"taux_pb":-1}`), http.StatusBadRequest)
	verifierStatut(t, appeler(t, mux, "POST", "/loans", admin, `{"account_id":1,"montant_emprunte_centimes":10.5,"taux_pb":300}`), http.StatusBadRequest)
	verifierStatut(t, appeler(t, mux, "POST", "/loans", admin, `{"account_id":99,"montant_emprunte_centimes":1000,"taux_pb":300}`), http.StatusNotFound)

	verifierStatut(t, appeler(t, mux, "POST", "/loans", admin, `{"account_id":1,"montant_emprunte_centimes":100000,"taux_pb":300}`), http.StatusCreated)
	if solde(t, 1) != 600000 {
		t.Fatalf("solde attendu 600000, obtenu %d", solde(t, 1))
	}
	if dernierAudit(t) != "LOAN_GRANTED" {
		t.Fatalf("audit attendu LOAN_GRANTED, obtenu %s", dernierAudit(t))
	}

	// Client devenu trop risqué : prêt refusé
	verifierStatut(t, appeler(t, mux, "PATCH", "/customers/1/kyc", admin, `{"kyc_valide":true,"score_risque":90}`), http.StatusOK)
	verifierStatut(t, appeler(t, mux, "POST", "/loans", admin, `{"account_id":1,"montant_emprunte_centimes":100000,"taux_pb":300}`), http.StatusForbidden)
	if dernierAudit(t) != "LOAN_REFUSED" {
		t.Fatalf("audit attendu LOAN_REFUSED, obtenu %s", dernierAudit(t))
	}
}

// --- Virements ---

func TestCreateTransfer_Regles(t *testing.T) {
	mux, admin := nouveauServeur(t)
	ouvrirCompte(t, mux, admin, `{"customer_id":1,"devise":"EUR"}`) // compte 2
	ouvrirCompte(t, mux, admin, `{"customer_id":1,"devise":"USD"}`) // compte 3

	cas := []struct {
		nom    string
		corps  string
		statut int
	}{
		{"montant nul", `{"account_source_id":1,"account_destination_id":2,"montant_centimes":0}`, http.StatusBadRequest},
		{"montant décimal", `{"account_source_id":1,"account_destination_id":2,"montant_centimes":1.5}`, http.StatusBadRequest},
		{"même compte", `{"account_source_id":1,"account_destination_id":1,"montant_centimes":10}`, http.StatusBadRequest},
		{"compte inconnu", `{"account_source_id":1,"account_destination_id":99,"montant_centimes":10}`, http.StatusNotFound},
		{"devises différentes", `{"account_source_id":1,"account_destination_id":3,"montant_centimes":10}`, http.StatusBadRequest},
		{"découvert dépassé d'un centime", `{"account_source_id":1,"account_destination_id":2,"montant_centimes":550001}`, http.StatusBadRequest},
		{"jusqu'au découvert", `{"account_source_id":1,"account_destination_id":2,"montant_centimes":550000}`, http.StatusCreated},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			verifierStatut(t, appeler(t, mux, "POST", "/transfers", admin, c.corps), c.statut)
		})
	}
	if solde(t, 1) != -50000 || solde(t, 2) != 550000 {
		t.Fatalf("soldes inattendus : %d / %d", solde(t, 1), solde(t, 2))
	}
}

func TestCreateTransfer_RisqueCritiqueAudite(t *testing.T) {
	mux, admin := nouveauServeur(t)
	ouvrirCompte(t, mux, admin, `{"customer_id":1,"devise":"EUR"}`)
	verifierStatut(t, appeler(t, mux, "PATCH", "/customers/1/kyc", admin, `{"kyc_valide":true,"score_risque":81}`), http.StatusOK)

	verifierStatut(t, appeler(t, mux, "POST", "/transfers", admin, `{"account_source_id":1,"account_destination_id":2,"montant_centimes":10}`), http.StatusForbidden)
	if dernierAudit(t) != "TRANSFER_BLOCKED" {
		t.Fatalf("audit attendu TRANSFER_BLOCKED, obtenu %s", dernierAudit(t))
	}
}

// Des virements concurrents dans les deux sens ne doivent ni créer ni détruire d'argent.
// À lancer avec -race.
func TestCreateTransfer_Concurrence(t *testing.T) {
	mux, admin := nouveauServeur(t)
	ouvrirCompte(t, mux, admin, `{"customer_id":1,"devise":"EUR"}`)

	const n = 200
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			corps := `{"account_source_id":1,"account_destination_id":2,"montant_centimes":1001}`
			if i%2 == 1 {
				corps = `{"account_source_id":2,"account_destination_id":1,"montant_centimes":507}`
			}
			req := httptest.NewRequest("POST", "/transfers", strings.NewReader(corps))
			req.Header.Set("Authorization", "Bearer "+admin)
			mux.ServeHTTP(httptest.NewRecorder(), req)
		}(i)
	}
	wg.Wait()

	if total := solde(t, 1) + solde(t, 2); total != 500000 {
		t.Fatalf("la masse monétaire a changé : %d au lieu de 500000", total)
	}
}

func TestFormaterMontant(t *testing.T) {
	cas := map[int64]string{0: "0.00", 5: "0.05", 123456: "1234.56", -50000: "-500.00", -7: "-0.07"}
	for centimes, attendu := range cas {
		if obtenu := formaterMontant(centimes); obtenu != attendu {
			t.Errorf("formaterMontant(%d) = %s, attendu %s", centimes, obtenu, attendu)
		}
	}
}
