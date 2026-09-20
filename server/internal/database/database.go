package database

import (
	"fmt"
	"time"

	"bedrud/config"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite" // Added for SQLite support
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

const (
	DBTypePostgres = "postgres"
	DBTypeSQLite   = "sqlite"
)

// Initialize sets up the database connection (server process; may log at info).
func Initialize(cfg *config.DatabaseConfig) error {
	return initialize(cfg, false)
}

// InitializeQuiet connects without GORM SQL dumps or JSON info logs.
func InitializeQuiet(cfg *config.DatabaseConfig) error {
	return initialize(cfg, true)
}

// OpenCLI connects and runs schema migrations without GORM SQL dumps or
// JSON info logs. Use from operator CLI commands (update, user, room, db).
func OpenCLI(cfg *config.DatabaseConfig) error {
	if err := initialize(cfg, true); err != nil {
		return err
	}
	return runMigrations(true)
}

func initialize(cfg *config.DatabaseConfig, quiet bool) error {
	var err error
	var dsn string
	var dialector gorm.Dialector

	// Map zerolog global level to GORM log level so GORM respects
	// the logger.level setting from config.yaml.
	gormLogLevel := gormLogLevelFromZerolog(zerolog.GlobalLevel())
	if quiet {
		gormLogLevel = logger.Silent
	}

	// Configure GORM
	gormConfig := GormConfig(gormLogLevel)

	// Determine database type and prepare dialector
	dbType := cfg.Type
	if dbType == "" {
		// Default to postgres if not specified in config, though config should have a default
		log.Warn().Msg("Database type not specified in config, defaulting to postgres")
		dbType = DBTypePostgres
	}

	if !quiet {
		log.Info().Str("databaseType", dbType).Msg("Initializing database")
	}

	switch dbType {
	case DBTypePostgres:
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
			cfg.Host,
			cfg.User,
			cfg.Password,
			cfg.DBName,
			cfg.Port,
			cfg.SSLMode,
		)
		dialector = postgres.Open(dsn)
		if !quiet {
			log.Info().Msg("Using PostgreSQL driver")
		}
	case DBTypeSQLite:
		if cfg.Path == "" {
			err = fmt.Errorf("SQLite database path (DB_PATH or config.database.path) is not configured")
			log.Error().Err(err).Msg("SQLite configuration error")
			return err
		}
		dsn = cfg.Path // For SQLite, DSN is the file path
		dialector = sqlite.Open(dsn)
		if !quiet {
			log.Info().Str("path", dsn).Msg("Using SQLite driver")
		}
	default:
		err = fmt.Errorf("unsupported database type: %s. Supported types are 'postgres' and 'sqlite'", dbType)
		log.Error().Err(err).Msg("Database configuration error")
		return err
	}

	// Connect to the database
	db, err = gorm.Open(dialector, gormConfig)
	if err != nil {
		log.Error().Err(err).Str("host", cfg.Host).Str("dbname", cfg.DBName).Str("type", dbType).Msg("Failed to connect to database")
		return err
	}

	// SQLite-specific optimizations
	if dbType == DBTypeSQLite {
		if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
			log.Warn().Err(err).Msg("Failed to enable SQLite foreign keys")
		}
		if err := db.Exec("PRAGMA journal_mode = WAL").Error; err != nil {
			log.Warn().Err(err).Msg("Failed to enable SQLite WAL mode")
		}
	}

	// Configure connection pool (these settings might have limited or no effect for SQLite)
	sqlDB, err := db.DB()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get underlying *sql.DB")
		return err
	}

	if cfg.Type == DBTypePostgres || cfg.Type == "" { // Apply pooling mainly for PostgreSQL
		if cfg.MaxIdleConns > 0 {
			sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
		}
		if cfg.MaxOpenConns > 0 {
			sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
		}
		if cfg.MaxLifetime > 0 {
			sqlDB.SetConnMaxLifetime(time.Duration(cfg.MaxLifetime) * time.Minute)
		}
	} else if cfg.Type == DBTypeSQLite {
		// WAL mode allows concurrent reads; single write at a time to avoid "database is locked"
		sqlDB.SetMaxOpenConns(1)
	}

	if !quiet {
		log.Info().Msg("Database connection established successfully")
	}
	return nil
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return db
}

// gormLogLevelFromZerolog maps a zerolog level to the corresponding GORM log level.
// GORM levels: Silent (no logs), Error, Warn, Info (all queries).
func gormLogLevelFromZerolog(level zerolog.Level) logger.LogLevel {
	switch {
	case level <= zerolog.DebugLevel:
		return logger.Info // all SQL when logger.level is debug/trace
	case level >= zerolog.ErrorLevel:
		return logger.Error
	default:
		// info (and unset): slow queries + errors only — not every AutoMigrate SELECT
		return logger.Warn
	}
}

// GormConfig returns the GORM configuration the server runs with.
//
// Anything that opens a handle must use this, tests included. The settings
// here change what migrations do, not just how they are logged:
// DisableForeignKeyConstraintWhenMigrating decides whether GORM derives
// foreign keys from model associations at all. Every foreign key this project
// wants is written by hand in RunMigrations, so that generation stays off — a
// handle opened without it gets constraints production never has, and
// migration behaviour under test stops matching deployment.
func GormConfig(logLevel logger.LogLevel) *gorm.Config {
	return &gorm.Config{
		Logger:                                   logger.Default.LogMode(logLevel),
		DisableForeignKeyConstraintWhenMigrating: true,
	}
}

// SetForTest sets the global database connection for testing.
// This bypasses Initialize, so the handle must already be built with
// GormConfig — see the note there.
func SetForTest(testDB *gorm.DB) {
	db = testDB
}

// ResetForTest closes and clears the global DB handle (tests only).
func ResetForTest() {
	if db != nil {
		_ = Close()
	}
	db = nil
}

// Close closes the database connection
func Close() error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
