package user

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/Caodongying/dongdong-IM/model"
	"github.com/Caodongying/dongdong-IM/proto/user"
	"github.com/Caodongying/dongdong-IM/utils/encrypt"
	"github.com/Caodongying/dongdong-IM/utils/logger"
	"github.com/Caodongying/dongdong-IM/utils/mysql"
	"github.com/Caodongying/dongdong-IM/utils/sync"
	"github.com/Caodongying/dongdong-IM/utils/trace"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UserServiceV1 struct{
	user.UnimplementedUserServiceServer
}

func NewUserServiceV1() *UserServiceV1 {
	return &UserServiceV1{}
}

// Register注册接口
// 基本逻辑：
// 参数校验，检查用户名是否已经存在，对密码加密，生成用户ID并入库
// 协程池异步更新缓存，提交异步任务
func (s *UserServiceV1) Register(ctx context.Context, req *user.RegisterRequest) (*user.RegisterResponse, error) {
	traceID := trace.GetTraceID(ctx)

	// 1. 参数校验
	if req.Username == "" || req.Password == "" || req.Email == "" {
		logger.Sugar.Warn("注册参数无效",
			zap.String("trace_id", traceID), // 日志携带TraceID
			zap.String("username", req.Username))
		return &user.RegisterResponse{
			Base: &user.BaseResponse{
				Code: user.ErrorCode_ERROR_CODE_PARAM_INVALID,
				Message: "用户名、密码和邮箱不能为空",
			},
		}, nil
	}

	// 2. 检查用户email是否已经存在，使用分片锁（defer解锁，避免panic泄露）
	// 这里要加锁，因为【检查email是否存在+创建用户】是原子操作。为了避免大量主键冲突错误，需要在应用层加锁
	sync.UserShardLock.Lock(req.Email)
	defer sync.UserShardLock.Unlock(req.Email)

	checkCtx, checkCancel := context.WithTimeout(ctx, 1*time.Second)
	defer checkCancel()

	emailExists, err := s.checkEmailExist(checkCtx, req.Email)
	if err != nil {
		logger.Sugar.Error("检查email是否存在失败",
			zap.String("trace_id", traceID),
			zap.String("email", req.Email),
			zap.Error(err))
		return &user.RegisterResponse{
			Base: &user.BaseResponse{
				Code: user.ErrorCode_ERROR_CODE_SYSTEM_ERROR,
				Message: "注册失败，请稍后再试",
			},
		}, nil
	}

	if emailExists {
		logger.Sugar.Info("注册失败，email已存在",
			zap.String("trace_id", traceID),
			zap.String("email", req.Email))
		return &user.RegisterResponse{
			Base: &user.BaseResponse{
				Code: user.ErrorCode_ERROR_CODE_USER_EXIST,
				Message: "该邮箱已被注册",
			},
		}, nil
	}

	// 3. 密码加密
	hashPwd, err := encrypt.BcryptEncrypt(req.Password)
	if err != nil {
		logger.Sugar.Error("密码加密失败",
			zap.String("trace_id", traceID),
			zap.String("email", req.Email),
			zap.Error(err))
		return &user.RegisterResponse{
			Base: &user.BaseResponse{
				Code: user.ErrorCode_ERROR_CODE_SYSTEM_ERROR,
				Message: "注册失败，请稍后再试",
			},
		}, nil
	}

	// 4. 生成用户ID并入库
	userID := uuid.NewString()

	// newUser必须是指针类型，因为调用Create(newUser)插入数据时，
	// 数据库会自动生成一些字段（比如CreatedAt/UpdatedAt时间戳等），
	// GORM 需要把这些数据库生成的值写回传入的对象中。
	// GORM 对所有CURD方法（Create/Update/Delete/Find 等）都统一要求传入指针
	newUser := &model.User{
		ID: userID,
		Username: req.Username,
		Email: req.Email,
		Password: hashPwd,
	}

	createCtx, createCancel := context.WithTimeout(ctx, 2*time.Second)
	defer createCancel()

	if err := mysql.DB.WithContext(createCtx).Create(newUser).Error; err != nil {
		logger.Sugar.Error("用户入库失败",
			zap.String("trace_id", traceID),
			zap.String("email", req.Email),
			zap.String("user_id", userID),
			zap.Error(err))
		return &user.RegisterResponse{
			Base: &user.BaseResponse{
				Code: user.ErrorCode_ERROR_CODE_SYSTEM_ERROR,
				Message: "注册失败，请稍后再试",
			},
		}, nil
	}

	// 5. 协程池异步更新缓存

	// 提交异步任务


}

// 检查用户email是否存在
func (s *UserServiceV1) checkEmailExist(ctx context.Context, email string) (bool, error) {
	var count int64

	err := mysql.DB.WithContext(ctx).
		Model(&model.User{}).
		Where("email = ?", email).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}