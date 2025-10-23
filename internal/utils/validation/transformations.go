package validation

import (
	"errors"
	"fmt"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"strings"

	"github.com/spf13/viper"
)

type MetaOpsLen struct {
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

// ValidateMetaOps parses the meta transformation operations
func ValidateMetaOps(MetaOpsInput []string) (*models.MetaOps, error) {
	logging.D(2, "Validating meta operations...")

	if len(MetaOpsInput) == 0 {
		logging.D(2, "No metadata operations passed in")
		return models.NewMetaOps(), nil // Return empty initialized struct
	}

	m := MetaOpsMapLength(MetaOpsInput, MetaOpsLen{})

	ops := &models.MetaOps{
		SetOverrides:     make(map[enums.OverrideMetaType]string, m.newLen),
		AppendOverrides:  make(map[enums.OverrideMetaType]string, m.apndLen),
		ReplaceOverrides: make(map[enums.OverrideMetaType]models.MOverrideReplacePair, m.replaceLen),
		DateTags:         make(map[string]models.MetaDateTag, m.dTagLen),
		DeleteDateTags:   make(map[string]models.MetaDateTag, m.delDTagLen),
		NewFields:        make([]models.MetaNewField, 0, m.newLen),
		Appends:          make([]models.MetaAppend, 0, m.apndLen),
		Prefixes:         make([]models.MetaPrefix, 0, m.pfxLen),
		Replaces:         make([]models.MetaReplace, 0, m.replaceLen),
		TrimSuffixes:     make([]models.MetaTrimSuffix, 0, m.trimSfxLen),
		TrimPrefixes:     make([]models.MetaTrimPrefix, 0, m.trimPfxLen),
		CopyToFields:     make([]models.CopyToField, 0, m.copyToFieldLen),
		PasteFromFields:  make([]models.PasteFromField, 0, m.pasteFromFieldLen),
	}

	for _, op := range MetaOpsInput {
		// Check validity
		parts := strings.Split(op, ":")

		if len(parts) < 3 || len(parts) > 4 {
			return nil, errors.New("malformed input meta-ops, each entry must be at least 3 parts, split by : (e.g. 'title:add:Video Title')")
		}

		field := parts[0]
		operation := parts[1]
		value := parts[2]

		switch strings.ToLower(operation) {
		case "set":
			switch field {
			case "all-credits", "credits-all":
				ops.SetOverrides[enums.OverrideMetaCredits] = value
			}

			newFieldModel := models.MetaNewField{
				Field: field,
				Value: value,
			}
			ops.NewFields = append(ops.NewFields, newFieldModel)
			logging.D(3, "Added new field op:\nField: %s\nValue: %s", newFieldModel.Field, newFieldModel.Value)

		case "append":
			switch field {
			case "all-credits", "credits-all":
				ops.AppendOverrides[enums.OverrideMetaCredits] = value
			}

			apndModel := models.MetaAppend{
				Field:  field,
				Suffix: value,
			}
			ops.Appends = append(ops.Appends, apndModel)
			logging.D(3, "Added new append op:\nField: %s\nAppend: %s", apndModel.Field, apndModel.Suffix)

		case "prefix":
			pfxModel := models.MetaPrefix{
				Field:  field,
				Prefix: value,
			}
			ops.Prefixes = append(ops.Prefixes, pfxModel)
			logging.D(3, "Added new prefix op:\nField: %s\nPrefix: %s", pfxModel.Field, pfxModel.Prefix)

		case "trim-suffix":
			tSfxModel := models.MetaTrimSuffix{
				Field:  field,
				Suffix: value,
			}
			ops.TrimSuffixes = append(ops.TrimSuffixes, tSfxModel)
			logging.D(3, "Added new suffix trim op:\nField: %s\nSuffix: %s", tSfxModel.Field, tSfxModel.Suffix)

		case "trim-prefix":
			tPfxModel := models.MetaTrimPrefix{
				Field:  field,
				Prefix: value,
			}
			ops.TrimPrefixes = append(ops.TrimPrefixes, tPfxModel)
			logging.D(3, "Added new prefix trim op:\nField: %s\nPrefix: %s", tPfxModel.Field, tPfxModel.Prefix)

		case "copy-to":
			c := models.CopyToField{
				Field: field,
				Dest:  value,
			}
			ops.CopyToFields = append(ops.CopyToFields, c)
			logging.D(3, "Added new copy/paste op:\nField: %s\nCopy To: %s", c.Field, c.Dest)

		case "paste-from":
			p := models.PasteFromField{
				Field:  field,
				Origin: value,
			}
			ops.PasteFromFields = append(ops.PasteFromFields, p)
			logging.D(3, "Added new copy/paste op:\nField: %s\nPaste From: %s", p.Field, p.Origin)

		case "replace":
			if len(parts) != 4 {
				return nil, errors.New("replacement should be in format 'field:replace:text:replacement'")
			}

			switch field {
			case "all-credits", "credits-all":
				ops.ReplaceOverrides[enums.OverrideMetaCredits] = models.MOverrideReplacePair{
					Value:       value,
					Replacement: parts[3],
				}
			}
			rModel := models.MetaReplace{
				Field:       field,
				Value:       value,
				Replacement: parts[3],
			}

			ops.Replaces = append(ops.Replaces, rModel)
			logging.D(3, "Added new replace operation:\nField: %s\nValue: %s\nReplacement: %s\n", rModel.Field, rModel.Value, rModel.Replacement)

		case "date-tag":
			if len(parts) != 4 {
				return nil, errors.New("date-tag should be in format 'field:date-tag:location:format' (Ymd is yyyy-mm-dd, ymd is yy-mm-dd)")
			}
			var loc enums.MetaDateTagLocation
			switch strings.ToLower(value) {
			case "prefix":
				loc = enums.DateTagLogPrefix
			case "suffix":
				loc = enums.DateTagLogSuffix
			default:
				return nil, errors.New("date tag location must be prefix, or suffix")
			}
			if e, err := dateEnum(parts[3]); err != nil {
				return nil, err
			} else {
				ops.DateTags[field] = models.MetaDateTag{
					Loc:    loc,
					Format: e,
				}
				logging.D(3, "Added new date tag operation:\nField: %s\nLocation: %s\nReplacement: %s\n", field, value, parts[3])
			}

		case "delete-date-tag":
			if len(parts) != 4 {
				return nil, errors.New("delete-date-tag should be in format 'field:delete-date-tag:location:format' (Ymd is yyyy-mm-dd, ymd is yy-mm-dd)")
			}
			var loc enums.MetaDateTagLocation

			switch strings.ToLower(value) {
			case "prefix":
				loc = enums.DateTagLogPrefix
			case "suffix":
				loc = enums.DateTagLogSuffix
			default:
				return nil, errors.New("date tag location must be prefix, or suffix")
			}
			if e, err := dateEnum(parts[3]); err != nil {
				return nil, err
			} else {
				ops.DeleteDateTags[field] = models.MetaDateTag{
					Loc:    loc,
					Format: e,
				}
				logging.D(3, "Added delete date tag operation:\nField: %s\nLocation: %s\nFormat %s\n", field, value, parts[3])
			}

		default:
			return nil, fmt.Errorf("unrecognized meta operation %q (valid operations: add, append, prefix, trim-suffix, trim-prefix, replace, date-tag, delete-date-tag, copy-to, copy-from)", parts[1])
		}
	}
	return ops, nil
}

// MetaOpsMapLength quickly grabs the lengths needed for each map
func MetaOpsMapLength(MetaOpsInput []string, m MetaOpsLen) MetaOpsLen {

	for _, op := range MetaOpsInput {
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
	logging.D(2, "Meta additions: %d\nMeta appends: %d\nMeta prefix: %d\nMeta suffix trim: %d\nMeta prefix trim: %d\nMeta replacements: %d\nDate tags: %d\nDelete date tags: %d\nCopy operations: %d\nPaste operations: %d", m.newLen, m.apndLen, m.pfxLen, m.trimSfxLen, m.trimPfxLen, m.replaceLen, m.dTagLen, m.delDTagLen, m.copyToFieldLen, m.pasteFromFieldLen)
	return m
}

// ValidateFilenameSuffixReplace checks if the input format for filename suffix replacement is valid
func ValidateFilenameSuffixReplace(filenameReplaceSuffixInput []string) error {
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

// ValidateDateReplaceFormat initializes the user's preferred format for dates
func ValidateDateReplaceFormat(dateFmt string) error {
	dateFmt = strings.TrimSpace(dateFmt)

	formatEnum, err := dateEnum(dateFmt)
	if err != nil {
		return err
	}

	viper.Set(keys.FileDateFmt, formatEnum)
	logging.D(1, "Set file date format to %v", formatEnum)

	return nil
}

// dateEnum returns the date format enum type
func dateEnum(dateFmt string) (formatEnum enums.DateFormat, err error) {
	if len(dateFmt) < 2 {
		return enums.DateFmtSkip, fmt.Errorf("invalid date format entered as %q, please enter up to three characters (where 'Y' is yyyy and 'y' is yy)", dateFmt)
	}

	switch dateFmt {
	case "Ymd":
		return enums.DateYyyyMmDd, nil
	case "ymd":
		return enums.DateYyMmDd, nil
	case "Ydm":
		return enums.DateYyyyDdMm, nil
	case "ydm":
		return enums.DateYyDdMm, nil
	case "dmY":
		return enums.DateDdMmYyyy, nil
	case "dmy":
		return enums.DateDdMmYy, nil
	case "mdY":
		return enums.DateMmDdYyyy, nil
	case "mdy":
		return enums.DateMmDdYy, nil
	case "md":
		return enums.DateMmDd, nil
	case "dm":
		return enums.DateDdMm, nil

		// Else, invalid operation
	default:
		return enums.DateFmtSkip, fmt.Errorf("invalid date format entered as %q, please enter up to three ymd characters (where capital Y is yyyy and y is yy)", dateFmt)
	}
}
