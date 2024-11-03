package config

import (
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/types"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"strings"
)

var (
	metaReplaceSuffixInput,
	metaReplacePrefixInput,
	metaNewFieldInput,
	filenameReplaceSuffixInput []string
)

// initTextReplace initializes text replacement functions
func initTextReplace() error {

	// Parse rename flag
	setRenameFlag()

	// Add new field
	if err := validateNewMetafields(); err != nil {
		return err
	}

	// Replace metafield value suffixes
	if err := validateMetaSuffixes(); err != nil {
		return err
	}

	// Replace metafield value prefixes
	if err := validateMetaPrefixes(); err != nil {
		return err
	}

	// Replace filename suffixes
	if err := validateFilenameSuffixReplace(); err != nil {
		return err
	}

	return nil
}

// validateFilenameSuffixReplace checks if the input format for filename suffix replacement is valid
func validateFilenameSuffixReplace() error {
	var filenameReplaceSuffix []types.FilenameReplaceSuffix

	for _, pair := range filenameReplaceSuffixInput {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) < 2 {
			return fmt.Errorf("invalid use of filename-replace-suffix, values must be written as (suffix:replacement)")
		}
		filenameReplaceSuffix = append(filenameReplaceSuffix, types.FilenameReplaceSuffix{
			Suffix:      parts[0],
			Replacement: parts[1],
		})
	}
	if len(filenameReplaceSuffix) > 0 {
		logging.PrintI("Meta replace suffixes: %v", filenameReplaceSuffix)
		Set(keys.FilenameReplaceSfx, filenameReplaceSuffix)
	}
	return nil
}

// validateMetaPrefixes checks if the input format for meta prefix alterations is valid
func validateMetaPrefixes() error {
	var metaReplacePrefix []types.MetaReplacePrefix

	for _, tuple := range metaReplacePrefixInput {
		parts := strings.SplitN(tuple, ":", 3)
		if len(parts) < 3 {
			return fmt.Errorf("invalid use of meta-replace-prefix, values must be written as (metatag:fieldprefix:replacement)")
		}
		metaReplacePrefix = append(metaReplacePrefix, types.MetaReplacePrefix{
			Field:       parts[0],
			Prefix:      parts[1],
			Replacement: parts[2],
		})
	}
	if len(metaReplacePrefix) > 0 {
		logging.PrintI("Meta replace prefixes: %v", metaReplacePrefix)
		Set(keys.MReplacePfx, metaReplacePrefix)
	}
	return nil
}

// validateMetaSuffixes checks if the input format for meta suffix alterations is valid
func validateMetaSuffixes() error {
	var metaReplaceSuffix []types.MetaReplaceSuffix

	for _, tuple := range metaReplaceSuffixInput {
		parts := strings.SplitN(tuple, ":", 3)
		if len(parts) < 3 {
			return fmt.Errorf("invalid use of meta-replace-suffix, values must be written as (metatag:fieldsuffix:replacement)")
		}
		metaReplaceSuffix = append(metaReplaceSuffix, types.MetaReplaceSuffix{
			Field:       parts[0],
			Suffix:      parts[1],
			Replacement: parts[2],
		})
	}
	if len(metaReplaceSuffix) > 0 {
		logging.PrintI("Meta replace suffixes: %v\n", metaReplaceSuffix)
		Set(keys.MReplaceSfx, metaReplaceSuffix)
	}
	return nil
}

// validateNewMetafields checks if the input format for metatag and field additions is valid
func validateNewMetafields() error {
	var metaNewField []types.MetaNewField

	for _, value := range metaNewFieldInput {
		parts := strings.SplitN(value, ":", 2)
		if len(parts) < 2 {
			return fmt.Errorf("invalid use of metadata addition, values must be written as (metatag:field)")
		}
		// Append each parsed field-value pair to the metaNewField array
		metaNewField = append(metaNewField, types.MetaNewField{
			Field: parts[0],
			Value: parts[1],
		})
	}
	if len(metaNewField) > 0 {
		logging.PrintI("Meta new fields to add: %v", metaNewField)
		Set(keys.MNewField, metaNewField)
	}
	return nil
}

// setRenameFlag sets the rename style to apply
func setRenameFlag() {

	var renameFlag enums.ReplaceToStyle
	argRenameFlag := GetString(keys.RenameStyle)

	switch argRenameFlag {
	case "spaces", "space":
		renameFlag = enums.SPACES
		logging.Print("Rename style selected: %v", argRenameFlag)

	case "underscores", "underscore":
		renameFlag = enums.UNDERSCORES
		logging.Print("Rename style selected: %v", argRenameFlag)
	default:
		logging.PrintD(1, "'Spaces' or 'underscores' not selected for renaming style, skipping these modifications.")
		renameFlag = enums.SKIP
	}
	Set(keys.Rename, renameFlag)
}

// initDateReplaceFormat initializes the user's preferred format for dates
func initDateReplaceFormat() error {

	var formatEnum enums.FilenameDateFormat
	dateFmt := GetString(keys.InputFileDatePfx)

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
	Set(keys.FileDateFmt, formatEnum)
	logging.PrintD(1, "Set file date format to %v", formatEnum)
	return nil
}
