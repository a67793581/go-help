package logger

import (
	"runtime"
)

type (
	CodeLocation struct {
		FuncName       string
		FileName       string
		LineNumber     int
		FullStackTrace string
	}
)

func GetCodeLocation() CodeLocation {
	return GetCodeLocationBySkip(2)
}

func GetCodeLocationBySkip(skip int) CodeLocation {
	pc, file, line, ok := runtime.Caller(skip)
	var funcName string
	if ok {
		funcName = runtime.FuncForPC(pc).Name()
	}
	return CodeLocation{
		FileName:   file,
		LineNumber: line,
		FuncName:   funcName,
	}
}
