package user

import (
	"context"
	"time"

	pb "github.com/Caodongying/dongdong-IM/proto/user"
)

type UserService struct{
	server pb.UnimplementedUserServiceServer
}

func (s *UserService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {

}