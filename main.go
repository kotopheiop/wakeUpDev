package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/joho/godotenv"
	"github.com/kotopheiop/isdayoff"
	"github.com/robfig/cron/v3"
)

type Reminder struct {
	Time    string `json:"time"`    // В формате HH:MM
	Message string `json:"message"` // HTML-сообщение
}

var (
	loc    *time.Location
	dayOff *isdayoff.Client
)

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("⛔ Переменная окружения %s не установлена", key)
	}
	return val
}

func mustParseInt64(s string) int64 {
	var id int64
	_, err := fmt.Sscanf(s, "%d", &id)
	if err != nil {
		log.Fatalf("Неверный формат числа: %s", s)
	}
	return id
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

func isWeekend() bool {
	if dayOff == nil {
		dayOff = isdayoff.New()
	}

	now := time.Now().In(loc)
	countryCode := isdayoff.CountryCodeRussia
	pre, covid := false, false
	year, month, day := now.Date()
	params := isdayoff.Params{
		CountryCode: &countryCode,
		Pre:         &pre,
		Covid:       &covid,
		Year:        year,
		Month:       &month,
		Day:         &day,
	}
	dayType, err := dayOff.Today(params)

	if err != nil || dayType == nil {
		log.Printf("⚠️ Не удалось определить выходной: %v", err)
		return false // лучше не пропускать напоминание в случае ошибки
	}

	// DayType "0" - рабочий день, "1" - выходной день
	return *dayType == "1"
}

func truncate(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit])
}

func parseHourMinute(timeStr string) (hour, minute int, err error) {
	_, err = fmt.Sscanf(timeStr, "%d:%d", &hour, &minute)
	if err != nil {
		return
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		err = fmt.Errorf("некорректное время: %s", timeStr)
	}
	return
}

func createCronJob(c *cron.Cron, r Reminder, bot *tgbotapi.BotAPI, chatID int64) error {
	hour, minute, err := parseHourMinute(r.Time)
	if err != nil {
		return err
	}

	// Cron формат: MIN HOUR * * *
	cronExpr := fmt.Sprintf("%d %d * * *", minute, hour)

	handler := func() {
		if isWeekend() {
			log.Printf("🏖 [%s] Выходной день, напоминание пропущено", r.Time)
			return
		}

		msg := tgbotapi.NewMessage(chatID, r.Message)
		msg.ParseMode = "HTML"
		if _, err := bot.Send(msg); err != nil {
			log.Printf("❌ Не отправлено [%s]: %v", r.Time, err)
		} else {
			log.Printf("✅ Отправлено [%s]: %s", r.Time, truncate(r.Message, 20))
		}
	}

	entryID, err := c.AddFunc(cronExpr, handler)
	if err == nil {
		log.Printf("✅ Задача на время %s запланирована ID: [%d]", r.Time, entryID)
	}

	return err
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env не найден, читаем переменные напрямую")
	}

	botToken := mustEnv("BOT_TOKEN")
	groupChatID := mustEnv("GROUP_CHAT_ID")
	reminderPath := mustEnv("REMINDERS_FILE")
	timezone := mustEnv("TIMEZONE")

	chatID := mustParseInt64(groupChatID)

	var err error
	loc, err = time.LoadLocation(timezone)
	if err != nil {
		log.Fatalf("❌ Ошибка часового пояса: %v", err)
	}

	bot, err := newTelegramBot(botToken)
	if err != nil {
		log.Fatalf("❌ Ошибка инициализации бота: %v", err)
	}
	log.Printf("🤖 Бот запущен как @%s", bot.Self.UserName)

	reminders, err := loadReminders(reminderPath)
	if err != nil {
		log.Fatalf("❌ Не могу загрузить напоминания: %v", err)
	}
	if len(reminders) == 0 {
		log.Fatalf("❌ Список напоминаний пуст, нечего запускать")
	}

	c := cron.New(cron.WithLocation(loc))

	for _, r := range reminders {
		if err := createCronJob(c, r, bot, chatID); err != nil {
			log.Printf("⚠️ Ошибка добавления задачи [%s]: %v", r.Time, err)
		}
	}

	c.Start()
	log.Println("📅 Все сообщения запланированы на отправку")

	select {} // блокировка main потока
}
