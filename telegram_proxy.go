package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"golang.org/x/net/proxy"
)

// telegramProxyConfig — настройки прокси для Telegram API из .env.
type telegramProxyConfig struct {
	URL *url.URL
}

func (c telegramProxyConfig) enabled() bool {
	return c.URL != nil
}

func (c telegramProxyConfig) maskedAddr() string {
	u := *c.URL
	u.User = nil
	return u.String()
}

func loadTelegramProxyConfig() (telegramProxyConfig, error) {
	host := strings.TrimSpace(os.Getenv("TELEGRAM_PROXY_HOST"))
	if host == "" {
		return telegramProxyConfig{}, nil
	}

	userpwd := strings.TrimSpace(os.Getenv("TELEGRAM_PROXY_USERPWD"))
	proxyURL, err := parseTelegramProxyURL(host, userpwd)
	if err != nil {
		return telegramProxyConfig{}, err
	}

	return telegramProxyConfig{URL: proxyURL}, nil
}

func parseTelegramProxyURL(host, userpwd string) (*url.URL, error) {
	proxyStr := strings.TrimSpace(host)
	if !strings.Contains(proxyStr, "://") {
		proxyStr = "http://" + proxyStr
	}

	u, err := url.Parse(proxyStr)
	if err != nil {
		return nil, fmt.Errorf("TELEGRAM_PROXY_HOST: %w", err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("TELEGRAM_PROXY_HOST: некорректный адрес %q", host)
	}

	if userpwd != "" && u.User == nil {
		user, pass, ok := strings.Cut(userpwd, ":")
		if !ok {
			return nil, fmt.Errorf("TELEGRAM_PROXY_USERPWD: ожидается формат user:password")
		}
		u.User = url.UserPassword(user, pass)
	}

	switch strings.ToLower(u.Scheme) {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, fmt.Errorf(
			"TELEGRAM_PROXY_HOST: схема %q не поддерживается (http, https, socks5, socks5h)",
			u.Scheme,
		)
	}

	return u, nil
}

func (c telegramProxyConfig) transport() (*http.Transport, error) {
	if !c.enabled() {
		return &http.Transport{}, nil
	}

	switch strings.ToLower(c.URL.Scheme) {
	case "http", "https":
		return &http.Transport{Proxy: http.ProxyURL(c.URL)}, nil
	case "socks5", "socks5h":
		dialer, err := proxy.FromURL(c.URL, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("SOCKS5 прокси: %w", err)
		}
		contextDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("SOCKS5 прокси: dialer не поддерживает DialContext")
		}
		return &http.Transport{DialContext: contextDialer.DialContext}, nil
	default:
		return nil, fmt.Errorf("неподдерживаемая схема прокси %q", c.URL.Scheme)
	}
}

func newTelegramHTTPClient() (*http.Client, error) {
	cfg, err := loadTelegramProxyConfig()
	if err != nil {
		return nil, err
	}
	if !cfg.enabled() {
		return &http.Client{}, nil
	}

	transport, err := cfg.transport()
	if err != nil {
		return nil, err
	}

	return &http.Client{Transport: transport}, nil
}

func newTelegramBot(token string) (*tgbotapi.BotAPI, error) {
	cfg, err := loadTelegramProxyConfig()
	if err != nil {
		return nil, err
	}

	client, err := newTelegramHTTPClient()
	if err != nil {
		return nil, err
	}

	if cfg.enabled() {
		log.Printf("🔒 Telegram API через прокси %s", cfg.maskedAddr())
	}

	return tgbotapi.NewBotAPIWithClient(token, client)
}
