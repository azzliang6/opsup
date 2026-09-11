package api

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

func authRateLimit(limit int, window time.Duration) gin.HandlerFunc {
	type bucket struct {
		until time.Time
		count int
	}
	var mu sync.Mutex
	clients := make(map[string]bucket)
	return func(c *gin.Context) {
		now := time.Now()
		ip := c.ClientIP()
		mu.Lock()
		for key, value := range clients {
			if !now.Before(value.until) {
				delete(clients, key)
			}
		}
		value, exists := clients[ip]
		if !exists {
			value.until = now.Add(window)
		}
		blocked := value.count >= limit || (!exists && len(clients) >= 4096)
		if !blocked {
			value.count++
			clients[ip] = value
		}
		mu.Unlock()
		if blocked {
			c.Header("Retry-After", strconv.Itoa(max(1, int(time.Until(value.until).Seconds())+1)))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many authentication attempts; try again later"})
			return
		}
		c.Next()
	}
}
