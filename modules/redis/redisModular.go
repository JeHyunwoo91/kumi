package redis

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/captv/kumi/modules/log"
	"github.com/garyburd/redigo/redis"
)

var Node_Env string

var (
	Pool *redis.Pool
)

var logger = log.NewLogger("REDIS")

func init() {
	Pool = newPool("127.0.0.1:6379")
	cleanupHook()
}

func newPool(server string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server)
			if err != nil {
				return nil, err
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

func cleanupHook() {
	logger.Debug("cleanupHook redis Pool")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	signal.Notify(c, syscall.SIGKILL)
	go func() {
		<-c
		Pool.Close()
		os.Exit(0)
	}()
}

// Get ...
func Get(key string) (data []byte, err error) {
	logger.Debug("Get ", key)
	conn := Pool.Get()
	defer conn.Close()

	data, err = redis.Bytes(conn.Do("GET", key))
	if err != nil {
		// logger.Debug("Error getting key [", key, "]: ", err)
		return
	}
	logger.Debug("Success getting key [", key, "]")
	return
}

// Set ...
func Set(key string, value []byte) (err error) {
	logger.Debug("Set ", key)
	conn := Pool.Get()
	defer conn.Close()

	v := string(value)
	if len(v) > 15 {
		v = v[0:12] + "..."
	}

	_, err = conn.Do("SET", key, value)
	if err != nil {
		return fmt.Errorf("Error setting key %s to %s: %v", key, v, err)
	}

	logger.Debug("Success setting key [", key, "] to ", v)
	return
}

// Setex ...
func Setex(key string, deadline int, value []byte) (err error) {
	logger.Debug("Setex ", key)
	conn := Pool.Get()
	defer conn.Close()

	v := string(value)
	if len(v) > 15 {
		v = v[0:12] + "..."
	}

	_, err = conn.Do("SETEX", key, deadline, value)
	if err != nil {
		return fmt.Errorf("Error setting key by expired %s to %s: %v", key, v, err)
	}

	logger.Debug("Success setting key by expired [", key, "] to ", v)
	return
}
