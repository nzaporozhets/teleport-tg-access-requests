package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/go-telegram/bot"
	"github.com/gravitational/teleport/api/client"
	"google.golang.org/grpc"
)

func main() {

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Invalid config:\n%v", err)
	}
	fmt.Printf("Config successfully loaded\n")

	//TODO parameterize from cfg
	creds := client.LoadIdentityFile("identity")

	teleport, err := client.New(ctx, client.Config{
		Addrs:       []string{cfg.TeleportProxyUrl},
		Credentials: []client.Credentials{creds},
		DialOpts: []grpc.DialOption{
			grpc.WithReturnConnectionError(),
		},
	})
	if err != nil {
		panic(err)
	}
	defer teleport.Close()

	plugin := &AccessRequestPlugin{
		cfg:            cfg,
		TeleportClient: teleport,
		tb:             &TgBot{chatID: cfg.TgChatId},
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(plugin.defaultHandler),
		bot.WithCallbackQueryDataHandler("request", bot.MatchTypePrefix, plugin.handleDecision),
	}

	b, err := bot.New(cfg.TgBotToken, opts...)
	if err != nil {
		log.Fatalf("Unable to connect to Telegram bot: \n%v", err)
	}

	plugin.bot = b

	go b.Start(ctx) // still needed to receive button clicks

	if err := plugin.Run(ctx); err != nil {
		panic(err)
	}

}
