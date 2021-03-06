package dbstore

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"

	"github.com/sekerez/polka/utils"
)

const envPath = "env/postgres.env"

var db *DB // Create singleton DB

type DB struct {
	path     string
	ctx      context.Context
	logger   *log.Logger
	conn     *pgxpool.Pool
	quit     chan bool
	bankId   map[string]uint16
	bankChan <-chan *utils.BankBalance
	accChan  <-chan *utils.Balance
}

func New(
	ctx context.Context,
	bankNumChan chan<- uint16,
	bankChan <-chan *utils.BankBalance,
	accChan <-chan *utils.Balance,
	bankRetChan chan<- *utils.BankBalance,
	accRetChan chan<- *utils.Balance,
) error {

	var (
		bankName    string
		bankId      uint16
		bankNum     uint16
		account     uint32
		bankBalance int64
		accBalance  int32
	)

	logger := log.New(os.Stderr, "[postgres] ", log.LstdFlags|log.Lshortfile)

	// Get environment variables and format url
	if err := godotenv.Load(envPath); err != nil {
		return err
	}

	logger.Printf("Created Logger")
	// Write db url
	path := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		os.Getenv("POSTGRESUSER"),
		os.Getenv("POSTGRESPASS"),
		os.Getenv("POSTGRESHOST"),
		os.Getenv("POSTGRESPORT"),
		os.Getenv("POSTGRESNAME"),
	)

	// Connect to database
	conn, err := pgxpool.Connect(ctx, path) // ConnPool?
	if err != nil {
		return err
	}

	// Insert variables inside object
	db = &DB{
		ctx:      ctx,
		path:     path,
		conn:     conn,
		logger:   logger,
		accChan:  accChan,
		bankChan: bankChan,
		quit:     make(chan bool),
	}

	db.logger.Printf("Max Connections: %d", conn.Stat().MaxConns())
	// Pass number of banks
	err = db.conn.QueryRow(db.ctx, bankNumQ).Scan(&bankNum)
	if err != nil {
		db.logger.Fatalf("Error querying banks length: %s", err)
		return err
	}
	db.logger.Printf("Sending over banknum: %d", bankNum)
	bankNumChan <- bankNum
	close(bankNumChan)

	// Restore bank balances
	rows, err := db.conn.Query(
		db.ctx,
		bankRetrieveQ,
	)
	if err != nil {
		db.logger.Fatalf("Could not retrieve bank balances: %s", err)
		return err
	}
	// Iterate through banks rows and send to memcache through channel
	for rows.Next() {
		err = rows.Scan(&bankId, &bankName, &bankBalance)
		if err != nil {
			db.logger.Printf("Could not retrieve bank balance row: %s", err)
			return err
		}

		bankRetChan <- &utils.BankBalance{
			Name:    bankName,
			BankId:  bankId,
			Balance: bankBalance,
		}
	}
	close(bankRetChan)

	// Restore account balances
	rows, err = db.conn.Query(
		db.ctx,
		accRetrieveQ,
	)
	if err != nil {
		db.logger.Fatalf("Could not retrieve account balances: %s", err)
		return err
	}
	// Iterate through accounts rows and send to memcache through channel
	for rows.Next() {
		err = rows.Scan(&bankName, &account, &accBalance)
		if err != nil {
			db.logger.Printf("Could not retrieve account balance row: %s", err)
			return err
		}

		accRetChan <- &utils.Balance{
			BankName: bankName,
			Account:  account,
			Balance:  accBalance,
		}
	}
	close(accRetChan)

	// Set up periodic update
	go updateDatabase()

	return nil
}

// updateDatabase is a goroutine that awaits bank and account backups from memstore
// and submits them to the database.
func updateDatabase() {
	for {
		select {
		// In case of a quit message, end the goroutine
		case <-db.quit:
			return
		// In case of a bank balance, update the banks table
		case bankBalance := <-db.bankChan:
			_, err := db.conn.Exec(
				db.ctx,
				updateBankBalanceQ,
				bankBalance.BankId,
				bankBalance.Balance,
			)
			if err != nil {
				db.logger.Printf("Error updating database: %s", err)
			}
		// In case of a account balance, update the accounts table
		case accBalance := <-db.accChan:
			_, err := db.conn.Exec(
				db.ctx,
				updateAccBalanceQ,
				accBalance.BankId,
				accBalance.Account,
				accBalance.Balance,
			)
			if err != nil {
				db.logger.Printf("Error updating database: %s", err)
			}
		}
	}
}

func Close() (err error) {
	db.quit <- true
	db.conn.Close()
	return
}
