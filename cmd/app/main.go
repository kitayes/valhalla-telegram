package main

import (
	"log"
	"os"
	"time"

	"valhalla-telegram/internal/delivery"
	"valhalla-telegram/internal/domain"
	"valhalla-telegram/internal/repository"
	"valhalla-telegram/internal/usecase"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func main() {
	botToken := getEnv("BOT_TOKEN", "7956067295:AAFiRbObm1jzuNJ0kXtwbvI97zuPArZMb90")

	defaultDSN := "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable"
	dbDSN := getEnv("DB_DSN", defaultDSN)

	var db *gorm.DB
	var err error

	maxRetries := 10
	for i := 1; i <= maxRetries; i++ {
		log.Printf("Trying to connect to DB (Attempt %d/%d)...", i, maxRetries)
		db, err = gorm.Open(postgres.Open(dbDSN), &gorm.Config{})

		if err == nil {
			log.Println("Success! Connected to Database.")
			break
		}

		log.Printf("DB connection failed: %v. Waiting 3 seconds...", err)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		log.Fatal("Could not connect to DB after multiple retries. Exiting.")
	}

	err = db.AutoMigrate(&domain.Team{}, &domain.Player{})
	if err != nil {
		log.Fatal("Migration error:", err)
	}
	log.Println("Database connection established & Migrations applied")

	playerRepo := repository.NewPlayerRepository(db)
	teamRepo := repository.NewTeamRepository(db)

	regUseCase := usecase.NewRegistrationUseCase(playerRepo, teamRepo)

	tgHandler, err := delivery.NewTelegramHandler(botToken, regUseCase)
	if err != nil {
		log.Fatal("Bot init error:", err)
	}

	// 6. Запуск
	log.Println("Valhalla Bot is running...")
	tgHandler.Start()
}
