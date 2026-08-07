package pkg

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

var Logger = newLogger()

type Formatter struct {
	logrus.TextFormatter
}

func newLogger() *logrus.Logger {
	l := logrus.New()

	// 开发环境用文本格式（带颜色），生产环境用 JSON
	env := os.Getenv("LAN_IM_ENV")
	if env == "production" {
		l.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05.000",
		})
	} else {
		l.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05.000",
		})
	}

	// 默认输出到 stdout，可通过环境变量指定日志文件
	logFile := os.Getenv("LAN_IM_LOG_FILE")
	if logFile != "" {
		if err := os.MkdirAll(filepath.Dir(logFile), 0755); err == nil {
			f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {
				l.SetOutput(io.MultiWriter(os.Stdout, f))
			}
		}
	} else {
		l.SetOutput(os.Stdout)
	}

	// 日志级别默认 Info，可通过环境变量调整
	level := os.Getenv("LAN_IM_LOG_LEVEL")
	switch strings.ToLower(level) {
	case "debug":
		l.SetLevel(logrus.DebugLevel)
	case "warn":
		l.SetLevel(logrus.WarnLevel)
	case "error":
		l.SetLevel(logrus.ErrorLevel)
	default:
		l.SetLevel(logrus.InfoLevel)
	}

	return l
}

// WithCaller 返回带有调用者信息的 Entry，用于定位日志输出位置
func WithCaller() *logrus.Entry {
	_, file, line, ok := runtime.Caller(2)
	if ok {
		// 只保留文件名，不要全路径
		short := filepath.Base(file)
		return Logger.WithField("src", fmt.Sprintf("%s:%d", short, line))
	}
	return logrus.NewEntry(Logger)
}

// 便捷方法 —— 带调用者信息
func Debug(args ...interface{})                 { WithCaller().Debug(args...) }
func Debugf(format string, args ...interface{}) { WithCaller().Debugf(format, args...) }
func Info(args ...interface{})                  { WithCaller().Info(args...) }
func Infof(format string, args ...interface{})  { WithCaller().Infof(format, args...) }
func Warn(args ...interface{})                  { WithCaller().Warn(args...) }
func Warnf(format string, args ...interface{})  { WithCaller().Warnf(format, args...) }
func Error(args ...interface{})                 { WithCaller().Error(args...) }
func Errorf(format string, args ...interface{}) { WithCaller().Errorf(format, args...) }
func Fatal(args ...interface{})                 { WithCaller().Fatal(args...) }
func Fatalf(format string, args ...interface{}) { WithCaller().Fatalf(format, args...) }

func Infoln(args ...interface{})                 { WithCaller().Infoln(args...) }
