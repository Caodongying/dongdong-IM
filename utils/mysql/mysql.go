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
// 目的是让 GORM 的 SQL 日志能顺利输出到 Zap 中（而不是自己另起炉灶打印到控制台）
type ZapGormLogger struct {
	ZapLogger *zap.SugaredLogger
}

// Printf 是 GORM 对日志写入器的要求（logger.Writer 接口）
// 当 GORM 想打印日志时，会调用这个方法，然后我们转发给 Zap
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
	// 利用了 GORM 内置的 gormLogger.New() 来处理复杂的 SQL 追踪逻辑（比如计算执行时间、判断慢查询），
	// 只是把最终写日志的动作，通过 Printf 转发给了 Zap。
	dbLogger := gormLogger.New(
		zapWriter,
		gormLogger.Config{
			SlowThreshold:             time.Second, // 慢查询阈值（超过1秒记录慢查询）
			LogLevel:                  gormLogger.Info, // 日志级别：Info会输出所有SQL
			IgnoreRecordNotFoundError: true,        // 当你用 db.First(&user) 没找到记录时，GORM 会返回 gorm.ErrRecordNotFound。在生产环境中，这通常不是系统错误，而是业务逻辑的一部分。开启这个选项后，这种"未找到"的情况就不会被记录为 Error 级别的日志，避免了大量的日志噪音。
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

	// 主要是为了应对 MySQL 的 wait_timeout 机制。MySQL 默认 8 小时无活动就会主动断开连接，
	// 但 Go 这边并不知道，以为这个连接还能用。结果一用就报 driver: bad connection。
	// 所以 ConnMaxLifetime 应该设置为比数据库 wait_timeout 更短的值，比如数据库是 8 小时，你就设为 4-6 小时，让 Go 主动、安全地换掉老连接
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

	// 全局存 gorm.DB 是为了用起来爽（ORM语法糖），关闭时拿 sql.DB 是为了干脏活（底层资源释放）。
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
	// MySQL 的连接字符串里包含明文密码，直接打到日志里是巨大的安全风险。这个函数用正则把 user:password@tcp... 里的密码部分替换成了 ***
	// 例如：root:123456@tcp(127.0.0.1:3306)/test → root:***@tcp(127.0.0.1:3306)/test
	// 匹配 : 到 @ 之间的所有字符（密码部分）
	reg := regexp.MustCompile(`(:[^:@]+)@`)

	return reg.ReplaceAllString(dsn, ":***@")

}