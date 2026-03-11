package mysql

import (
	"time"

	"github.com/Caodongying/dongdong-IM/utils/config"
	"github.com/Caodongying/dongdong-IM/utils/logger"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"go.uber.org/zap"
)

var DB *gorm.DB

// 初始化MySQL
func Init() {
	cfg := config.Viper.GetConfig().MySQL
	// Gorm日志配置（对接Zap）
	gormLogger := logger.New(
		logger.Sugar,
		logger.Config{
			SlowThreshold:             time.Second, // 慢查询阈值
			LogLevel:                  logger.Info, // 日志级别
			IgnoreRecordNotFoundError: true,        // 忽略记录不存在错误
			Colorful:                  false,       // 非彩色输出
		},
	)
	// 连接MySQL
	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		logger.Sugar.Panic("mysql connect failed", zap.Error(err))
	}
	// 获取底层sql.DB，配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		logger.Sugar.Panic("get mysql sql.DB failed", zap.Error(err))
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTime) * time.Second)
	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		logger.Sugar.Panic("mysql ping failed", zap.Error(err))
	}
	DB = db
	logger.Sugar.Info("mysql init success", zap.String("dsn", cfg.DSN))
}

// Close 优雅关闭MySQL连接
func Close() {
	sqlDB, err := DB.DB()
	if err != nil {
		logger.Sugar.Error("get mysql sql.DB failed when close", zap.Error(err))
		return
	}
	if err := sqlDB.Close(); err != nil {
		logger.Sugar.Error("mysql close failed", zap.Error(err))
		return
	}
	logger.Sugar.Info("mysql closed success")
}