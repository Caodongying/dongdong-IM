package config

import (
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var Viper *configManager

type Config struct {
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
}

type configManager struct {
	v *viper.Viper
	c *configManager
}

func InitViper() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	// viper.AddConfigPath("../config") // 适配不同启动目录

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		panic("配置文件读取失败: " + err.Error())
	}

	viper.AutomaticEnv()
	fmt.Println("配置初始化成功")
}