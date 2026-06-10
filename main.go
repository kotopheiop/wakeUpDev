package main

import (
	"bytes"
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
	Time            string `json:"time"`                        // В формате HH:MM
	Message         string `json:"message"`                     // HTML-сообщение
	PreHolidayEarly bool   `json:"pre_holiday_early,omitempty"` // отправить на час раньше в предпраздничный день
	LastWorkingDay  bool   `json:"last_working_day,omitempty"`  // только в последний рабочий день месяца
}

type ChatConfig struct {
	Name      string     `json:"name,omitempty"` // подпись чата для конфига и логов (не влияет на отправку)
	ChatID    int64      `json:"chat_id"`
	Reminders []Reminder `json:"reminders"`
}

func chatLabel(chat ChatConfig) string {
	if chat.Name != "" {
		return fmt.Sprintf("%s (chat=%d)", chat.Name, chat.ChatID)
	}
	return fmt.Sprintf("chat=%d", chat.ChatID)
}

type Config struct {
	Chats []ChatConfig `json:"chats"`
}

type sendSlot int

const (
	sendSlotNormal sendSlot = iota
	sendSlotEarly
)

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

func loadConfig(path string, fallbackChatID int64) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var reminders []Reminder
		if err := json.Unmarshal(data, &reminders); err != nil {
			return Config{}, err
		}
		if fallbackChatID == 0 {
			return Config{}, fmt.Errorf("legacy-формат требует GROUP_CHAT_ID")
		}
		return Config{Chats: []ChatConfig{{ChatID: fallbackChatID, Reminders: reminders}}}, nil
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func dayOffParams() isdayoff.Params {
	countryCode := isdayoff.CountryCodeRussia
	pre, covid := true, false
	return isdayoff.Params{
		CountryCode: &countryCode,
		Pre:         &pre,
		Covid:       &covid,
	}
}

func ensureDayOffClient() {
	if dayOff == nil {
		dayOff = isdayoff.New()
	}
}

func getTodayDayType() (*isdayoff.DayType, error) {
	ensureDayOffClient()

	now := time.Now().In(loc)
	year, month, day := now.Date()
	d := day
	params := dayOffParams()
	params.Year = year
	params.Month = &month
	params.Day = &d

	return dayOff.Today(params)
}

func isNonWorkingDay() bool {
	dayType, err := getTodayDayType()
	if err != nil || dayType == nil {
		log.Printf("⚠️ Не удалось определить тип дня: %v", err)
		return false
	}
	return *dayType == isdayoff.DayTypeNonWorking
}

func isPreHoliday() bool {
	dayType, err := getTodayDayType()
	if err != nil || dayType == nil {
		log.Printf("⚠️ Не удалось определить предпраздничный день: %v", err)
		return false
	}
	return *dayType == isdayoff.DayTypeHalfHoliday
}

func isWorkingDayType(dt isdayoff.DayType) bool {
	return dt == isdayoff.DayTypeWorking ||
		dt == isdayoff.DayTypeHalfHoliday ||
		dt == isdayoff.DayTypeWorkingCovid
}

func isLastWorkingDayOfMonth() bool {
	ensureDayOffClient()

	now := time.Now().In(loc)
	year, month, today := now.Date()
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()

	m := month
	params := dayOffParams()
	params.Year = year
	params.Month = &m

	days, err := dayOff.GetBy(params)
	if err != nil || len(days) != lastDay {
		log.Printf("⚠️ Не удалось определить последний рабочий день месяца: %v", err)
		return false
	}

	lastWorkingDay := 0
	for i, dt := range days {
		if isWorkingDayType(dt) {
			lastWorkingDay = i + 1
		}
	}
	return today == lastWorkingDay
}

func matchPreHolidaySlot(preHolidayEarly bool, slot sendSlot, preHoliday bool) bool {
	if !preHolidayEarly {
		return slot == sendSlotNormal
	}
	if slot == sendSlotEarly {
		return preHoliday
	}
	return !preHoliday
}

func shouldSend(r Reminder, slot sendSlot) bool {
	if isNonWorkingDay() {
		return false
	}
	if r.LastWorkingDay && !isLastWorkingDayOfMonth() {
		return false
	}
	return matchPreHolidaySlot(r.PreHolidayEarly, slot, isPreHoliday())
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

func subtractHour(hour, minute int) (int, int) {
	t := time.Date(2000, 1, 1, hour, minute, 0, 0, time.UTC).Add(-time.Hour)
	return t.Hour(), t.Minute()
}

func addCronJob(c *cron.Cron, hour, minute int, r Reminder, bot *tgbotapi.BotAPI, chatID int64, slot sendSlot) error {
	cronExpr := fmt.Sprintf("%d %d * * *", minute, hour)
	timeLabel := fmt.Sprintf("%02d:%02d", hour, minute)

	handler := func() {
		if !shouldSend(r, slot) {
			reason := "условие не выполнено"
			if isNonWorkingDay() {
				reason = "выходной день"
			} else if r.LastWorkingDay && !isLastWorkingDayOfMonth() {
				reason = "не последний рабочий день месяца"
			} else if r.PreHolidayEarly && slot == sendSlotEarly && !isPreHoliday() {
				reason = "не предпраздничный день (ранний слот)"
			} else if r.PreHolidayEarly && slot == sendSlotNormal && isPreHoliday() {
				reason = "предпраздничный день (отправка на час раньше)"
			}
			log.Printf("🏖 [%s chat=%d] Пропущено (%s)", timeLabel, chatID, reason)
			return
		}

		msg := tgbotapi.NewMessage(chatID, r.Message)
		msg.ParseMode = "HTML"
		if _, err := bot.Send(msg); err != nil {
			log.Printf("❌ Не отправлено [%s chat=%d]: %v", timeLabel, chatID, err)
		} else {
			log.Printf("✅ Отправлено [%s chat=%d]: %s", timeLabel, chatID, truncate(r.Message, 20))
		}
	}

	entryID, err := c.AddFunc(cronExpr, handler)
	if err == nil {
		log.Printf("✅ Задача [%s chat=%d] запланирована ID: [%d]", timeLabel, chatID, entryID)
	}
	return err
}

func createCronJobs(c *cron.Cron, r Reminder, bot *tgbotapi.BotAPI, chatID int64) error {
	hour, minute, err := parseHourMinute(r.Time)
	if err != nil {
		return err
	}

	if r.PreHolidayEarly {
		earlyHour, earlyMinute := subtractHour(hour, minute)
		if err := addCronJob(c, earlyHour, earlyMinute, r, bot, chatID, sendSlotEarly); err != nil {
			return err
		}
	}
	return addCronJob(c, hour, minute, r, bot, chatID, sendSlotNormal)
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env не найден, читаем переменные напрямую")
	}

	botToken := mustEnv("BOT_TOKEN")
	reminderPath := mustEnv("REMINDERS_FILE")
	timezone := mustEnv("TIMEZONE")

	var fallbackChatID int64
	if groupChatID := os.Getenv("GROUP_CHAT_ID"); groupChatID != "" {
		fallbackChatID = mustParseInt64(groupChatID)
	}

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

	cfg, err := loadConfig(reminderPath, fallbackChatID)
	if err != nil {
		log.Fatalf("❌ Не могу загрузить конфигурацию: %v", err)
	}
	if len(cfg.Chats) == 0 {
		log.Fatalf("❌ Список чатов пуст, нечего запускать")
	}

	c := cron.New(cron.WithLocation(loc))
	totalJobs := 0

	for _, chat := range cfg.Chats {
		if chat.ChatID == 0 {
			log.Fatalf("❌ chat_id не задан в конфигурации")
		}
		if len(chat.Reminders) == 0 {
			log.Printf("⚠️ %s: нет напоминаний", chatLabel(chat))
			continue
		}
		log.Printf("📬 %s: %d напоминаний", chatLabel(chat), len(chat.Reminders))
		for _, r := range chat.Reminders {
			if err := createCronJobs(c, r, bot, chat.ChatID); err != nil {
				log.Printf("⚠️ Ошибка добавления задачи [%s %s]: %v", r.Time, chatLabel(chat), err)
			} else {
				totalJobs++
			}
		}
	}

	if totalJobs == 0 {
		log.Fatalf("❌ Не удалось запланировать ни одного напоминания")
	}

	c.Start()
	log.Printf("📅 Запланировано %d напоминаний в %d чат(ах)", totalJobs, len(cfg.Chats))

	select {}
}
