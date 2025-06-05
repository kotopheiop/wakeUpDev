package main

import (
	"encoding/json"
	"os"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/joho/godotenv"
)

func TestEnvVariables(t *testing.T) {
	err := godotenv.Load(".env")
	if err != nil {
		t.Fatalf("Не удалось загрузить .env: %v", err)
	}

	requiredVars := []string{"BOT_TOKEN", "GROUP_CHAT_ID", "TIMEZONE"}
	for _, v := range requiredVars {
		if os.Getenv(v) == "" {
			t.Errorf("Переменная %s не задана", v)
		}
	}
}

func TestRemindersJSON(t *testing.T) {
	data, err := os.ReadFile("reminders.json")
	if err != nil {
		t.Fatalf("Не удалось прочитать reminders.json: %v", err)
	}

	var reminders []Reminder
	if err := json.Unmarshal(data, &reminders); err != nil {
		t.Fatalf("Ошибка парсинга reminders.json: %v", err)
	}

	if len(reminders) == 0 {
		t.Error("Файл reminders.json пуст")
	}

	for i, r := range reminders {
		if r.Time == "" {
			t.Errorf("Reminder #%d: поле time пустое", i)
		}
		if r.Message == "" {
			t.Errorf("Reminder #%d: поле message пустое", i)
		}
	}
}

func TestTelegramConnection(t *testing.T) {
	err := godotenv.Load(".env")
	if err != nil {
		t.Fatalf("Не удалось загрузить .env: %v", err)
	}

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		t.Fatal("BOT_TOKEN не задан")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		t.Fatalf("Ошибка авторизации бота: %v", err)
	}

	if bot.Self.UserName == "" {
		t.Error("Бот не получил имя пользователя — возможно, неверный токен")
	}
}
