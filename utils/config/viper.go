package config

import (
	"path/filepath"
	fs "github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var Viper *configManager

type Config struct {
	Server struct {
		HTTPPort        string `mapstructure:"http_port"`
		GRPCPort        string `mapstructure:"grpc_port"`
		Mode            string `mapstructure:"mode"`
		ShutdownTimeout int    `mapstructure:"shutdown_timeout"`
	} `mapstructure:"server"`
	Logger struct {
		Level      string `mapstructure:"level"`
		Filename   string `mapstructure:"filename"`
		MaxSize    int    `mapstructure:"max_size"`
		MaxBackups int    `mapstructure:"max_backups"`
		MaxAge     int    `mapstructure:"max_age"`
		Compress   bool   `mapstructure:"compress"`
	} `mapstructure:"logger"`
	MySQL struct {
		DSN             string `mapstructure:"dsn"`
		MaxOpenConns    int    `mapstructure:"max_open_conns"`
		MaxIdleConns    int    `mapstructure:"max_idle_conns"`
		ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
		ConnMaxIdleTime int    `mapstructure:"conn_max_idle_time"`
	} `mapstructure:"mysql"`
	Redis struct {
		Addr         string `mapstructure:"addr"`
		Password     string `mapstructure:"password"`
		DB           int    `mapstructure:"db"`
		PoolSize     int    `mapstructure:"pool_size"`
		MinIdleConns int    `mapstructure:"min_idle_conns"`
		ConnMaxLifetime int `mapstructure:"conn_max_lifetime"`
		LockExpire   int    `mapstructure:"lock_expire"`
		LockRenew    int    `mapstructure:"lock_renew"`
	} `mapstructure:"redis"`
	JWT struct {
		Secret  string `mapstructure:"secret"`
		Expire  int    `mapstructure:"expire"`
		Issuer  string `mapstructure:"issuer"`
	} `mapstructure:"jwt"`
	Pool struct {
		CoreSize    int `mapstructure:"core_size"`
		MaxSize     int `mapstructure:"max_size"`
		QueueSize   int `mapstructure:"queue_size"`
		IdleTimeout int `mapstructure:"idle_timeout"`
	} `mapstructure:"pool"`
	Limit struct {
		Rate  int `mapstructure:"rate"`
		Burst int `mapstructure:"burst"`
	} `mapstructure:"limit"`
	Snowflake struct {
		MachineID int64 `mapstructure:"machine_id"`
	} `mapstructure:"snowflake"`
	WebSocket struct {
		WriteWait      int `mapstructure:"write_wait"`
		PongWait       int `mapstructure:"pong_wait"`
		PingPeriod     int `mapstructure:"ping_period"`
		MaxMessageSize int `mapstructure:"max_message_size"`
		SendBufferSize int `mapstructure:"send_buffer_size"`
	} `mapstructure:"websocket"`
}

type configManager struct {
	v *viper.Viper
	c *Config
}

// 初始化Viper
func Init() {
	v := viper.New()
	// 配置文件路径
	v.SetConfigFile(filepath.Join("config", "config.yaml"))
	v.SetConfigType("yaml")
	// 读取配置
	if err := v.ReadInConfig(); err != nil {
		panic("无法读取config配置文件: " + err.Error())
	}
	// 映射到结构体
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		panic("无法解析config配置: " + err.Error())
	}
	// 开启热更新
	// 生效前提：业务代码需实时从 Viper.GetConfig() 获取配置，而非启动时缓存，否则热重载的配置无法生效。
	v.WatchConfig()
	v.OnConfigChange(func(e fs.Event) {
		zap.L().Info("重新加载配置文件...", zap.String("file", e.Name))
		if err := v.Unmarshal(&c); err != nil {
			zap.L().Error("重新加载配置失败", zap.Error(err))
		}
	})
	// 赋值全局变量
	Viper = &configManager{
		v: v,
		c: &c,
	}
	zap.L().Info("成功加载config配置文件", zap.String("file", v.ConfigFileUsed()))
}

// GetConfig 获取强类型配置
func (cm *configManager) GetConfig() *Config {
	return cm.c
}

// GetViper 获取原生Viper实例
func (cm *configManager) GetViper() *viper.Viper {
	return cm.v
}
