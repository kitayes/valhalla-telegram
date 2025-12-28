package usecase

import (
	"fmt"
	"strconv"
	"valhalla-telegram/internal/domain"
	"valhalla-telegram/internal/repository"
)

type RegistrationUseCase interface {
	RegisterUser(tgID int64, username, firstName string) string
	StartSoloRegistration(tgID int64) string
	HandleUserInput(tgID int64, input string) (string, bool) // bool = нужен ли выбор кнопок (роль)
}

type regUseCase struct {
	repo repository.PlayerRepository
}

func NewRegistrationUseCase(repo repository.PlayerRepository) RegistrationUseCase {
	return &regUseCase{repo: repo}
}

func (uc *regUseCase) RegisterUser(tgID int64, username, firstName string) string {
	p := &domain.Player{TelegramID: tgID, TelegramUsername: username, FirstName: firstName}
	uc.repo.CreateOrUpdate(p)
	return fmt.Sprintf("Привет, %s! Ты в системе.", firstName)
}

func (uc *regUseCase) StartSoloRegistration(tgID int64) string {
	uc.repo.UpdateState(tgID, domain.StateWaitingGameID)
	return "📝 Начинаем регистрацию.\nВведите ваш **Game ID** (основной, без скобок):"
}

// Главная логика FSM
func (uc *regUseCase) HandleUserInput(tgID int64, input string) (string, bool) {
	player, _ := uc.repo.GetByTelegramID(tgID)

	switch player.FSMState {
	case domain.StateWaitingGameID:
		// Валидация ID (можно добавить проверку на цифры)
		uc.repo.UpdateGameData(tgID, "game_id", input)
		uc.repo.UpdateState(tgID, domain.StateWaitingZoneID)
		return "Отлично. Теперь введите **Zone ID** (цифры в скобках):", false

	case domain.StateWaitingZoneID:
		uc.repo.UpdateGameData(tgID, "zone_id", input)
		uc.repo.UpdateState(tgID, domain.StateWaitingStars)
		return "Принято. Сколько у вас **звезд** (Stars) в текущем сезоне? (введите число)", false

	case domain.StateWaitingStars:
		stars, err := strconv.Atoi(input)
		if err != nil {
			return "⚠️ Пожалуйста, введите число.", false
		}
		uc.repo.UpdateGameData(tgID, "stars", stars)
		uc.repo.UpdateState(tgID, domain.StateWaitingRole)
		return "Почти все! Выберите вашу **основную роль**:", true // true = покажи клавиатуру

	case domain.StateWaitingRole:
		// Тут мы ожидаем текст с кнопки (например "Gold")
		uc.repo.UpdateGameData(tgID, "main_role", input)
		uc.repo.UpdateState(tgID, domain.StateIdle) // Сброс состояния
		return "🎉 Регистрация завершена! Ждите формирования команд.", false

	default:
		return "Я не понимаю. Нажмите /reg_solo для начала.", false
	}
}
