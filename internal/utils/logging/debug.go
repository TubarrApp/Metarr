package utils

import (
	consts "Metarr/internal/domain/constants"
	keys "Metarr/internal/domain/keys"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/spf13/viper"
)

var (
	Level int = -1 // Pre initialization
	mu    sync.Mutex
)

func PrintE(l int, format string, args ...interface{}) string {

	mu.Lock()
	defer mu.Unlock()
	var msg string

	_, file, line, _ := runtime.Caller(1)
	file = filepath.Base(file)
	tag := fmt.Sprintf("[File: %s : Line: %d] ", file, line)

	if Level < 0 {
		Level = viper.GetInt(keys.DebugLevel)
	}
	if l <= viper.GetInt(keys.DebugLevel) {

		if len(args) != 0 && args != nil {
			msg = fmt.Sprintf(consts.RedError+format+tag+"\n", args...)
		} else {
			msg = fmt.Sprintf(consts.RedError + format + tag + "\n")
		}
		fmt.Print(msg)

		Write(consts.LogError, msg, nil)
	}

	return msg
}

func PrintS(l int, format string, args ...interface{}) string {

	mu.Lock()
	defer mu.Unlock()
	var msg string

	_, file, line, _ := runtime.Caller(1)
	file = filepath.Base(file)
	tag := fmt.Sprintf("[File: %s : Line: %d] ", file, line)

	if Level < 0 {
		Level = viper.GetInt(keys.DebugLevel)
	}
	if l <= viper.GetInt(keys.DebugLevel) {

		if len(args) != 0 && args != nil {
			msg = fmt.Sprintf(consts.GreenSuccess+format+tag+"\n", args...)
		} else {
			msg = fmt.Sprintf(consts.GreenSuccess + format + tag + "\n")
		}
		fmt.Print(msg)

		Write(consts.LogSuccess, msg, nil)
	}

	return msg
}

func PrintD(l int, format string, args ...interface{}) string {

	mu.Lock()
	defer mu.Unlock()
	var msg string

	_, file, line, _ := runtime.Caller(1)
	file = filepath.Base(file)
	tag := fmt.Sprintf("[File: %s : Line: %d] ", file, line)

	if Level < 0 {
		Level = viper.GetInt(keys.DebugLevel)
	}
	if l <= viper.GetInt(keys.DebugLevel) && l != 0 { // Debug messages don't appear by default

		if len(args) != 0 && args != nil {
			msg = fmt.Sprintf(consts.YellowDebug+format+tag+"\n", args...)
		} else {
			msg = fmt.Sprintf(consts.YellowDebug + format + tag + "\n")
		}
		fmt.Print(msg)

		Write(consts.LogSuccess, msg, nil)
	}

	return msg
}

func PrintI(format string, args ...interface{}) string {

	mu.Lock()
	defer mu.Unlock()
	var msg string

	if len(args) != 0 && args != nil {
		msg = fmt.Sprintf(consts.BlueInfo+format+"\n", args...)
	} else {
		msg = fmt.Sprintf(consts.BlueInfo + format + "\n")
	}
	fmt.Print(msg)
	Write(consts.LogInfo, msg, nil)

	return msg
}

func Print(format string, args ...interface{}) string {

	mu.Lock()
	defer mu.Unlock()
	var msg string

	if len(args) != 0 && args != nil {
		msg = fmt.Sprintf(format+"\n", args...)
	} else {
		msg = fmt.Sprintf(format + "\n")
	}
	fmt.Print(msg)
	Write(consts.LogBasic, msg, nil)

	return msg
}
