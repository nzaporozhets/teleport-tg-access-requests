package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/gravitational/teleport/api/types"
)

type TgBot struct {
	chatID int64
}

func (tb *TgBot) sendKeyboard(ctx context.Context, b *bot.Bot, r types.AccessRequest) {
	var rqReason string
	if rqReason = r.GetRequestReason(); rqReason == "" {
		rqReason = "[Not provided]"
	}
	rqText := fmt.Sprintf("New access request:\n User: %s \n Requested roles: %s \n Reason: %s \n ID: %s", r.GetUser(), strings.Join(r.GetRoles(), ", "), rqReason, r.GetName())
	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Approve", CallbackData: approveButtonID + ":" + r.GetName()},
				{Text: "❌ Deny", CallbackData: denyButtonID + ":" + r.GetName()},
			},
		},
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      tb.chatID,
		Text:        rqText,
		ReplyMarkup: kb,
	})
	if err != nil {
		// handle/log properly in real code
		println("send failed:", err.Error())
	}
}
