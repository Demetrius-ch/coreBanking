package auth

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func limiteurDeTest() (*Limiteur, *time.Time) {
	l := NouveauLimiteur(3, time.Minute)
	horloge := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	l.maintenant = func() time.Time { return horloge }
	return l, &horloge
}

func TestLimiteur_BlocageEtDeblocage(t *testing.T) {
	l, horloge := limiteurDeTest()

	for i := 1; i <= 3; i++ {
		if ok, _ := l.Tenter("a@test.fr"); !ok {
			t.Fatalf("tentative %d refusée trop tôt", i)
		}
	}
	ok, attente := l.Tenter("a@test.fr")
	if ok || attente != time.Minute {
		t.Fatalf("4e tentative : attendu blocage d'une minute, obtenu ok=%v attente=%v", ok, attente)
	}
	// Les autres clés ne sont pas affectées
	if ok, _ := l.Tenter("b@test.fr"); !ok {
		t.Fatal("une autre clé ne doit pas être bloquée")
	}

	*horloge = horloge.Add(30 * time.Second)
	if ok, attente := l.Tenter("a@test.fr"); ok || attente != 30*time.Second {
		t.Fatalf("toujours bloqué attendu (reste 30s), obtenu ok=%v attente=%v", ok, attente)
	}

	*horloge = horloge.Add(31 * time.Second)
	if ok, _ := l.Tenter("a@test.fr"); !ok {
		t.Fatal("la clé doit être débloquée après la fenêtre")
	}
}

func TestLimiteur_SuccesReinitialise(t *testing.T) {
	l, _ := limiteurDeTest()
	l.Tenter("a@test.fr")
	l.Tenter("a@test.fr")
	l.Succes("a@test.fr")
	for i := 1; i <= 3; i++ {
		if ok, _ := l.Tenter("a@test.fr"); !ok {
			t.Fatalf("tentative %d refusée après un succès", i)
		}
	}
}

func TestLimiteur_FenetreExpiree(t *testing.T) {
	l, horloge := limiteurDeTest()
	l.Tenter("a@test.fr")
	l.Tenter("a@test.fr")
	l.Tenter("a@test.fr")
	*horloge = horloge.Add(2 * time.Minute)
	if ok, _ := l.Tenter("a@test.fr"); !ok {
		t.Fatal("les anciennes tentatives ne doivent plus compter")
	}
}

// Des tentatives parallèles ne peuvent pas dépasser la limite
func TestLimiteur_Concurrence(t *testing.T) {
	l, _ := limiteurDeTest()
	var autorisees atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := l.Tenter("a@test.fr"); ok {
				autorisees.Add(1)
			}
		}()
	}
	wg.Wait()
	if n := autorisees.Load(); n != 3 {
		t.Fatalf("3 tentatives autorisées attendues, obtenu %d", n)
	}
}
