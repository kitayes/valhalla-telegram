package delivery

import (
	"log"
	"strings"
	"valhalla-telegram/internal/usecase"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var adminIDs = []int64{
	123456789, // Твой ID
	987654321, // ID второго админа
}

func isAdmin(id int64) bool {
	for _, admin := range adminIDs {
		if admin == id {
			return true
		}
	}
	return false
}

type TelegramHandler struct {
	bot     *tgbotapi.BotAPI
	useCase usecase.RegistrationUseCase
}

func NewTelegramHandler(token string, uc usecase.RegistrationUseCase) (*TelegramHandler, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)

	return &TelegramHandler{bot: bot, useCase: uc}, nil
}

func (h *TelegramHandler) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := h.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		msg := update.Message
		chatID := msg.Chat.ID
		text := msg.Text
		user := msg.From

		// Регистрируем пользователя при любом контакте, чтобы он был в базе
		h.useCase.RegisterUser(chatID, user.UserName, user.FirstName)

		var response string
		var showKeyboard bool

		// --- АДМИНСКИЕ КОМАНДЫ ---
		if isAdmin(chatID) {
			if strings.HasPrefix(text, "/admin") {
				response = "👮 Админ-панель:\n\n" +
					"/export - Скачать список команд (Excel/CSV)\n" +
					"/broadcast [текст] - Рассылка всем капитанам\n" +
					"/close_reg - Закрыть регистрацию\n" +
					"/open_reg - Открыть регистрацию\n" +
					"/del_team [Название] - Удалить команду\n" +
					"/reset_user [ChatID] - Сброс FSM"
				h.sendMessage(chatID, response, false)
				continue
			}

			if text == "/export" {
				csvData, err := h.useCase.GenerateTeamsCSV()
				if err != nil {
					h.sendMessage(chatID, "Ошибка генерации: "+err.Error(), false)
				} else {
					// Отправка файла
					fileBytes := tgbotapi.FileBytes{
						Name:  "teams_export.csv",
						Bytes: csvData,
					}
					docMsg := tgbotapi.NewDocument(chatID, fileBytes)
					h.bot.Send(docMsg)
				}
				continue
			}

			if strings.HasPrefix(text, "/broadcast ") {
				msgText := strings.TrimPrefix(text, "/broadcast ")
				captains, _ := h.useCase.GetBroadcastList()

				count := 0
				for _, capID := range captains {
					h.sendMessage(capID, "📢 ОФИЦИАЛЬНОЕ ОБЪЯВЛЕНИЕ:\n\n"+msgText, false)
					count++
				}
				h.sendMessage(chatID, response+string(rune(count))+" капитанов получили сообщение.", false)
				continue
			}

			if text == "/close_reg" {
				h.useCase.SetRegistrationOpen(false)
				h.sendMessage(chatID, "⛔ Регистрация закрыта.", false)
				continue
			}
			if text == "/open_reg" {
				h.useCase.SetRegistrationOpen(true)
				h.sendMessage(chatID, "✅ Регистрация открыта.", false)
				continue
			}

			if strings.HasPrefix(text, "/del_team ") {
				teamName := strings.TrimPrefix(text, "/del_team ")
				resp := h.useCase.AdminDeleteTeam(teamName)
				h.sendMessage(chatID, resp, false)
				continue
			}
		}

		// --- ПОЛЬЗОВАТЕЛЬСКИЕ КОМАНДЫ ---
		switch text {
		case "/start":
			response = "Добро пожаловать в Valhalla Cup!\n\n" +
				"/reg_solo - Регистрация соло (поиск команды)\n" +
				"/reg_team - Регистрация своей команды (для капитанов)\n" +
				"/my_team - Моя команда и статус\n" +
				"/checkin - Подтвердить участие (Check-in)\n" +
				"/delete_team - Распустить команду (только капитан)"

		case "/reg_solo":
			response = h.useCase.StartSoloRegistration(chatID)
		case "/reg_team":
			response = h.useCase.StartTeamRegistration(chatID)
		case "/my_team":
			response = h.useCase.GetTeamInfo(chatID)
		case "/checkin":
			response = h.useCase.ToggleCheckIn(chatID)
		case "/delete_team":
			response = h.useCase.DeleteTeam(chatID)

		default:
			response, showKeyboard = h.useCase.HandleUserInput(chatID, text)
		}

		h.sendMessage(chatID, response, showKeyboard)
	}
}

func (h *TelegramHandler) sendMessage(chatID int64, text string, showKeyboard bool) {
	if text == "" {
		return
	}
	msg := tgbotapi.NewMessage(chatID, text)

	if showKeyboard {
		// Пример клавиатуры ролей
		keyboard := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Gold"),
				tgbotapi.NewKeyboardButton("Exp"),
				tgbotapi.NewKeyboardButton("Mid"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Roam"),
				tgbotapi.NewKeyboardButton("Jungle"),
			),
		)
		msg.ReplyMarkup = keyboard
	} else {
		msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	}

	h.bot.Send(msg)
}
