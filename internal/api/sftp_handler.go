package api

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/azzliang6/opsup/internal/crypto"
	"github.com/azzliang6/opsup/internal/models"
	"github.com/azzliang6/opsup/internal/sftpclient"
)

type fileInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`
	ModTime time.Time `json:"modTime"`
	IsDir   bool      `json:"isDir"`
	Path    string    `json:"path"`
}

func connectSFTP(c *gin.Context, encKey string) (*sftpclient.Client, error) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	server, err := models.GetServer(getDB(c), id)
	if err != nil || server == nil {
		return nil, fmt.Errorf("server not found")
	}

	privateKey, _ := crypto.Decrypt(server.PrivateKey, encKey)
	var passwordStr string
	if server.Password != "" {
		pwBytes, err := crypto.Decrypt(server.Password, encKey)
		if err == nil {
			passwordStr = string(pwBytes)
		}
	}

	if server.JumpServerID != nil && *server.JumpServerID != 0 {
		jumpServer, err := models.GetServer(getDB(c), *server.JumpServerID)
		if err != nil || jumpServer == nil {
			return nil, fmt.Errorf("jump server not found")
		}
		jumpKey, _ := crypto.Decrypt(jumpServer.PrivateKey, encKey)
		var jumpPassword string
		if jumpServer.Password != "" {
			jpBytes, err := crypto.Decrypt(jumpServer.Password, encKey)
			if err == nil {
				jumpPassword = string(jpBytes)
			}
		}
		return sftpclient.NewSFTPClientViaJump(
			jumpServer.Host, jumpServer.Port, jumpServer.Username, jumpServer.AuthType, jumpKey, jumpPassword,
			server.Host, server.Port, server.Username, server.AuthType, privateKey, passwordStr,
		)
	}

	return sftpclient.NewSFTPClient(
		server.Host, server.Port, server.Username, server.AuthType, privateKey, passwordStr,
	)
}

func SFTPList(encKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		dirPath := c.Query("path")
		if dirPath == "" {
			dirPath = "/"
		}

		client, err := connectSFTP(c, encKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer client.Close()

		entries, err := client.ReadDir(dirPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		files := make([]fileInfo, 0, len(entries))
		for _, entry := range entries {
			files = append(files, fileInfo{
				Name:    entry.Name(),
				Size:    entry.Size(),
				Mode:    entry.Mode().String(),
				ModTime: entry.ModTime(),
				IsDir:   entry.IsDir(),
				Path:    path.Join(dirPath, entry.Name()),
			})
		}

		c.JSON(http.StatusOK, files)
	}
}

func SFTPUpload(encKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		dirPath := c.PostForm("path")
		if dirPath == "" {
			dirPath = "/"
		}

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no file provided"})
			return
		}
		defer file.Close()

		client, err := connectSFTP(c, encKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer client.Close()

		remotePath := path.Join(dirPath, header.Filename)
		dst, err := client.Create(remotePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("create file: %v", err)})
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("upload: %v", err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "uploaded", "path": remotePath})
	}
}

func SFTPDownload(encKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		filePath := c.Query("path")
		if filePath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
			return
		}

		client, err := connectSFTP(c, encKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer client.Close()

		remoteFile, err := client.Open(filePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("open file: %v", err)})
			return
		}
		defer remoteFile.Close()

		stat, err := remoteFile.Stat()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Header("Content-Disposition", "attachment; filename="+filepath.Base(filePath))
		c.Header("Content-Length", strconv.FormatInt(stat.Size(), 10))
		c.DataFromReader(http.StatusOK, stat.Size(), "application/octet-stream", remoteFile, nil)
	}
}

func SFTPMkdir(encKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Path string `json:"path" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		client, err := connectSFTP(c, encKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer client.Close()

		if err := client.MkdirAll(req.Path); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "created"})
	}
}

func SFTPDelete(encKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		filePath := c.Query("path")
		if filePath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
			return
		}

		client, err := connectSFTP(c, encKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer client.Close()

		stat, err := client.Stat(filePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if stat.IsDir() {
			err = client.RemoveDirectory(filePath)
		} else {
			err = client.Remove(filePath)
		}

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}
