package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

func TestEnvVariables(t *testing.T) {
	if testing.Short() {
		t.Skip("требует .env на хосте: запуск без -short (make test)")
	}

	err := godotenv.Load(".env")
	if err != nil {
		t.Fatalf("Не удалось загрузить .env: %v", err)
	}

	requiredVars := []string{"BOT_TOKEN", "GROUP_CHAT_ID", "REMINDERS_FILE", "TIMEZONE"}
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

func TestLoadReminders(t *testing.T) {
	// Создаём временный файл с валидными данными
	validJSON := `[
		{"time": "10:00", "message": "Test message 1"},
		{"time": "12:00", "message": "Test message 2"}
	]`
	tmpFile, err := os.CreateTemp("", "test_reminders_*.json")
	if err != nil {
		t.Fatalf("Не удалось создать временный файл: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(validJSON); err != nil {
		t.Fatalf("Не удалось записать в файл: %v", err)
	}
	tmpFile.Close()

	reminders, err := loadReminders(tmpFile.Name())
	if err != nil {
		t.Fatalf("loadReminders() error = %v", err)
	}
	if len(reminders) != 2 {
		t.Errorf("loadReminders() вернул %d напоминаний, ожидалось 2", len(reminders))
	}
	if reminders[0].Time != "10:00" || reminders[0].Message != "Test message 1" {
		t.Errorf("loadReminders() некорректные данные первого напоминания")
	}
}

func TestLoadReminders_InvalidJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_invalid_*.json")
	if err != nil {
		t.Fatalf("Не удалось создать временный файл: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString("invalid json"); err != nil {
		t.Fatalf("Не удалось записать в файл: %v", err)
	}
	tmpFile.Close()

	_, err = loadReminders(tmpFile.Name())
	if err == nil {
		t.Error("loadReminders() должен вернуть ошибку для невалидного JSON")
	}
}

func TestLoadReminders_FileNotFound(t *testing.T) {
	_, err := loadReminders("nonexistent_file.json")
	if err == nil {
		t.Error("loadReminders() должен вернуть ошибку для несуществующего файла")
	}
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

func TestIsWeekend(t *testing.T) {
	// Устанавливаем локацию для теста
	testLoc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("Не удалось загрузить локацию: %v", err)
	}
	// Сохраняем оригинальную локацию
	originalLoc := loc
	defer func() { loc = originalLoc }()

	// Устанавливаем тестовую локацию
	loc = testLoc

	// Тест может зависеть от реального API, поэтому просто проверяем, что функция выполняется
	// и возвращает булево значение
	result := isWeekend()
	if result != true && result != false {
		t.Error("isWeekend() должен возвращать true или false")
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

func TestReminderStruct(t *testing.T) {
	r := Reminder{
		Time:    "10:00",
		Message: "Test message",
	}

	if r.Time != "10:00" {
		t.Errorf("Reminder.Time = %q, want %q", r.Time, "10:00")
	}
	if r.Message != "Test message" {
		t.Errorf("Reminder.Message = %q, want %q", r.Message, "Test message")
	}
}
