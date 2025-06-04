APP_NAME = dailyDevPing
ENV_FILE = .env
ENV_EXAMPLE = .env.example
BIN = ./$(APP_NAME)
LOG_DIR = ./logs
RUN_DIR = ./run
LOG_FILE = $(LOG_DIR)/$(APP_NAME).log
PID_FILE = $(RUN_DIR)/$(APP_NAME).pid

.PHONY: .env build run start stop logs clean

.env:
	@if [ ! -f $(ENV_FILE) ]; then \
		cp $(ENV_EXAMPLE) $(ENV_FILE); \
		echo "✅ $(ENV_FILE) создан на основе $(ENV_EXAMPLE)"; \
	else \
		echo "✅ $(ENV_FILE) уже существует, пропускаем копирование"; \
	fi

build: .env
	@echo "🔧 Сборка $(APP_NAME)..."
	@go build -o $(BIN) main.go
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
	@tail -f $(LOG_FILE)

clean:
	rm -f $(BIN) $(LOG_FILE) $(PID_FILE)
	@echo "🗑️ Удалены файлы сборки и логов"