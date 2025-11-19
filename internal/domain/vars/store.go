// Package vars stores global variables.
package vars

import "github.com/TubarrApp/gocommon/benchmark"

var (
	// BenchmarkFiles holds the reference to the program's benchmarking files.
	BenchmarkFiles *benchmark.BenchFiles
	errorArray     = make([]error, 0)
)

// AddToErrorArray adds an error to the error array under lock.
func AddToErrorArray(err error) {
	errorArray = append(errorArray, err)
}

// GetErrorArray returns the error array.
func GetErrorArray() []error {
	return errorArray
}
