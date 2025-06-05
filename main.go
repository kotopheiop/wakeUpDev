package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/anatoliyfedorenko/isdayoff"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/joho/godotenv"
)

const N_WORKERS = 3 // Кол-во одновременных горутин отправки сообщений

type Reminder struct {
	Time    string `json:"time"`    // В формате HH:MM
	Message string `json:"message"` // HTML-сообщение
}

type ReminderJob struct {
	Reminder Reminder
	Bot      *tgbotapi.BotAPI
	ChatID   int64
	Loc      *time.Location
}

var loc *time.Location

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

func parseTime(timestr string) (time.Time, error) {
	now := time.Now().In(loc)
	var hour, minute int
	_, err := fmt.Sscanf(timestr, "%d:%d", &hour, &minute)
	if err != nil {
		return time.Time{}, err
	}
	t := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)
	if t.Before(now) {
		t = t.Add(24 * time.Hour)
	}
	return t, nil
}

func isWeekend() bool {
	now := time.Now().In(loc)

	dayOff := isdayoff.New()
	countryCode := isdayoff.CountryCodeRussia
	pre, covid := false, false
	year, month, day := now.Date()

	dayType, _ := dayOff.Today(isdayoff.Params{
		CountryCode: &countryCode,
		Pre:         &pre,
		Covid:       &covid,
		Year:        year,
		Month:       &month,
		Day:         &day,
	})

	return *dayType == isdayoff.DayTypeNonWorking
}

func scheduleReminderWorker(jobs <-chan ReminderJob) {
	for job := range jobs {
		r := job.Reminder
		timeToSend, err := parseTime(r.Time)
		if err != nil {
			log.Printf("⚠️ Ошибка времени в напоминании: %v", err)
			continue
		}
		dur := time.Until(timeToSend)
		log.Printf("⏳ [%s] через %s", minimizeString(r.Message, 20), dur.Round(time.Second))
		time.Sleep(dur)

		msg := tgbotapi.NewMessage(job.ChatID, r.Message)
		msg.ParseMode = "HTML"
		if _, err := job.Bot.Send(msg); err != nil {
			log.Printf("❌ Не удалось отправить: %v", err)
		} else {
			log.Printf("✅ Напоминание отправлено: \"%s\"", minimizeString(r.Message, 20))
		}
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env не найден, читаем переменные окружения напрямую")
	}

	botToken := mustEnv("BOT_TOKEN")
	groupChatID := mustEnv("GROUP_CHAT_ID")
	chatID := mustParseInt64(groupChatID)
	reminderPath := mustEnv("REMINDERS_FILE")
	timezone := mustEnv("TIMEZONE")

	var err error

	loc, err = time.LoadLocation(timezone)
	if err != nil {
		log.Fatalf("❌ Ошибка загрузки часового пояса %s: %v", timezone, err)
	}

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

		if isWeekend() {
			log.Println("🏖 Сегодня выходной. Напоминания не будут отправлены.")
		} else {
			type TimedReminder struct {
				Reminder Reminder
				When     time.Time
				Dur      time.Duration
			}
			var sorted []TimedReminder

			for _, r := range reminders {
				t, err := parseTime(r.Time)
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

			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].When.Before(sorted[j].When)
			})

			jobs := make(chan ReminderJob, len(sorted))

			for i := 0; i < N_WORKERS; i++ {
				go scheduleReminderWorker(jobs)
			}

			for _, tr := range sorted {
				jobs <- ReminderJob{
					Reminder: tr.Reminder,
					Bot:      bot,
					ChatID:   chatID,
					Loc:      loc,
				}
			}

			close(jobs)
			log.Printf("✅ Все %d напоминаний отправлены в пул", len(sorted))
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

func minimizeString(s string, max int) string {
	cleaned := strings.ReplaceAll(s, "\n", " ")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	cleaned = strings.ReplaceAll(cleaned, "\t", " ")
	runes := []rune(cleaned)
	if len(runes) <= max {
		return cleaned
	}
	return string(runes[:max]) + "..."
}
