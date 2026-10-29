package auth

import (
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"
)

func init() {
	IterationsPBKDF2 = 1000
	if err := ConfigurerSecret([]byte(strings.Repeat("k", 32))); err != nil {
		panic(err)
	}
}

func TestMotDePasse(t *testing.T) {
	hash, err := HacherMotDePasse("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := VerifierMotDePasse("correct horse", hash); err != nil || !ok {
		t.Fatalf("le bon mot de passe doit être accepté (err=%v)", err)
	}
	if ok, _ := VerifierMotDePasse("mauvais", hash); ok {
		t.Fatal("un mauvais mot de passe ne doit pas être accepté")
	}
	if autre, _ := HacherMotDePasse("correct horse"); autre == hash {
		t.Fatal("deux hachages du même mot de passe doivent différer (sel aléatoire)")
	}
	if _, err := VerifierMotDePasse("x", "md5$abc"); err == nil {
		t.Fatal("un format de hash inconnu doit être une erreur")
	}
}

func TestToken_AllerRetour(t *testing.T) {
	token, err := GenererToken(42, 3)
	if err != nil {
		t.Fatal(err)
	}
	if id, version, err := VerifierToken(token); err != nil || id != 42 || version != 3 {
		t.Fatalf("attendu 42/3, obtenu %d/%d (err=%v)", id, version, err)
	}
}

func TestToken_Falsifications(t *testing.T) {
	token, _ := GenererToken(42, 0)
	parties := strings.Split(token, ".")

	// Payload modifié (sub=1) avec l'ancienne signature
	payloadAdmin := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":1,"iat":0,"exp":9999999999}`))
	// Algorithme "none"
	enteteNone := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	// Jeton expiré correctement signé
	expire := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":42,"iat":0,"exp":` + strconv.FormatInt(time.Now().Add(-time.Minute).Unix(), 10) + `}`))

	cas := map[string]string{
		"vide":            "",
		"deux parties":    parties[0] + "." + parties[1],
		"payload modifié": parties[0] + "." + payloadAdmin + "." + parties[2],
		"alg none":        enteteNone + "." + parties[1] + ".",
		"signature vide":  parties[0] + "." + parties[1] + ".",
		"expiré":          enteteJWT + "." + expire + "." + signer(enteteJWT+"."+expire),
		"signature autre": parties[0] + "." + parties[1] + "." + signer("autre"),
	}
	for nom, jeton := range cas {
		if _, _, err := VerifierToken(jeton); err == nil {
			t.Errorf("%s : le jeton aurait dû être refusé", nom)
		}
	}
}

func TestConfigurerSecret_TropCourt(t *testing.T) {
	if err := ConfigurerSecret([]byte("court")); err == nil {
		t.Fatal("un secret trop court doit être refusé")
	}
}
