package core

import "fmt"

// Error 是可直接展示给用户的业务错误；其他 error 表示意外故障。
type Error struct{ msg string }

func (e *Error) Error() string { return e.msg }

// Fail 构造业务错误，格式化消息。
func Fail(format string, args ...any) error { return &Error{msg: fmt.Sprintf(format, args...)} }
