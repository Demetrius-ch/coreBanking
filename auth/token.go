package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// DureeToken est la durée de validité d'un jeton d'accès
const DureeToken = 8 * time.Hour

// En-tête JWT figé : tout autre algorithme (dont "none") est refusé
var enteteJWT = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

var secret []byte

var ErrTokenInvalide = errors.New("jeton invalide ou expiré")

// ConfigurerSecret définit la clé HMAC de signature des jetons (32 octets minimum)
func ConfigurerSecret(cle []byte) error {
	if len(cle) < 32 {
		return errors.New("le secret JWT doit faire au moins 32 octets")
	}
	secret = cle
	return nil
}

type claims struct {
	Sub int   `json:"sub"` // ID utilisateur
	Ver int   `json:"ver"` // Version de jeton de l'utilisateur à l'émission
	Iat int64 `json:"iat"`
	Exp int64 `json:"exp"`
}

// GenererToken émet un JWT HS256 pour l'utilisateur et sa version de jeton courante
func GenererToken(userID, version int) (string, error) {
	if secret == nil {
		return "", errors.New("secret JWT non configuré")
	}
	maintenant := time.Now()
	payload, err := json.Marshal(claims{Sub: userID, Ver: version, Iat: maintenant.Unix(), Exp: maintenant.Add(DureeToken).Unix()})
	if err != nil {
		return "", err
	}
	signe := enteteJWT + "." + base64.RawURLEncoding.EncodeToString(payload)
	return signe + "." + signer(signe), nil
}

// VerifierToken contrôle la signature et l'expiration, puis renvoie
// l'ID utilisateur et la version de jeton qu'il porte
func VerifierToken(token string) (int, int, error) {
	if secret == nil {
		return 0, 0, ErrTokenInvalide
	}
	parties := strings.Split(token, ".")
	if len(parties) != 3 || parties[0] != enteteJWT {
		return 0, 0, ErrTokenInvalide
	}
	attendue := signer(parties[0] + "." + parties[1])
	if !hmac.Equal([]byte(parties[2]), []byte(attendue)) {
		return 0, 0, ErrTokenInvalide
	}

	payload, err := base64.RawURLEncoding.DecodeString(parties[1])
	if err != nil {
		return 0, 0, ErrTokenInvalide
	}
	var c claims
	if err := json.Unmarshal(payload, &c); err != nil || c.Sub <= 0 {
		return 0, 0, ErrTokenInvalide
	}
	if time.Now().Unix() >= c.Exp {
		return 0, 0, ErrTokenInvalide
	}
	return c.Sub, c.Ver, nil
}

func signer(donnees string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(donnees))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
