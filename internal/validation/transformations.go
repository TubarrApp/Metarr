package validation

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"strings"
)

// ValidateSetMetaOps parses the meta transformation operations
func ValidateSetMetaOps(metaOpsInput []string) error {
	logging.D(2, "Validating meta operations...")
	if len(metaOpsInput) == 0 {
		return nil
	}
	const invalidWarning = "removing invalid meta operation %q. (Correct format style: 'title:prefix:[DOG CLIPS] ', 'title:date-tag:prefix:ymd')"

	ops := models.NewMetaOps()
	validOpsForPrintout := make([]string, 0, len(metaOpsInput))

	for _, op := range metaOpsInput {
		parts := EscapedSplit(op, ':')
		if len(parts) < 3 || len(parts) > 4 {
			return fmt.Errorf(invalidWarning, op)
		}

		field := parts[0]
		operation := parts[1]

		switch len(parts) {
		case 3:
			value := parts[2]

			switch strings.ToLower(operation) {
			case "set":
				switch field {
				case "all-credits", "credits-all":
					ops.SetOverrides[enums.OverrideMetaCredits] = value
				}
				newFieldModel := models.MetaSetField{
					Field: UnescapeSplit(field, ":"),
					Value: UnescapeSplit(value, ":"),
				}
				ops.SetFields = append(ops.SetFields, newFieldModel)
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added new field op:\nField: %s\nValue: %s", newFieldModel.Field, newFieldModel.Value)

			case "append":
				apndModel := models.MetaAppend{
					Field:  UnescapeSplit(field, ":"),
					Append: UnescapeSplit(value, ":"),
				}
				ops.Appends = append(ops.Appends, apndModel)
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added new append op:\nField: %s\nAppend: %s", apndModel.Field, apndModel.Append)

			case "prefix":
				pfxModel := models.MetaPrefix{
					Field:  UnescapeSplit(field, ":"),
					Prefix: UnescapeSplit(value, ":"),
				}
				ops.Prefixes = append(ops.Prefixes, pfxModel)
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added new prefix op:\nField: %s\nPrefix: %s", pfxModel.Field, pfxModel.Prefix)

			case "copy-to":
				c := models.CopyToField{
					Field: UnescapeSplit(field, ":"),
					Dest:  UnescapeSplit(value, ":"),
				}
				ops.CopyToFields = append(ops.CopyToFields, c)
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added new copy/paste op:\nField: %s\nCopy To: %s", c.Field, c.Dest)

			case "paste-from":
				p := models.PasteFromField{
					Field:  UnescapeSplit(field, ":"),
					Origin: UnescapeSplit(value, ":"),
				}
				ops.PasteFromFields = append(ops.PasteFromFields, p)
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added new copy/paste op:\nField: %s\nPaste From: %s", p.Field, p.Origin)
			}
		case 4:
			switch strings.ToLower(operation) {
			case "replace":
				findStr := parts[2]
				replacement := parts[3]
				rModel := models.MetaReplace{
					Field:       UnescapeSplit(field, ":"),
					Value:       UnescapeSplit(findStr, ":"),
					Replacement: UnescapeSplit(replacement, ":"),
				}
				ops.Replaces = append(ops.Replaces, rModel)
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added new replace operation:\nField: %s\nValue: %s\nReplacement: %s\n", rModel.Field, rModel.Value, rModel.Replacement)

			case "date-tag":
				loc := parts[2]
				dateFmt := parts[3]
				var dateTagLocation enums.DateTagLocation
				switch strings.ToLower(loc) {
				case "prefix":
					dateTagLocation = enums.DateTagLocPrefix
				case "suffix":
					dateTagLocation = enums.DateTagLocSuffix
				default:
					return fmt.Errorf("date tag location must be prefix, or suffix, skipping op %v", op)
				}
				e, err := dateEnum(dateFmt)
				if err != nil {
					return err
				}
				ops.DateTags[field] = models.MetaDateTag{
					// Don't need field, using map
					Loc:    dateTagLocation,
					Format: e,
				}
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added new date tag operation:\nField: %s\nLocation: %s\nReplacement: %s\n", field, loc, dateFmt)

			case "delete-date-tag":
				loc := parts[2]
				dateFmt := parts[3]
				var dateTagLocation enums.DateTagLocation
				switch strings.ToLower(loc) {
				case "prefix":
					dateTagLocation = enums.DateTagLocPrefix
				case "suffix":
					dateTagLocation = enums.DateTagLocSuffix
				case "all":
					dateTagLocation = enums.DateTagLocAll
				default:
					return fmt.Errorf("date tag location must be prefix, suffix, pr all. Skipping op %v", op)
				}
				e, err := dateEnum(dateFmt)
				if err != nil {
					return err
				}
				ops.DeleteDateTags[field] = models.MetaDeleteDateTag{
					// Don't need field, using map
					Loc:    dateTagLocation,
					Format: e,
				}
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added delete date tag operation:\nField: %s\nLocation: %s\nFormat %s\n", field, loc, dateFmt)

			case "replace-suffix":
				findSuffix := parts[2]
				replaceStr := parts[3]
				ops.ReplaceSuffixes = append(ops.ReplaceSuffixes, models.MetaReplaceSuffix{
					Field:       UnescapeSplit(field, ":"),
					Suffix:      UnescapeSplit(findSuffix, ":"),
					Replacement: UnescapeSplit(replaceStr, ":"),
				})
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added new replace suffix operation:\nFind Suffix: %s\nReplace With: %s\n", findSuffix, replaceStr)

			case "replace-prefix":
				findPrefix := parts[2]
				replaceStr := parts[3]
				ops.ReplacePrefixes = append(ops.ReplacePrefixes, models.MetaReplacePrefix{
					Field:       UnescapeSplit(field, ":"),
					Prefix:      UnescapeSplit(findPrefix, ":"),
					Replacement: UnescapeSplit(replaceStr, ":"),
				})
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added new trim prefix operation:\nFind Prefix: %s\nReplace With: %s\n", findPrefix, replaceStr)

			default:
				return fmt.Errorf(invalidWarning, op)
			}
		default:
			return fmt.Errorf(invalidWarning, op)
		}
	}
	if len(validOpsForPrintout) == 0 {
		return fmt.Errorf("no valid meta operations were entered. Got: %v", metaOpsInput)
	}
	logging.I("Added %d meta operations: %v", len(validOpsForPrintout), validOpsForPrintout)

	// Set values into Viper
	abstractions.Set(keys.MetaOpsModels, ops)
	return nil
}

// ValidateSetFilenameOps checks and validates filename operations.
func ValidateSetFilenameOps(filenameOpsInput []string) error {
	if len(filenameOpsInput) == 0 {
		logging.D(2, "No filename operations to add.")
		return nil
	}
	const invalidWarning = "removing invalid filename operation %q. (Correct format style: 'prefix:[COOL VIDEOS] ', 'date-tag:prefix:ymd')"

	fOpModel := models.NewFilenameOps()
	validOpsForPrintout := make([]string, 0, len(filenameOpsInput))

	for _, op := range filenameOpsInput {
		parts := EscapedSplit(op, ':')
		if len(parts) < 2 || len(parts) > 3 {
			return fmt.Errorf(invalidWarning, op)
		}
		operation := parts[0]
		switch len(parts) {
		case 2:
			opValue := parts[1]
			switch strings.ToLower(operation) {
			case "prefix":
				fOpModel.Prefixes = append(fOpModel.Prefixes, models.FOpPrefix{
					Value: UnescapeSplit(opValue, ":"),
				})
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added new prefix operation:\nPrefix: %s\n", opValue)

			case "append":
				fOpModel.Appends = append(fOpModel.Appends, models.FOpAppend{
					Value: UnescapeSplit(opValue, ":"),
				})
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added new append operation:\nAppend: %s\n", opValue)

			case "set":
				if fOpModel.Set.IsSet {
					return fmt.Errorf("only one set operation can be run per batch. Skipping operation %q", op)
				}
				fOpModel.Set = models.FOpSet{
					IsSet: true,
					Value: opValue,
				}
				validOpsForPrintout = append(validOpsForPrintout, op)
			}
		case 3:
			switch strings.ToLower(operation) {
			case "date-tag":
				if fOpModel.DateTag.DateFormat != enums.DateFmtSkip {
					return fmt.Errorf("only one date tag accepted per run to prevent user error")
				}
				tagLoc := parts[1]
				dateFmt := parts[2]
				var tagLocEnum enums.DateTagLocation
				switch tagLoc {
				case "prefix":
					tagLocEnum = enums.DateTagLocPrefix
				case "suffix":
					tagLocEnum = enums.DateTagLocSuffix
				default:
					return fmt.Errorf("invalid filename date tag entry. Should be 'date-tag:prefix/suffix:ymd'")
				}
				e, err := dateEnum(dateFmt)
				if err != nil {
					return fmt.Errorf("invalid date format, should be 'ymd', 'Ydm' (etc)")
				}
				fOpModel.DateTag = models.FOpDateTag{
					Loc:        tagLocEnum,
					DateFormat: e,
				}
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added date tag operation:\nLocation: %s\nFormat %s\n", tagLoc, dateFmt)

			case "delete-date-tag":
				if fOpModel.DeleteDateTags.DateFormat != enums.DateFmtSkip {
					return fmt.Errorf("only one delete date tag accepted, try using 'all' to replace all instances")
				}
				tagLoc := parts[1]
				dateFmt := parts[2]
				var tagLocEnum enums.DateTagLocation
				switch tagLoc {
				case "prefix":
					tagLocEnum = enums.DateTagLocPrefix
				case "suffix":
					tagLocEnum = enums.DateTagLocSuffix
				case "all":
					tagLocEnum = enums.DateTagLocAll
				default:
					return fmt.Errorf("invalid filename delete-date-tag entry. Should be 'delete-date-tag:prefix/suffix/all:ymd'")
				}
				e, err := dateEnum(dateFmt)
				if err != nil {
					return fmt.Errorf("invalid date format, should be 'ymd', 'Ydm' (etc)")
				}
				fOpModel.DeleteDateTags = models.FOpDeleteDateTag{
					Loc:        tagLocEnum,
					DateFormat: e,
				}
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added delete date tag operation:\nLocation: %s\nFormat %s\n", tagLoc, dateFmt)

			case "replace":
				findStr := parts[1]
				replaceStr := parts[2]
				fOpModel.Replaces = append(fOpModel.Replaces, models.FOpReplace{
					FindString:  UnescapeSplit(findStr, ":"),
					Replacement: UnescapeSplit(replaceStr, ":"),
				})
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added new replace operation:\nFind Strings: %s\nReplace With: %s\n", findStr, replaceStr)

			case "replace-suffix":
				findSuffix := parts[1]
				replaceStr := parts[2]
				fOpModel.ReplaceSuffixes = append(fOpModel.ReplaceSuffixes, models.FOpReplaceSuffix{
					Suffix:      UnescapeSplit(findSuffix, ":"),
					Replacement: UnescapeSplit(replaceStr, ":"),
				})
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added new trim suffix operation:\nFind Suffix: %s\nReplace With: %s\n", findSuffix, replaceStr)

			case "replace-prefix":
				findPrefix := parts[1]
				replaceStr := parts[2]
				fOpModel.ReplacePrefixes = append(fOpModel.ReplacePrefixes, models.FOpReplacePrefix{
					Prefix:      UnescapeSplit(findPrefix, ":"),
					Replacement: UnescapeSplit(replaceStr, ":"),
				})
				validOpsForPrintout = append(validOpsForPrintout, op)
				logging.D(3, "Added new trim prefix operation:\nFind Prefix: %s\nReplace With: %s\n", findPrefix, replaceStr)

			default:
				return fmt.Errorf(invalidWarning, op)
			}
		}
	}
	if len(validOpsForPrintout) == 0 {
		return fmt.Errorf("no valid filename operations were entered. Got: %v", filenameOpsInput)
	}
	logging.I("Added %d filename operations: %v", len(validOpsForPrintout), validOpsForPrintout)

	// Set values into Viper
	abstractions.Set(keys.FilenameOpsModels, fOpModel)
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
