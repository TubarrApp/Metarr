package models

import (
	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// JSONGroupFields lists the JSON fields each "all-x" group covers. All-dates is absent as it
// only applies as an override, its fields taking different forms (year, timestamp).
var JSONGroupFields = map[string][]string{
	sharedconsts.MetaOpAllCredits: {
		sharedtags.JCreator, sharedtags.JPerformer, sharedtags.JAuthor, sharedtags.JArtist, sharedtags.JChannel,
		sharedtags.JDirector, sharedtags.JActor, sharedtags.JStudio, sharedtags.JProducer, sharedtags.JWriter,
		sharedtags.JUploader, sharedtags.JPublisher, sharedtags.JComposer,
	},
	sharedconsts.MetaOpAllDescriptions: {
		sharedtags.JLongDesc, sharedtags.JLongUnderscoreDesc, sharedtags.JDescription,
		sharedtags.JSynopsis, sharedtags.JSummary, sharedtags.JComment,
	},
	sharedconsts.MetaOpAllTitles: {sharedtags.JTitle, sharedtags.JFulltitle, sharedtags.JSubtitle},
}

// NFOGroupFields lists the NFO tags each "all-x" group covers, with all-dates absent as above.
var NFOGroupFields = map[string][]string{
	sharedconsts.MetaOpAllCredits: {
		sharedtags.NActors, sharedtags.NDirector, sharedtags.NProducer, sharedtags.NWriter,
		sharedtags.NPublisher, sharedtags.NComposer, sharedtags.NStudio, sharedtags.NProductionCompany,
	},
	sharedconsts.MetaOpAllDescriptions: {sharedtags.NPlot, sharedtags.NOutline, sharedtags.NDescription},
	sharedconsts.MetaOpAllTitles:       {sharedtags.NTitle, sharedtags.NOriginalTitle, sharedtags.NSubtitle, sharedtags.NTagline},
}

// ExpandGroups returns a copy of the operations with each "all-x" field, or copy-to destination,
// replaced by one operation per field the group covers in groups.
func (m *MetaOps) ExpandGroups(groups map[string][]string) *MetaOps {
	out := *m
	out.Appends = expandGroup(m.Appends, func(o *MetaAppend) *string { return &o.Field }, groups)
	out.Prefixes = expandGroup(m.Prefixes, func(o *MetaPrefix) *string { return &o.Field }, groups)
	out.Replaces = expandGroup(m.Replaces, func(o *MetaReplace) *string { return &o.Field }, groups)
	out.ReplacePrefixes = expandGroup(m.ReplacePrefixes, func(o *MetaReplacePrefix) *string { return &o.Field }, groups)
	out.ReplaceSuffixes = expandGroup(m.ReplaceSuffixes, func(o *MetaReplaceSuffix) *string { return &o.Field }, groups)
	out.RegexReplaces = expandGroup(m.RegexReplaces, func(o *MetaRegexReplace) *string { return &o.Field }, groups)
	out.Cuts = expandGroup(m.Cuts, func(o *MetaCut) *string { return &o.Field }, groups)
	out.CopyToFields = expandGroup(m.CopyToFields, func(o *CopyToField) *string { return &o.Dest }, groups)
	out.PasteFromFields = expandGroup(m.PasteFromFields, func(o *PasteFromField) *string { return &o.Field }, groups)
	out.DateTags = expandGroupKeys(m.DateTags, groups)
	out.DeleteDateTags = expandGroupKeys(m.DeleteDateTags, groups)
	return &out
}

// expandGroup replaces each operation whose field names a group with one per field in it.
func expandGroup[T any](ops []T, field func(*T) *string, groups map[string][]string) []T {
	out := make([]T, 0, len(ops))
	for _, op := range ops {
		fields, isGroup := groups[*field(&op)]
		if !isGroup {
			out = append(out, op)
			continue
		}
		for _, f := range fields {
			*field(&op) = f
			out = append(out, op)
		}
	}
	return out
}

// expandGroupKeys replaces each key naming a group with one entry per field in it, letting an
// entry for a single field take precedence over its group's.
func expandGroupKeys[V any](m map[string]V, groups map[string][]string) map[string]V {
	out := make(map[string]V, len(m))
	for k, v := range m {
		for _, f := range groups[k] {
			out[f] = v
		}
	}
	for k, v := range m {
		if _, isGroup := groups[k]; !isGroup {
			out[k] = v
		}
	}
	return out
}
