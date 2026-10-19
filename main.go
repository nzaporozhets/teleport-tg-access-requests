package main

import (
	 "context"
  	 "os"
	 "os/signal"
	 "fmt"
	 "log"
	 "github.com/go-telegram/bot"
	 "github.com/go-telegram/bot/models"
)

//tmp plumbing to replace actual provider
var approvers = map[int64]bool{
	111111111: true,
	222222222: true,

}
var requestID = "01a154cd-9af0-71fd-8acf-e3d35ce28ce1"

func main() {

	cfg, err := loadConfig()
	if err != nil{
		log.Fatalf("Invalid config:\n%v",err)
	}
	fmt.Printf("Config successfully loaded\n")

	opts:= []bot.Option{
		bot.WithDefaultHandler(defaultHandler),
		bot.WithCallbackQueryDataHandler("request", bot.MatchTypePrefix, handleDecision),
	}
    
	b, err := bot.New(cfg.TgBotToken, opts...)
	if err != nil{
		log.Fatalf("Unable to connect to Telegram bot: \n%v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Send the keyboard message right away, in a goroutine so it
	// doesn't block b.Start (or just call it before Start; both work).
	tb := &TgBot{
		chatID:   cfg.TgChatId,
		rqID: requestID,
	}
	go tb.sendKeyboard(ctx, b)
	
	b.Start(ctx) // still needed to receive button clicks


}
func handleDecision(ctx context.Context, b *bot.Bot, update *models.Update){
	cb := update.CallbackQuery
	if cb.Message.Message == nil {
		return // ignore updates that aren't plain messages
	}
	if !checkUserTgTrait(cb.From.ID) {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "You're not allowed to do this.",
			ShowAlert:       true, // shows a popup only the clicker sees
		})
		fmt.Println("Rejected the decision from Telegram user with ID", cb.From.ID)
		return
	}
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cb.ID,
		ShowAlert:       false,
	})
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: cb.Message.Message.Chat.ID,
		Text:   "Selected action: " + cb.Data,
	})	
}

func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	fmt.Printf("Nothing to do ")
}

func checkUserTgTrait (tgUID int64) bool{
	if !approvers[tgUID]{
		return false
	}
	return true
}
