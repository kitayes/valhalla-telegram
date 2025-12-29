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
	StartTeamRegistration(tgID int64) string
	HandleUserInput(tgID int64, input string) (string, bool)
	DeleteTeam(tgID int64) string
	GetTeamInfo(tgID int64) string
}

type regUseCase struct {
	playerRepo repository.PlayerRepository
	teamRepo   repository.TeamRepository
}

func NewRegistrationUseCase(pRepo repository.PlayerRepository, tRepo repository.TeamRepository) RegistrationUseCase {
	return &regUseCase{playerRepo: pRepo, teamRepo: tRepo}
}

func (uc *regUseCase) RegisterUser(tgID int64, username, firstName string) string {
	p := &domain.Player{TelegramID: tgID, TelegramUsername: username, FirstName: firstName}
	uc.playerRepo.CreateOrUpdate(p)
	return fmt.Sprintf("Привет, %s! Ты в системе.", firstName)
}

func (uc *regUseCase) HandleUserInput(tgID int64, input string) (string, bool) {
	player, _ := uc.playerRepo.GetByTelegramID(tgID)

	switch player.FSMState {
	case domain.StateWaitingGameID:
		uc.playerRepo.UpdateGameData(tgID, "game_id", input)
		uc.playerRepo.UpdateState(tgID, domain.StateWaitingZoneID)
		return "Отлично. Теперь введите **Zone ID** (цифры в скобках):", false

	case domain.StateWaitingZoneID:
		uc.playerRepo.UpdateGameData(tgID, "zone_id", input)
		uc.playerRepo.UpdateState(tgID, domain.StateWaitingStars)
		return "Принято. Сколько у вас **звезд** (Stars) в текущем сезоне? (введите число)", false

	case domain.StateWaitingStars:
		stars, err := strconv.Atoi(input)
		if err != nil {
			return "⚠️ Пожалуйста, введите число.", false
		}
		uc.playerRepo.UpdateGameData(tgID, "stars", stars)
		uc.playerRepo.UpdateState(tgID, domain.StateWaitingRole)
		return "Почти все! Выберите вашу **основную роль**:", true // true = покажи клавиатуру

	case domain.StateWaitingRole:
		uc.playerRepo.UpdateGameData(tgID, "main_role", input)
		uc.playerRepo.UpdateState(tgID, domain.StateIdle) // Сброс состояния
		return "Регистрация завершена! Ждите формирования команд.", false

	case domain.StateWaitingTeamName:
		team, err := uc.teamRepo.CreateTeam(input)
		if err != nil {
			return "Такое имя команды уже занято или произошла ошибка. Попробуйте другое:", false
		}

		uc.playerRepo.UpdateGameData(tgID, "team_id", team.ID)

		uc.playerRepo.UpdateState(tgID, domain.StateIdle)

		return fmt.Sprintf("🏆 Команда **%s** успешно создана! Вы назначены капитаном.", team.Name), false

	default:
		return "Я не понимаю. Нажмите /reg_solo или /reg_team.", false
	}
}

func (uc *regUseCase) StartSoloRegistration(tgID int64) string {
	player, _ := uc.playerRepo.GetByTelegramID(tgID)

	if player.TeamID != nil {
		return "Вы уже состоите в команде! Чтобы зарегистрироваться как соло, сначала покиньте команду или удалите её (/delete_team)."
	}

	uc.playerRepo.UpdateState(tgID, domain.StateWaitingGameID)
	return "Начинаем регистрацию соло-игрока.\nВведите ваш **Game ID** (основной, без скобок):"
}

func (uc *regUseCase) StartTeamRegistration(tgID int64) string {
	player, _ := uc.playerRepo.GetByTelegramID(tgID)

	if player.TeamID != nil {
		return "Вы уже состоите в команде! Вы не можете создать новую, пока не покинете текущую."
	}

	uc.playerRepo.UpdateState(tgID, domain.StateWaitingTeamName)
	return "Вы регистрируете новую команду.\nВведите **Название команды**:"
}

func (uc *regUseCase) DeleteTeam(tgID int64) string {
	player, _ := uc.playerRepo.GetByTelegramID(tgID)

	if player.TeamID == nil {
		return "⚠️ У вас нет команды, чтобы её удалять."
	}

	teamID := *player.TeamID

	err := uc.playerRepo.ResetTeamID(teamID)
	if err != nil {
		return "Ошибка при роспуске игроков."
	}

	err = uc.teamRepo.DeleteTeam(teamID)
	if err != nil {
		return "Ошибка при удалении команды."
	}

	return "🗑 Команда успешно распущена. Все игроки теперь свободны."
}

func (uc *regUseCase) GetTeamInfo(tgID int64) string {
	player, _ := uc.playerRepo.GetByTelegramID(tgID)

	if player.TeamID == nil {
		return "Вы не состоите в команде. Используйте /reg_team для создания или /reg_solo для поиска."
	}

	team, err := uc.teamRepo.GetTeamByID(*player.TeamID)
	if err != nil {
		return "Ошибка получения данных команды."
	}

	members, _ := uc.playerRepo.GetTeamMembers(*player.TeamID)

	report := fmt.Sprintf("Команда: %s\n\n👥 Состав:\n", team.Name)
	for i, p := range members {
		role := p.MainRole
		if role == "" {
			role = "Не определена"
		}

		report += fmt.Sprintf("%d. %s (Ранг: %d⭐) — %s\n", i+1, p.GameNickname, p.Stars, role)
	}

	return report
}
