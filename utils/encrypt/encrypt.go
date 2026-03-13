// 使用bcrypt 加密算法，是不可逆加密

package encrypt

import (
	"golang.org/x/crypto/bcrypt"
	"github.com/Caodongying/dongdong-IM/utils/logger"
	"go.uber.org/zap"
)

// bcrypt加密成本因子（取值范围4-31，生产环境常用10-12）
// 数值越大，加密时的计算量越大、耗时越长，破解难度也越高
const bcryptCost = 12

// 密码加密函数
func BcryptEncrypt(password string) (string, error) {
	// 生成密码的哈希值
	// GenerateFromPassword 会自动生成随机盐（salt）
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		logger.Sugar.Error("bcrypt加密失败", zap.Error(err))
		return "", err
	}
	return string(hash), nil
}

// 密码验证
func BcryptVerify(password, hash string) bool {
	// CompareHashAndPassword 会从哈希值中解析出盐和成本因子
	// 然后用同样的规则计算输入密码的哈希值，再对比是否一致
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}