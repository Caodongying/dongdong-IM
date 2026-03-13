// JWT 生成与解析，含过期验证 + 签发者验证

// 流程概述：
// 程序启动时初始化JWT 密钥、过期时间、签发者 →
// 业务侧调用GenerateToken传入用户信息，生成带签名的JWT字符串 →
// 客户端携带Token请求时，调用ParseToken验证Token的合法性（签名、过期、签发者），解析出用户信息供业务使用。

package jwt

import (
	"time"
	"errors"

	"github.com/golang-jwt/jwt/v4"
	"github.com/Caodongying/dongdong-IM/utils/config"
	"github.com/Caodongying/dongdong-IM/utils/logger"
	"go.uber.org/zap"
)

var jwtSecret []byte // JWT签名密钥
var jwtExpire time.Duration // Token过期时长
var jwtIssuer string

// JWT载荷声明
type Payload struct {
	// 两个自定义字段类似于Cognito里的sub/email
	UserID   string `json:"user_id"`
	Email string `json:"email"`
	jwt.RegisteredClaims
}


func Init() {
	cfg := config.Viper.GetConfig().JWT
	jwtSecret = []byte(cfg.Secret) // 密钥
	jwtExpire = time.Duration(cfg.Expire) * time.Minute // 过期时间
	jwtIssuer = cfg.Issuer // 签发者
	logger.Sugar.Info("jwt初始化成功", zap.String("签发者", jwtIssuer), zap.Duration("过期时间", jwtExpire))
}

// 生成Token
func GenerateToken(userID, email string) (string, error) {
	// 1. 组装载荷：自定义业务字段 + 标准声明字段
	claims := Payload{
		UserID:   userID,
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(jwtExpire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// 2. 指定签名算法（对称加密SHA256）
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 3. 用jwtSecret对 Token 进行签名，防止篡改
	return token.SignedString(jwtSecret)
}

// 解析Token
func ParseToken(tokenStr string) (*Payload, error) {
	// 1. 解析Token字符串，获得自定义Claims
	token, err := jwt.ParseWithClaims(tokenStr, &Payload{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		logger.Sugar.Error("jwt解析失败", zap.Error(err), zap.String("token", tokenStr))
		return nil, err
	}
	// 验证Token有效性
	if claims, ok := token.Claims.(*Payload); ok && token.Valid {
		// 验证签发者
		if claims.Issuer != jwtIssuer {
			logger.Sugar.Warn("jwt签发者不匹配", zap.String("expected", jwtIssuer), zap.String("actual", claims.Issuer))
			return nil, errors.New("token签发者无效")
		}
		return claims, nil
	}
	logger.Sugar.Error("jwt token无效", zap.String("token", tokenStr))
	return nil, errors.New("token无效")
}
