package usecase

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"valhalla-telegram/internal/domain"
	"valhalla-telegram/internal/repository"
)

type RegistrationUseCase interface {
	RegisterUser(tgID int64, username, firstName string) string
	HandleUserInput(tgID int64, input string) (string, bool)

	// Команды пользователя
	StartSoloRegistration(tgID int64) string
	StartTeamRegistration(tgID int64) string
	DeleteTeam(tgID int64) string
	GetTeamInfo(tgID int64) string
	ToggleCheckIn(tgID int64) string

	// Админские функции
	SetRegistrationOpen(isOpen bool)
	IsRegistrationOpen() bool
	GenerateTeamsCSV() ([]byte, error)
	GetBroadcastList() ([]int64, error)
	AdminDeleteTeam(teamName string) string
	AdminResetUser(tgID int64) string
}

type regUseCase struct {
	playerRepo repository.PlayerRepository
	teamRepo   repository.TeamRepository

	isRegistrationOpen bool
}

func NewRegistrationUseCase(pRepo repository.PlayerRepository, tRepo repository.TeamRepository) RegistrationUseCase {
	return &regUseCase{
		playerRepo:         pRepo,
		teamRepo:           tRepo,
		isRegistrationOpen: true,
	}
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
	return "", false
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
		return fmt.Sprintf("Ник '%s' принят. Введите Game ID (основные цифры):", input), false

	case "id":
		if _, err := strconv.Atoi(input); err != nil {
			return "Game ID должен состоять только из цифр. Попробуйте снова:", false
		}

		if isCaptain {
			uc.playerRepo.UpdateGameData(captainID, "game_id", input)
		} else {
			uc.playerRepo.UpdateLastTeammateData(teamID, "game_id", input)
		}
		uc.playerRepo.UpdateState(captainID, fmt.Sprintf("team_reg_zone_%d", slot))
		return "Введите Zone ID (цифры в скобках, например 2024):", false

	case "zone":
		if _, err := strconv.Atoi(input); err != nil {
			return "Zone ID должен быть числом. Попробуйте снова:", false
		}

		if isCaptain {
			uc.playerRepo.UpdateGameData(captainID, "zone_id", input)
		} else {
			uc.playerRepo.UpdateLastTeammateData(teamID, "zone_id", input)
		}
		uc.playerRepo.UpdateState(captainID, fmt.Sprintf("team_reg_rank_%d", slot))
		return "Укажите текущее количество звезд (Rank) цифрой:", false

	case "rank":
		stars, err := strconv.Atoi(input)
		if err != nil {
			return "Введите число (количество звезд).", false
		}

		if isCaptain {
			uc.playerRepo.UpdateGameData(captainID, "stars", stars)
		} else {
			uc.playerRepo.UpdateLastTeammateData(teamID, "stars", stars)
		}

		uc.playerRepo.UpdateState(captainID, fmt.Sprintf("team_reg_role_%d", slot))

		msg := "Выберите роль:"
		if slot >= 6 {
			msg = "Это игрок замены. Выберите роль:"
		}
		return msg, true

	case "role":
		if isCaptain {
			uc.playerRepo.UpdateGameData(captainID, "main_role", input)
		} else {
			uc.playerRepo.UpdateLastTeammateData(teamID, "main_role", input)
		}

		uc.playerRepo.UpdateState(captainID, fmt.Sprintf("team_reg_contact_%d", slot))
		return "Принято. Введите Telegram Username игрока для связи (или поставьте прочерк '-' если нет):", false

	case "contact":
		if input != "-" && !strings.HasPrefix(input, "@") && len(input) > 1 {
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
			uc.playerRepo.UpdateState(captainID, domain.StateIdle)
			return "Поздравляю! Команда полностью зарегистрирована.\nИспользуйте /my_team чтобы проверить состав.", false
		}
	}

	return "Ошибка шага регистрации.", false
}

func (uc *regUseCase) StartSoloRegistration(tgID int64) string {
	if !uc.isRegistrationOpen {
		return "Регистрация закрыта."
	}

	player, _ := uc.playerRepo.GetByTelegramID(tgID)
	if player.TeamID != nil {
		return "Вы уже в команде. Сначала покиньте её (/delete_team)."
	}
	uc.playerRepo.UpdateState(tgID, domain.StateWaitingNickname)
	return "Начинаем соло-регистрацию.\nВведите ваш игровой никнейм:"
}

func (uc *regUseCase) StartTeamRegistration(tgID int64) string {
	if !uc.isRegistrationOpen {
		return "Регистрация на турнир сейчас ЗАКРЫТА."
	}

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

	checkInStatus := "НЕ ПОДТВЕРЖДЕНО"
	if team.IsCheckedIn {
		checkInStatus = "ГОТОВЫ К ИГРЕ"
	}

	report := fmt.Sprintf("🛡 Команда: %s\nСтатус: %s\n", team.Name, checkInStatus)
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
			"%d. %s [%s]\n   Rank: %d Stars (Zone: %s)\n   Role: %s\n   ТГ: %s\n\n",
			i+1, p.GameNickname, status, p.Stars, p.ZoneID, p.MainRole, p.TelegramUsername,
		)
	}
	return report
}

func (uc *regUseCase) SetRegistrationOpen(isOpen bool) {
	uc.isRegistrationOpen = isOpen
}

func (uc *regUseCase) IsRegistrationOpen() bool {
	return uc.isRegistrationOpen
}

func (uc *regUseCase) AdminDeleteTeam(teamName string) string {
	team, err := uc.teamRepo.GetTeamByName(teamName)
	if err != nil {
		return fmt.Sprintf("Команда '%s' не найдена.", teamName)
	}
	uc.playerRepo.ResetTeamID(team.ID)
	uc.teamRepo.DeleteTeam(team.ID)
	return fmt.Sprintf("Команда '%s' успешно удалена админом.", teamName)
}

func (uc *regUseCase) AdminResetUser(tgID int64) string {
	uc.playerRepo.UpdateState(tgID, domain.StateIdle)
	return "Состояние пользователя сброшено."
}

func (uc *regUseCase) GetBroadcastList() ([]int64, error) {
	captains, err := uc.playerRepo.GetAllCaptains()
	if err != nil {
		return nil, err
	}
	var ids []int64
	for _, c := range captains {
		if c.TelegramID != nil {
			ids = append(ids, *c.TelegramID)
		}
	}
	return ids, nil
}

func (uc *regUseCase) GenerateTeamsCSV() ([]byte, error) {
	teams, err := uc.teamRepo.GetAllTeams()
	if err != nil {
		return nil, err
	}

	b := &bytes.Buffer{}
	w := csv.NewWriter(b)

	w.Write([]string{"Team ID", "Team Name", "Checked In", "Role", "Nickname", "Game ID", "Zone ID", "Rank", "Telegram", "Is Captain"})

	for _, team := range teams {
		for _, p := range team.Players {
			checkInStr := "NO"
			if team.IsCheckedIn {
				checkInStr = "YES"
			}

			record := []string{
				fmt.Sprintf("%d", team.ID),
				team.Name,
				checkInStr,
				string(p.MainRole),
				p.GameNickname,
				p.GameID,
				p.ZoneID,
				fmt.Sprintf("%d", p.Stars),
				p.TelegramUsername,
				fmt.Sprintf("%t", p.IsCaptain),
			}
			w.Write(record)
		}
	}
	w.Flush()
	return b.Bytes(), nil
}

func (uc *regUseCase) ToggleCheckIn(tgID int64) string {
	if !uc.isRegistrationOpen {
	}

	player, _ := uc.playerRepo.GetByTelegramID(tgID)
	if player.TeamID == nil || !player.IsCaptain {
		return "Только капитан команды может делать Check-in."
	}

	team, err := uc.teamRepo.GetTeamByID(*player.TeamID)
	if err != nil {
		return "Ошибка команды."
	}

	newState := !team.IsCheckedIn
	uc.teamRepo.SetCheckIn(team.ID, newState)

	status := "ВЫ ПОДТВЕРДИЛИ УЧАСТИЕ!"
	if !newState {
		status = "Вы отменили подтверждение участия."
	}
	return fmt.Sprintf("Статус команды '%s':\n%s", team.Name, status)
}
