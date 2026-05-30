package proxy

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	httpServer "github.com/leyl1ne/ApiGateway/internal/http"
	"github.com/leyl1ne/ApiGateway/internal/logger"
)

type ReverseProxy struct {
	targetURL *url.URL
	basicAuth httpServer.BasicAuth
	log       logger.Logger
}

func NewReverseProxy(targetURL string, log logger.Logger, basicAuth httpServer.BasicAuth) (*ReverseProxy, error) {
	const op = "http.proxy.NewReverseProxy"

	target, err := url.Parse(targetURL)
	if err != nil {
		log.Error("failed to parse target URL",
			logger.Field{Key: "url", Value: targetURL},
			logger.Err(err),
		)
		return &ReverseProxy{}, fmt.Errorf("%s: %w", op, ErrFailedParseURL)
	}

	return &ReverseProxy{
		targetURL: target,
		basicAuth: basicAuth,
		log:       log,
	}, nil
}

// ReverseProxy конфигурирует и возвращает gin.HandlerFunc,
// который проксирует запрос на указанный targetURL.
//
// Логика работы:
//  1. Создаётся httputil.ReverseProxy с кастомным Director,
//     который переписывает схему, хост и путь запроса.
//  2. Заголовки, добавленные middleware (X-User-ID, X-User-Role, X-Company-ID),
//     автоматически пробрасываются, потому что Director работает
//     с тем же объектом http.Request, который уже содержит эти заголовки.
//  3. Модификатор Response модифицирует ответ при необходимости
//     (например, добавляет заголовки).
func (p *ReverseProxy) ProxyHandler() gin.HandlerFunc {

	proxy := &httputil.ReverseProxy{
		Director:       p.makeDirector(),
		ModifyResponse: p.makeResponseModifier(),
		ErrorHandler:   p.makeErrorHandler(),
		Transport:      p.makeTransport(),
	}

	return func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// PrefixProxy проксирует запросы, добавляя указанный префикс к пути.
// Например, если prefix="/api/v1/users", а входящий путь "/users/me",
// то downstream-сервис получит "/api/v1/users/me".
//
// Это полезно, если downstream-сервис ожидает запросы с другим префиксом,
// чем SPA отправляет на gateway.
func (p *ReverseProxy) PrefixProxyHandler(prefix string) gin.HandlerFunc {

	director := p.makeDirector()
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			director(req)
			req.URL.Path = prefix + req.URL.Path
			req.URL.RawPath = prefix + req.URL.RawPath
		},
		ModifyResponse: p.makeResponseModifier(),
		ErrorHandler:   p.makeErrorHandler(),
		Transport:      p.makeTransport(),
	}

	return func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func (p *ReverseProxy) makeDirector() func(req *http.Request) {
	return func(req *http.Request) {
		req.URL.Scheme = p.targetURL.Scheme
		req.URL.Host = p.targetURL.Host

		// Путь запроса сохраняется как есть — gateway не переписывает пути.
		// SPA отправляет /auth/login → gateway проксирует на UserService /auth/login.
		req.Host = p.targetURL.Host

		// Удаляем заголовок Origin, чтобы downstream-сервис не отклонял запрос
		// из-за CORS (CORS уже обработан на уровне gateway).
		req.Header.Del("Origin")

		if p.basicAuth.Enabled {
			credentials := base64.StdEncoding.EncodeToString(
				[]byte(p.basicAuth.Username + ":" + p.basicAuth.Password),
			)

			basicAuthHeader := "Basic " + credentials
			req.Header.Set("Authorization", basicAuthHeader)
		}
	}
}

func (p *ReverseProxy) makeResponseModifier() func(*http.Response) error {
	return func(resp *http.Response) error {
		p.log.Info("upstream response",
			logger.Field{Key: "status", Value: resp.StatusCode},
			logger.Field{Key: "path", Value: resp.Request.URL.Path},
			logger.Field{Key: "headers", Value: resp.Header},
		)
		return nil
	}
}

func (p *ReverseProxy) makeErrorHandler() func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		p.log.Error("proxy error",
			logger.Field{Key: "path", Value: r.URL.Path},
			logger.Err(err),
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"message":"service unavailable"}}`))
	}
}

func (p *ReverseProxy) makeTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}
}
