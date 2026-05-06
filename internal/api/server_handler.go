package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/azzliang6/opsup/internal/crypto"
	"github.com/azzliang6/opsup/internal/models"
	"github.com/azzliang6/opsup/internal/sshclient"
)

type serverCreateReq struct {
	Name         string `json:"name" binding:"required"`
	Host         string `json:"host" binding:"required"`
	Port         int    `json:"port"`
	Username     string `json:"username" binding:"required"`
	AuthType     string `json:"auth_type"`
	PrivateKey   string `json:"private_key"`
	Password     string `json:"password"`
	CopyKeyFrom  int64  `json:"copy_key_from"`
	JumpServerID *int64 `json:"jump_server_id"`
	Group        string `json:"group"`
	Description  string `json:"description"`
}

func ListServers(c *gin.Context) {
	servers, err := models.ListServers(getDB(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]models.ServerListItem, 0, len(servers))
	for _, s := range servers {
		items = append(items, s.ToListItem())
	}
	c.JSON(http.StatusOK, items)
}

func GetServer(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	server, err := models.GetServer(getDB(c), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if server == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	server.PrivateKey = ""
	c.JSON(http.StatusOK, server)
}

func resolveEncryptedKey(c *gin.Context, req *serverCreateReq, encKeyHex string) (string, error) {
	if req.AuthType == "password" {
		return "", nil
	}
	if req.PrivateKey != "" {
		return crypto.Encrypt([]byte(req.PrivateKey), encKeyHex)
	}
	if req.CopyKeyFrom > 0 {
		src, err := models.GetServer(getDB(c), req.CopyKeyFrom)
		if err != nil || src == nil {
			return "", nil
		}
		return src.PrivateKey, nil
	}
	return "", nil
}

func resolveEncryptedPassword(req *serverCreateReq, encKeyHex string) (string, error) {
	if req.AuthType != "password" || req.Password == "" {
		return "", nil
	}
	return crypto.Encrypt([]byte(req.Password), encKeyHex)
}

func CreateServer(encKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req serverCreateReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Port == 0 {
			req.Port = 22
		}
		if req.AuthType == "" {
			req.AuthType = "key"
		}

		var encPrivKey, encPassword string
		var err error

		if req.AuthType == "password" {
			if req.Password == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "password is required for password auth"})
				return
			}
			encPassword, err = resolveEncryptedPassword(&req, encKey)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption failed"})
				return
			}
		} else {
			encPrivKey, err = resolveEncryptedKey(c, &req, encKey)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption failed"})
				return
			}
			if encPrivKey == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "private_key or copy_key_from is required"})
				return
			}
		}

		s := &models.Server{
			Name:         req.Name,
			Host:         req.Host,
			Port:         req.Port,
			Username:     req.Username,
			AuthType:     req.AuthType,
			PrivateKey:   encPrivKey,
			Password:     encPassword,
			Description:  req.Description,
			Group:        req.Group,
			JumpServerID: req.JumpServerID,
		}

		if err := models.CreateServer(getDB(c), s); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, s.ToListItem())
	}
}

func UpdateServer(encKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

		var req serverCreateReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Port == 0 {
			req.Port = 22
		}
		if req.AuthType == "" {
			req.AuthType = "key"
		}

		// Get current server for fallback values
		current, err := models.GetServer(getDB(c), id)
		if err != nil || current == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}

		var encPrivKey, encPassword string

		if req.AuthType == "password" {
			// Password auth: keep existing key, handle password
			encPrivKey = current.PrivateKey
			if req.Password != "" {
				encPassword, err = crypto.Encrypt([]byte(req.Password), encKey)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption failed"})
					return
				}
			} else {
				encPassword = current.Password
			}
		} else {
			// Key auth
			encPassword = current.Password
			if req.PrivateKey != "" {
				encPrivKey, err = crypto.Encrypt([]byte(req.PrivateKey), encKey)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption failed"})
					return
				}
			} else if req.CopyKeyFrom > 0 {
				src, err := models.GetServer(getDB(c), req.CopyKeyFrom)
				if err != nil || src == nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "source server not found"})
					return
				}
				encPrivKey = src.PrivateKey
			} else {
				encPrivKey = current.PrivateKey
			}
		}

		s := &models.Server{
			ID:           id,
			Name:         req.Name,
			Host:         req.Host,
			Port:         req.Port,
			Username:     req.Username,
			AuthType:     req.AuthType,
			PrivateKey:   encPrivKey,
			Password:     encPassword,
			Description:  req.Description,
			Group:        req.Group,
			JumpServerID: req.JumpServerID,
		}

		if err := models.UpdateServer(getDB(c), s); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, s.ToListItem())
	}
}

func DeleteServer(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := models.DeleteServer(getDB(c), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func decryptServerKey(server *models.Server, encKey string) ([]byte, error) {
	if server.PrivateKey == "" {
		return nil, nil
	}
	return crypto.Decrypt(server.PrivateKey, encKey)
}

func decryptServerPassword(server *models.Server, encKey string) (string, error) {
	if server.Password == "" {
		return "", nil
	}
	plain, err := crypto.Decrypt(server.Password, encKey)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func TestConnection(encKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		server, err := models.GetServer(getDB(c), id)
		if err != nil || server == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}

		privateKey, _ := decryptServerKey(server, encKey)
		password, _ := decryptServerPassword(server, encKey)

		var testErr error
		if server.JumpServerID != nil && *server.JumpServerID != 0 {
			jumpServer, err := models.GetServer(getDB(c), *server.JumpServerID)
			if err != nil || jumpServer == nil {
				c.JSON(http.StatusOK, gin.H{"success": false, "error": "jump server not found"})
				return
			}
			jumpKey, _ := decryptServerKey(jumpServer, encKey)
			jumpPassword, _ := decryptServerPassword(jumpServer, encKey)
			testErr = sshclient.TestDialViaJump(
				jumpServer.Host, jumpServer.Port, jumpServer.Username, jumpServer.AuthType, jumpKey, jumpPassword,
				server.Host, server.Port, server.Username, server.AuthType, privateKey, password,
			)
		} else {
			testErr = sshclient.TestDial(server.Host, server.Port, server.Username, server.AuthType, privateKey, password)
		}

		if testErr != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": testErr.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func getDB(c *gin.Context) *sql.DB {
	return c.MustGet("db").(*sql.DB)
}

// Ensure fmt import is used
var _ = fmt.Sprintf
