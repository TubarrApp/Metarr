package cmd

import (
	"Metarr/internal/enums"
	"Metarr/internal/keys"
	"Metarr/internal/logging"
	"Metarr/internal/models"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// initTextReplace initializes text replacement functions
func initTextReplace() error {

	// Parse rename flag
	var renameFlag enums.ReplaceToStyle

	argRenameFlag := viper.GetString(keys.RenameStyle)
	switch argRenameFlag {
	case "spaces", "space":
		renameFlag = enums.SPACES
		logging.Print("Rename style selected: %v", argRenameFlag)

	case "underscores", "underscore":
		renameFlag = enums.UNDERSCORES
		logging.Print("Rename style selected: %v", argRenameFlag)

	case "skip", "none":
		renameFlag = enums.SKIP
	default:
		return fmt.Errorf("invalid rename flag entered")
	}
	viper.Set(keys.Rename, renameFlag)

	// Add new field
	errMsg := fmt.Errorf("invalid use of metadata addition, values must be written as (metatag:field:value)")
	var metaNewField []models.MetaNewField

	for _, value := range metaNewFieldInput {
		parts := strings.SplitN(value, ":", 2)
		if len(parts) < 2 {
			return errMsg
		}
		// Append each parsed field-value pair to the metaNewField array
		metaNewField = append(metaNewField, models.MetaNewField{
			Field: parts[0],
			Value: parts[1],
		})
	}
	if len(metaNewField) > 0 {
		logging.PrintI("Meta new fields to add: %v", metaNewField)
		viper.Set(keys.MNewField, metaNewField)
	}

	// Replace metafield value suffixes
	errMsg = fmt.Errorf("invalid use of meta-replace-suffix, values must be written as (metatag:field suffix:replacement)")
	var metaReplaceSuffix []models.MetaReplaceSuffix

	for _, tuple := range metaReplaceSuffixInput {
		parts := strings.SplitN(tuple, ":", 3)
		if len(parts) < 3 {
			return errMsg
		}
		metaReplaceSuffix = append(metaReplaceSuffix, models.MetaReplaceSuffix{
			Field:       parts[0],
			Suffix:      parts[1],
			Replacement: parts[2],
		})
	}
	if len(metaReplaceSuffix) > 0 {
		logging.PrintI("Meta replace suffixes: %v\n", metaReplaceSuffix)
		viper.Set(keys.MReplaceSfx, metaReplaceSuffix)
	}

	// Replace metafield value prefixes
	errMsg = fmt.Errorf("invalid use of meta-replace-suffix, values must be written as (metatag:field prefix:replacement)")
	var metaReplacePrefix []models.MetaReplacePrefix

	for _, tuple := range metaReplacePrefixInput {
		parts := strings.SplitN(tuple, ":", 3)
		if len(parts) < 3 {
			return errMsg
		}
		metaReplacePrefix = append(metaReplacePrefix, models.MetaReplacePrefix{
			Field:       parts[0],
			Prefix:      parts[1],
			Replacement: parts[2],
		})
	}
	if len(metaReplacePrefix) > 0 {
		logging.PrintI("Meta replace prefixes: %v", metaReplacePrefix)
		viper.Set(keys.MReplacePfx, metaReplacePrefix)
	}

	// Replace filename suffixes
	errMsg = fmt.Errorf("invalid use of filename-replace-suffix, values must be written as (suffix:replacement)")
	var filenameReplaceSuffix []models.FilenameReplaceSuffix

	for _, pair := range filenameReplaceSuffixInput {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) < 2 {
			return errMsg
		}
		filenameReplaceSuffix = append(filenameReplaceSuffix, models.FilenameReplaceSuffix{
			Suffix:      parts[0],
			Replacement: parts[1],
		})
	}
	if len(filenameReplaceSuffix) > 0 {
		logging.PrintI("Meta replace suffixes: %v", filenameReplaceSuffix)
		viper.Set(keys.FilenameReplaceSfx, filenameReplaceSuffix)
	}

	return nil
}

// initDateReplaceFormat initializes the user's preferred format for dates
func initDateReplaceFormat() error {

	dateFmt := viper.GetString(keys.InputFileDatePfx)

	var formatEnum enums.FilenameDateFormat

	if dateFmt == "" {
		formatEnum = enums.FILEDATE_SKIP
	} else if len(dateFmt) != 3 {
		return fmt.Errorf("invalid date format entered, please enter three characters (where 'Y' is yyyy and 'y' is yy)")
	} else {
		switch dateFmt {
		case "Ymd":
			formatEnum = enums.FILEDATE_YYYY_MM_DD
		case "ymd":
			formatEnum = enums.FILEDATE_YY_MM_DD
		case "Ydm":
			formatEnum = enums.FILEDATE_YYYY_DD_MM
		case "ydm":
			formatEnum = enums.FILEDATE_YY_DD_MM
		case "dmY":
			formatEnum = enums.FILEDATE_DD_MM_YYYY
		case "dmy":
			formatEnum = enums.FILEDATE_DD_MM_YY
		case "mdY":
			formatEnum = enums.FILEDATE_MM_DD_YYYY
		case "mdy":
			formatEnum = enums.FILEDATE_MM_DD_YY
		case "":
			formatEnum = enums.FILEDATE_SKIP
		default:
			return fmt.Errorf("invalid date format entered, please enter three characters (where capital Y is yyyy and y is yy)")
		}
	}

	viper.Set(keys.FileDateFmt, formatEnum)
	logging.PrintD(1, "Set file date format to %v", formatEnum)
	return nil
}
