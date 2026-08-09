# Core Banking Engine (Go / Golang)

Moteur backend de Core Banking monolithique, haute performance et **thread-safe** développé en Go.
Le système gère l'ouverture de comptes sous contraintes métier (KYC), le traitement synchrone des virements/prêts et la traçabilité des opérations.

## Points Forts Techniques

- **Thread-Safety & Concurrence** : Protection contre les _Race Conditions_ via encapsulation fine de `sync.RWMutex`. Validé avec `go test -race`.
- **Persistance Non-Bloquante** : Écriture asynchrone sur disque (JSON) via Goroutines pour garantir des temps de réponse HTTP minimes.
- **Routage HTTP Moderne** : Utilisation des fonctionnalités natives Go 1.22+ (`net/http` ServeMux).
- **Observabilité & Conformité** : Journalisation d'audit immutable (`AuditLog`) et middleware de logging pour mesurer la latence par endpoint.

## 🛠 Stack Technique

- **Langage** : Go
- **Protocole** : REST API (`net/http`)
- **Tests** : Suite de tests unitaires natives (`go test -race ./...`)

## 📌 Utilisation Rapide

```bash
# Lancement du serveur
go run main.go

# Exécution des tests de concurrence
go test -race ./...
```
