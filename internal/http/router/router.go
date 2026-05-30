package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	config "github.com/leyl1ne/ApiGateway/internal/config/gateway"
	"github.com/leyl1ne/ApiGateway/internal/http/middleware"
	"github.com/leyl1ne/ApiGateway/internal/http/proxy"
	"github.com/leyl1ne/ApiGateway/internal/logger"
)

// Setup создаёт и конфигурирует Gin-роутер со всеми маршрутами.
//
// Структура маршрутов(инъекция X-Request-ID):
//
//	Публичные (без авторизации):
//	  GET  /health              → UserService /health
//	  POST /auth/register       → UserService /auth/register
//	  POST /auth/login          → UserService /auth/login
//	  POST /auth/refresh        → UserService /auth/refresh
//	  POST /auth/logout         → UserService /auth/logout
//
//	Защищённые (требуют JWT → инъекция X-User-ID, X-User-Role, X-Company-ID):
//	  GET    /users/me          → UserService /users/me
//	  PATCH  /users/me          → UserService /users/me
//	  GET    /users/:id         → UserService /users/:id
//	  POST   /companies         → UserService /companies
//	  GET    /companies/:id     → UserService /companies/:id
//	  GET    /companies/:id/users → UserService /companies/:id/users
func Setup(
	cfg *config.Config,
	tokenProvider middleware.TokenProvider,
	log logger.Logger,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.InjectRequestIDHeadersMiddleware())
	r.Use(middleware.LoggerMiddleware(log))

	// ─── CORS ────────────────────────────────────────────────────────────
	r.Use(middleware.CorsMiddleware(&cfg.CORS))

	// ─── Gateway health (K8s) ───────────────────────────────────────
	r.GET("/health", HealthHandler)

	// ─── Прокси для UserService ──────────────────────────────────────────
	userServiceProxy, err := proxy.NewReverseProxy(cfg.Services.UserService.URL, log, cfg.Server.HTTP.BasicAuth)
	if err != nil {
		log.Fatal("failed create user service proxy", logger.Err(err))
	}

	// ─── Публичные маршруты (без авторизации) ───────────────────────────
	public := r.Group("/")
	{
		public.GET("/docs", userServiceProxy.ProxyHandler())
		public.POST("/auth/register", userServiceProxy.ProxyHandler())
		public.POST("/auth/login", userServiceProxy.ProxyHandler())
		public.POST("/auth/refresh", userServiceProxy.ProxyHandler())
		public.POST("/auth/logout", userServiceProxy.ProxyHandler())
	}

	// ─── Защищённые маршруты (JWT + инъекция заголовков) ─────────────────
	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware(tokenProvider))
	protected.Use(middleware.InjectUserHeadersMiddleware())
	{
		// Пользователи
		protected.GET("/users/me", userServiceProxy.ProxyHandler())
		protected.PATCH("/users/me", userServiceProxy.ProxyHandler())
		protected.GET("/users/:id", userServiceProxy.ProxyHandler())

		// Компании
		protected.POST("/companies", userServiceProxy.ProxyHandler())
		protected.GET("/companies/:id", userServiceProxy.ProxyHandler())
		protected.GET("/companies/:id/users", userServiceProxy.ProxyHandler())
	}

	return r
}

func HealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "api-gateway",
	})
}
