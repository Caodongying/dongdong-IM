package user

import (
	"context"
	"time"

	"github.com/Caodongying/dongdong-IM/model"
	"github.com/Caodongying/dongdong-IM/pool"
	"github.com/Caodongying/dongdong-IM/proto/user"
	"github.com/Caodongying/dongdong-IM/utils/encrypt"
	"github.com/Caodongying/dongdong-IM/utils/jwt"
	"github.com/Caodongying/dongdong-IM/utils/logger"
	"github.com/Caodongying/dongdong-IM/utils/mysql"
	"github.com/Caodongying/dongdong-IM/utils/redis"
	customSync "github.com/Caodongying/dongdong-IM/utils/sync"
	"github.com/Caodongying/dongdong-IM/utils/trace"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UserServiceV1 struct {
	user.UnimplementedUserServiceServer
}

func NewUserServiceV1() *UserServiceV1 {
	return &UserServiceV1{}
}

// Register 注册接口
// 基本逻辑：
// 参数校验 -> 检查邮箱是否已存在 -> 密码加密 -> 生成用户ID并入库 -> 异步更新缓存
func (s *UserServiceV1) Register(ctx context.Context, req *user.RegisterRequest) (*user.RegisterResponse, error) {
	traceID := trace.GetTraceID(ctx)

	// 1. 参数校验
	if req.Username == "" || req.Password == "" || req.Email == "" {
		logger.Sugar.Warn("注册参数无效",
			zap.String("trace_id", traceID),
			zap.String("username", req.Username))
		return &user.RegisterResponse{
			Base: &user.BaseResponse{
				Code:    user.ErrorCode_ERROR_CODE_PARAM_INVALID,
				Message: "用户名、密码和邮箱不能为空",
			},
		}, nil
	}

	// 2. 检查用户 email 是否已经存在，使用分片锁（defer 解锁，避免 panic 泄露）
	// 这里要加锁，因为【检查 email 是否存在 + 创建用户】是原子操作。
	// 为了避免大量主键冲突错误，需要在应用层加锁
	customSync.UserShardLock.Lock(req.Email)
	defer customSync.UserShardLock.Unlock(req.Email)

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
				Code:    user.ErrorCode_ERROR_CODE_SYSTEM_ERROR,
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
				Code:    user.ErrorCode_ERROR_CODE_USER_EXIST,
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
				Code:    user.ErrorCode_ERROR_CODE_SYSTEM_ERROR,
				Message: "注册失败，请稍后再试",
			},
		}, nil
	}

	// 4. 生成用户 ID 并入库
	userID := uuid.NewString()

	// newUser 必须是指针类型，因为调用 Create(newUser) 插入数据时，
	// 数据库会自动生成一些字段（比如 CreatedAt/UpdatedAt 时间戳等），
	// GORM 需要把这些数据库生成的值写回传入的对象中。
	newUser := &model.User{
		ID:       userID,
		Username: req.Username,
		Email:    req.Email,
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
				Code:    user.ErrorCode_ERROR_CODE_SYSTEM_ERROR,
				Message: "注册失败，请稍后再试",
			},
		}, nil
	}

	// 5. 协程池异步更新缓存
	// 缓存更新用异步的原因————
	// 缓存更新失败不应该影响注册的成功返回。
	// 最坏情况：缓存没更新，下次查询走数据库（缓存 miss），不影响正确性。
	err = pool.SubmitWithRetry(ctx, func(taskCtx context.Context) {
		cacheCtx, cacheCancel := context.WithTimeout(taskCtx, 2*time.Second)
		defer cacheCancel()

		if cacheErr := redis.SetUserCache(cacheCtx, userID, req.Email); cacheErr != nil {
			logger.Sugar.Error("异步更新用户缓存失败",
				zap.String("trace_id", trace.GetTraceID(taskCtx)),
				zap.String("user_id", userID),
				zap.Error(cacheErr))
		}
	}, 3, 100*time.Millisecond)

	if err != nil {
		// 缓存任务提交失败不影响注册结果，只记日志
		logger.Sugar.Warn("缓存任务提交失败",
			zap.String("trace_id", traceID),
			zap.String("user_id", userID),
			zap.Error(err))
	}

	logger.Sugar.Info("用户注册成功",
		zap.String("trace_id", traceID),
		zap.String("user_id", userID),
		zap.String("email", req.Email))

	return &user.RegisterResponse{
		Base: &user.BaseResponse{
			Code:    user.ErrorCode_ERROR_CODE_SUCCESS,
			Message: "注册成功",
		},
		UserId: userID,
	}, nil
}

// Login 登录接口
// 参数校验 -> 查询用户 -> 验证密码 -> 签发 JWT Token
func (s *UserServiceV1) Login(ctx context.Context, req *user.LoginRequest) (*user.LoginResponse, error) {
	traceID := trace.GetTraceID(ctx)

	// 1. 参数校验
	if req.Email == "" || req.Password == "" {
		return &user.LoginResponse{
			Base: &user.BaseResponse{
				Code:    user.ErrorCode_ERROR_CODE_PARAM_INVALID,
				Message: "邮箱和密码不能为空",
			},
		}, nil
	}

	// 2. 查询用户
	queryCtx, queryCancel := context.WithTimeout(ctx, 1*time.Second)
	defer queryCancel()

	var dbUser model.User
	err := mysql.DB.WithContext(queryCtx).
		Where("email = ?", req.Email).
		First(&dbUser).Error
	if err != nil {
		logger.Sugar.Info("登录失败，用户不存在",
			zap.String("trace_id", traceID),
			zap.String("email", req.Email))
		// 不区分"用户不存在"和"密码错误"，防止枚举攻击
		return &user.LoginResponse{
			Base: &user.BaseResponse{
				Code:    user.ErrorCode_ERROR_CODE_PASSWORD_ERROR,
				Message: "邮箱或密码错误",
			},
		}, nil
	}

	// 3. 验证密码
	// bcrypt 的哈希值中包含了 salt 和 cost factor，
	// CompareHashAndPassword 会从哈希值中解析出这些参数，
	// 用同样的规则对输入密码再算一次哈希，然后对比。
	// 所以不需要单独存 salt，这是 bcrypt 相比 MD5+salt 的优势。
	if !encrypt.BcryptVerify(req.Password, dbUser.Password) {
		logger.Sugar.Info("登录失败，密码错误",
			zap.String("trace_id", traceID),
			zap.String("email", req.Email))
		return &user.LoginResponse{
			Base: &user.BaseResponse{
				Code:    user.ErrorCode_ERROR_CODE_PASSWORD_ERROR,
				Message: "邮箱或密码错误",
			},
		}, nil
	}

	// 4. 签发 JWT Token
	token, err := jwt.GenerateToken(dbUser.ID, dbUser.Email)
	if err != nil {
		logger.Sugar.Error("JWT 签发失败",
			zap.String("trace_id", traceID),
			zap.String("user_id", dbUser.ID),
			zap.Error(err))
		return &user.LoginResponse{
			Base: &user.BaseResponse{
				Code:    user.ErrorCode_ERROR_CODE_SYSTEM_ERROR,
				Message: "登录失败，请稍后再试",
			},
		}, nil
	}

	logger.Sugar.Info("用户登录成功",
		zap.String("trace_id", traceID),
		zap.String("user_id", dbUser.ID))

	return &user.LoginResponse{
		Base: &user.BaseResponse{
			Code:    user.ErrorCode_ERROR_CODE_SUCCESS,
			Message: "登录成功",
		},
		UserId:      dbUser.ID,
		AccessToken: token,
	}, nil
}

// checkEmailExist 检查用户 email 是否存在
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
