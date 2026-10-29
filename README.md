# Core Banking Engine (Go / Golang)

Moteur backend de Core Banking monolithique, haute performance et **thread-safe** développé en Go.
Le système gère l'ouverture de comptes sous contraintes métier (KYC), le traitement synchrone des virements/prêts et la traçabilité des opérations.

## Points Forts Techniques

- **Thread-Safety & Concurrence** : Protection contre les _Race Conditions_ via encapsulation fine de `sync.RWMutex`. Validé avec `go test -race`.
- **Persistance Non-Bloquante** : Écriture asynchrone sur disque (JSON) via Goroutines pour garantir des temps de réponse HTTP minimes.
- **Routage HTTP Moderne** : Utilisation des fonctionnalités natives Go 1.22+ (`net/http` ServeMux).
- **Observabilité & Conformité** : Journalisation d'audit immutable (`AuditLog`) et middleware de logging pour mesurer la latence par endpoint.
- **Montants exacts** : tous les montants sont des entiers en centimes (`int64`), les taux en points de base (350 = 3,50 %).
- **Authentification & Autorisation** : JWT HS256 et mots de passe PBKDF2-SHA256 (bibliothèque standard uniquement). Rôles `client` / `admin`, un client n'accède qu'à ses propres données. Déconnexion révoquant toutes les sessions, blocage de 15 min après 5 tentatives de connexion ratées par email.
- **Persistance sûre** : écriture atomique (fichier temporaire + renommage), refus de démarrer sur un fichier corrompu.
- **Exploitation** : timeouts HTTP (anti-Slowloris) et arrêt propre sur `SIGINT`/`SIGTERM` avec sauvegarde finale.

## 🛠 Stack Technique

- **Langage** : Go
- **Protocole** : REST API (`net/http`)
- **Tests** : Suite de tests unitaires natives (`go test -race ./...`)

## 📌 Utilisation Rapide

```bash
# Lancement du serveur
JWT_SECRET="$(openssl rand -base64 48)" \
ADMIN_EMAIL=admin@banque.fr ADMIN_PASSWORD='mot-de-passe-solide' \
go run main.go

# Exécution des tests de concurrence
go test -race ./...
```

## ⚙️ Configuration

| Variable | Rôle | Défaut |
|---|---|---|
| `PORT` | Port d'écoute | `8080` |
| `DATA_FILE` | Fichier de persistance | `banking.json` |
| `JWT_SECRET` | Clé de signature des jetons (≥ 32 octets) | aléatoire à chaque démarrage (jetons invalidés au redémarrage) |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | Administrateur créé au démarrage s'il n'existe pas (mot de passe ≥ 12 caractères) | aucun |

## 🔌 API

Authentification : en-tête `Authorization: Bearer <token>` (jeton obtenu via `/auth/register` ou `/auth/login`, valide 8 h).
Un client qui demande une ressource d'un autre client reçoit `404`.

| Méthode | Route | Accès | Description |
|---|---|---|---|
| POST | `/auth/register` | public | `{nom, email, mot_de_passe}` → crée client (KYC en attente) + utilisateur, renvoie un jeton |
| POST | `/auth/login` | public | `{email, mot_de_passe}` → jeton (`429` + `Retry-After` après 5 échecs) |
| POST | `/auth/logout` | connecté | Révoque tous les jetons de l'utilisateur |
| GET | `/me` | connecté | Utilisateur, fiche client et comptes |
| GET | `/customers/{id}` | propriétaire / admin | Fiche client |
| GET | `/customers/{id}/accounts` | propriétaire / admin | Comptes du client |
| POST | `/accounts` | client (pour lui-même) / admin | `{devise, customer_id?}` → compte à solde nul (KYC validé requis) |
| GET | `/accounts/{id}` | propriétaire / admin | Détail du compte |
| GET | `/accounts/{id}/transfers` | propriétaire / admin | Virements émis et reçus |
| GET | `/accounts/{id}/loans` | propriétaire / admin | Prêts du compte |
| POST | `/transfers` | propriétaire du compte source / admin | `{account_source_id, account_destination_id, montant_centimes}` |
| GET | `/customers` | admin | Liste des clients |
| POST | `/customers` | admin | `{nom}` → fiche client sans accès en ligne |
| PATCH | `/customers/{id}/kyc` | admin | `{kyc_valide, score_risque}` |
| PATCH | `/accounts/{id}/decouvert` | admin | `{decouvert_centimes}` (KYC validé et risque ≤ 80 requis si > 0) |
| POST | `/loans` | admin | `{account_id, montant_emprunte_centimes, taux_pb}` |
| GET | `/audit` | admin | Journal d'audit |
