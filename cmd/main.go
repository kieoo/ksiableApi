package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"ksiableApi/internal"
	"ksiableApi/internal/log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"time"
)

func main() {
	//logfile, _ := os.Create("gin.log")
	//gin.DefaultWriter = io.MultiWriter(logfile, os.Stdout)

	r := gin.New()
	r.Use(corsMiddleware())
	r.Use(log.LoggerToFile())

	r.GET("version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"msg": "1.1.1"})
	})

	ks := r.Group("ksiable")
	{
		ks.POST("upKubeconfs", internal.UpKubeconfs)
		ks.POST("reloadInfo", internal.ReloadInfo)
		ks.POST("exec", internal.Exec)
		ks.POST("loadLog", internal.LoadLog)
		ks.POST("cancelLog", internal.CancelLoad)
	}

	internal.ExecInit()

	srv := &http.Server{
		Addr:    GetRunPort(),
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Logger().Infof("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Logger().Info("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Logger().Infof("Server Shutdown:%s", err)
	}
	log.Logger().Info("Server exiting")
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		method := c.Request.Method
		origin := c.Request.Header.Get("Origin")
		var filterHost = [...]string{origin}
		// filterHost 做过滤器，防止不合法的域名访问
		var isAccess = false
		for _, v := range filterHost {
			match, _ := regexp.MatchString(v, origin)
			if match {
				isAccess = true
			}
		}
		if isAccess {
			// 核心处理方式
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
			c.Header("Access-Control-Allow-Methods", "GET, OPTIONS, POST, PUT, DELETE")
			c.Set("content-type", "application/json")
		}
		//放行所有OPTIONS方法
		if method == "OPTIONS" {
			c.JSON(http.StatusOK, "Options Request!")
		}

		c.Next()
	}
}

// timeout middleware wraps the request context with a timeout
func timeoutMiddleware(timeout time.Duration) func(c *gin.Context) {
	return func(c *gin.Context) {

		// wrap the request context with a timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)

		defer func() {
			// check if context timeout was reached
			if ctx.Err() == context.DeadlineExceeded {

				// write response and abort the request
				c.Writer.WriteHeader(http.StatusGatewayTimeout)
				c.Abort()
			}
			//cancel to clear resources after finished
			cancel()
		}()

		// replace request with context wrapped request
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func GetRunPort() string {
	port := os.Getenv("GOPORT")
	if port == "" {
		port = "7001"
	}
	return fmt.Sprintf(":%s", port)
}
