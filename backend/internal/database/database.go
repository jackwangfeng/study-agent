package database

import (
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Initialize creates and initializes database connection
func Initialize(dsn string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	// Use SQLite if dsn starts with "sqlite:"
	if strings.HasPrefix(dsn, "sqlite:") {
		db, err = gorm.Open(sqlite.Open(strings.TrimPrefix(dsn, "sqlite:")), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
	} else {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
	}

	if err != nil {
		return nil, err
	}

	// Get underlying SQL DB for connection pooling
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	return db, nil
}

// Migrate runs database migrations.
// study-agent doesn't ship pgvector / HNSW yet — chemistry tutoring doesn't
// need ANN over chat history (we keep mistake records by id, not embeddings).
// If we ever add embedding-based concept search over the 错题本, restore the
// pgvector extension + halfvec index here.
//
// What still has to happen explicitly: relax user_accounts.phone NOT NULL —
// the original schema (SMS-only login era) made phone required, but Google
// sign-in introduces accounts without phone. AutoMigrate won't drop the
// constraint by itself, so we do it here.
func Migrate(db *gorm.DB, models ...interface{}) error {
	if db.Dialector.Name() == "postgres" {
		var phoneNotNull bool
		db.Raw(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name='user_accounts'
			  AND column_name='phone'
			  AND is_nullable='NO'
		)`).Scan(&phoneNotNull)
		if phoneNotNull {
			if err := db.Exec("ALTER TABLE user_accounts ALTER COLUMN phone DROP NOT NULL").Error; err != nil {
				return err
			}
		}
	}

	return db.AutoMigrate(models...)
}
