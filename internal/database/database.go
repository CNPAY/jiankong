package database

import (
	"database/sql"
	"domain-monitor/internal/config"
	"domain-monitor/internal/models"
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

var DB *gorm.DB

// InitDB initializes the database connection
func InitDB(cfg *config.DatabaseConfig) error {
	var err error

	// Connect to database based on type
	switch cfg.Type {
	case "sqlite":
		// Use pure Go SQLite driver (modernc.org/sqlite)
		sqlDB, err := sql.Open("sqlite", cfg.Path)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}

		DB, err = gorm.Open(sqlite.Dialector{
			Conn: sqlDB,
		}, &gorm.Config{})
		if err != nil {
			return fmt.Errorf("failed to initialize GORM: %w", err)
		}
	// Add support for MySQL and PostgreSQL in the future
	// case "mysql":
	// case "postgres":
	default:
		return fmt.Errorf("unsupported database type: %s", cfg.Type)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto migrate the schema
	if err := DB.AutoMigrate(
		&models.Domain{},
		&models.Notification{},
		&models.Setting{},
		&models.User{},
	); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	return nil
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}
