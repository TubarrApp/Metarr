package logging

import (
	"Metarr/internal/consts"
	"Metarr/internal/keys"
	"fmt"
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

	if Level < 0 {
		Level = viper.GetInt(keys.DebugLevel)
	}
	if l <= viper.GetInt(keys.DebugLevel) {

		if len(args) != 0 && args != nil {
			msg = fmt.Sprintf(consts.RedError+format+"\n", args...)
		} else {
			msg = fmt.Sprintf(consts.RedError + format + "\n")
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

	if Level < 0 {
		Level = viper.GetInt(keys.DebugLevel)
	}
	if l <= viper.GetInt(keys.DebugLevel) {

		if len(args) != 0 && args != nil {
			msg = fmt.Sprintf(consts.GreenSuccess+format+"\n", args...)
		} else {
			msg = fmt.Sprintf(consts.GreenSuccess + format + "\n")
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

	if Level < 0 {
		Level = viper.GetInt(keys.DebugLevel)
	}
	if l <= viper.GetInt(keys.DebugLevel) && l != 0 { // Debug messages don't appear by default

		if len(args) != 0 && args != nil {
			msg = fmt.Sprintf(consts.YellowDebug+format+"\n", args...)
		} else {
			msg = fmt.Sprintf(consts.YellowDebug + format + "\n")
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
