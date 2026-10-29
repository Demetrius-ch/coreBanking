package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// IterationsPBKDF2 suit la recommandation OWASP pour PBKDF2-HMAC-SHA256.
// Modifiable uniquement pour accélérer les tests.
var IterationsPBKDF2 = 600_000

const (
	tailleSel  = 16
	tailleHash = 32
)

// HacherMotDePasse renvoie "pbkdf2-sha256$<itérations>$<sel>$<hash>"
func HacherMotDePasse(motDePasse string) (string, error) {
	sel := make([]byte, tailleSel)
	if _, err := rand.Read(sel); err != nil {
		return "", err
	}
	hash, err := pbkdf2.Key(sha256.New, motDePasse, sel, IterationsPBKDF2, tailleHash)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s",
		IterationsPBKDF2,
		base64.RawStdEncoding.EncodeToString(sel),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifierMotDePasse compare en temps constant un mot de passe à son hash stocké
func VerifierMotDePasse(motDePasse, stocke string) (bool, error) {
	parties := strings.Split(stocke, "$")
	if len(parties) != 4 || parties[0] != "pbkdf2-sha256" {
		return false, errors.New("format de hash inconnu")
	}
	iterations, err := strconv.Atoi(parties[1])
	if err != nil || iterations <= 0 {
		return false, errors.New("nombre d'itérations invalide")
	}
	sel, err := base64.RawStdEncoding.DecodeString(parties[2])
	if err != nil {
		return false, err
	}
	attendu, err := base64.RawStdEncoding.DecodeString(parties[3])
	if err != nil {
		return false, err
	}

	calcule, err := pbkdf2.Key(sha256.New, motDePasse, sel, iterations, len(attendu))
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(calcule, attendu) == 1, nil
}
