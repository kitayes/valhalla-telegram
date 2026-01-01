package usecase

import (
	"fmt"
	"strconv"
	"strings"
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
	idPtr := &tgID
	p := &domain.Player{TelegramID: idPtr, TelegramUsername: username, FirstName: firstName}
	uc.playerRepo.CreateOrUpdate(p)
	return fmt.Sprintf("Привет, %s! Добро пожаловать в Valhalla Cup.", firstName)
}

func (uc *regUseCase) HandleUserInput(tgID int64, input string) (string, bool) {
	player, _ := uc.playerRepo.GetByTelegramID(tgID)

	if strings.HasPrefix(player.FSMState, "team_reg_") {
		return uc.handleTeamLoop(player, input)
	}

	switch player.FSMState {
	case domain.StateWaitingNickname:
		uc.playerRepo.UpdateGameData(tgID, "game_nickname", input)
		uc.playerRepo.UpdateState(tgID, domain.StateWaitingGameID)
		return "Принято. Теперь введите ваш Game ID (Mobile Legends ID):", false

	case domain.StateWaitingGameID:
		uc.playerRepo.UpdateGameData(tgID, "game_id", input)
		uc.playerRepo.UpdateState(tgID, domain.StateWaitingZoneID)
		return "Отлично. Теперь введите Zone ID (цифры в скобках):", false

	case domain.StateWaitingZoneID:
		uc.playerRepo.UpdateGameData(tgID, "zone_id", input)
		uc.playerRepo.UpdateState(tgID, domain.StateWaitingStars)
		return "Принято. Какое ваше максимальное количество звезд? (число)", false

	case domain.StateWaitingStars:
		stars, err := strconv.Atoi(input)
		if err != nil {
			return "Пожалуйста, введите число.", false
		}
		uc.playerRepo.UpdateGameData(tgID, "stars", stars)
		uc.playerRepo.UpdateState(tgID, domain.StateWaitingRole)
		return "Почти все! Выберите вашу основную роль:", true

	case domain.StateWaitingRole:
		uc.playerRepo.UpdateGameData(tgID, "main_role", input)
		uc.playerRepo.UpdateState(tgID, domain.StateIdle)
		return "Регистрация соло-игрока завершена! Ждите анонсов.", false

	case domain.StateWaitingTeamName:
		team, err := uc.teamRepo.CreateTeam(input)
		if err != nil {
			return "Такое название уже занято. Придумайте другое:", false
		}

		uc.playerRepo.UpdateGameData(tgID, "team_id", team.ID)
		uc.playerRepo.UpdateGameData(tgID, "is_captain", true)

		uc.playerRepo.UpdateState(tgID, "team_reg_nick_1")

		return fmt.Sprintf(
			"Команда '%s' создана!\nТеперь заполним анкету состава (7 человек).\n\n--- Игрок №1 (Вы/Капитан) ---\nВведите ваш игровой Никнейм:",
			team.Name,
		), false

	default:
		return "Команда не распознана. Используйте меню или /start", false
	}
}

func (uc *regUseCase) handleTeamLoop(captain *domain.Player, input string) (string, bool) {
	parts := strings.Split(captain.FSMState, "_")
	if len(parts) < 4 {
		return "Ошибка состояния FSM. Напишите /start", false
	}

	step := parts[2]
	slotStr := parts[3]
	slot, _ := strconv.Atoi(slotStr)
	teamID := *captain.TeamID
	captainID := *captain.TelegramID

	isCaptain := slot == 1

	switch step {
	case "nick":
		if isCaptain {
			uc.playerRepo.UpdateGameData(captainID, "game_nickname", input)
		} else {
			isSub := slot >= 6
			newPlayer := &domain.Player{
				TeamID:       &teamID,
				GameNickname: input,
				IsSubstitute: isSub,
			}
			if err := uc.playerRepo.CreateTeammate(newPlayer); err != nil {
				return "Ошибка сохранения. Попробуйте еще раз:", false
			}
		}

		uc.playerRepo.UpdateState(captainID, fmt.Sprintf("team_reg_id_%d", slot))
		return fmt.Sprintf("Ник '%s' принят. Введите Game ID (Mobile Legends ID):", input), false

	case "id":
		if isCaptain {
			uc.playerRepo.UpdateGameData(captainID, "game_id", input)
		} else {
			uc.playerRepo.UpdateLastTeammateData(teamID, "game_id", input)
		}

		uc.playerRepo.UpdateState(captainID, fmt.Sprintf("team_reg_role_%d", slot))

		msg := "Выберите роль:"
		if slot >= 6 {
			msg = "Это игрок замены. Выберите роль (или 'Замена/Любая'):"
		}
		return msg, true

	case "role":
		if isCaptain {
			uc.playerRepo.UpdateGameData(captainID, "main_role", input)
		} else {
			uc.playerRepo.UpdateLastTeammateData(teamID, "main_role", input)
		}

		uc.playerRepo.UpdateState(captainID, fmt.Sprintf("team_reg_contact_%d", slot))
		return "Принято. Введите Telegram Username для связи (например @Dichotomya):", false

	case "contact":
		if !strings.HasPrefix(input, "@") && len(input) > 1 {
			input = "@" + input
		}

		if isCaptain {
			uc.playerRepo.UpdateGameData(captainID, "telegram_username", input)
		} else {
			uc.playerRepo.UpdateLastTeammateData(teamID, "telegram_username", input)
		}

		if slot < 7 {
			nextSlot := slot + 1
			uc.playerRepo.UpdateState(captainID, fmt.Sprintf("team_reg_nick_%d", nextSlot))

			status := "Основа"
			if nextSlot >= 6 {
				status = "ЗАМЕНА"
			}

			msg := fmt.Sprintf("Игрок №%d сохранен.\n\n--- Игрок №%d (%s) ---\nВведите игровой Никнейм:", slot, nextSlot, status)
			return msg, false
		} else {
			// Все 7 игроков заполнены
			uc.playerRepo.UpdateState(captainID, domain.StateIdle)
			return "Поздравляю! Команда полностью зарегистрирована (5 основы + 2 замены).\nИспользуйте /my_team чтобы проверить состав.", false
		}
	}

	return "Ошибка шага регистрации.", false
}

func (uc *regUseCase) StartSoloRegistration(tgID int64) string {
	player, _ := uc.playerRepo.GetByTelegramID(tgID)
	if player.TeamID != nil {
		return "Вы уже в команде. Сначала покиньте её (/delete_team)."
	}
	uc.playerRepo.UpdateState(tgID, domain.StateWaitingNickname)
	return "Начинаем соло-регистрацию.\nВведите ваш игровой никнейм:"
}

func (uc *regUseCase) StartTeamRegistration(tgID int64) string {
	player, _ := uc.playerRepo.GetByTelegramID(tgID)
	if player.TeamID != nil {
		return "Вы уже в команде. Нельзя создать новую."
	}

	uc.playerRepo.UpdateState(tgID, domain.StateWaitingTeamName)
	return "Регистрация новой команды (7 человек).\nВведите Название команды:"
}

func (uc *regUseCase) DeleteTeam(tgID int64) string {
	player, _ := uc.playerRepo.GetByTelegramID(tgID)
	if player.TeamID == nil {
		return "У вас нет команды."
	}
	if !player.IsCaptain {
		return "Только капитан может распустить команду."
	}

	teamID := *player.TeamID

	uc.playerRepo.ResetTeamID(teamID)
	uc.teamRepo.DeleteTeam(teamID)

	return "Команда распущена."
}

func (uc *regUseCase) GetTeamInfo(tgID int64) string {
	player, _ := uc.playerRepo.GetByTelegramID(tgID)
	if player.TeamID == nil {
		return "Вы не в команде."
	}

	team, err := uc.teamRepo.GetTeamByID(*player.TeamID)
	if err != nil {
		return "Ошибка поиска команды."
	}

	members, _ := uc.playerRepo.GetTeamMembers(*player.TeamID)

	report := fmt.Sprintf("🛡 Команда: %s\n", team.Name)
	report += "----------------------\n"

	for i, p := range members {
		status := "Основа"
		if p.IsSubstitute {
			status = "ЗАМЕНА"
		}
		if p.IsCaptain {
			status += " (Капитан)"
		}

		report += fmt.Sprintf(
			"%d. %s [%s]\n   ID: %s\n   Роль: %s\n   ТГ: %s\n\n",
			i+1,
			p.GameNickname,
			status,
			p.GameID,
			p.MainRole,
			p.TelegramUsername,
		)
	}

	return report
}
