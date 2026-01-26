package config

import (
	"fmt"

	"github.com/spf13/viper"
)

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