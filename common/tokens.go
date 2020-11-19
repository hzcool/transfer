package common

import (
	"fmt"
	"os"
	"sync"
	"time"
)
var sm sync.Map  // 储存所有的token

func GetNewToken(expire time.Duration) string {
	token := NewGuid()
	sm.Store(token,1)
	time.AfterFunc(expire, func() {
		fmt.Println(token)
		sm.Delete(token)
	})
	return token
}

func ExistToken(token string) bool {
	_,ok := sm.Load(token)
	return ok
}

func NewGuid() string  {
	f, _ := os.OpenFile("/dev/urandom", os.O_RDONLY, 0)
	b := make([]byte, 16)
	f.Read(b)
	f.Close()
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}