package cfg

import (
	"errors"
	"fmt"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
	"strings"

	"github.com/spf13/viper"
)

var (
	filenameReplaceSuffixInput []string
)

type metaOpsLen struct {
	newLen,
	apndLen,
	pfxLen,
	trimSfxLen,
	trimPfxLen,
	replaceLen,
	dTagLen,
	delDTagLen,
	copyToFieldLen,
	pasteFromFieldLen int
}

// initTextReplace initializes text replacement functions
func initTextReplace() error {

	// Parse rename flag
	setRenameFlag()

	logging.D(1, "About to validate meta operations")
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

	logging.D(1, "Validating meta operations...")

	metaOpsInput := viper.GetStringSlice(keys.MetaOps)
	if len(metaOpsInput) == 0 {
		logging.D(2, "No metadata operations passed in")
		return nil
	}

	m := metaOpsMapLength(metaOpsInput, metaOpsLen{})

	// Add new field
	newField := make([]models.MetaNewField, 0, m.newLen)
	models.SetOverrideMap = make(map[enums.OverrideMetaType]string, m.newLen)

	// Replacements
	replace := make([]models.MetaReplace, 0, m.replaceLen)
	models.ReplaceOverrideMap = make(map[enums.OverrideMetaType]models.MOverrideReplacePair, m.replaceLen)

	// Append
	apnd := make([]models.MetaAppend, 0, m.apndLen)
	models.AppendOverrideMap = make(map[enums.OverrideMetaType]string, m.apndLen)

	// Prefix
	pfx := make([]models.MetaPrefix, 0, m.pfxLen)

	// Trim prefix/suffix
	trimSfx := make([]models.MetaTrimSuffix, 0, m.trimSfxLen)
	trimPfx := make([]models.MetaTrimPrefix, 0, m.trimPfxLen)

	// Date tagging ops
	dateTag := make(map[string]models.MetaDateTag, m.dTagLen)
	delDateTag := make(map[string]models.MetaDateTag, m.delDTagLen)

	// Copy to and from fields
	copyToField := make([]models.CopyToField, 0, m.copyToFieldLen)
	pasteFromField := make([]models.PasteFromField, 0, m.pasteFromFieldLen)

	for _, op := range metaOpsInput {

		// Check validity
		parts := strings.Split(op, ":")

		if len(parts) < 3 || len(parts) > 4 {
			return errors.New("malformed input meta-ops, each entry must be at least 3 parts, split by : (e.g. 'title:add:Video Title')")
		}

		field := parts[0]
		operation := parts[1]
		value := parts[2]

		switch strings.ToLower(operation) {
		case "set":
			switch field {
			case "all-credits", "credits-all":
				models.SetOverrideMap[enums.OVERRIDE_META_CREDITS] = value
			}

			newFieldModel := models.MetaNewField{
				Field: field,
				Value: value,
			}
			newField = append(newField, newFieldModel)
			fmt.Println()
			logging.D(3, "Added new field op:\nField: %s\nValue: %s", newFieldModel.Field, newFieldModel.Value)
			fmt.Println()

		case "append":
			switch field {
			case "all-credits", "credits-all":
				models.AppendOverrideMap[enums.OVERRIDE_META_CREDITS] = value
			}

			apndModel := models.MetaAppend{
				Field:  field,
				Suffix: value,
			}
			apnd = append(apnd, apndModel)
			fmt.Println()
			logging.D(3, "Added new append op:\nField: %s\nAppend: %s", apndModel.Field, apndModel.Suffix)
			fmt.Println()

		case "prefix":
			pfxModel := models.MetaPrefix{
				Field:  field,
				Prefix: value,
			}
			pfx = append(pfx, pfxModel)
			fmt.Println()
			logging.D(3, "Added new prefix op:\nField: %s\nPrefix: %s", pfxModel.Field, pfxModel.Prefix)
			fmt.Println()

		case "trim-suffix":
			tSfxModel := models.MetaTrimSuffix{
				Field:  field,
				Suffix: value,
			}
			trimSfx = append(trimSfx, tSfxModel)
			fmt.Println()
			logging.D(3, "Added new suffix trim op:\nField: %s\nSuffix: %s", tSfxModel.Field, tSfxModel.Suffix)
			fmt.Println()

		case "trim-prefix":
			tPfxModel := models.MetaTrimPrefix{
				Field:  field,
				Prefix: value,
			}
			trimPfx = append(trimPfx, tPfxModel)
			fmt.Println()
			logging.D(3, "Added new prefix trim op:\nField: %s\nPrefix: %s", tPfxModel.Field, tPfxModel.Prefix)
			fmt.Println()

		case "copy-to":
			c := models.CopyToField{
				Field: field,
				Dest:  value,
			}
			copyToField = append(copyToField, c)
			fmt.Println()
			logging.D(3, "Added new copy/paste op:\nField: %s\nCopy To: %s", c.Field, c.Dest)
			fmt.Println()

		case "paste-from":
			p := models.PasteFromField{
				Field:  field,
				Origin: value,
			}
			pasteFromField = append(pasteFromField, p)
			fmt.Println()
			logging.D(3, "Added new copy/paste op:\nField: %s\nPaste From: %s", p.Field, p.Origin)
			fmt.Println()

		case "replace":
			if len(parts) != 4 {
				return errors.New("replacement should be in format 'field:replace:text:replacement'")
			}

			switch field {
			case "all-credits", "credits-all":
				models.ReplaceOverrideMap[enums.OVERRIDE_META_CREDITS] = models.MOverrideReplacePair{
					Value:       value,
					Replacement: parts[3],
				}
			}
			rModel := models.MetaReplace{
				Field:       field,
				Value:       value,
				Replacement: parts[3],
			}

			replace = append(replace, rModel)
			fmt.Println()
			logging.D(3, "Added new replace operation:\nField: %s\nValue: %s\nReplacement: %s\n", rModel.Field, rModel.Value, rModel.Replacement)
			fmt.Println()

		case "date-tag":
			if len(parts) != 4 {
				return errors.New("date-tag should be in format 'field:date-tag:location:format' (Ymd is yyyy-mm-dd, ymd is yy-mm-dd)")
			}
			var loc enums.MetaDateTagLocation

			switch strings.ToLower(value) {
			case "prefix":
				loc = enums.DATE_TAG_LOC_PFX
			case "suffix":
				loc = enums.DATE_TAG_LOC_SFX
			default:
				return errors.New("date tag location must be prefix, or suffix")
			}
			if e, err := dateEnum(parts[3]); err != nil {
				return err
			} else {
				dateTag[field] = models.MetaDateTag{
					Loc:    loc,
					Format: e,
				}
				fmt.Println()
				logging.D(3, "Added new date tag operation:\nField: %s\nLocation: %s\nReplacement: %s\n", field, value, parts[3])
				fmt.Println()
			}

		case "delete-date-tag":
			if len(parts) != 4 {
				return errors.New("date-tag should be in format 'field:date-tag:location:format' (Ymd is yyyy-mm-dd, ymd is yy-mm-dd)")
			}
			var loc enums.MetaDateTagLocation

			switch strings.ToLower(value) {
			case "prefix":
				loc = enums.DATE_TAG_LOC_PFX
			case "suffix":
				loc = enums.DATE_TAG_LOC_SFX
			default:
				return errors.New("date tag location must be prefix, or suffix")
			}
			if e, err := dateEnum(parts[3]); err != nil {
				return err
			} else {
				delDateTag[field] = models.MetaDateTag{
					Loc:    loc,
					Format: e,
				}
				fmt.Println()
				logging.D(3, "Added delete date tag operation:\nField: %s\nLocation: %s\nReplacement: %s\n", field, value, parts[3])
				fmt.Println()
			}

		default:
			return fmt.Errorf("unrecognized meta operation %q (valid operations: add, append, prefix, trim-suffix, trim-prefix, replace, date-tag, delete-date-tag, copy-to, copy-from)", parts[1])
		}
	}

	if len(apnd) > 0 {
		logging.I("Appending: %v", apnd)
		viper.Set(keys.MAppend, apnd)
	}

	if len(newField) > 0 {
		logging.I("New meta fields: %v", newField)
		viper.Set(keys.MNewField, newField)
	}

	if len(pfx) > 0 {
		logging.I("Prefixing: %v", apnd)
		viper.Set(keys.MPrefix, pfx)
	}

	if len(trimPfx) > 0 {
		logging.I("Trimming prefix: %v", trimPfx)
		viper.Set(keys.MTrimPrefix, trimPfx)
	}

	if len(trimSfx) > 0 {
		logging.I("Trimming suffix: %v", trimSfx)
		viper.Set(keys.MTrimSuffix, trimSfx)
	}

	if len(replace) > 0 {
		logging.I("Replacing text: %v", replace)
		viper.Set(keys.MReplaceText, replace)
	}

	if len(copyToField) > 0 {
		logging.I("Copying to fields: %v", copyToField)
		viper.Set(keys.MCopyToField, copyToField)
	}

	if len(pasteFromField) > 0 {
		logging.I("Pasting from fields: %v", pasteFromField)
		viper.Set(keys.MPasteFromField, pasteFromField)
	}

	if len(dateTag) > 0 {
		logging.I("Adding date tags: %v", dateTag)
		viper.Set(keys.MDateTagMap, dateTag)
	}

	if len(delDateTag) > 0 {
		logging.I("Deleting date tags: %v", delDateTag)
		viper.Set(keys.MDelDateTagMap, delDateTag)
	}

	return nil
}

// metaOpsMapLength quickly grabs the lengths needed for each map
func metaOpsMapLength(metaOpsInput []string, m metaOpsLen) metaOpsLen {

	for _, op := range metaOpsInput {
		if i := strings.IndexByte(op, ':'); i >= 0 {
			if j := strings.IndexByte(op[i+1:], ':'); j >= 0 {
				op = op[i+1 : i+1+j]

				switch op {
				case "set":
					m.newLen++
				case "append":
					m.apndLen++
				case "prefix":
					m.pfxLen++
				case "trim-suffix":
					m.trimSfxLen++
				case "trim-prefix":
					m.trimPfxLen++
				case "replace":
					m.replaceLen++
				case "date-tag":
					m.dTagLen++
				case "delete-date-tag":
					m.delDTagLen++
				case "copy-to":
					m.copyToFieldLen++
				case "paste-from":
					m.pasteFromFieldLen++
				}
			}
		}
	}
	fmt.Println()
	logging.D(2, "Meta additions: %d\nMeta appends: %d\nMeta prefix: %d\nMeta suffix trim: %d\nMeta prefix trim: %d\nMeta replacements: %d\nDate tags: %d\nDelete date tags: %d\nCopy operations: %d\nPaste operations: %d", m.newLen, m.apndLen, m.pfxLen, m.trimSfxLen, m.trimPfxLen, m.replaceLen, m.dTagLen, m.delDTagLen, m.copyToFieldLen, m.pasteFromFieldLen)
	fmt.Println()
	return m
}

// validateFilenameSuffixReplace checks if the input format for filename suffix replacement is valid
func validateFilenameSuffixReplace() error {
	filenameReplaceSuffix := make([]models.FilenameReplaceSuffix, 0, len(filenameReplaceSuffixInput))

	for _, pair := range filenameReplaceSuffixInput {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) < 2 {
			return errors.New("invalid use of filename-replace-suffix, values must be written as (suffix:replacement)")
		}
		filenameReplaceSuffix = append(filenameReplaceSuffix, models.FilenameReplaceSuffix{
			Suffix:      parts[0],
			Replacement: parts[1],
		})
	}
	if len(filenameReplaceSuffix) > 0 {
		logging.I("Meta replace suffixes: %v", filenameReplaceSuffix)
		viper.Set(keys.FilenameReplaceSfx, filenameReplaceSuffix)
	}
	return nil
}

// setRenameFlag sets the rename style to apply
func setRenameFlag() {

	var renameFlag enums.ReplaceToStyle
	argRenameFlag := viper.GetString(keys.RenameStyle)

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

	case "fixes", "fix", "fixes-only", "fixesonly":
		renameFlag = enums.RENAMING_FIXES_ONLY
		logging.P("Rename style selected: %v", argRenameFlag)

	default:
		logging.D(1, "'Spaces', 'underscores' or 'fixes-only' not selected for renaming style, skipping these modifications.")
		renameFlag = enums.RENAMING_SKIP
	}
	viper.Set(keys.Rename, renameFlag)
}

// initDateReplaceFormat initializes the user's preferred format for dates
func initDateReplaceFormat() error {

	if viper.IsSet(keys.InputFileDatePfx) {
		dateFmt := viper.GetString(keys.InputFileDatePfx)

		// Trim whitespace for more robust validation
		dateFmt = strings.TrimSpace(dateFmt)

		formatEnum, err := dateEnum(dateFmt)
		if err != nil {
			return err
		}

		viper.Set(keys.FileDateFmt, formatEnum)
		logging.D(1, "Set file date format to %v", formatEnum)
	}
	return nil
}

// dateEnum returns the date format enum type
func dateEnum(dateFmt string) (formatEnum enums.DateFormat, err error) {

	if len(dateFmt) < 2 {
		return enums.DATEFMT_SKIP, fmt.Errorf("invalid date format entered as %q, please enter up to three characters (where 'Y' is yyyy and 'y' is yy)", dateFmt)
	} else {
		switch dateFmt {
		case "Ymd":
			return enums.DATEFMT_YYYY_MM_DD, nil
		case "ymd":
			return enums.DATEFMT_YY_MM_DD, nil
		case "Ydm":
			return enums.DATEFMT_YYYY_DD_MM, nil
		case "ydm":
			return enums.DATEFMT_YY_DD_MM, nil
		case "dmY":
			return enums.DATEFMT_DD_MM_YYYY, nil
		case "dmy":
			return enums.DATEFMT_DD_MM_YY, nil
		case "mdY":
			return enums.DATEFMT_MM_DD_YYYY, nil
		case "mdy":
			return enums.DATEFMT_MM_DD_YY, nil
		case "md":
			return enums.DATEFMT_MM_DD, nil
		case "dm":
			return enums.DATEFMT_DD_MM, nil
		}
	}
	return enums.DATEFMT_SKIP, fmt.Errorf("invalid date format entered as %q, please enter up to three ymd characters (where capital Y is yyyy and y is yy)", dateFmt)
}
