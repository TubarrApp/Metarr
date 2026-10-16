// Package vars stores global variables.
package vars

import (
	"runtime"
	"slices"
	"sync"

	"github.com/TubarrApp/gocommon/benchmark"
)

// BenchmarkFiles holds the reference to the program's benchmarking files.
var BenchmarkFiles *benchmark.BenchFiles

// OS is the system the program is running on, e.g. 'linux', 'windows', 'darwin'.
var OS = runtime.GOOS

// errorArray contains the array of errors.
var (
	errorArray   []error
	errorArrayMu sync.Mutex
)

// AddToErrorArray adds an error to the error array under lock.
func AddToErrorArray(err error) {
	errorArrayMu.Lock()
	defer errorArrayMu.Unlock()
	errorArray = append(errorArray, err)
}

// GetErrorArray returns a copy of the error array.
func GetErrorArray() []error {
	errorArrayMu.Lock()
	defer errorArrayMu.Unlock()
	return slices.Clone(errorArray)
}
