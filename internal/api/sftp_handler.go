package api

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/azzliang6/opsup/internal/sftpclient"
	"github.com/gin-gonic/gin"
)

type fileInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`
	ModTime time.Time `json:"modTime"`
	IsDir   bool      `json:"isDir"`
	Path    string    `json:"path"`
}

const maxUploadBody = 256 * 1024 * 1024
const sftpOperationTimeout = 15 * time.Minute

func sftpOperation(c *gin.Context) context.CancelFunc {
	ctx, cancel := context.WithTimeout(c.Request.Context(), sftpOperationTimeout)
	c.Request = c.Request.WithContext(ctx)
	deadline, _ := ctx.Deadline()
	controller := http.NewResponseController(c.Writer)
	_ = controller.SetReadDeadline(deadline)
	_ = controller.SetWriteDeadline(deadline)
	return func() {
		cancel()
		_ = controller.SetReadDeadline(time.Time{})
		_ = controller.SetWriteDeadline(time.Time{})
	}
}

func connectSFTP(c *gin.Context, encKey string) (*sftpclient.Client, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("invalid server id")
	}
	server, err := loadConnectionServer(c.Request.Context(), getDB(c), id)
	if err != nil {
		return nil, fmt.Errorf("load server: %w", err)
	}
	if server == nil {
		return nil, fmt.Errorf("server not found")
	}
	owner, err := dialServer(c.Request.Context(), getDB(c), server, encKey)
	if err != nil {
		return nil, err
	}
	return sftpclient.NewClient(c.Request.Context(), owner)
}

func SFTPList(encKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cancel := sftpOperation(c)
		defer cancel()
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
		cancel := sftpOperation(c)
		defer cancel()
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBody)
		err := c.Request.ParseMultipartForm(8 * 1024 * 1024)
		if c.Request.MultipartForm != nil {
			defer c.Request.MultipartForm.RemoveAll()
		}
		if err != nil {
			status := http.StatusBadRequest
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				status = http.StatusRequestEntityTooLarge
			}
			c.JSON(status, gin.H{"error": "invalid upload or request exceeds 256 MiB"})
			return
		}
		dirPath := c.Request.FormValue("path")
		if dirPath == "" {
			dirPath = "/"
		}
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no file provided"})
			return
		}
		defer file.Close()
		filename := path.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
		if filename == "." || filename == "/" || filename == ".." || strings.ContainsRune(filename, 0) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filename"})
			return
		}

		client, err := connectSFTP(c, encKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer client.Close()

		remotePath := path.Join(dirPath, filename)
		if err := client.Upload(remotePath, file); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "uploaded", "path": remotePath})
	}
}

func SFTPDownload(encKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cancel := sftpOperation(c)
		defer cancel()
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

		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(filePath)}))
		c.Header("Content-Length", strconv.FormatInt(stat.Size(), 10))
		c.DataFromReader(http.StatusOK, stat.Size(), "application/octet-stream", remoteFile, nil)
	}
}

func SFTPMkdir(encKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cancel := sftpOperation(c)
		defer cancel()
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
		cancel := sftpOperation(c)
		defer cancel()
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
