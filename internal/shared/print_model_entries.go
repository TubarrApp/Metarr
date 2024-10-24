package shared

import (
	"Metarr/internal/consts"
	"Metarr/internal/models"
	"fmt"
	"reflect"
	"sync"
)

var muPrint sync.Mutex

// CreateModelPrintout prints out the values stored in a struct.
// taskName allows you to enter your own identifier for this task.
func CreateModelPrintout(m *models.FileData, taskName string, args ...interface{}) string {
	muPrint.Lock()
	defer muPrint.Unlock()

	output := "\n\n================= " + consts.ColorCyan + "Printing metadata fields for:" + consts.ColorReset + " '" + consts.ColorReset + m.JSONBaseName + "' =================\n"

	if taskName != "" {
		str := fmt.Sprintf("'"+taskName+"'", args...)
		output += "\n" + consts.ColorGreen + "Printing model at point of task " + consts.ColorReset + str + "\n"
	}

	// Add fields from the struct
	output += consts.ColorYellow + "\nFile Information:\n" + consts.ColorReset
	output += printStructFields(m)

	output += consts.ColorYellow + "\nCredits:\n" + consts.ColorReset
	output += printStructFields(m.MCredits)

	output += consts.ColorYellow + "\nTitles and descriptions:\n" + consts.ColorReset
	output += printStructFields(m.MTitleDesc)

	output += consts.ColorYellow + "\nDates and timestamps:\n" + consts.ColorReset
	output += printStructFields(m.MDates)

	output += consts.ColorYellow + "\nWebpage data:\n" + consts.ColorReset
	output += printStructFields(m.MWebData)

	output += consts.ColorYellow + "\nShow data:\n" + consts.ColorReset
	output += printStructFields(m.MShowData)

	output += consts.ColorYellow + "\nOther data:\n" + consts.ColorReset
	output += printStructFields(m.MOther)

	output += "\n\n================= " + consts.ColorYellow + "End metadata fields for:" + consts.ColorReset + " '" + m.JSONBaseName + "' =================\n\n"

	return output
}

// Function to print the fields of a struct using reflection
func printStructFields(s interface{}) string {
	val := reflect.ValueOf(s)

	// Dereference pointer
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return fmt.Sprintf("Expected a struct, got %s\n", val.Kind())
	}

	typ := val.Type() // Get the type of the struct
	output := ""

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)      // Get field metadata
		fieldValue := val.Field(i) // Get field value

		// Skip zero or empty fields
		if fieldValue.IsZero() {
			output += field.Name + consts.ColorRed + " [empty]\n" + consts.ColorReset
			continue
		}

		fieldName := field.Name                                    // Get the field name
		fieldValueStr := fmt.Sprintf("%v", fieldValue.Interface()) // Convert the value to a string

		// Append the field name and value in key-value format
		output += fmt.Sprintf("%s: %s\n", fieldName, fieldValueStr)
	}

	return output
}
