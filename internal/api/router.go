package api

import (
	"database/sql"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/azzliang6/opsup/internal/auth"
	"github.com/gin-gonic/gin"
)

type AppConfig struct {
	JWTSecret     string
	EncryptionKey string
	DistFS        embed.FS
	DB            *sql.DB
}

func SetupRouter(cfg AppConfig) *gin.Engine {
	r := gin.New()
	r.SetTrustedProxies(nil)
	r.Use(gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		return fmt.Sprintf("%s %s %q %d %s\n", p.TimeStamp.Format(time.RFC3339), p.Method, p.Request.URL.Path, p.StatusCode, p.Latency)
	}))
	// Gin's default panic dump includes the request URL, which can contain a WS token.
	r.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		log.Printf("request panic: %v", recovered)
		c.AbortWithStatus(http.StatusInternalServerError)
	}))
	r.Use(func(c *gin.Context) {
		c.Set("db", cfg.DB)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Frame-Options", "DENY")
		if c.Request.URL.Path == "/rdp.html" {
			c.Header("X-Frame-Options", "SAMEORIGIN")
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Header("Cache-Control", "no-store")
			if !strings.HasSuffix(c.Request.URL.Path, "/files/upload") {
				c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
			}
		}
		c.Next()
	})

	api := r.Group("/api")
	{
		authGroup := api.Group("/auth")
		authGroup.GET("/status", AuthStatus)
		limited := authGroup.Group("", authRateLimit(10, time.Minute))
		limited.POST("/setup", AuthSetup(cfg.JWTSecret))
		limited.POST("/login", AuthLogin(cfg.JWTSecret))

		protected := api.Group("", auth.JWTAuthMiddleware(cfg.JWTSecret))
		servers := protected.Group("/servers")
		servers.GET("", ListServers)
		servers.POST("", CreateServer(cfg.EncryptionKey))
		servers.GET("/:id", GetServer)
		servers.PUT("/:id", UpdateServer(cfg.EncryptionKey))
		servers.DELETE("/:id", DeleteServer)
		servers.POST("/:id/test", TestConnection(cfg.EncryptionKey))
		files := servers.Group("/:id/files")
		files.GET("", SFTPList(cfg.EncryptionKey))
		files.POST("/upload", SFTPUpload(cfg.EncryptionKey))
		files.GET("/download", SFTPDownload(cfg.EncryptionKey))
		files.POST("/mkdir", SFTPMkdir(cfg.EncryptionKey))
		files.DELETE("", SFTPDelete(cfg.EncryptionKey))
		api.GET("/terminal/:serverId", TerminalHandler(cfg))
		api.GET("/rdp/:serverId", RDPHandler(cfg))
	}

	distFS, err := fs.Sub(cfg.DistFS, "ui/dist")
	if err == nil {
		fileServer := http.FileServer(http.FS(distFS))
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if path == "/api" || strings.HasPrefix(path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			if path != "/" && fileExists(distFS, path) {
				if strings.HasPrefix(path, "/assets/") {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					c.Header("Cache-Control", "no-cache")
				}
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			c.Header("Cache-Control", "no-cache")
			c.Request.URL.Path = "/"
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
	}
	return r
}

func fileExists(fsys fs.FS, path string) bool {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return false
	}
	info, err := fs.Stat(fsys, path)
	return err == nil && !info.IsDir()
}
