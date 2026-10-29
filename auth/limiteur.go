package auth

import (
	"sync"
	"time"
)

// Limiteur bloque une clé (ex : un email) après trop de tentatives de connexion
// sans succès dans une fenêtre de temps, pour freiner les attaques par force brute.
// La tentative est comptée dès qu'elle est autorisée (et non après l'échec) :
// des requêtes envoyées en parallèle ne peuvent pas dépasser la limite.
type Limiteur struct {
	mu            sync.Mutex
	maxTentatives int
	fenetre       time.Duration
	compteurs     map[string]*compteur
	maintenant    func() time.Time // injectable pour les tests
	dernierMenage time.Time
}

type compteur struct {
	tentatives   int
	debut        time.Time // début de la fenêtre courante
	bloqueJusqua time.Time
}

// NouveauLimiteur autorise maxTentatives tentatives sans succès par fenêtre ;
// la suivante bloque la clé pendant une fenêtre complète.
func NouveauLimiteur(maxTentatives int, fenetre time.Duration) *Limiteur {
	return &Limiteur{
		maxTentatives: maxTentatives,
		fenetre:       fenetre,
		compteurs:     make(map[string]*compteur),
		maintenant:    time.Now,
	}
}

// Tenter enregistre une tentative pour la clé et indique si elle est permise ;
// sinon renvoie la durée d'attente restante.
func (l *Limiteur) Tenter(cle string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	maintenant := l.maintenant()
	l.menage(maintenant)

	c, ok := l.compteurs[cle]
	if !ok {
		c = &compteur{debut: maintenant}
		l.compteurs[cle] = c
	}
	if reste := c.bloqueJusqua.Sub(maintenant); reste > 0 {
		return false, reste
	}
	if maintenant.Sub(c.debut) > l.fenetre {
		c.tentatives, c.debut = 0, maintenant
	}

	c.tentatives++
	if c.tentatives > l.maxTentatives {
		c.bloqueJusqua = maintenant.Add(l.fenetre)
		c.tentatives, c.debut = 0, c.bloqueJusqua
		return false, l.fenetre
	}
	return true, 0
}

// Succes remet le compteur de la clé à zéro
func (l *Limiteur) Succes(cle string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.compteurs, cle)
}

// menage supprime les entrées expirées, au plus une fois par fenêtre (sous verrou)
func (l *Limiteur) menage(maintenant time.Time) {
	if maintenant.Sub(l.dernierMenage) < l.fenetre {
		return
	}
	l.dernierMenage = maintenant
	for cle, c := range l.compteurs {
		if maintenant.After(c.bloqueJusqua) && maintenant.Sub(c.debut) > l.fenetre {
			delete(l.compteurs, cle)
		}
	}
}
