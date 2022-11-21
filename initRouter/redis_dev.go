//go:build dev

package initRouter

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
)

func InitRedis() {
	client := redis.NewClient(&redis.Options{
		//Addr: "xx.xx.xx.xx:6379",
		Addr: "127.0.0.1:6379",
		//Password: redisConf.Password, // no password set
		DB: 0, // 0 means to use default DB
	})
	pong, err := client.Ping(context.Background()).Result()
	if err != nil {
		log.Printf("redis connect ping failed, err: %v", err)
		panic(err)
	} else {
		log.Printf("redis connect ping response:%v \"pong\"", pong)
		GVA_REDIS = client
		fmt.Println("Redis连接成功!!!")
	}
}
