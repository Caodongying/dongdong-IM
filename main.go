package main

import (
	"github.com/Caodongying/dongdong-IM/utils/config"
	"github.com/Caodongying/dongdong-IM/utils/logger"
)

func main() {
	config.InitViper()
	logger.InitZap()
}