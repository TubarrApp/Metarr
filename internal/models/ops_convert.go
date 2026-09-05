package models

import (
	"fmt"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/logger"
	"net"
	"net/url"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedenums"
	"github.com/TubarrApp/gocommon/sharedfilters"
	"github.com/TubarrApp/gocommon/sharedmodels"
)

// creditsOverrideFields set every credit field at once rather than a single field.
var creditsOverrideFields = map[string]struct{}{
	"all-credits": {},
	"credits-all": {},
}

// MetaOpsFromShared folds flat shared operations into Metarr's aggregate dispatch model.
func MetaOpsFromShared(ops []sharedmodels.MetaOps) (*MetaOps, error) {
	out := NewMetaOps()

	for _, op := range ops {
		switch op.OpType {
		case sharedconsts.OpSet:
			if _, ok := creditsOverrideFields[op.Field]; ok {
				out.SetOverrides[enums.OverrideMetaCredits] = op.OpValue
			}
			out.SetFields = append(out.SetFields, MetaSetField{
				Field: op.Field,
				Value: op.OpValue,
			})

		case sharedconsts.OpAppend:
			out.Appends = append(out.Appends, MetaAppend{
				Field:  op.Field,
				Append: op.OpValue,
			})

		case sharedconsts.OpPrefix:
			out.Prefixes = append(out.Prefixes, MetaPrefix{
				Field:  op.Field,
				Prefix: op.OpValue,
			})

		case sharedconsts.OpCopyTo:
			out.CopyToFields = append(out.CopyToFields, CopyToField{
				Field: op.Field,
				Dest:  op.OpValue,
			})

		case sharedconsts.OpPasteFrom:
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

		case sharedconsts.OpAppend:
			out.Appends = append(out.Appends, FOpAppend{Value: op.OpValue})

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

// channelHost reduces a channel URL to a comparable host, tolerating a bare domain
// such as "google.com" as well as a full URL.
func channelHost(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	host := s
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		host = u.Host
	} else if u, err := url.Parse("//" + s); err == nil && u.Host != "" {
		host = u.Host
	}

	host = strings.ToLower(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.TrimPrefix(host, "www.")
}

// MatchesChannel reports whether any of the file's URLs belong to the same host as
// chanURL, which is how a channel-scoped operation is resolved for a single file.
//
// Metarr has no channel concept of its own, so the video's own URLs stand in for it.
func (w *MetadataWebData) MatchesChannel(chanURL string) bool {
	if w == nil {
		return false
	}

	want := channelHost(chanURL)
	if want == "" {
		return false
	}

	candidates := append([]string{w.WebpageURL, w.VideoURL, w.Domain, w.Referer}, w.TryURLs...)
	for _, c := range candidates {
		if got := channelHost(c); got != "" && got == want {
			return true
		}
	}
	return false
}

// ResolveMetaOps rebuilds the file's meta operations, keeping those that apply to it.
//
// An operation is kept when it names no channel or names this file's channel. A
// filtered entry is kept when its filters all match meta, which is why this runs per
// file rather than at startup: neither the channel nor the metadata is known before.
func (fd *FileData) ResolveMetaOps(ops []sharedmodels.MetaOps, filtered []sharedmodels.FilteredMetaOps, meta map[string]any) error {
	resolved := make([]sharedmodels.MetaOps, 0, len(ops))
	for _, op := range ops {
		if fd.appliesToChannel(op.ChannelURL) {
			resolved = append(resolved, op)
			continue
		}
		logger.Pl.D(2, "Skipping meta operation %q scoped to channel %q for this file", op.OpType, op.ChannelURL)
	}

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
			if !fd.appliesToChannel(op.ChannelURL) {
				continue
			}
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

// appliesToChannel reports whether an operation scoped to chanURL applies to this file.
func (fd *FileData) appliesToChannel(chanURL string) bool {
	return chanURL == "" || fd.MWebData.MatchesChannel(chanURL)
}

// ResolveFilenameOps rebuilds the file's filename operations, keeping those that apply.
//
// Mirrors [FileData.ResolveMetaOps]: an operation is kept when it names no channel or
// names this file's channel, and a filtered entry is kept when its filters all match
// meta. The filters read metadata even though the operations act on the filename.
func (fd *FileData) ResolveFilenameOps(ops []sharedmodels.FilenameOps, filtered []sharedmodels.FilteredFilenameOps, meta map[string]any) error {
	resolved := make([]sharedmodels.FilenameOps, 0, len(ops))
	for _, op := range ops {
		if fd.appliesToChannel(op.ChannelURL) {
			resolved = append(resolved, op)
			continue
		}
		logger.Pl.D(2, "Skipping filename operation %q scoped to channel %q for this file", op.OpType, op.ChannelURL)
	}

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
			if !fd.appliesToChannel(op.ChannelURL) {
				continue
			}
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
