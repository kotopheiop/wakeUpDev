package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/joho/godotenv"
)

type Reminder struct {
	Time    string `json:"time"`    // В формате HH:MM
	Message string `json:"message"` // HTML-сообщение
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("⛔ Переменная окружения %s не установлена", key)
	}
	return val
}

func loadReminders(path string) ([]Reminder, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var reminders []Reminder
	if err := json.Unmarshal(data, &reminders); err != nil {
		return nil, err
	}
	return reminders, nil
}

func parseTimeMoscow(timestr string) (time.Time, error) {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return time.Time{}, err
	}
	now := time.Now().In(loc)
	parts := strings.Split(timestr, ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("некорректный формат времени: %s", timestr)
	}
	var hour, min int
	_, err = fmt.Sscanf(timestr, "%d:%d", &hour, &min)
	if err != nil {
		return time.Time{}, err
	}
	t := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, loc)
	if t.Before(now) {
		t = t.Add(24 * time.Hour)
	}
	return t, nil
}

func isWeekend(t time.Weekday) bool {
	return t == time.Saturday || t == time.Sunday
}

func scheduleReminder(bot *tgbotapi.BotAPI, chatID int64, r Reminder, loc *time.Location) {
	timeToSend, err := parseTimeMoscow(r.Time)
	if err != nil {
		log.Printf("⚠️ Ошибка времени в напоминании: %v", err)
		return
	}
	dur := time.Until(timeToSend)
	log.Printf("⏳ Напоминание \"%s\" через %s (%s)", r.Message, dur.Round(time.Second), timeToSend.Format(time.RFC822))
	time.Sleep(dur)

	msg := tgbotapi.NewMessage(chatID, r.Message)
	msg.ParseMode = "HTML"
	if _, err := bot.Send(msg); err != nil {
		log.Printf("❌ Не удалось отправить: %v", err)
	} else {
		log.Printf("✅ Напоминание отправлено: \"%s\"", r.Message)
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️ .env не найден, читаем переменные окружения напрямую")
	}

	botToken := mustEnv("BOT_TOKEN")
	groupChatID := mustEnv("GROUP_CHAT_ID")
	chatID := mustParseInt64(groupChatID)
	reminderPath := mustEnv("REMINDERS_FILE") // например reminders.json

	loc, _ := time.LoadLocation("Europe/Moscow")
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("❌ Ошибка инициализации бота: %v", err)
	}
	log.Printf("✅ Бот запущен как @%s", bot.Self.UserName)

	for {
		reminders, err := loadReminders(reminderPath)
		if err != nil {
			log.Fatalf("❌ Не удалось загрузить напоминания: %v", err)
		}

		weekday := time.Now().In(loc).Weekday()
		if isWeekend(weekday) {
			log.Println("🏖 Сегодня выходной. Напоминания не будут отправлены.")
		} else {
			type TimedReminder struct {
				Reminder Reminder
				When     time.Time
				Dur      time.Duration
			}
			var sorted []TimedReminder

			for _, r := range reminders {
				t, err := parseTimeMoscow(r.Time)
				if err != nil {
					log.Printf("⚠️ Ошибка времени: %v", err)
					continue
				}
				sorted = append(sorted, TimedReminder{
					Reminder: r,
					When:     t,
					Dur:      time.Until(t),
				})
			}

			// Сортировка по времени отправки
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].When.Before(sorted[j].When)
			})

			for _, tr := range sorted {
				log.Printf("⏳ Напоминание \"%s\" через %s (%s)", tr.Reminder.Message, tr.Dur.Round(time.Second), tr.When.Format(time.RFC822))
				go scheduleReminder(bot, chatID, tr.Reminder, loc)
			}

			log.Printf("✅ Все %d напоминаний запланированы", len(sorted))
		}

		now := time.Now().In(loc)
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 1, 0, 0, loc)
		sleepDur := time.Until(next)
		log.Printf("😴 Бот уходит в сон до %s (через %s)", next.Format(time.RFC822), sleepDur.Round(time.Second))
		time.Sleep(sleepDur)
	}

}

func mustParseInt64(s string) int64 {
	var id int64
	_, err := fmt.Sscanf(s, "%d", &id)
	if err != nil {
		log.Fatalf("Неверный формат числа: %s", s)
	}
	return id
}
