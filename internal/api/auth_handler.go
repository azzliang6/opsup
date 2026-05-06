package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/azzliang6/opsup/internal/auth"
	"github.com/azzliang6/opsup/internal/models"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type setupRequest struct {
	Username string `json:"username" binding:"required,min=3"`
	Password string `json:"password" binding:"required,min=6"`
}

// AuthStatus checks if the system has been initialized.
func AuthStatus(c *gin.Context) {
	count, err := models.CountUsers(getDB(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"initialized": count > 0})
}

// AuthSetup creates the first admin user.
func AuthSetup(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		count, _ := models.CountUsers(getDB(c))
		if count > 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "already initialized"})
			return
		}

		var req setupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
			return
		}

		user, err := models.CreateUser(getDB(c), req.Username, hash)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		token, expiresAt, _ := auth.GenerateToken(user.ID, user.Username, jwtSecret)
		c.JSON(http.StatusOK, gin.H{
			"token":      token,
			"expires_at": expiresAt,
			"username":   user.Username,
		})
	}
}

// AuthLogin authenticates a user.
func AuthLogin(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		user, err := models.GetUserByUsername(getDB(c), req.Username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if user == nil || !auth.CheckPassword(req.Password, user.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		token, expiresAt, _ := auth.GenerateToken(user.ID, user.Username, jwtSecret)
		c.JSON(http.StatusOK, gin.H{
			"token":      token,
			"expires_at": expiresAt,
			"username":   user.Username,
		})
	}
}
