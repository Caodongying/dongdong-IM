package user

import (
	"context"
	"go.uber.org/zap"
	"github.com/Caodongying/dongdong-IM/utils/logger"
	user "github.com/Caodongying/dongdong-IM/proto/user"
	"github.com/Caodongying/dongdong-IM/utils/trace"
)

type UserServiceV1 struct{
	user.UnimplementedUserServiceServer
}

func NewUserServiceV1() *UserServiceV1 {
	return &UserServiceV1{}
}

// Register注册接口
// 基本逻辑——
// 参数校验，检查用户名是否已经存在，对密码加密，生成用户ID并入库
// 协程池异步更新缓存，提交异步任务
func (s *UserServiceV1) Register(ctx context.Context, req *user.RegisterRequest) (*user.RegisterResponse, error) {
	traceID := trace.GetTraceID(ctx)
	// 参数校验
	if req.Username == "" || req.Password == "" || req.Email == "" {
		logger.Sugar.Warn("注册参数无效",
			zap.String("trace_id", traceID), // 日志携带TraceID
			zap.String("username", req.Username))
		return &user.RegisterResponse{
			Base: &user.BaseResponse{
				Code: user.ErrorCode_ERROR_CODE_PARAM_INVALID,
				Message: "参数无效",
			},
		}, nil
	}

	// 检查email是否已经存在(defer解锁，避免panic泄露)



}

func (s *UserServiceV1) checkEmailExist(ctx context.Context, email string) (bool, error) {
	var count int // int64

	err := mysql.DB.WithContext(ctx).
		
}