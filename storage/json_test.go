package storage

import (
	"gestionbancaire/models"
	"os"
	"path/filepath"
	"testing"
)

func TestMaxID(t *testing.T) {
	customers := []models.Customer{
		{ID: 1, Nom: "Demetrius"},
		{ID: 5, Nom: "Alice"},
		{ID: 3, Nom: "Bob"},
	}

	max := maxID(len(customers), func(i int) int { return customers[i].ID })

	if max != 5 {
		t.Errorf("Attendu ID max = 5, mais obtenu %d", max)
	}
}

func TestChargerFichier_CorrompuNonEcrase(t *testing.T) {
	chemin := filepath.Join(t.TempDir(), "banking.json")
	ConfigurerFichier(chemin)
	os.WriteFile(chemin, []byte("{pas du json"), 0644)

	if err := ChargerFichier(); err == nil {
		t.Fatal("une erreur était attendue pour un fichier corrompu")
	}
	contenu, _ := os.ReadFile(chemin)
	if string(contenu) != "{pas du json" {
		t.Fatal("le fichier corrompu a été écrasé")
	}
}

func TestSauvegarde_AllerRetour(t *testing.T) {
	chemin := filepath.Join(t.TempDir(), "banking.json")
	ConfigurerFichier(chemin)
	if err := ChargerFichier(); err != nil {
		t.Fatal(err)
	}

	Store.Lock()
	EnregistrerAudit("TEST", "aller-retour")
	Store.Unlock()
	for i := 0; i < 20; i++ {
		SauvegarderAsync()
	}
	AttendreSauvegardes()

	if err := ChargerFichier(); err != nil {
		t.Fatal(err)
	}
	if len(Store.AuditLogs) != 1 || Store.AuditLogs[0].Action != "TEST" || DernierAuditID != 1 {
		t.Fatalf("état rechargé inattendu : %+v", Store.AuditLogs)
	}
	if fichiers, _ := filepath.Glob(chemin + ".*.tmp"); len(fichiers) != 0 {
		t.Fatalf("fichiers temporaires restants : %v", fichiers)
	}
}
