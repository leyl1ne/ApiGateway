package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/leyl1ne/ApiGateway/internal/auth/jwt"
	config "github.com/leyl1ne/ApiGateway/internal/config/gateway"
	"github.com/leyl1ne/ApiGateway/internal/http/response"
	"github.com/leyl1ne/ApiGateway/internal/logger"
)

// Заголовки, которые gateway добавляет в запрос перед проксированием
// в downstream-сервисы. Сервисы могут доверять этим заголовкам,
// поскольку они выставляются gateway'ем после валидации JWT.
const (
	HeaderUserID    = "X-User-ID"
	HeaderUserRole  = "X-User-Role"
	HeaderCompanyID = "X-Company-ID"
	HeaderRequestID = "X-Request-ID"
)

// TokenProvider — интерфейс, который реализует JWTValidator.
type TokenProvider interface {
	Validate(tokenString string) (jwt.Payload, error)
}

// UserContext хранит данные пользователя, извлечённые из JWT.
type UserContext struct {
	UserID    string
	UserRole  string
	CompanyID string
}

type contextKey string

const userContextKey contextKey = "user"
const requestIDContextKey contextKey = "request_id"

// GetUser извлекает UserContext из gin.Context.
func GetUser(c *gin.Context) (UserContext, bool) {
	val, exists := c.Get(userContextKey)
	if !exists {
		return UserContext{}, false
	}
	user, ok := val.(UserContext)
	if !ok {
		return UserContext{}, false
	}
	return user, true
}

// setUser сохраняет UserContext в gin.Context.
func setUser(c *gin.Context, user UserContext) {
	c.Set(userContextKey, user)
}

func setRequestID(c *gin.Context, requestID string) {
	c.Set(requestIDContextKey, requestID)
}

func GetRequestID(c *gin.Context) (string, bool) {
	val, exists := c.Get(requestIDContextKey)
	if !exists {
		return "", false
	}

	id, ok := val.(string)
	if !ok {
		return "", false
	}
	return id, true
}

// AuthMiddleware проверяет JWT-токен из заголовка Authorization,
// извлекает Payload и сохраняет его в контекст запроса.
// Для маршрутов, не требующих авторизации, этот middleware не применяется.
func AuthMiddleware(tokenProvider TokenProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.WriteErrorAbort(c, http.StatusUnauthorized, "authorization header is required")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		tokenString = strings.TrimSpace(tokenString)

		if tokenString == "" || tokenString == authHeader {
			response.WriteErrorAbort(c, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		payload, err := tokenProvider.Validate(tokenString)
		if err != nil {
			if errors.Is(err, jwt.ErrExpiredToken) {
				response.WriteErrorAbort(c, http.StatusUnauthorized, "token expired")
				return
			}

			response.WriteErrorAbort(c, http.StatusUnauthorized, "invalid token")
			return
		}

		setUser(c, UserContext{
			UserID:    payload.UserID,
			UserRole:  payload.UserRole,
			CompanyID: payload.CompanyID,
		})

		c.Next()
	}
}

// InjectHeadersMiddleware добавляет заголовки X-User-ID, X-User-Role, X-Company-ID
// в запрос перед его проксированием в downstream-сервис.
// Должен применяться ПОСЛЕ AuthMiddleware.
func InjectUserHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := GetUser(c)
		if !exists {
			// Если пользователь не найден в контексте — значит,
			// AuthMiddleware не был применён. Пропускаем без инъекции.
			c.Next()
			return
		}

		c.Request.Header.Set(HeaderUserID, user.UserID)
		c.Request.Header.Set(HeaderUserRole, user.UserRole)
		c.Request.Header.Set(HeaderCompanyID, user.CompanyID)

		c.Next()
	}
}

func LoggerMiddleware(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := log.With(
			logger.Field{Key: "component", Value: "middleware/logger"},
		)

		reqID, exists := GetRequestID(c)
		if !exists {
			reqID = uuid.NewString()
		}

		reqLog := log.With(
			logger.Field{Key: "request_id", Value: reqID},
			logger.Field{Key: "method", Value: c.Request.Method},
			logger.Field{Key: "path", Value: c.FullPath()},
		)

		c.Next()

		defer func() {
			reqLog.Info("request completed",
				logger.Field{Key: "status", Value: c.Writer.Status()},
			)
		}()
	}
}

func InjectRequestIDHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		reqID := uuid.NewString()
		setRequestID(c, reqID)

		c.Request.Header.Set(HeaderRequestID, reqID)

		c.Next()
	}
}

// corsMiddleware конфигурирует CORS на основе YAML-конфига.
func CorsMiddleware(cfg *config.CORSConfig) gin.HandlerFunc {
	corsConfig := cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     cfg.AllowedMethods,
		AllowHeaders:     cfg.AllowedHeaders,
		ExposeHeaders:    cfg.ExposedHeaders,
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           time.Duration(cfg.MaxAge) * time.Second,
	}

	// Если origins содержит "*", переключаемся на AllowAllOrigins
	for _, origin := range cfg.AllowedOrigins {
		if origin == "*" {
			corsConfig.AllowAllOrigins = true
			corsConfig.AllowOrigins = nil
			break
		}
	}

	// При AllowCredentials=true и конкретных origins используем
	// AllowOriginFunc для корректного возврата Origin в ответе.
	if corsConfig.AllowCredentials && !corsConfig.AllowAllOrigins {
		originsMap := make(map[string]bool, len(cfg.AllowedOrigins))
		for _, o := range cfg.AllowedOrigins {
			originsMap[o] = true
		}
		corsConfig.AllowOriginFunc = func(origin string) bool {
			return originsMap[origin]
		}
		corsConfig.AllowOrigins = nil // нельзя использовать одновременно с AllowOriginFunc
	}

	return cors.New(corsConfig)
}
