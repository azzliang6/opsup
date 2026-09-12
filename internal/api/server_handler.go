package api

import (
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/azzliang6/opsup/internal/crypto"
	"github.com/azzliang6/opsup/internal/models"
	"github.com/gin-gonic/gin"
)

type serverCreateReq struct {
	Protocol           string `json:"protocol"`
	RDPDomain          string `json:"rdp_domain"`
	RDPCertFingerprint string `json:"rdp_cert_fingerprint"`

	Name         string `json:"name" binding:"required"`
	Host         string `json:"host" binding:"required"`
	Port         int    `json:"port"`
	Username     string `json:"username" binding:"required"`
	AuthType     string `json:"auth_type"`
	HostKey      string `json:"host_key"`
	PrivateKey   string `json:"private_key"`
	Password     string `json:"password"`
	CopyKeyFrom  int64  `json:"copy_key_from"`
	JumpServerID *int64 `json:"jump_server_id"`
	Group        string `json:"group"`
	Description  string `json:"description"`
}

func (r *serverCreateReq) validate() error {
	if r.Protocol == "" {
		r.Protocol = "ssh"
	}
	if r.Protocol != "ssh" && r.Protocol != "rdp" {
		return fmt.Errorf("protocol must be ssh or rdp")
	}
	if r.Protocol == "rdp" && r.Port == 0 {
		r.Port = 3389
	}
	if r.Port == 0 {
		r.Port = 22
	}
	if r.Port < 1 || r.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if r.Protocol == "rdp" {
		if r.Password != "" || r.PrivateKey != "" || r.CopyKeyFrom != 0 || r.HostKey != "" || r.JumpServerID != nil {
			return fmt.Errorf("RDP V1 does not store credentials or support SSH settings")
		}
		r.AuthType = "password"
		r.RDPCertFingerprint = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(r.RDPCertFingerprint), ":", ""))
		if r.RDPCertFingerprint != "" {
			b, err := hex.DecodeString(r.RDPCertFingerprint)
			if err != nil || len(b) != 32 {
				return fmt.Errorf("RDP certificate fingerprint must be SHA256 (64 hex digits)")
			}
		}
		return nil
	}
	r.RDPDomain, r.RDPCertFingerprint = "", ""
	if r.AuthType == "" {
		r.AuthType = "key"
	}
	if r.AuthType != "key" && r.AuthType != "password" {
		return fmt.Errorf("auth_type must be key or password")
	}
	r.HostKey = strings.TrimSpace(r.HostKey)
	if r.HostKey != "" {
		decoded, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(r.HostKey, "SHA256:"))
		if !strings.HasPrefix(r.HostKey, "SHA256:") || err != nil || len(decoded) != 32 {
			return fmt.Errorf("host_key must be an SHA256 SSH host fingerprint")
		}
	}
	if r.JumpServerID != nil && *r.JumpServerID <= 0 {
		return fmt.Errorf("invalid jump_server_id")
	}
	return nil
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
	c.JSON(http.StatusOK, server.ToListItem())
}

func resolveCredentials(db *sql.DB, req *serverCreateReq, current *models.Server, key string) (string, string, error) {
	if req.Protocol == "rdp" {
		return "", "", nil
	}
	privateKey, password := "", ""
	if current != nil {
		privateKey, password = current.PrivateKey, current.Password
	}
	var err error
	if req.AuthType == "password" {
		if req.Password != "" {
			password, err = crypto.Encrypt([]byte(req.Password), key)
		}
		if err == nil && password == "" {
			err = fmt.Errorf("password is required for password auth")
		}
	} else {
		if req.PrivateKey != "" {
			privateKey, err = crypto.Encrypt([]byte(req.PrivateKey), key)
		} else if req.CopyKeyFrom > 0 {
			source, queryErr := models.GetServer(db, req.CopyKeyFrom)
			if queryErr != nil {
				return "", "", queryErr
			}
			if source == nil || source.PrivateKey == "" {
				return "", "", fmt.Errorf("source server has no private key")
			}
			privateKey = source.PrivateKey
		}
		if err == nil && privateKey == "" {
			err = fmt.Errorf("private_key or copy_key_from is required")
		}
	}
	return privateKey, password, err
}

func CreateServer(encKey string) gin.HandlerFunc {
	return saveServer(encKey, false)
}

func UpdateServer(encKey string) gin.HandlerFunc {
	return saveServer(encKey, true)
}

func saveServer(encKey string, update bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req serverCreateReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := req.validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var current *models.Server
		var id int64
		if update {
			var err error
			id, err = strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil || id <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
				return
			}
			current, err = models.GetServer(getDB(c), id)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if current == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
				return
			}
		}
		if req.JumpServerID != nil {
			if *req.JumpServerID == id {
				c.JSON(http.StatusBadRequest, gin.H{"error": "server cannot jump through itself"})
				return
			}
			jump, err := models.GetServer(getDB(c), *req.JumpServerID)
			if err != nil || jump == nil || jump.Protocol == "rdp" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "jump server not found"})
				return
			}
		}
		privateKey, password, err := resolveCredentials(getDB(c), &req, current, encKey)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		server := &models.Server{
			Protocol: req.Protocol, RDPDomain: req.RDPDomain, RDPCertFingerprint: req.RDPCertFingerprint,
			ID: id, Name: req.Name, Host: req.Host, Port: req.Port,
			Username: req.Username, AuthType: req.AuthType, HostKey: req.HostKey,
			PrivateKey: privateKey, Password: password, Description: req.Description,
			Group: req.Group, JumpServerID: req.JumpServerID,
		}
		status := http.StatusCreated
		if update {
			err = models.UpdateServer(getDB(c), server)
			status = http.StatusOK
		} else {
			err = models.CreateServer(getDB(c), server)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(status, server.ToListItem())
	}
}

func DeleteServer(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := models.DeleteServer(getDB(c), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func TestConnection(encKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		server, err := models.GetServer(getDB(c), id)
		if err != nil || server == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		if server.Protocol == "rdp" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Open the RDP tab to test authentication"})
			return
		}
		client, err := dialServer(c.Request.Context(), getDB(c), server, encKey)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
			return
		}
		client.Close()
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func getDB(c *gin.Context) *sql.DB {
	return c.MustGet("db").(*sql.DB)
}
