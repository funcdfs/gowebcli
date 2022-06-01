package main

import (
	"context"
	"fmt"
	"gowebcli/dao/mysql"
	"gowebcli/dao/redis"
	"gowebcli/logger"
	"gowebcli/routes"
	"gowebcli/settings"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/viper"

	"go.uber.org/zap"
)

// GO WEB 比较通用的脚手架模板

func main() {
	// 1 加载配置文件 (使用 viper 读取配置文件)
	if err := settings.Init(); err != nil {
		fmt.Printf("init settings failed. err: %v\n", err)
		return
	}
	// 2 初始化日志
	if err := logger.Init(); err != nil {
		fmt.Printf("init logger failed. err: %v\n", err)
	}
	defer zap.L().Sync() // 把缓冲区的日志追加到日志文件中
	zap.L().Debug("logger init success...")
	// 3 初始化 mysql 连接
	if err := mysql.Init(); err != nil {
		fmt.Printf("init mysql failed. err: %v\n", err)
	}
	defer mysql.Close()
	// 4 初始化 redis 连接
	if err := redis.Init(); err != nil {
		fmt.Printf("init redis failed. err: %v\n", err)
	}
	defer redis.Close()
	// 5 注册路由

	r := routes.SetUp()

	// 6 启动服务（优雅关机与平滑重启）

	srv := &http.Server{
		Addr:    fmt.Sprintf("%d", viper.GetInt("app.port")),
		Handler: r,
	}

	go func() {
		// 开启一个goroutine启动服务
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// 等待中断信号来优雅地关闭服务器，为关闭服务器操作设置一个5秒的超时
	quit := make(chan os.Signal, 1) // 创建一个接收信号的通道
	// kill 默认会发送 syscall.SIGTERM 信号
	// kill -2 发送 syscall.SIGINT 信号，我们常用的Ctrl+C就是触发系统SIGINT信号
	// kill -9 发送 syscall.SIGKILL 信号，但是不能被捕获，所以不需要添加它
	// signal.Notify把收到的 syscall.SIGINT或syscall.SIGTERM 信号转发给quit
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM) // 此处不会阻塞
	<-quit                                               // 阻塞在此，当接收到上述两种信号时才会往下执行
	//log.Println("Shutdown Server ...")
	zap.L().Info("Shutdown Server ...")
	// 创建一个5秒超时的context

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// 5秒内优雅关闭服务（将未处理完的请求处理完再关闭服务），超过5秒就超时退出
	if err := srv.Shutdown(ctx); err != nil {
		//log.Fatal("Server Shutdown: ", err)
		zap.L().Fatal("server shutdown", zap.Error(err))
	}

	//log.Println("Server exiting")
	zap.L().Info("Server exiting")
}
