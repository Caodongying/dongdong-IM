package mysql

import (
	"context"
	"time"
	"regexp"

	"github.com/Caodongying/dongdong-IM/utils/config"
	customLogger "github.com/Caodongying/dongdong-IM/utils/logger"
	gormLogger "gorm.io/gorm/logger"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"go.uber.org/zap"
)

var DB *gorm.DB

// ZapGormLogger - 适配GORM的logger.Writer接口的Zap日志包装器
type ZapGormLogger struct {
	ZapLogger *zap.SugaredLogger
}

// Printf是gorm.io/gorm/logger.Writer接口的核心方法
// GORM会调用这个方法输出日志，因此需要将其转发到Zap
func (z *ZapGormLogger) Printf(format string, v ...interface{}) {
	z.ZapLogger.Infof(format, v...)
}


func Init() {
	if customLogger.Sugar == nil {
		panic("logger未初始化，请先调用logger.Init()")
	}

	cfg := config.Viper.GetConfig().MySQL
	// 验证DSN是否为空
	if cfg.DSN == "" {
		customLogger.Sugar.Panic("MySQL DSN配置为空")
	}

	// 1. 创建Zap到GORM的日志适配器
	zapWriter := &ZapGormLogger{
		ZapLogger: customLogger.Sugar,
	}

	// 2. 配置GORM日志
	dbLogger := gormLogger.New(
		zapWriter,
		gormLogger.Config{
			SlowThreshold:             time.Second, // 慢查询阈值（超过1秒记录慢查询）
			LogLevel:                  gormLogger.Info, // 日志级别：Info会输出所有SQL
			IgnoreRecordNotFoundError: true,        // 忽略"记录不存在"的错误日志
			Colorful:                  false,       // 关闭彩色输出（JSON日志不需要）
		},
	)

	// 3. 连接MySQL
	customLogger.Sugar.Debug("开始连接MySQL", zap.String("dsn", maskDSN(cfg.DSN))) // 脱敏输出DSN
	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		Logger: dbLogger,
	})
	if err != nil {
		customLogger.Sugar.Panic("开始连接MySQL", zap.Error(err))
	}

	// 4. 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		customLogger.Sugar.Panic("获取sql.DB实例失败", zap.Error(err))
	}
	// 设置连接池参数
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTime) * time.Second)

	// 5. 测试连接（带上下文）
	if err := sqlDB.PingContext(context.Background()); err != nil {
		customLogger.Sugar.Panic("mysql ping失败", zap.Error(err))
	}

	DB = db
	customLogger.Sugar.Info("mysql初始化成功", zap.String("dsn", maskDSN(cfg.DSN)))
}

// Close 优雅关闭MySQL连接
func Close() {
	if DB == nil {
		customLogger.Sugar.Warn("mysql DB实例为空，跳过关闭")
		return
	}

	sqlDB, err := DB.DB()
	if err != nil {
		customLogger.Sugar.Error("获取sql.DB实例失败（关闭时）", zap.Error(err))
		return
	}

	if err := sqlDB.Close(); err != nil {
		customLogger.Sugar.Error("mysql关闭失败", zap.Error(err))
		return
	}

	customLogger.Sugar.Info("mysql连接已优雅关闭")
}

// maskDSN 脱敏DSN，避免日志中泄露密码
func maskDSN(dsn string) string {
	// 替换DSN中的密码部分为***
	// 例如：root:123456@tcp(127.0.0.1:3306)/test → root:***@tcp(127.0.0.1:3306)/test
	// 匹配 : 到 @ 之间的所有字符（密码部分）
	reg := regexp.MustCompile(`(:).+?(@)`)

	return reg.ReplaceAllString(dsn, "***")

}