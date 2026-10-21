package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	//TG
	TgBotToken string
	TgChatId   int64
	//Teleport
	TeleportProxyUrl string
	//TeleportCreds
	//Optional
	TgTraitKey string
}

func loadConfig() (Config, error) {
	var errs []error
	required := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", key))
		}
		return v
	}
	requiredInt64 := func(key string) int64 {
		raw := required(key)
		if raw == "" {
			return 0
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s needs to be an integer: %w", key, err))
		}
		return n
	}
	optional := func(key string, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}

	cfg := Config{
		TgBotToken:       required("TELEGRAM_BOT_TOKEN"),
		TgChatId:         requiredInt64("TELEGRAM_CHAT_ID"),
		TeleportProxyUrl: required("TELEPORT_PROXY"),
		TgTraitKey:       optional("TG_TRAIT_KEY", "tg_uid"),
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	return cfg, nil

}
