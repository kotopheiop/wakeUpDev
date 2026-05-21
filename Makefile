APP_NAME = wakeUpDev
ENV_FILE = .env
ENV_EXAMPLE = .env.example
BIN = ./$(APP_NAME)
LOG_DIR = ./logs
RUN_DIR = ./run
LOG_FILE = $(LOG_DIR)/$(APP_NAME).log
PID_FILE = $(RUN_DIR)/$(APP_NAME).pid
COMPOSE_FILE = docker/docker-compose.yml

# snap/go в WSL: /run/user/$UID часто недоступен — принудительно /tmp
export XDG_RUNTIME_DIR := /tmp

GO_TEST = go test -v ./...
GO_TEST_SHORT = go test -short -v ./...

.PHONY: .env test test-short build run start stop logs clean docker-up docker-down docker-build

.env:
	@if [ ! -f $(ENV_FILE) ]; then \
		cp $(ENV_EXAMPLE) $(ENV_FILE); \
		echo "✅ $(ENV_FILE) создан на основе $(ENV_EXAMPLE)"; \
	else \
		echo "✅ $(ENV_FILE) уже существует, пропускаем копирование"; \
	fi

test: .env
	@echo "🧪 Запуск всех тестов (включая Telegram API)..."
	@$(GO_TEST) && echo "✅ Тесты пройдены" || (echo "❌ Тесты не прошли." && exit 1)

test-short:
	@echo "🧪 Запуск unit-тестов (-short)..."
	@$(GO_TEST_SHORT) && echo "✅ Тесты пройдены" || (echo "❌ Тесты не прошли." && exit 1)

build: test-short
	@echo "🔧 Сборка $(APP_NAME)..."
	@go build -o $(BIN) .
	@echo "✅ Сборка завершена"

run: build
	@echo "🚀 Запуск $(APP_NAME) в активном режиме (логи в терминал)"
	@./$(BIN)

start: build
	@mkdir -p $(LOG_DIR) $(RUN_DIR)
	@echo "🚀 Запуск $(APP_NAME) в фоне"
	@sh -c 'nohup $(BIN) >> $(LOG_FILE) 2>&1 & echo $$! > $(PID_FILE)'
	@echo "📌 PID сохранён в $(PID_FILE)"
	@echo "📜 Логи: tail -f $(LOG_FILE)"

stop:
	@echo "🛑 Остановка $(APP_NAME)..."
	@if [ -f $(PID_FILE) ]; then \
		PID=`cat $(PID_FILE)`; \
		if [ -n "$$PID" ] && kill -0 $$PID 2>/dev/null; then \
			kill $$PID && rm -f $(PID_FILE) && echo "✅ Остановлено (PID $$PID)"; \
		else \
			echo "⚠️ Некорректный или несуществующий PID: $$PID"; \
			rm -f $(PID_FILE); \
		fi \
	else \
		echo "⚠️ PID-файл не найден"; \
	fi

logs:
	@if [ ! -f $(LOG_FILE) ]; then \
		echo "⚠️ Лог-файл не найден: $(LOG_FILE). Сначала запустите make start"; \
		exit 1; \
	fi
	@tail -f $(LOG_FILE)

clean:
	rm -f $(BIN) $(LOG_FILE) $(PID_FILE)
	@echo "🗑️ Удалены файлы сборки и логов"

docker-build: .env
	@echo "🐳 Сборка Docker-образа..."
	docker compose -f $(COMPOSE_FILE) build

docker-up: .env
	docker compose -f $(COMPOSE_FILE) up -d --build
	@echo "🚀 Wake Up Dev Bot запущен в фоне через docker-compose."
	@echo "ℹ️ Для остановки: make docker-down"

docker-down:
	docker compose -f $(COMPOSE_FILE) down
	@echo "🛑 Wake Up Dev Bot остановлен и контейнер удалён."
