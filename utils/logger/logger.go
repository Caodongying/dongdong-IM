// Zap日志，含 traceID + 文件切割 + 结构化

package logger

import (
	"os"

	"github.com/Caodongying/dongdong-IM/utils/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"fmt"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Log *zap.Logger
	Sugar *zap.SugaredLogger
)

// 初始化Zap日志
func Init() {
	cfg := config.Viper.GetConfig().Logger
	// 日志写入器（文件切割）
	w := lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}
	// 编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	// 日志级别
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		// 日志级别解析失败，使用默认值，但是记录警告
		zap.L().Warn(fmt.Sprintf("无效的日志级别: %s，使用默认级别info", level))
		level = zapcore.InfoLevel
	}
	// 核心配置（控制台+文件双输出）
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig), // 内容怎么写
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(&w)),// 写到哪里
		level, // 写的级别
	)
	// 开启调用者信息+堆栈跟踪
	Log = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	Sugar = Log.Sugar()
	// 替换全局zap日志
	zap.ReplaceGlobals(Log)
	Sugar.Info("Zap logger初始化成功")
}

// Close 关闭日志（刷新缓冲区）
func Close() {
	_ = Log.Sync()
	_ = Sugar.Sync()
}
