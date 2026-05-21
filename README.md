# wakeUpDev

![Go Version](https://img.shields.io/badge/Go-1.25-blue.svg)
![Test Coverage](https://img.shields.io/badge/coverage-37.8%25-yellow.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)

⏰ Telegram-бот, который отправляет **одно или несколько напоминаний** в группу разработчиков(или любую другую 🫡) в
заданное время по указанному часовому поясу

## 💡 Возможности
✅ Поддерживает:

- ⏰ Одно или несколько сообщений по расписанию
- 🗂 Настройку расписания через `reminders.json`
- 📝 HTML-форматирование в сообщениях
- 📅 Пропуск выходных дней автоматически
- ⚙️ Запуск в фоне или в активном режиме
- 🧾 Ведение логов в `./logs/wakeUpDev.log`
- 🐳 Возможность запуска в Docker контейнере
- 🔒 Прокси для Telegram API (HTTP/HTTPS, SOCKS5)

## 📦 Пример конфига `reminders.json`

```json
[
  {
    "time": "10:00",
    "message": "☕ <b>Доброе утро, команда!</b>"
  },
  {
    "time": "13:00",
    "message": "🍝 Пора обедать, не забывайте отдыхать!"
  },
  {
    "time": "18:50",
    "message": "📌 Запушьте изменения перед завершением дня!"
  }
]
```

## ⚙️ Быстрый старт

1. Склонируйте репозиторий и создайте `.env`:

   ```bash
   make .env
   ```

   Пример `.env`:

   ```env
   BOT_TOKEN=your_telegram_bot_token
   REMINDERS_FILE=reminders.json
   GROUP_CHAT_ID=-1001234567890
   TIMEZONE="Europe/Moscow"
   ```

   **Прокси (опционально)** — если Telegram API недоступен напрямую, задайте `TELEGRAM_PROXY_HOST`.
   Прокси включается автоматически, когда этот ключ не пустой. Логин и пароль — в `TELEGRAM_PROXY_USERPWD`
   (формат `user:password`, как в cURL).

   | Переменная | Обязательна | Описание |
   |------------|-------------|----------|
   | `TELEGRAM_PROXY_HOST` | Нет | Адрес прокси. Без схемы подразумевается HTTP (`host:port` или `http://host:port`). Поддерживаются `http`, `https`, `socks5`, `socks5h` |
   | `TELEGRAM_PROXY_USERPWD` | Нет | Учётные данные `user:password`. Не нужна, если логин уже в URL (`http://user:pass@host:port`) |

   HTTP-прокси:

   ```env
   TELEGRAM_PROXY_HOST=127.0.0.1:8080
   TELEGRAM_PROXY_USERPWD=myuser:mypassword
   ```

   SOCKS5:

   ```env
   TELEGRAM_PROXY_HOST=socks5://127.0.0.1:1080
   TELEGRAM_PROXY_USERPWD=myuser:mypassword
   ```

   Учётные данные в URL (тогда `TELEGRAM_PROXY_USERPWD` можно не задавать):

   ```env
   TELEGRAM_PROXY_HOST=http://myuser:mypassword@127.0.0.1:8080
   ```

2. Создайте файл `reminders.json` с нужным расписанием.

3. Запустите бота:

▶️ **В Docker контейнере (если Go не установлен, но есть Docker)**:

   ```bash
   make docker-up
   ```

▶️ **На локальной машине (в фоне, лог сохраняется в файл)**:

   ```bash
   make start
   ```

▶️ **На локальной машине (в терминале, лог в stdout)**:

   ```bash
   make run
   ```

4. Убедитесь, что бот добавлен в Telegram-группу и имеет право отправлять сообщения.

---

## 🛠 Команды Makefile

| Команда              | Описание                                                       |
|----------------------|----------------------------------------------------------------|
| `make .env`          | Создание `.env` из `.env.example`                              |
| `make test`          | Все тесты, включая проверку Telegram API                       |
| `make test-short`    | Только unit-тесты (без сети, как при `make build` / Docker)    |
| `make build`         | `test-short` + сборка бинарника `./wakeUpDev`                  |
| `make run`           | Сборка и запуск в активном режиме (stdout)                     |
| `make start`         | Сборка, запуск в фоне, лог в `logs/`                           |
| `make stop`          | Завершение процесса по PID из `run/wakeUpDev.pid`              |
| `make logs`          | Просмотр логов в `logs/wakeUpDev.log`                          |
| `make clean`         | Удаление бинарника, PID-файла и лога                           |
| `make docker-build`  | Сборка Docker-образа без запуска                               |
| `make docker-up`     | Сборка и запуск контейнера через Docker Compose                |
| `make docker-down`   | Остановка контейнера                                           |

> **Docker:** при сборке образа выполняется `go test -short` (без обращения к Telegram).
> Секреты из `.env` в образ не попадают (см. `.dockerignore`). Конфиг и расписание
> монтируются из хоста; в `.env` для Docker удобно `REMINDERS_FILE=reminders.json`.

---

## 🛠 Требования

* Go 1.25+ или Docker
* Telegram Bot API токен
* ID группы или супергруппы (в формате -100... для супергрупп)

## 🧪 Тестирование

Проект включает набор unit-тестов для проверки основных функций:

```bash
# Unit-тесты (быстро, без Telegram API)
make test-short

# Полный прогон, включая TestTelegramConnection
make test

# Покрытие
go test -short -coverprofile=coverage.out .
go tool cover -html=coverage.out
```

**Покрытие тестами:** 37.8% (основные функции: `parseHourMinute`, `truncate`, `loadReminders`, `mustParseInt64`, `isWeekend`, настройка прокси)

---

Если всё готово — запускай `make start` или `make docker-up`, и бот сам будет напоминать команде о важных вещах, но **не
в выходные** ✌️
