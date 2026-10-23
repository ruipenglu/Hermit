package core

import (
	"net"
	"strconv"
	"time"
)

// CheckPort 确认端口当前可绑定（未被其他进程占用）。
func CheckPort(port int) error {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return Fail("端口 %d 已被占用", port)
	}
	listener.Close()
	return nil
}

// Listening 检查本地端口是否有服务在监听。
func Listening(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
