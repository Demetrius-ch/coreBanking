package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"gestionbancaire/models"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// 1. Structure d'encapsulation globale pour la sérialisation JSON
type DataStore struct {
	// Le verrou qui protège le store
	mu        sync.RWMutex
	Version   int               `json:"version"`
	Users     []models.User     `json:"users"`
	Customers []models.Customer `json:"customers"`
	Accounts  []models.Account  `json:"accounts"`
	Transfers []models.Transfer `json:"transfers"`
	Loans     []models.Loan     `json:"loans"`
	AuditLogs []models.AuditLog `json:"audit_logs"`
}

// donneesFichier reflète le contenu du fichier JSON (sans le verrou, copiable)
type donneesFichier struct {
	Version   int               `json:"version"`
	Users     []models.User     `json:"users"`
	Customers []models.Customer `json:"customers"`
	Accounts  []models.Account  `json:"accounts"`
	Transfers []models.Transfer `json:"transfers"`
	Loans     []models.Loan     `json:"loans"`
	AuditLogs []models.AuditLog `json:"audit_logs"`
}

// VersionFormat identifie le format du fichier (2 = montants en centimes + utilisateurs)
const VersionFormat = 2

// 2. Variables globales d'état
var (
	Store DataStore

	// Auto-incréments pour chaque domaine
	DernierUserID     int
	DernierCustomerID int
	DernierAccountID  int
	DernierTransferID int
	DernierLoanID     int
	DernierAuditID    int

	nomFichier = "banking.json"

	// sauvegardeMu sérialise les écritures disque : la capture de l'état et
	// l'écriture se font sous ce verrou, donc le dernier écrit est toujours le plus récent.
	sauvegardeMu sync.Mutex
	// sauvegardesEnCours permet d'attendre la fin des sauvegardes asynchrones
	sauvegardesEnCours sync.WaitGroup
)

// ConfigurerFichier change le chemin du fichier de persistance
func ConfigurerFichier(chemin string) {
	nomFichier = chemin
}

// 3. Sauvegarde de l'état complet dans le fichier JSON
func SauvegarderFichier() error {
	sauvegardeMu.Lock()
	defer sauvegardeMu.Unlock()

	Store.mu.RLock()
	donnees, err := json.MarshalIndent(&Store, "", "  ")
	Store.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("conversion JSON : %w", err)
	}

	return ecrireAtomiquement(nomFichier, donnees)
}

// SauvegarderAsync lance une sauvegarde en arrière-plan et journalise l'éventuelle erreur.
// Peut être appelée alors que le verrou du store est détenu.
func SauvegarderAsync() {
	sauvegardesEnCours.Add(1)
	go func() {
		defer sauvegardesEnCours.Done()
		if err := SauvegarderFichier(); err != nil {
			log.Printf("ERREUR sauvegarde %s : %v", nomFichier, err)
		}
	}()
}

// AttendreSauvegardes bloque jusqu'à la fin des sauvegardes asynchrones en cours
func AttendreSauvegardes() {
	sauvegardesEnCours.Wait()
}

// ecrireAtomiquement écrit dans un fichier temporaire puis le renomme :
// un crash en cours d'écriture ne laisse jamais un fichier tronqué.
func ecrireAtomiquement(chemin string, donnees []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(chemin), filepath.Base(chemin)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // sans effet si le renommage a réussi

	if _, err := tmp.Write(donnees); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0600); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), chemin)
}

// 4. Chargement initial et synchronisation des ID.
// Un fichier illisible ou corrompu renvoie une erreur au lieu d'être écrasé.
func ChargerFichier() error {
	Store.mu.Lock()

	contenu, err := os.ReadFile(nomFichier)
	switch {
	case errors.Is(err, os.ErrNotExist):
		fmt.Printf("Aucun fichier '%s' trouvé. Initialisation du stock d'origine...\n", nomFichier)

		Store.Version = VersionFormat
		Store.Users = []models.User{}
		Store.Customers = []models.Customer{
			{ID: 1, Nom: "Demetrius", KycValide: true, ScoreRisque: 10},
		}
		Store.Accounts = []models.Account{
			{ID: 1, CustomerID: 1, SoldeCentimes: 500000, DecouvertCentimes: 50000, Devise: "EUR"},
		}
		Store.Transfers = []models.Transfer{}
		Store.Loans = []models.Loan{}
		Store.AuditLogs = []models.AuditLog{}

	case err != nil:
		Store.mu.Unlock()
		return fmt.Errorf("lecture de %s : %w", nomFichier, err)

	default:
		var d donneesFichier
		if err := json.Unmarshal(contenu, &d); err != nil {
			Store.mu.Unlock()
			return fmt.Errorf("décodage de %s (fichier corrompu ?) : %w", nomFichier, err)
		}
		if d.Version != VersionFormat {
			Store.mu.Unlock()
			return fmt.Errorf("%s est au format %d, format %d attendu (ancien format en float : supprimez ou migrez le fichier)", nomFichier, d.Version, VersionFormat)
		}
		Store.Version = d.Version
		Store.Users = d.Users
		Store.Customers = d.Customers
		Store.Accounts = d.Accounts
		Store.Transfers = d.Transfers
		Store.Loans = d.Loans
		Store.AuditLogs = d.AuditLogs
	}

	// Recalcul du plus grand ID pour chaque tranche
	calculerDerniersID()

	fmt.Printf("Initialisation réussie | Utilisateurs: %d | Clients: %d | Comptes: %d | Prêts: %d\n",
		len(Store.Users), len(Store.Customers), len(Store.Accounts), len(Store.Loans))
	Store.mu.Unlock()

	if errors.Is(err, os.ErrNotExist) {
		return SauvegarderFichier()
	}
	return nil
}

// EnregistrerAudit ajoute une entrée au journal d'audit (doit être appelée sous verrou)
func EnregistrerAudit(action string, details string) {
	DernierAuditID++
	Store.AuditLogs = append(Store.AuditLogs, models.AuditLog{
		ID:         DernierAuditID,
		Action:     action,
		Details:    details,
		Horodatage: time.Now(),
	})
}

// TrouverUtilisateur renvoie l'utilisateur d'ID donné ou nil (doit être appelée sous verrou)
func TrouverUtilisateur(id int) *models.User {
	for i := range Store.Users {
		if Store.Users[i].ID == id {
			return &Store.Users[i]
		}
	}
	return nil
}

// TrouverUtilisateurParEmail renvoie l'utilisateur d'email donné ou nil (doit être appelée sous verrou)
func TrouverUtilisateurParEmail(email string) *models.User {
	for i := range Store.Users {
		if Store.Users[i].Email == email {
			return &Store.Users[i]
		}
	}
	return nil
}

// TrouverClient renvoie le client d'ID donné ou nil (doit être appelée sous verrou)
func TrouverClient(id int) *models.Customer {
	for i := range Store.Customers {
		if Store.Customers[i].ID == id {
			return &Store.Customers[i]
		}
	}
	return nil
}

// TrouverCompte renvoie le compte d'ID donné ou nil (doit être appelée sous verrou)
func TrouverCompte(id int) *models.Account {
	for i := range Store.Accounts {
		if Store.Accounts[i].ID == id {
			return &Store.Accounts[i]
		}
	}
	return nil
}

// Fonction utilitaire privée pour déterminer les maxID
func calculerDerniersID() {
	DernierUserID = maxID(len(Store.Users), func(i int) int { return Store.Users[i].ID })
	DernierCustomerID = maxID(len(Store.Customers), func(i int) int { return Store.Customers[i].ID })
	DernierAccountID = maxID(len(Store.Accounts), func(i int) int { return Store.Accounts[i].ID })
	DernierTransferID = maxID(len(Store.Transfers), func(i int) int { return Store.Transfers[i].ID })
	DernierLoanID = maxID(len(Store.Loans), func(i int) int { return Store.Loans[i].ID })
	DernierAuditID = maxID(len(Store.AuditLogs), func(i int) int { return Store.AuditLogs[i].ID })
}

func maxID(length int, getID func(int) int) int {
	max := 0
	for i := 0; i < length; i++ {
		id := getID(i)
		if id > max {
			max = id
		}
	}
	return max
}

func (ds *DataStore) Lock() {
	ds.mu.Lock()
}
func (ds *DataStore) Unlock() {
	ds.mu.Unlock()
}

func (ds *DataStore) RLock() {
	ds.mu.RLock()
}

func (ds *DataStore) RUnlock() {
	ds.mu.RUnlock()
}
