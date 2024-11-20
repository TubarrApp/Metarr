package logging

import (
	"fmt"
	"metarr/internal/domain/consts"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	tagBaseLen = 1 + // "["
		len(consts.ColorBlue) +
		9 + // "Function: "
		len(consts.ColorReset) +
		3 + // " - "
		len(consts.ColorBlue) +
		5 + // "File: "
		len(consts.ColorReset) +
		3 + // " : "
		len(consts.ColorBlue) +
		5 + // "Line: "
		len(consts.ColorReset) +
		2 // "]\n"
)

var (
	Level int = -1 // Pre initialization is -1
)

func E(l int, format string, args ...interface{}) string {
	if Level < l {
		return ""
	}

	pc, file, intLine, _ := runtime.Caller(1)
	file = filepath.Base(file)
	funcName := filepath.Base(runtime.FuncForPC(pc).Name())
	line := strconv.Itoa(intLine)

	var b strings.Builder
	b.Grow(len(consts.RedError) + len(format) + (len(args) * 32) + tagBaseLen + len(file) + len(funcName) + len(line))

	b.WriteString(consts.RedError)

	// Write formatted message
	if len(args) != 0 && args != nil {
		fmt.Fprintf(&b, format, args...)
	} else {
		b.WriteString(format)
	}

	b.WriteString(" [")
	b.WriteString(consts.ColorBlue)
	b.WriteString("Function: ")
	b.WriteString(consts.ColorReset)
	b.WriteString(funcName)
	b.WriteString(" - ")
	b.WriteString(consts.ColorBlue)
	b.WriteString("File: ")
	b.WriteString(consts.ColorReset)
	b.WriteString(file)
	b.WriteString(" : ")
	b.WriteString(consts.ColorBlue)
	b.WriteString("Line: ")
	b.WriteString(consts.ColorReset)
	b.WriteString(line)
	b.WriteString("]\n")

	msg := b.String()

	fmt.Print(msg)
	writeLog(msg, l)

	return msg
}

func S(l int, format string, args ...interface{}) string {
	if Level < l {
		return ""
	}

	var b strings.Builder
	b.Grow(len(consts.GreenSuccess) + len(format) + len(consts.ColorReset) + (len(args) * 32) + 1)
	b.WriteString(consts.GreenSuccess)

	// Write formatted message
	if len(args) != 0 && args != nil {
		fmt.Fprintf(&b, format, args...)
	} else {
		b.WriteString(format)
	}

	b.WriteString("\n")
	msg := b.String()
	fmt.Print(msg)
	writeLog(msg, l)

	return msg
}

func D(l int, format string, args ...interface{}) string {
	if Level < l {
		return ""
	}

	pc, file, intLine, _ := runtime.Caller(1)
	file = filepath.Base(file)
	funcName := filepath.Base(runtime.FuncForPC(pc).Name())
	line := strconv.Itoa(intLine)

	var b strings.Builder
	b.Grow(len(consts.YellowDebug) + len(format) + (len(args) * 32) + tagBaseLen + len(file) + len(funcName) + len(line))
	b.WriteString(consts.YellowDebug)

	// Write formatted message
	if len(args) != 0 && args != nil {
		fmt.Fprintf(&b, format, args...)
	} else {
		b.WriteString(format)
	}

	b.WriteString(" [")
	b.WriteString(consts.ColorBlue)
	b.WriteString("Function: ")
	b.WriteString(consts.ColorReset)
	b.WriteString(funcName)
	b.WriteString(" - ")
	b.WriteString(consts.ColorBlue)
	b.WriteString("File: ")
	b.WriteString(consts.ColorReset)
	b.WriteString(file)
	b.WriteString(" : ")
	b.WriteString(consts.ColorBlue)
	b.WriteString("Line: ")
	b.WriteString(consts.ColorReset)
	b.WriteString(line)
	b.WriteString("]\n")

	msg := b.String()

	fmt.Print(msg)
	writeLog(msg, l)

	return msg
}

func I(format string, args ...interface{}) string {

	var b strings.Builder
	b.Grow(len(consts.BlueInfo) + len(format) + len(consts.ColorReset) + (len(args) * 32) + 1)
	b.WriteString(consts.BlueInfo)

	// Write formatted message
	if len(args) != 0 && args != nil {
		fmt.Fprintf(&b, format, args...)
	} else {
		b.WriteString(format)
	}

	b.WriteString("\n")
	msg := b.String()
	fmt.Print(msg)
	writeLog(msg, 0)

	return msg
}

func P(format string, args ...interface{}) string {

	var b strings.Builder
	b.Grow(len(format) + (len(args) * 32) + 1)

	// Write formatted message
	if len(args) != 0 && args != nil {
		fmt.Fprintf(&b, format, args...)
	} else {
		b.WriteString(format)
	}

	b.WriteString("\n")
	msg := b.String()
	fmt.Print(msg)
	writeLog(msg, 0)

	return msg
}
