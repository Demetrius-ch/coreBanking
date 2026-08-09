package storage

import (
	"encoding/json"
	"fmt"
	"gestionbancaire/models"
	"os"
	"sync"
	"time"
)

// 1. Structure d'encapsulation globale pour la sérialisation JSON
type DataStore struct {
	//Le verouu qui protège le store
	mu        sync.RWMutex
	Customers []models.Customer `json:"customers"`
	Accounts  []models.Account  `json:"accounts"`
	Transfers []models.Transfer `json:"transfers"`
	Loans     []models.Loan     `json:"loans"`
	AuditLogs []models.AuditLog `json:"audit_logs"`
}

// 2. Variables globales d'état
var (
	Store DataStore

	// Auto-incréments pour chaque domaine
	DernierCustomerID int
	DernierAccountID  int
	DernierTransferID int
	DernierLoanID     int
	DernierAuditID    int

	nomFichier = "banking.json"
)

// 3. Sauvegarde de l'état complet dans le fichier JSON
func SauvegarderFichier() error {
	Store.mu.RLock()
	defer Store.mu.RUnlock()
	return sauvegarderSansVerrou()
}

func sauvegarderSansVerrou() error {
	donnees, err := json.MarshalIndent(&Store, "", "  ")
	if err != nil {
		fmt.Println("Erreur lors de la conversion JSON :", err)
		return err
	}

	return os.WriteFile(nomFichier, donnees, 0644)

}

// 4. Chargement initial et synchronisation des ID
func ChargerFichier() {
	Store.mu.Lock()
	defer Store.mu.Unlock()
	donnees, err := os.ReadFile(nomFichier)

	if err != nil {
		fmt.Println("Aucun fichier 'banking.json' trouvé. Initialisation du stock d'origine...")

		// Données par défaut pour tester le premier lancement
		// Store = DataStore{
		// 	Customers: []models.Customer{
		// 		{ID: 1, Nom: "Demetrius", KycValide: true, ScoreRisque: 10},
		// 	},

		// 	Accounts: []models.Account{
		// 		{ID: 1, CustomerID: 1, Solde: 5000.0, Decouvert: 500.0, Devise: "EUR"},
		// 	},
		// 	Transfers: []models.Transfer{},
		// 	Loans:     []models.Loan{},
		// 	AuditLogs: []models.AuditLog{},
		// }
		Store.Customers = []models.Customer{
			{ID: 1, Nom: "Demetrius", KycValide: true, ScoreRisque: 10},
		}
		Store.Accounts = []models.Account{
			{ID: 1, CustomerID: 1, Solde: 5000.0, Decouvert: 500.0, Devise: "EUR"},
		}
		Store.Transfers = []models.Transfer{}
		Store.Loans = []models.Loan{}
		Store.AuditLogs = []models.AuditLog{}

		sauvegarderSansVerrou()
	} else {
		err = json.Unmarshal(donnees, &Store)
		if err != nil {
			fmt.Println("Erreur lors du décodage du fichier JSON :", err)
			return
		}
	}

	// Recalcul du plus grand ID pour chaque tranche
	calculerDerniersID()

	fmt.Printf("Initialisation réussie | Clients: %d | Comptes: %d | Prêts: %d\n",
		len(Store.Customers), len(Store.Accounts), len(Store.Loans))
}

// EnregistrerAudit ajoute une entrée au journal d'audit (doit être appelée sous verrou)
// Dans storage/json.go

// EnregistrerAudit ajoute une entrée au journal d'audit (doit être appelée sous verrou)
func EnregistrerAudit(action string, details string) {
	DernierAuditID++
	log := models.AuditLog{
		ID:        DernierAuditID,
		Action:    action,
		Details:   details,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	Store.AuditLogs = append(Store.AuditLogs, log)
}

// Fonction utilitaire privée pour déterminer les maxID
func calculerDerniersID() {
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
