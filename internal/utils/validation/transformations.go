package validation

import (
	"errors"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"strings"
)

// ValidateMetaOps parses the meta transformation operations
func ValidateMetaOps(MetaOpsInput []string) (*models.MetaOps, error) {
	logging.D(2, "Validating meta operations...")

	if len(MetaOpsInput) == 0 {
		logging.D(2, "No metadata operations passed in")
		return models.NewMetaOps(), nil // Return empty initialized struct
	}

	ops := models.NewMetaOps()
	for _, op := range MetaOpsInput {
		// Check validity
		parts := strings.Split(op, ":")

		if len(parts) < 3 || len(parts) > 4 {
			return nil, errors.New("malformed input meta-ops, each entry must be at least 3 parts, split by : (e.g. 'title:set:Video Title')")
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
			var loc enums.DateTagLocation
			switch strings.ToLower(value) {
			case "prefix":
				loc = enums.DateTagLocPrefix
			case "suffix":
				loc = enums.DateTagLocSuffix
			default:
				return nil, errors.New("date tag location must be prefix, or suffix")
			}
			e, err := dateEnum(parts[3])
			if err != nil {
				return nil, err
			}

			ops.DateTags[field] = models.MetaDateTag{
				Loc:    loc,
				Format: e,
			}
			logging.D(3, "Added new date tag operation:\nField: %s\nLocation: %s\nReplacement: %s\n", field, value, parts[3])

		case "delete-date-tag":
			if len(parts) != 4 {
				return nil, errors.New("delete-date-tag should be in format 'field:delete-date-tag:location:format' (Ymd is yyyy-mm-dd, ymd is yy-mm-dd)")
			}
			var loc enums.DateTagLocation

			switch strings.ToLower(value) {
			case "prefix":
				loc = enums.DateTagLocPrefix
			case "suffix":
				loc = enums.DateTagLocSuffix
			default:
				return nil, errors.New("date tag location must be prefix, or suffix")
			}

			e, err := dateEnum(parts[3])
			if err != nil {
				return nil, err
			}

			ops.DeleteDateTags[field] = models.MetaDateTag{
				Loc:    loc,
				Format: e,
			}
			logging.D(3, "Added delete date tag operation:\nField: %s\nLocation: %s\nFormat %s\n", field, value, parts[3])

		default:
			return nil, fmt.Errorf("unrecognized meta operation %q (valid operations: set, append, prefix, trim-suffix, trim-prefix, replace, date-tag, delete-date-tag, copy-to, paste-from)", parts[1])
		}
	}
	return ops, nil
}

// ValidateSetFilenameOps checks and validates filename operations.
func ValidateSetFilenameOps(filenameOps []string) error {
	if len(filenameOps) == 0 {
		logging.D(2, "No filename operations to add.")
		return nil
	}
	const invalidWarning = "Removing invalid filename operation %q. (Correct format style: 'prefix:[COOL VIDEOS] ', 'date-tag:prefix:ymd')"
	fOpModel := &models.FilenameOps{}
	validOps := make([]string, 0, len(filenameOps))
	for _, op := range filenameOps {
		opParts := strings.Split(op, ":")
		if len(opParts) < 2 || len(opParts) > 3 {
			logging.W(invalidWarning, op)
			continue
		}
		opType := opParts[0]
		switch len(opParts) {
		case 2:
			opValue := opParts[1]
			switch opType {
			case "prefix":
				fOpModel.Prefixes = append(fOpModel.Prefixes, models.FOpPrefix{
					Value: opValue,
				})
				validOps = append(validOps, op)
			case "append":
				fOpModel.Appends = append(fOpModel.Appends, models.FOpAppend{
					Value: opValue,
				})
				validOps = append(validOps, op)
			}
		case 3:
			switch opType {
			case "date-tag":
				if fOpModel.DateTag != nil {
					logging.W("Only one date tag accepted per run to prevent user error")
					continue
				}
				tagLoc := opParts[1]
				dateFmt := opParts[2]
				var tagLocEnum enums.DateTagLocation
				switch tagLoc {
				case "prefix":
					tagLocEnum = enums.DateTagLocPrefix
				case "suffix":
					tagLocEnum = enums.DateTagLocSuffix
				default:
					logging.E("Invalid filename date tag entry. Should be 'date-tag:prefix/suffix:ymd'")
					continue
				}
				e, err := dateEnum(dateFmt)
				if err != nil {
					logging.E("Invalid date format, should be 'ymd', 'Ydm' (etc)")
					continue
				}
				fOpModel.DateTag = &models.FOpDateTag{
					Loc:        tagLocEnum,
					DateFormat: e,
				}
				validOps = append(validOps, op)
			case "delete-date-tag":
				if fOpModel.DeleteDateTags != nil {
					logging.W("Only one delete date tag accepted, try using 'all' to replace all instances")
					continue
				}
				tagLoc := opParts[1]
				dateFmt := opParts[2]
				var tagLocEnum enums.DateTagLocation
				switch tagLoc {
				case "prefix":
					tagLocEnum = enums.DateTagLocPrefix
				case "suffix":
					tagLocEnum = enums.DateTagLocSuffix
				case "all":
					tagLocEnum = enums.DateTagLocAll
				default:
					logging.E("Invalid filename delete-date-tag entry. Should be 'delete-date-tag:prefix/suffix/all:ymd'")
					continue
				}
				e, err := dateEnum(dateFmt)
				if err != nil {
					logging.E("Invalid date format, should be 'ymd', 'Ydm' (etc)")
					continue
				}
				fOpModel.DeleteDateTags = &models.FOpDeleteDateTag{
					Loc:        tagLocEnum,
					DateFormat: e,
				}
				validOps = append(validOps, op)
			case "replace":
				findStr := opParts[1]
				replaceStr := opParts[2]
				fOpModel.Replaces = append(fOpModel.Replaces, models.FOpReplace{
					FindString:  findStr,
					Replacement: replaceStr,
				})
				validOps = append(validOps, op)
			case "trim-suffix":
				findSuffix := opParts[1]
				replaceStr := opParts[2]
				fOpModel.ReplaceSuffixes = append(fOpModel.ReplaceSuffixes, models.FOpReplaceSuffix{
					Suffix:      findSuffix,
					Replacement: replaceStr,
				})
				validOps = append(validOps, op)
			case "trim-prefix":
				findPrefix := opParts[1]
				replaceStr := opParts[2]
				fOpModel.ReplacePrefixes = append(fOpModel.ReplacePrefixes, models.FOpReplacePrefix{
					Prefix:      findPrefix,
					Replacement: replaceStr,
				})
				validOps = append(validOps, op)
			default:
				logging.E(invalidWarning, op)
				continue
			}
		}
	}
	if len(validOps) == 0 {
		return fmt.Errorf("no valid filename operations were entered. Got: %v", filenameOps)
	}
	logging.I("Added %d filename operations: %v", len(validOps), validOps)

	// Set values into Viper
	abstractions.Set(keys.FilenameOpsModels, fOpModel)
	if fOpModel.DeleteDateTags != nil {
		abstractions.Set(keys.FilenameDeleteDateTags, fOpModel.DeleteDateTags)
	}

	return nil
}

// dateEnum returns the date format enum type
func dateEnum(dateFmt string) (formatEnum enums.DateFormat, err error) {
	if len(dateFmt) < 2 || len(dateFmt) > 3 {
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
