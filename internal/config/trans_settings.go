package config

import (
	"fmt"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
	"strings"
)

var (
	filenameReplaceSuffixInput []string
)

// initTextReplace initializes text replacement functions
func initTextReplace() error {

	// Parse rename flag
	setRenameFlag()

	// Meta operations
	if err := validateMetaOps(); err != nil {
		return err
	}

	// Replace filename suffixes
	if err := validateFilenameSuffixReplace(); err != nil {
		return err
	}

	return nil
}

// validateMetaOps parses the meta transformation operations
func validateMetaOps() error {

	metaOpsInput := GetStringSlice(keys.MetaOps)
	if len(metaOpsInput) == 0 {
		logging.D(2, "No metadata operations passed in")
		return nil
	}

	newLen, apndLen, pfxLen, trimSfxLen, trimPfxLen, replaceLen := metaOpsMapLength(metaOpsInput)

	newField := make([]*models.MetaNewField, 0, newLen)

	apnd := make([]*models.MetaAppend, 0, apndLen)
	pfx := make([]*models.MetaPrefix, 0, pfxLen)

	trimSfx := make([]*models.MetaTrimSuffix, 0, trimSfxLen)
	trimPfx := make([]*models.MetaTrimPrefix, 0, trimPfxLen)

	replace := make([]*models.MetaReplace, 0, replaceLen)

	for _, op := range metaOpsInput {

		// Check validity
		parts := strings.Split(op, ":")

		if len(parts) < 3 || len(parts) > 4 {
			return fmt.Errorf("malformed input meta-ops, each entry must be at least 3 parts, split by : (e.g. 'title:add:Video Title')")
		}

		field := parts[0]
		operation := parts[1]
		value := parts[2]

		switch operation {
		case "add":
			newFieldModel := &models.MetaNewField{
				Field: field,
				Value: value,
			}
			newField = append(newField, newFieldModel)
			fmt.Println()
			logging.D(3, "Added new field op:\nField: %s\nValue: %s", newFieldModel.Field, newFieldModel.Value)
			fmt.Println()

		case "append":
			apndModel := &models.MetaAppend{
				Field:  field,
				Suffix: value,
			}
			apnd = append(apnd, apndModel)
			fmt.Println()
			logging.D(3, "Added new append op:\nField: %s\nAppend: %s", apndModel.Field, apndModel.Suffix)
			fmt.Println()

		case "prefix":
			pfxModel := &models.MetaPrefix{
				Field:  field,
				Prefix: value,
			}
			pfx = append(pfx, pfxModel)
			fmt.Println()
			logging.D(3, "Added new prefix op:\nField: %s\nPrefix: %s", pfxModel.Field, pfxModel.Prefix)
			fmt.Println()

		case "trim-suffix":
			tSfxModel := &models.MetaTrimSuffix{
				Field:  field,
				Suffix: value,
			}
			trimSfx = append(trimSfx, tSfxModel)
			fmt.Println()
			logging.D(3, "Added new suffix trim op:\nField: %s\nSuffix: %s", tSfxModel.Field, tSfxModel.Suffix)
			fmt.Println()

		case "trim-prefix":
			tPfxModel := &models.MetaTrimPrefix{
				Field:  field,
				Prefix: value,
			}
			trimPfx = append(trimPfx, tPfxModel)
			fmt.Println()
			logging.D(3, "Added new prefix trim op:\nField: %s\nPrefix: %s", tPfxModel.Field, tPfxModel.Prefix)
			fmt.Println()

		case "replace":
			if len(parts) != 4 {
				return fmt.Errorf("replacement should be in format 'field:replace:text:replacement'")
			}
			rModel := &models.MetaReplace{
				Field:       field,
				Value:       value,
				Replacement: parts[3],
			}
			replace = append(replace, rModel)
			fmt.Println()
			logging.D(3, "Added new replace operation:\nField: %s\nValue: %s\nReplacement: %s\n", rModel.Field, rModel.Value, rModel.Replacement)
			fmt.Println()

		default:
			return fmt.Errorf("unrecognized meta operation '%s' (valid operations: add, append, prefix, trim-suffix, trim-prefix)", parts[1])
		}
	}

	if len(apnd) > 0 {
		logging.I("Appending: %v", apnd)
		Set(keys.MAppend, apnd)
	}

	if len(newField) > 0 {
		logging.I("New meta fields: %v", newField)
		Set(keys.MNewField, newField)
	}

	if len(pfx) > 0 {
		logging.I("Prefixing: %v", apnd)
		Set(keys.MPrefix, pfx)
	}

	if len(trimPfx) > 0 {
		logging.I("Trimming prefix: %v", trimPfx)
		Set(keys.MTrimPrefix, trimPfx)
	}

	if len(trimSfx) > 0 {
		logging.I("Trimming suffix: %v", trimSfx)
		Set(keys.MTrimSuffix, trimSfx)
	}

	if len(replace) > 0 {
		logging.I("Replacing text: %v", replace)
		Set(keys.MReplaceText, replace)
	}

	return nil
}

// metaOpsMapLength quickly grabs the lengths needed for each map
func metaOpsMapLength(metaOpsInput []string) (new, apnd, pfx, sfxTrim, pfxTrim, replace int) {
	for _, op := range metaOpsInput {
		if i := strings.IndexByte(op, ':'); i >= 0 {
			if j := strings.IndexByte(op[i+1:], ':'); j >= 0 {
				op = op[i+1 : i+1+j]

				switch op {
				case "add":
					new++
				case "append":
					apnd++
				case "prefix":
					pfx++
				case "trim-suffix":
					sfxTrim++
				case "trim-prefix":
					pfxTrim++
				case "replace":
					replace++
				}
			}
		}

	}
	fmt.Println()
	logging.D(2, "Meta additions: %d\nMeta appends: %d\nMeta prefix: %d\nMeta suffix trim: %d\nMeta prefix trim: %d\nMeta replacements: %d\n", new, apnd, pfx, sfxTrim, pfxTrim, replace)
	fmt.Println()
	return new, apnd, pfx, sfxTrim, pfxTrim, replace
}

// validateFilenameSuffixReplace checks if the input format for filename suffix replacement is valid
func validateFilenameSuffixReplace() error {
	filenameReplaceSuffix := make([]*models.FilenameReplaceSuffix, 0, len(filenameReplaceSuffixInput))

	for _, pair := range filenameReplaceSuffixInput {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) < 3 {
			return fmt.Errorf("invalid use of filename-replace-suffix, values must be written as (suffix:replacement)")
		}
		filenameReplaceSuffix = append(filenameReplaceSuffix, &models.FilenameReplaceSuffix{
			Suffix:      parts[0],
			Replacement: parts[1],
		})
	}
	if len(filenameReplaceSuffix) > 0 {
		logging.I("Meta replace suffixes: %v", filenameReplaceSuffix)
		Set(keys.FilenameReplaceSfx, filenameReplaceSuffix)
	}
	return nil
}

// setRenameFlag sets the rename style to apply
func setRenameFlag() {

	var renameFlag enums.ReplaceToStyle
	argRenameFlag := GetString(keys.RenameStyle)

	// Trim whitespace for more robust validation
	argRenameFlag = strings.TrimSpace(argRenameFlag)
	argRenameFlag = strings.ToLower(argRenameFlag)

	switch argRenameFlag {
	case "spaces", "space":
		renameFlag = enums.RENAMING_SPACES
		logging.P("Rename style selected: %v", argRenameFlag)

	case "underscores", "underscore":
		renameFlag = enums.RENAMING_UNDERSCORES
		logging.P("Rename style selected: %v", argRenameFlag)

	case "fixes-only", "fixesonly":
		renameFlag = enums.RENAMING_FIXES_ONLY
		logging.P("Rename style selected: %v", argRenameFlag)

	default:
		logging.D(1, "'Spaces' or 'underscores' not selected for renaming style, skipping these modifications.")
		renameFlag = enums.RENAMING_SKIP
	}
	Set(keys.Rename, renameFlag)
}

// initDateReplaceFormat initializes the user's preferred format for dates
func initDateReplaceFormat() error {

	var formatEnum enums.FilenameDateFormat
	dateFmt := GetString(keys.InputFileDatePfx)

	// Trim whitespace for more robust validation
	dateFmt = strings.TrimSpace(dateFmt)

	if dateFmt == "" || len(dateFmt) == 0 {
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
		default:
			return fmt.Errorf("invalid date format entered, please enter three characters (where capital Y is yyyy and y is yy)")
		}
	}
	Set(keys.FileDateFmt, formatEnum)
	logging.D(1, "Set file date format to %v", formatEnum)
	return nil
}
