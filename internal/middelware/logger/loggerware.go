package logger

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type zaploger interface {
	GinLogger() gin.HandlerFunc
	GinRecover(stack bool) gin.HandlerFunc
}

type ZapLogger struct {
}

func NewZapLogger(Viper *viper.Viper) (tmp *ZapLogger, err error) {
	writerSync := getWriterSync(Viper)
	encoder := getEncoder()
	level, err := getLeve(Viper)
	if err != nil {
		return nil, err
	}
	core := zapcore.NewCore(encoder, writerSync, *level)
	logger := zap.New(core, zap.AddCaller())
	zap.ReplaceGlobals(logger)
	return tmp, nil
}

func getWriterSync(viper *viper.Viper) zapcore.WriteSyncer {
	lumberjackLogger := &lumberjack.Logger{
		Filename:   viper.GetString("log.filename"),
		MaxSize:    viper.GetInt("log.max_size"),
		MaxAge:     viper.GetInt("log.max_age"),
		MaxBackups: viper.GetInt("log.max_backups"),
		Compress:   true,
	}
	return zapcore.AddSync(lumberjackLogger)
}

func getEncoder() zapcore.Encoder {
	encoder := zap.NewDevelopmentEncoderConfig()
	encoder.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewConsoleEncoder(encoder)
}

func getLeve(viper *viper.Viper) (*zapcore.Level, error) {
	l := new(zapcore.Level)
	if err := l.UnmarshalText([]byte(viper.GetString("log.level"))); err != nil {
		fmt.Printf("read log level faile:%s\n", err.Error())
		return nil, err
	}
	return l, nil
}

func (logger *ZapLogger) GinLogger() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		path := ctx.Request.URL.Path
		raw := ctx.Request.URL.RawQuery

		ctx.Next()
		cost := time.Since(start)

		zap.L().Sugar().Info(
			zap.String("path", path),
			zap.Int("Status", ctx.Writer.Status()),
			zap.String("query", raw),
			zap.String("IP", ctx.ClientIP()),
			zap.String("User-Agent", ctx.Request.UserAgent()),
			zap.Any("ERROR", ctx.Errors.ByType(gin.ErrorTypePrivate)),
			zap.Duration("cost", cost),
		)
	}
}

func (logger *ZapLogger) GinRecover(stack bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				var isBrokenPipe bool
				if ne, ok := rec.(*net.OpError); ok {
					// Broken pipe（管道破裂） 通常发生在服务器向客户端写数据时，客户端已经关闭了连接（例如用户刷新页面、关闭浏览器、网络中断）。
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if se.Err == syscall.EPIPE || se.Err == syscall.ECONNRESET {
							isBrokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(ctx.Request, false)
				if isBrokenPipe {
					zap.L().Sugar().Error(
						zap.String("path", ctx.Request.URL.Path),
						zap.Any("error", rec),
						zap.String("request", string(httpRequest)),
					)
					ctx.Error(fmt.Errorf("%v", rec))
					ctx.Abort()
					return
				}
				if stack {
					zap.L().Sugar().Error("[Recover from panic]",
						zap.Any("error", rec),
						zap.String("request", string(httpRequest)),
						zap.String("stack", string(debug.Stack())),
					)
				} else {
					zap.L().Sugar().Error("[Recover from panic]",
						"error", rec,
						"request", string(httpRequest),
					)
				}
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
		}()

		ctx.Next()
	}
}
