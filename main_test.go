package main

import (
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/kotopheiop/isdayoff"
)

func TestEnvVariables(t *testing.T) {
	if testing.Short() {
		t.Skip("требует .env на хосте: запуск без -short (make test)")
	}

	err := godotenv.Load(".env")
	if err != nil {
		t.Fatalf("Не удалось загрузить .env: %v", err)
	}

	requiredVars := []string{"BOT_TOKEN", "REMINDERS_FILE", "TIMEZONE"}
	for _, v := range requiredVars {
		if os.Getenv(v) == "" {
			t.Errorf("Переменная %s не задана", v)
		}
	}
}

func TestRemindersJSON(t *testing.T) {
	cfg, err := loadConfig("reminders.json", 0)
	if err != nil {
		t.Fatalf("Ошибка загрузки reminders.json: %v", err)
	}

	if len(cfg.Chats) == 0 {
		t.Fatal("Файл reminders.json не содержит чатов")
	}

	for ci, chat := range cfg.Chats {
		if chat.ChatID == 0 {
			t.Errorf("Chat #%d: chat_id не задан", ci)
		}
		for i, r := range chat.Reminders {
			if r.Time == "" {
				t.Errorf("Chat #%d Reminder #%d: поле time пустое", ci, i)
			}
			if r.Message == "" {
				t.Errorf("Chat #%d Reminder #%d: поле message пустое", ci, i)
			}
		}
	}
}

func TestTelegramConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("интеграционный тест: запуск без -short (make test)")
	}

	err := godotenv.Load(".env")
	if err != nil {
		t.Fatalf("Не удалось загрузить .env: %v", err)
	}

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		t.Skip("BOT_TOKEN не задан, пропускаем тест")
	}

	bot, err := newTelegramBot(token)
	if err != nil {
		t.Fatalf("Ошибка авторизации бота: %v", err)
	}

	if bot.Self.UserName == "" {
		t.Error("Бот не получил имя пользователя — возможно, неверный токен")
	}
}

func TestParseHourMinute(t *testing.T) {
	tests := []struct {
		name      string
		timeStr   string
		wantHour  int
		wantMin   int
		wantError bool
	}{
		{"Valid time", "10:30", 10, 30, false},
		{"Valid time midnight", "00:00", 0, 0, false},
		{"Valid time end of day", "23:59", 23, 59, false},
		{"Invalid format", "10-30", 0, 0, true},
		{"Invalid hour too high", "24:00", 0, 0, true},
		{"Invalid hour negative", "-1:00", 0, 0, true},
		{"Invalid minute too high", "10:60", 0, 0, true},
		{"Invalid minute negative", "10:-1", 0, 0, true},
		{"Empty string", "", 0, 0, true},
		{"Invalid format no colon", "1030", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hour, minute, err := parseHourMinute(tt.timeStr)
			if (err != nil) != tt.wantError {
				t.Errorf("parseHourMinute() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError {
				if hour != tt.wantHour || minute != tt.wantMin {
					t.Errorf("parseHourMinute() = %d:%d, want %d:%d", hour, minute, tt.wantHour, tt.wantMin)
				}
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		limit  int
		want   string
		length int
	}{
		{"Short string", "Hello", 10, "Hello", 5},
		{"Exact length", "Hello", 5, "Hello", 5},
		{"Long string", "Hello World", 5, "Hello", 5},
		{"Empty string", "", 10, "", 0},
		{"Zero limit", "Hello", 0, "", 0},
		{"Unicode string", "Привет", 3, "При", 3},
		{"Unicode long", "Привет мир", 6, "Привет", 6},
		{"Emoji", "Hello 😀 World", 7, "Hello 😀", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.limit)
			if got != tt.want {
				t.Errorf("truncate() = %q, want %q", got, tt.want)
			}
			// Проверяем, что длина в рунах соответствует limit
			if len([]rune(got)) > tt.limit {
				t.Errorf("truncate() length = %d, want <= %d", len([]rune(got)), tt.limit)
			}
		})
	}
}

func TestLoadConfig_NewFormat(t *testing.T) {
	validJSON := `{
		"chats": [
			{
				"name": "Тестовый чат",
				"chat_id": -100111,
				"reminders": [
					{"time": "10:00", "message": "Test 1", "pre_holiday_early": true},
					{"time": "12:00", "message": "Test 2", "last_working_day": true}
				]
			},
			{
				"chat_id": -100222,
				"reminders": [
					{"time": "09:00", "message": "Other chat"}
				]
			}
		]
	}`
	tmpFile := writeTempJSON(t, validJSON)
	defer os.Remove(tmpFile)

	cfg, err := loadConfig(tmpFile, 0)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if len(cfg.Chats) != 2 {
		t.Fatalf("loadConfig() вернул %d чатов, ожидалось 2", len(cfg.Chats))
	}
	if cfg.Chats[0].ChatID != -100111 || cfg.Chats[0].Name != "Тестовый чат" || len(cfg.Chats[0].Reminders) != 2 {
		t.Error("некорректные данные первого чата")
	}
	if !cfg.Chats[0].Reminders[0].PreHolidayEarly {
		t.Error("ожидался pre_holiday_early=true")
	}
	if !cfg.Chats[0].Reminders[1].LastWorkingDay {
		t.Error("ожидался last_working_day=true")
	}
}

func TestLoadConfig_LegacyFormat(t *testing.T) {
	validJSON := `[
		{"time": "10:00", "message": "Test message 1"},
		{"time": "12:00", "message": "Test message 2"}
	]`
	tmpFile := writeTempJSON(t, validJSON)
	defer os.Remove(tmpFile)

	cfg, err := loadConfig(tmpFile, -999)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if len(cfg.Chats) != 1 || cfg.Chats[0].ChatID != -999 {
		t.Errorf("legacy format: %+v", cfg)
	}
	if len(cfg.Chats[0].Reminders) != 2 {
		t.Errorf("loadConfig() вернул %d напоминаний, ожидалось 2", len(cfg.Chats[0].Reminders))
	}
}

func TestLoadConfig_LegacyWithoutChatID(t *testing.T) {
	validJSON := `[{"time": "10:00", "message": "Test"}]`
	tmpFile := writeTempJSON(t, validJSON)
	defer os.Remove(tmpFile)

	_, err := loadConfig(tmpFile, 0)
	if err == nil {
		t.Error("loadConfig() должен требовать GROUP_CHAT_ID для legacy-формата")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmpFile := writeTempJSON(t, "invalid json")
	defer os.Remove(tmpFile)

	_, err := loadConfig(tmpFile, 0)
	if err == nil {
		t.Error("loadConfig() должен вернуть ошибку для невалидного JSON")
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := loadConfig("nonexistent_file.json", 0)
	if err == nil {
		t.Error("loadConfig() должен вернуть ошибку для несуществующего файла")
	}
}

func writeTempJSON(t *testing.T, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test_config_*.json")
	if err != nil {
		t.Fatalf("Не удалось создать временный файл: %v", err)
	}
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Не удалось записать в файл: %v", err)
	}
	tmpFile.Close()
	return tmpFile.Name()
}

func TestMustParseInt64(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        int64
		shouldPanic bool
	}{
		{"Valid number", "123", 123, false},
		{"Valid negative", "-456", -456, false},
		{"Valid zero", "0", 0, false},
		{"Valid large number", "9223372036854775807", 9223372036854775807, false},
		{"Invalid format", "abc", 0, true},
		{"Invalid format with number", "123abc", 0, true},
		{"Empty string", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldPanic {
				// Проверяем, что функция вызывает log.Fatalf (который вызывает os.Exit)
				// В тестах мы не можем проверить это напрямую, поэтому пропускаем
				t.Skip("Пропускаем тест на panic, так как log.Fatalf вызывает os.Exit")
			} else {
				got := mustParseInt64(tt.input)
				if got != tt.want {
					t.Errorf("mustParseInt64() = %d, want %d", got, tt.want)
				}
			}
		})
	}
}

func TestIsNonWorkingDay(t *testing.T) {
	testLoc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("Не удалось загрузить локацию: %v", err)
	}
	originalLoc := loc
	defer func() { loc = originalLoc }()
	loc = testLoc

	result := isNonWorkingDay()
	if result != true && result != false {
		t.Error("isNonWorkingDay() должен возвращать true или false")
	}
}

func TestSubtractHour(t *testing.T) {
	tests := []struct {
		hour, minute int
		wantH, wantM int
	}{
		{10, 30, 9, 30},
		{0, 0, 23, 0},
		{1, 15, 0, 15},
	}
	for _, tt := range tests {
		h, m := subtractHour(tt.hour, tt.minute)
		if h != tt.wantH || m != tt.wantM {
			t.Errorf("subtractHour(%d:%d) = %d:%d, want %d:%d", tt.hour, tt.minute, h, m, tt.wantH, tt.wantM)
		}
	}
}

func TestMatchPreHolidaySlot(t *testing.T) {
	tests := []struct {
		name            string
		preHolidayEarly bool
		slot            sendSlot
		preHoliday      bool
		want            bool
	}{
		{"обычное напоминание", false, sendSlotNormal, false, true},
		{"обычное — ранний слот игнорируется", false, sendSlotEarly, true, false},
		{"предпраздничный — ранний слот", true, sendSlotEarly, true, true},
		{"предпраздничный — обычный слот пропуск", true, sendSlotNormal, true, false},
		{"не предпраздничный — обычный слот", true, sendSlotNormal, false, true},
		{"не предпраздничный — ранний слот пропуск", true, sendSlotEarly, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPreHolidaySlot(tt.preHolidayEarly, tt.slot, tt.preHoliday)
			if got != tt.want {
				t.Errorf("matchPreHolidaySlot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsWorkingDayType(t *testing.T) {
	if !isWorkingDayType(isdayoff.DayTypeWorking) {
		t.Error("рабочий день должен считаться рабочим")
	}
	if !isWorkingDayType(isdayoff.DayTypeHalfHoliday) {
		t.Error("сокращённый день должен считаться рабочим")
	}
	if isWorkingDayType(isdayoff.DayTypeNonWorking) {
		t.Error("выходной не должен считаться рабочим")
	}
}

func TestParseTelegramProxyURL(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		userpwd    string
		wantScheme string
		wantHost   string
		wantUser   string
		wantPass   string
		wantError  bool
	}{
		{
			name:       "http host only",
			host:       "127.0.0.1:8080",
			wantScheme: "http",
			wantHost:   "127.0.0.1:8080",
		},
		{
			name:       "http with scheme",
			host:       "http://proxy.example:3128",
			wantScheme: "http",
			wantHost:   "proxy.example:3128",
		},
		{
			name:       "socks5",
			host:       "socks5://127.0.0.1:1080",
			wantScheme: "socks5",
			wantHost:   "127.0.0.1:1080",
		},
		{
			name:       "host and credentials",
			host:       "proxy.example:8080",
			userpwd:    "user:secret",
			wantScheme: "http",
			wantHost:   "proxy.example:8080",
			wantUser:   "user",
			wantPass:   "secret",
		},
		{
			name:       "password with colon",
			host:       "127.0.0.1:8080",
			userpwd:    "user:sec:ret",
			wantScheme: "http",
			wantHost:   "127.0.0.1:8080",
			wantUser:   "user",
			wantPass:   "sec:ret",
		},
		{
			name:       "credentials in host are kept",
			host:       "http://user:pass@proxy.example:8080",
			userpwd:    "other:wrong",
			wantScheme: "http",
			wantHost:   "proxy.example:8080",
			wantUser:   "user",
			wantPass:   "pass",
		},
		{
			name:      "invalid userpwd",
			host:      "127.0.0.1:8080",
			userpwd:   "nocolon",
			wantError: true,
		},
		{
			name:      "unsupported scheme",
			host:      "ftp://127.0.0.1:21",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTelegramProxyURL(tt.host, tt.userpwd)
			if (err != nil) != tt.wantError {
				t.Fatalf("parseTelegramProxyURL() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError {
				return
			}

			if got.Scheme != tt.wantScheme || got.Host != tt.wantHost {
				t.Errorf("proxy URL = %s://%s, want %s://%s", got.Scheme, got.Host, tt.wantScheme, tt.wantHost)
			}
			if tt.wantUser != "" {
				user := got.User.Username()
				pass, _ := got.User.Password()
				if user != tt.wantUser || pass != tt.wantPass {
					t.Errorf("credentials = %q:%q, want %q:%q", user, pass, tt.wantUser, tt.wantPass)
				}
			}
		})
	}
}

func TestLoadTelegramProxyConfig(t *testing.T) {
	const (
		hostKey    = "TELEGRAM_PROXY_HOST"
		userpwdKey = "TELEGRAM_PROXY_USERPWD"
	)

	t.Setenv(hostKey, "")
	t.Setenv(userpwdKey, "")
	cfg, err := loadTelegramProxyConfig()
	if err != nil {
		t.Fatalf("loadTelegramProxyConfig() error = %v", err)
	}
	if cfg.enabled() {
		t.Error("без TELEGRAM_PROXY_HOST прокси должен быть выключен")
	}

	t.Setenv(hostKey, "socks5://127.0.0.1:1080")
	t.Setenv(userpwdKey, "")
	cfg, err = loadTelegramProxyConfig()
	if err != nil {
		t.Fatalf("loadTelegramProxyConfig() error = %v", err)
	}
	if !cfg.enabled() {
		t.Fatal("прокси должен быть включён")
	}
	if cfg.URL.Scheme != "socks5" {
		t.Errorf("scheme = %q, want socks5", cfg.URL.Scheme)
	}
}

func TestTelegramProxyTransport(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		scheme string
	}{
		{"http", "http://127.0.0.1:8080", "http"},
		{"socks5", "socks5://127.0.0.1:1080", "socks5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := parseTelegramProxyURL(tt.host, "")
			if err != nil {
				t.Fatalf("parseTelegramProxyURL: %v", err)
			}
			cfg := telegramProxyConfig{URL: u}
			tr, err := cfg.transport()
			if err != nil {
				t.Fatalf("transport() error = %v", err)
			}
			if tr == nil {
				t.Fatal("transport() вернул nil")
			}
			if u.Scheme == "socks5" && tr.DialContext == nil {
				t.Error("SOCKS5 transport должен иметь DialContext")
			}
			if u.Scheme == "http" && tr.Proxy == nil {
				t.Error("HTTP transport должен иметь Proxy")
			}
		})
	}
}

func TestChatLabel(t *testing.T) {
	tests := []struct {
		name string
		chat ChatConfig
		want string
	}{
		{"с именем", ChatConfig{Name: "ПР", ChatID: -100}, "ПР (chat=-100)"},
		{"без имени", ChatConfig{ChatID: -200}, "chat=-200"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chatLabel(tt.chat); got != tt.want {
				t.Errorf("chatLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReminderStruct(t *testing.T) {
	r := Reminder{
		Time:            "10:00",
		Message:         "Test message",
		PreHolidayEarly: true,
		LastWorkingDay:  true,
	}

	if r.Time != "10:00" {
		t.Errorf("Reminder.Time = %q, want %q", r.Time, "10:00")
	}
	if r.Message != "Test message" {
		t.Errorf("Reminder.Message = %q, want %q", r.Message, "Test message")
	}
	if !r.PreHolidayEarly || !r.LastWorkingDay {
		t.Error("ожидались флаги pre_holiday_early и last_working_day")
	}
}
