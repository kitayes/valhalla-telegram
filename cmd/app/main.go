package main

import (
	"log"
	"os"

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

	db, err := gorm.Open(postgres.Open(dbDSN), &gorm.Config{})
	if err != nil {
		log.Fatal("DB Connect error:", err)
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
