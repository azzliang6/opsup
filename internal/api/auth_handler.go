package api

import (
	"errors"
	"net/http"

	"github.com/azzliang6/opsup/internal/auth"
	"github.com/azzliang6/opsup/internal/models"
	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type setupRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=12"`
}

func AuthStatus(c *gin.Context) {
	count, err := models.CountUsers(getDB(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read initialization status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"initialized": count > 0})
}

func AuthSetup(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		count, err := models.CountUsers(getDB(c))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read initialization status"})
			return
		}
		if count > 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "already initialized"})
			return
		}
		var req setupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(req.Password) > 72 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "password must not exceed 72 UTF-8 bytes"})
			return
		}
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
			return
		}
		user, err := models.CreateFirstUser(getDB(c), req.Username, hash)
		if errors.Is(err, models.ErrAlreadyInitialized) {
			c.JSON(http.StatusForbidden, gin.H{"error": "already initialized"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create administrator"})
			return
		}
		respondWithToken(c, user, jwtSecret)
	}
}

func AuthLogin(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		user, err := models.GetUserByUsername(getDB(c), req.Username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to authenticate"})
			return
		}
		if user == nil || !auth.CheckPassword(req.Password, user.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		respondWithToken(c, user, jwtSecret)
	}
}

func respondWithToken(c *gin.Context, user *models.User, secret string) {
	token, expiresAt, err := auth.GenerateToken(user.ID, user.Username, secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "expires_at": expiresAt, "username": user.Username})
}
