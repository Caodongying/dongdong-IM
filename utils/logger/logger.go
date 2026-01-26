package logger

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"fmt"
	"os"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Logger *zap.Logger
	Sugar *zap.SugaredLogger
)

func InitZap() {
	// 从配置读取参数
	level := viper.GetString("logger.level")
	filename := viper.GetString("logger.filename")
	maxSize := viper.GetInt("logger.max_size")
	maxBackup := viper.GetInt("logger.max_backup")
	maxAge := viper.GetInt("logger.max_age")

	// 解析日志级别
	zapLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		// 日志级别解析失败，使用默认值，但是记录警告
		zap.L().Warn(fmt.Sprintf("无效的日志级别: %s，使用默认级别info", level))
		zapLevel = zapcore.InfoLevel
	}

	// encoder配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "name",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.RFC3339TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 日志文件切割，防止过长日志
	hook := lumberjack.Logger{
		Filename: filename,
		MaxSize: maxSize,
		MaxBackups: maxBackup,
		MaxAge: maxAge,
		Compress: true,
	}

	// 核心配置（console + 文件双输出)
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig), // 内容怎么写
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(&hook)), // 写到哪里
		zapLevel, // 写的级别
	)

	// 构建logger
	Logger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	Sugar = Logger.Sugar()

	defer Logger.Sync()
	defer Sugar.Sync()

	Sugar.Infow("Zap日志初始化成功", zap.String("level", level), zap.String("filename", filename))
}