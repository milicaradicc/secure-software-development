package database

import (
    "log"
    "os"

    "github.com/glebarez/sqlite"
    "github.com/milicaradic/oblak/internal/models"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init() {
    dbPath := os.Getenv("DATABASE_PATH")
    if dbPath == "" {
        dbPath = "./oblak.db"
    }

    var err error
    DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
        Logger: logger.Default.LogMode(logger.Warn),
    })
    if err != nil {
        log.Fatalf("Ne mogu da otvorim bazu: %v", err)
    }

    // Auto migracija
    err = DB.AutoMigrate(&models.User{}, &models.Function{})
    if err != nil {
        log.Fatalf("Migracija neuspela: %v", err)
    }

    log.Println("Baza inicijalizovana")
}