package pan

import (
	"errors"
	"fmt"
)

const (
	NOERR   int = 0
	UNKNOWN int = 9999
)

type DriverErrorInterface interface {
	GetCode() int
	GetMsg() string
	GetErr() error
	GetData() interface{}
	Error() string
}

// DriverError 定义全局的基础异常
type DriverError struct {
	Code int
	Msg  string
	Err  error
	Data interface{}
}

func (e *DriverError) GetCode() int {
	return e.Code
}

func (e *DriverError) GetMsg() string {
	return e.Msg
}

func (e *DriverError) GetErr() error {
	return e.Err
}

func (e *DriverError) GetData() interface{} {
	return e.Data
}

func (e *DriverError) Error() string {
	m := make(map[string]any)
	m["code"] = e.Code
	m["msg"] = e.Msg
	m["err"] = e.Err
	m["data"] = e.Data
	errorStr := ""
	for key, value := range m {
		if value != nil {
			errorStr = e.appendKeyValue(errorStr, key, value)
		}
	}
	return errorStr
}

func (e *DriverError) appendKeyValue(errorStr, key string, value interface{}) string {
	errorStr = errorStr + key
	errorStr = errorStr + "="
	stringVal, ok := value.(string)
	if !ok {
		stringVal = fmt.Sprint(value)
	}
	return errorStr + fmt.Sprintf("%q", stringVal) + " "
}

func NoError() DriverErrorInterface {
	return nil
}

func OnlyError(error error) DriverErrorInterface {
	return MsgError("", error)
}

func OnlyCode(code int) DriverErrorInterface {
	return CodeMsg(code, "")
}

func OnlyMsg(msg string) DriverErrorInterface {
	return MsgError(msg, nil)
}

func MsgError(msg string, error error) DriverErrorInterface {
	return MsgErrorData(msg, error, nil)
}

func MsgErrorData(msg string, error error, data interface{}) DriverErrorInterface {
	return CodeMsgErrorData(UNKNOWN, msg, error, data)
}

func CodeMsgError(code int, msg string, error error) DriverErrorInterface {
	return CodeMsgErrorData(code, msg, error, nil)
}

func CodeMsg(code int, msg string) DriverErrorInterface {
	return CodeMsgErrorData(code, msg, nil, nil)
}

func CodeMsgErrorData(code int, msg string, error error, data interface{}) DriverErrorInterface {
	var err *DriverError
	if errors.As(error, &err) {
		return &DriverError{
			Code: code,
			Msg:  err.Msg + " " + msg,
			Err:  err.Err,
			Data: data,
		}
	}
	return &DriverError{
		Code: code,
		Msg:  msg,
		Err:  error,
		Data: data,
	}
}
