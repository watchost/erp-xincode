// Copyright 2026 zhouhouping. All Rights Reserved.

package metrics

import (
	"net"
	"net/http"
	"net/http/pprof"

	"github.com/gin-gonic/gin"
)

// IsPrivateIP checks whether the given IP is a private/loopback IP address.
func IsPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return true
	}
	return false
}

// RegisterPProf registers pprof debugging endpoints on the given gin.Engine,
// protected with an intranet-only IP check (and optional debug token).
func RegisterPProf(r *gin.Engine) {
	group := r.Group("/debug/pprof")
	group.Use(func(c *gin.Context) {
		clientIP := c.ClientIP()
		// Allow loopback and private networks (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7)
		if !IsPrivateIP(clientIP) && c.GetHeader("X-Internal-Debug-Token") == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    10004,
				"message": "pprof is restricted to internal network access only",
			})
			return
		}
		c.Next()
	})

	group.GET("/", gin.WrapF(pprof.Index))
	group.GET("/cmdline", gin.WrapF(pprof.Cmdline))
	group.GET("/profile", gin.WrapF(pprof.Profile))
	group.POST("/symbol", gin.WrapF(pprof.Symbol))
	group.GET("/symbol", gin.WrapF(pprof.Symbol))
	group.GET("/trace", gin.WrapF(pprof.Trace))
	group.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
	group.GET("/block", gin.WrapH(pprof.Handler("block")))
	group.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
	group.GET("/heap", gin.WrapH(pprof.Handler("heap")))
	group.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
	group.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
}
