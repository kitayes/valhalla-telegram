package delivery

import (
	"valhalla-telegram/internal/domain"
	"valhalla-telegram/internal/usecase"

	"gopkg.in/telebot.v3"
)

type Handler struct {
	uc  usecase.RegistrationUseCase
	bot *telebot.Bot
}

func NewHandler(b *telebot.Bot, uc usecase.RegistrationUseCase) *Handler {
	return &Handler{bot: b, uc: uc}
}

func (h *Handler) InitRoutes() {
	h.bot.Handle("/start", h.OnStart)
	h.bot.Handle("/reg_solo", h.OnRegSolo)
	h.bot.Handle("/reg_team", h.OnRegTeam)

	h.bot.Handle(telebot.OnText, h.OnTextMsg)
}

func (h *Handler) OnStart(c telebot.Context) error {
	user := c.Sender()
	msg := h.uc.RegisterUser(user.ID, user.Username, user.FirstName)
	return c.Send(msg)
}

func (h *Handler) OnRegSolo(c telebot.Context) error {
	msg := h.uc.StartSoloRegistration(c.Sender().ID)
	return c.Send(msg)
}

func (h *Handler) OnTextMsg(c telebot.Context) error {
	user := c.Sender()
	text := c.Text()

	responseMsg, showKeyboard := h.uc.HandleUserInput(user.ID, text)

	if showKeyboard {
		menu := &telebot.ReplyMarkup{ResizeKeyboard: true}
		btnGold := menu.Text(string(domain.RoleGold))
		btnExp := menu.Text(string(domain.RoleExp))
		btnMid := menu.Text(string(domain.RoleMid))
		btnRoam := menu.Text(string(domain.RoleRoam))
		btnJungle := menu.Text(string(domain.RoleJungle))

		menu.Reply(
			menu.Row(btnGold, btnExp),
			menu.Row(btnMid, btnRoam, btnJungle),
		)
		return c.Send(responseMsg, menu)
	}

	return c.Send(responseMsg, &telebot.ReplyMarkup{RemoveKeyboard: true})
}

func (h *Handler) OnRegTeam(c telebot.Context) error {
	msg := h.uc.StartTeamRegistration(c.Sender().ID)
	return c.Send(msg)
}
