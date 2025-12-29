package main

import (
	"log"
	"os"
	"time"

	"valhalla-telegram/internal/delivery"
	"valhalla-telegram/internal/domain"
	"valhalla-telegram/internal/repository"
	"valhalla-telegram/internal/usecase"

	"gopkg.in/telebot.v3"
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
	dbDSN := getEnv("DB_DSN", "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable")

	db, err := gorm.Open(postgres.Open(dbDSN), &gorm.Config{})
	if err != nil {
		log.Fatal("DB Connect error:", err)
	}

	err = db.AutoMigrate(&domain.Team{}, &domain.Player{})
	if err != nil {
		log.Fatal("Migration error:", err)
	}
	log.Println("Database connection established & Migrations applied")

	pref := telebot.Settings{
		Token:  botToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		log.Fatal("Bot init error:", err)
	}

	playerRepo := repository.NewPlayerRepository(db)
	teamRepo := repository.NewTeamRepository(db)

	regUseCase := usecase.NewRegistrationUseCase(playerRepo, teamRepo)

	handler := delivery.NewHandler(b, regUseCase)

	handler.InitRoutes()

	log.Println("Bot is running...")
	b.Start()
}
