package main

import (
	"os"
	"strconv"
	"fmt"
	"errors"
)

type Config struct {
	//TG
	TgBotToken string
	TgChatId int64
	//Teleport
	//TeleportProxyUrl string
	//TeleportCreds
}

func loadConfig() (Config, error){
	var errs [ ]error
	required := func (key string) string{
		v:= os.Getenv(key)	
		if v == "" {
			errs = append (errs, fmt.Errorf("%s is required", key))
		}
		return v
	}
	requiredInt64 := func (key string) int64{
		raw := required(key)
		if raw == ""{
			return 0
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil{
			errs = append (errs, fmt.Errorf("%s needs to be an integer: %w", key, err))
		} 
		return n
	}
	
	cfg := Config{
		TgBotToken: required("TELEGRAM_BOT_TOKEN"),
		TgChatId: requiredInt64("TELEGRAM_CHAT_ID"),
	}
	if err := errors.Join(errs...); err != nil{
		return Config{}, err
	}
	return cfg, nil

}