// Package vars stores global variables.
package vars

import (
	"runtime"

	"github.com/TubarrApp/gocommon/benchmark"
)

// BenchmarkFiles holds the reference to the program's benchmarking files.
var BenchmarkFiles *benchmark.BenchFiles

// OS is the system the program is running on, e.g. 'linux', 'windows', 'darwin'.
var OS = runtime.GOOS

// errorArray contains the array of errors.
var errorArray = make([]error, 0)

// AddToErrorArray adds an error to the error array under lock.
func AddToErrorArray(err error) {
	errorArray = append(errorArray, err)
}

// GetErrorArray returns the error array.
func GetErrorArray() []error {
	return errorArray
}
