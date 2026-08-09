package storage

import (
	"gestionbancaire/models"
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
