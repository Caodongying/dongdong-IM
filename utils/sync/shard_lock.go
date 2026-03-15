// 分片锁，解决全局锁竞争

package sync

import (
	"sync"
	"hash/fnv"
)

// 分片锁
type ShardLock struct {
	shards []*sync.Mutex
	num    int // 分片数
}

// 全局用户分片锁（用户模块专用）
// 这是package包级别的变量声明。其初始化发生在程序启动时、包被首次导入的时候，而且只执行一次
// 整个程序运行期间，UserShardLock 只会被创建一次
var UserShardLock = NewShardLock(32) // 32分片，兼顾性能和竞争

func NewShardLock(num int) *ShardLock {
	if num <= 0 {
		num = 16
	}
	shards := make([]*sync.Mutex, num)
	for i := 0; i < num; i++ {
		shards[i] = &sync.Mutex{} // 为每个分片初始化一把独立的互斥锁
	}

	// 必须返回指针，因为如果返回值类型ShardLock{}
	// 每次调用就会复制整个shards切片，这样每一次调用UserShardLock.Lock("user1")
	// 都会拷贝UserShardLock，这样锁就不冲突了
	// Go里，值类型的方法调用会隐式拷贝整个值
	return &ShardLock{
		shards: shards,
		num:    num,
	}
}

// 根据key获取对应的分片锁————
// 1. 通过哈希函数将任意字符串类型的key转换成一个数值
// 2. 对分片数取模，确保结果落在0~num-1范围内,从而映射到唯一的一把锁
func (sl *ShardLock) getLock(key string) *sync.Mutex {
	h := fnv.New32a() // 创建FNV-32a哈希函数（高效、低碰撞）
	h.Write([]byte(key)) // 将key转换成字节并写入哈希函数
	return sl.shards[h.Sum32()%uint32(sl.num)] // 哈希值取模，映射到对应分片的锁
}


func (sl *ShardLock) Lock(key string) {
	sl.getLock(key).Lock()
}


func (sl *ShardLock) Unlock(key string) {
	sl.getLock(key).Unlock()
}