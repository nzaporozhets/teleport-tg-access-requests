package main

import (
	 "context"
  	 "os"
	 "os/signal"
	 "fmt"
	 "log"
	 "github.com/go-telegram/bot"
)

func main() {

	cfg, err := loadConfig()
	if err != nil{
		log.Fatalf("Invalid config:\n%v",err)
	}
	fmt.Printf("Config successfully loaded\n")

	a := &AccessRequestPlugin{
		
	}

	opts:= []bot.Option{
		bot.WithDefaultHandler(a.defaultHandler),
		bot.WithCallbackQueryDataHandler("request", bot.MatchTypePrefix, a.handleDecision),
	}
    
	b, err := bot.New(cfg.TgBotToken, opts...)
	if err != nil{
		log.Fatalf("Unable to connect to Telegram bot: \n%v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	tb := &TgBot{
		chatID:   cfg.TgChatId,
		rqID: requestID,
	}
	
	go tb.sendKeyboard(ctx, b)
	
	b.Start(ctx) // still needed to receive button clicks


}

