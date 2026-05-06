package api

import (
	"database/sql"
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/azzliang6/opsup/internal/auth"
)

type AppConfig struct {
	JWTSecret     string
	EncryptionKey string
	DistFS        embed.FS
	DB            *sql.DB
}

func SetupRouter(cfg AppConfig) *gin.Engine {
	r := gin.Default()

	// Inject DB into all contexts
	r.Use(func(c *gin.Context) {
		c.Set("db", cfg.DB)
		c.Next()
	})

	// CORS for development
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	api := r.Group("/api")
	{
		authGroup := api.Group("/auth")
		{
			authGroup.GET("/status", AuthStatus)
			authGroup.POST("/setup", AuthSetup(cfg.JWTSecret))
			authGroup.POST("/login", AuthLogin(cfg.JWTSecret))
		}

		protected := api.Group("")
		protected.Use(auth.JWTAuthMiddleware(cfg.JWTSecret))
		{
			servers := protected.Group("/servers")
			{
				servers.GET("", ListServers)
				servers.POST("", CreateServer(cfg.EncryptionKey))
				servers.GET("/:id", GetServer)
				servers.PUT("/:id", UpdateServer(cfg.EncryptionKey))
				servers.DELETE("/:id", DeleteServer)
				servers.POST("/:id/test", TestConnection(cfg.EncryptionKey))

				// SFTP file management
				files := servers.Group("/:id/files")
				{
					files.GET("", SFTPList(cfg.EncryptionKey))
					files.POST("/upload", SFTPUpload(cfg.EncryptionKey))
					files.GET("/download", SFTPDownload(cfg.EncryptionKey))
					files.POST("/mkdir", SFTPMkdir(cfg.EncryptionKey))
					files.DELETE("", SFTPDelete(cfg.EncryptionKey))
				}
			}
		}

		// Terminal WebSocket: validates JWT from query param (browsers can't set WS headers)
		api.GET("/terminal/:serverId", TerminalHandler(cfg))
	}

	// Serve embedded SPA
	distFS, err := fs.Sub(cfg.DistFS, "ui/dist")
	if err == nil {
		fileServer := http.FileServer(http.FS(distFS))
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if path != "/" && fileExists(distFS, path) {
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			c.Request.URL.Path = "/"
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
	}

	return r
}

func fileExists(fsys fs.FS, path string) bool {
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	if path == "" {
		return false
	}
	info, err := fs.Stat(fsys, path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
