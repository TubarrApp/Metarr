package models

import (
	"fmt"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/logger"
	"regexp"
	"slices"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedenums"
	"github.com/TubarrApp/gocommon/sharedfilters"
	"github.com/TubarrApp/gocommon/sharedmodels"
)

// MetaOpsFromShared folds flat shared operations into Metarr's aggregate dispatch model.
func MetaOpsFromShared(ops []sharedmodels.MetaOps) (*MetaOps, error) {
	out := NewMetaOps()

	for _, op := range ops {
		switch op.OpType {
		case sharedconsts.OpSet:
			// "all-x" operations override a field group and are never written as fields themselves.
			switch op.Field {
			case sharedconsts.MetaOpAllCredits:
				out.SetOverrides[enums.OverrideMetaCredits] = op.OpValue
			case sharedconsts.MetaOpAllDates:
				out.SetOverrides[enums.OverrideMetaDates] = op.OpValue
			case sharedconsts.MetaOpAllDescriptions:
				out.SetOverrides[enums.OverrideMetaDescriptions] = op.OpValue
			case sharedconsts.MetaOpAllTitles:
				out.SetOverrides[enums.OverrideMetaTitles] = op.OpValue
			default:
				out.SetFields = append(out.SetFields, MetaSetField{
					Field: op.Field,
					Value: op.OpValue,
				})
			}

		case sharedconsts.OpPrefix:
			out.Prefixes = append(out.Prefixes, MetaPrefix{
				Field:  op.Field,
				Prefix: op.OpValue,
			})

		case sharedconsts.OpSuffix:
			out.Suffixes = append(out.Suffixes, MetaSuffix{
				Field:  op.Field,
				Suffix: op.OpValue,
			})

		case sharedconsts.OpCopyTo:
			if op.OpValue == sharedconsts.MetaOpAllDates {
				out.DatesFrom = op.Field
				break
			}
			out.CopyToFields = append(out.CopyToFields, CopyToField{
				Field: op.Field,
				Dest:  op.OpValue,
			})

		case sharedconsts.OpPasteFrom:
			if op.Field == sharedconsts.MetaOpAllDates {
				out.DatesFrom = op.OpValue
				break
			}
			out.PasteFromFields = append(out.PasteFromFields, PasteFromField{
				Field:  op.Field,
				Origin: op.OpValue,
			})

		case sharedconsts.OpDateTag:
			loc, dateFmt, err := dateTagParts(op.OpType, op.OpLoc, op.DateFormat, false)
			if err != nil {
				return nil, err
			}
			out.DateTags[op.Field] = MetaDateTag{Loc: loc, Format: dateFmt}

		case sharedconsts.OpDeleteDateTag:
			loc, dateFmt, err := dateTagParts(op.OpType, op.OpLoc, op.DateFormat, true)
			if err != nil {
				return nil, err
			}
			out.DeleteDateTags[op.Field] = MetaDeleteDateTag{Loc: loc, Format: dateFmt}

		case sharedconsts.OpReplace:
			out.Replaces = append(out.Replaces, MetaReplace{
				Field:       op.Field,
				Value:       op.OpFindString,
				Replacement: op.OpValue,
			})

		case sharedconsts.OpReplacePrefix:
			out.ReplacePrefixes = append(out.ReplacePrefixes, MetaReplacePrefix{
				Field:       op.Field,
				Prefix:      op.OpFindString,
				Replacement: op.OpValue,
			})

		case sharedconsts.OpReplaceSuffix:
			out.ReplaceSuffixes = append(out.ReplaceSuffixes, MetaReplaceSuffix{
				Field:       op.Field,
				Suffix:      op.OpFindString,
				Replacement: op.OpValue,
			})

		case sharedconsts.OpRegexReplace:
			re, err := regexp.Compile(op.OpFindString)
			if err != nil {
				return nil, fmt.Errorf("invalid regex-replace pattern %q for field %q: %w", op.OpFindString, op.Field, err)
			}
			out.RegexReplaces = append(out.RegexReplaces, MetaRegexReplace{
				Field:       op.Field,
				Regex:       re,
				Replacement: op.OpValue,
			})

		case sharedconsts.OpKeepAfter, sharedconsts.OpKeepBefore:
			out.Cuts = append(out.Cuts, MetaCut{
				Field:      op.Field,
				Delim:      op.OpValue,
				KeepBefore: op.OpType == sharedconsts.OpKeepBefore,
			})

		default:
			return nil, fmt.Errorf("unhandled meta operation type %q for field %q", op.OpType, op.Field)
		}
		logger.Pl.D(3, "Added meta operation %q for field %q", op.OpType, op.Field)
	}
	return out, nil
}

// FilenameOpsFromShared folds flat shared operations into Metarr's aggregate dispatch
// model. Set and the two date tag operations are limited to one each per run.
func FilenameOpsFromShared(ops []sharedmodels.FilenameOps) (*FilenameOps, error) {
	out := NewFilenameOps()

	for _, op := range ops {
		switch op.OpType {
		case sharedconsts.OpPrefix:
			out.Prefixes = append(out.Prefixes, FOpPrefix{Value: op.OpValue})

		case sharedconsts.OpSuffix:
			out.Suffixes = append(out.Suffixes, FOpSuffix{Value: op.OpValue})

		case sharedconsts.OpSet:
			if out.Set.IsSet {
				return nil, fmt.Errorf("only one set operation can be run per batch, got a second with value %q", op.OpValue)
			}
			out.Set = FOpSet{IsSet: true, Value: op.OpValue}

		case sharedconsts.OpDateTag:
			if out.DateTag.DateFormat != enums.DateFmtSkip {
				return nil, fmt.Errorf("only one date tag accepted per run to prevent user error")
			}
			loc, dateFmt, err := dateTagParts(op.OpType, op.OpLoc, op.DateFormat, false)
			if err != nil {
				return nil, err
			}
			out.DateTag = FOpDateTag{Loc: loc, DateFormat: dateFmt}

		case sharedconsts.OpDeleteDateTag:
			if out.DeleteDateTags.DateFormat != enums.DateFmtSkip {
				return nil, fmt.Errorf("only one delete date tag accepted, try using %q to replace all instances", sharedconsts.OpLocAll)
			}
			loc, dateFmt, err := dateTagParts(op.OpType, op.OpLoc, op.DateFormat, true)
			if err != nil {
				return nil, err
			}
			out.DeleteDateTags = FOpDeleteDateTag{Loc: loc, DateFormat: dateFmt}

		case sharedconsts.OpReplace:
			out.Replaces = append(out.Replaces, FOpReplace{
				FindString:  op.OpFindString,
				Replacement: op.OpValue,
			})

		case sharedconsts.OpReplacePrefix:
			out.ReplacePrefixes = append(out.ReplacePrefixes, FOpReplacePrefix{
				Prefix:      op.OpFindString,
				Replacement: op.OpValue,
			})

		case sharedconsts.OpReplaceSuffix:
			out.ReplaceSuffixes = append(out.ReplaceSuffixes, FOpReplaceSuffix{
				Suffix:      op.OpFindString,
				Replacement: op.OpValue,
			})

		case sharedconsts.OpRegexReplace:
			re, err := regexp.Compile(op.OpFindString)
			if err != nil {
				return nil, fmt.Errorf("invalid regex-replace pattern %q: %w", op.OpFindString, err)
			}
			out.RegexReplaces = append(out.RegexReplaces, FOpRegexReplace{Regex: re, Replacement: op.OpValue})

		case sharedconsts.OpKeepAfter, sharedconsts.OpKeepBefore:
			out.Cuts = append(out.Cuts, TextCut{Delim: op.OpValue, KeepBefore: op.OpType == sharedconsts.OpKeepBefore})

		default:
			return nil, fmt.Errorf("unhandled filename operation type %q", op.OpType)
		}
		logger.Pl.D(3, "Added filename operation %q", op.OpType)
	}
	return out, nil
}

// dateTagParts resolves the location and format of a date tag operation.
func dateTagParts(opType, opLoc, dateFormat string, allowAll bool) (enums.DateTagLocation, enums.DateFormat, error) {
	loc, err := sharedenums.ParseDateTagLocation(opLoc, allowAll)
	if err != nil {
		return loc, enums.DateFmtSkip, fmt.Errorf("%s: %w", opType, err)
	}

	dateFmt, err := sharedenums.ParseDateFormat(dateFormat)
	if err != nil {
		return loc, dateFmt, fmt.Errorf("%s: %w", opType, err)
	}
	return loc, dateFmt, nil
}

// ResolveMetaOps rebuilds the file's meta operations from ops and the filtered entries whose filters all match meta.
//
// Runs per file rather than at startup, since the metadata isn't known before.
func (fd *FileData) ResolveMetaOps(ops []sharedmodels.MetaOps, filtered []sharedmodels.FilteredMetaOps, meta map[string]any) error {
	resolved := slices.Clone(ops)
	for _, fmo := range filtered {
		matched, err := sharedfilters.MatchAll(meta, fmo.Filters)
		if err != nil {
			return err
		}
		if !matched {
			logger.Pl.D(2, "Filters %v did not match, skipping %d operation(s)", fmo.Filters, len(fmo.MetaOps))
			continue
		}

		for _, op := range fmo.MetaOps {
			logger.Pl.I("Filters matched, applying meta operation %q to field %q", op.OpType, op.Field)
			resolved = append(resolved, op)
		}
	}

	built, err := MetaOpsFromShared(resolved)
	if err != nil {
		return err
	}
	fd.MetaOps = built
	return nil
}

// ResolveFilenameOps rebuilds the file's filename operations from ops and the filtered entries whose filters all match
// meta, as [FileData.ResolveMetaOps] does. The filters read metadata even though the operations act on the filename.
func (fd *FileData) ResolveFilenameOps(ops []sharedmodels.FilenameOps, filtered []sharedmodels.FilteredFilenameOps, meta map[string]any) error {
	resolved := slices.Clone(ops)
	for _, ffo := range filtered {
		matched, err := sharedfilters.MatchAll(meta, ffo.Filters)
		if err != nil {
			return err
		}
		if !matched {
			logger.Pl.D(2, "Filters %v did not match, skipping %d filename operation(s)", ffo.Filters, len(ffo.FilenameOps))
			continue
		}

		for _, op := range ffo.FilenameOps {
			logger.Pl.I("Filters matched, applying filename operation %q", op.OpType)
			resolved = append(resolved, op)
		}
	}

	built, err := FilenameOpsFromShared(resolved)
	if err != nil {
		return err
	}
	fd.FilenameOps = built
	return nil
}
