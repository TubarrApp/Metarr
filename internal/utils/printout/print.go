package printout

import (
	"fmt"
	"metarr/internal/domain/consts"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"reflect"
	"strings"
	"sync"
)

var muPrint sync.Mutex

// CreateModelPrintout prints out the values stored in a struct.
// taskName allows you to enter your own identifier for this task.
func CreateModelPrintout(model any, filename, taskName string, args ...any) {
	if model == nil {
		logging.E(0, "Model entered nil for taskname %q", taskName)
		return
	}

	muPrint.Lock()
	defer muPrint.Unlock()

	var b strings.Builder
	b.Grow(20000)

	// Helper function to add sections
	addSection := func(title string, content string) {
		b.WriteString(consts.ColorYellow + "\n" + title + ":\n" + consts.ColorReset)
		b.WriteString(content)
	}

	// Header
	b.WriteString("\n\n================= ")
	b.WriteString(consts.ColorCyan + "Printing metadata fields for: " + consts.ColorReset)
	b.WriteString("'" + consts.ColorReset + filename + "'")
	b.WriteString(" =================\n")

	if taskName != "" {
		str := fmt.Sprintf("'"+taskName+"'", args...)
		b.WriteString("\n" + consts.ColorGreen + "Printing model at point of task " + consts.ColorReset + str + "\n")
	}

	// Add fields from the struct
	addSection("File Information", printStructFields(model))

	switch m := model.(type) {
	case *models.FileData:

		addSection("Credits", printStructFields(m.MCredits))
		addSection("Titles and descriptions", printStructFields(m.MTitleDesc))
		addSection("Dates and timestamps", printStructFields(m.MDates))
		addSection("Webpage data", printStructFields(m.MWebData))
		addSection("Show data", printStructFields(m.MShowData))
		addSection("Other data", printStructFields(m.MOther))

	case *models.NFOData:
		// Credits section
		b.WriteString(consts.ColorYellow + "\nCredits:\n" + consts.ColorReset)

		// Handle each slice type separately
		for _, actor := range m.Actors {
			b.WriteString(printStructFields(actor.Name))
		}
		for _, director := range m.Directors {
			b.WriteString(printStructFields(director))
		}
		for _, producer := range m.Producers {
			b.WriteString(printStructFields(producer))
		}
		for _, publisher := range m.Publishers {
			b.WriteString(printStructFields(publisher))
		}
		for _, studio := range m.Studios {
			b.WriteString(printStructFields(studio))
		}
		for _, writer := range m.Writers {
			b.WriteString(printStructFields(writer))
		}

		addSection("Titles and descriptions", printStructFields(m.Title)+
			printStructFields(m.Description)+
			printStructFields(m.Plot))

		addSection("Webpage data", printStructFields(m.WebpageInfo))

		addSection("Show data", printStructFields(m.ShowInfo.Show)+
			printStructFields(m.ShowInfo.EpisodeID)+
			printStructFields(m.ShowInfo.EpisodeTitle)+
			printStructFields(m.ShowInfo.SeasonNumber))
	}

	// Footer
	b.WriteString("\n\n================= ")
	b.WriteString(consts.ColorYellow + "End metadata fields for: " + consts.ColorReset)
	b.WriteString("'" + filename + "'")
	b.WriteString(" =================\n\n")

	logging.P(b.String())
}

// Function to print the fields of a struct using reflection
func printStructFields(s any) string {
	val := reflect.ValueOf(s)

	// Dereference pointer
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return fmt.Sprintf("Expected a struct, got %s\n", val.Kind())
	}

	typ := val.Type()

	var b strings.Builder
	b.Grow(val.NumField() * 1024)

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)      // Get field metadata
		fieldValue := val.Field(i) // Get field value

		// Skip zero or empty fields
		if fieldValue.IsZero() {
			b.WriteString(field.Name + consts.ColorRed + " [empty]\n" + consts.ColorReset)
			continue
		}

		fieldName := field.Name
		fieldValueStr := fmt.Sprintf("%v", fieldValue.Interface()) // Convert the value to a string

		// Append the field name and value in key-value format
		b.WriteString(fmt.Sprintf("%s: %s\n", fieldName, fieldValueStr))
	}

	return b.String()
}

// Print out the fetched fields
func PrintGrabbedFields(fieldType string, p map[string]string) {
	muPrint.Lock()
	defer muPrint.Unlock()

	fmt.Println()
	logging.I("Found and stored %s metadata fields from metafile:", fieldType)
	fmt.Println()

	for k, v := range p {
		if k != "" && v != "" {
			logging.P(fmt.Sprintf(consts.ColorGreen + "Key: " + consts.ColorReset + k + consts.ColorYellow + "\nValue: " + consts.ColorReset + v + "\n"))
		}
	}
	fmt.Println()
}