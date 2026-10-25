package models

import (
	"slices"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// JSONGroupFields maps a field named in an operation to every JSON field the operation edits.
//
// An "all-x" name covers every field of that kind. A plain name also covers its other forms, so the edit
// reaches whichever form ends up in the video. All-dates is absent as it only applies as an override.
var JSONGroupFields = map[string][]string{
	// Every field of a kind.
	sharedconsts.MetaOpAllCredits: {
		sharedtags.JCreator, sharedtags.JPerformer, sharedtags.JAuthor, sharedtags.JArtist, sharedtags.JAlbumArtist,
		sharedtags.JChannel, sharedtags.JDirector, sharedtags.JActor, sharedtags.JStudio, sharedtags.JProducer,
		sharedtags.JWriter, sharedtags.JUploader, sharedtags.JPublisher, sharedtags.JComposer,
	},
	sharedconsts.MetaOpAllDescriptions: jsonDescriptionFields,
	sharedconsts.MetaOpAllTitles:       {sharedtags.JTitle, sharedtags.JFulltitle, sharedtags.JSubtitle},

	// A plain field and its other forms.
	sharedtags.JArtist:      {sharedtags.JArtist, sharedtags.JAlbumArtist},
	sharedtags.JDescription: jsonDescriptionFields,
	sharedtags.JTitle:       {sharedtags.JTitle, sharedtags.JFulltitle},
	sharedtags.JYear:        {sharedtags.JYear, sharedtags.JReleaseYear},
}

// jsonDescriptionFields lists the JSON description fields.
var jsonDescriptionFields = []string{
	sharedtags.JLongDesc, sharedtags.JLongUnderscoreDesc, sharedtags.JDescription,
	sharedtags.JSynopsis, sharedtags.JSummary, sharedtags.JComment,
}

// NFOGroupFields maps a tag named in an operation to every NFO tag the operation edits, as JSONGroupFields does.
var NFOGroupFields = map[string][]string{
	// Every tag of a kind.
	sharedconsts.MetaOpAllCredits: {
		sharedtags.NActors, sharedtags.NDirector, sharedtags.NProducer, sharedtags.NWriter,
		sharedtags.NPublisher, sharedtags.NComposer, sharedtags.NStudio, sharedtags.NProductionCompany,
	},
	sharedconsts.MetaOpAllDescriptions: nfoDescriptionFields,
	sharedconsts.MetaOpAllTitles:       {sharedtags.NTitle, sharedtags.NOriginalTitle, sharedtags.NSubtitle, sharedtags.NTagline},

	// A plain tag and its other forms.
	sharedtags.NDescription: nfoDescriptionFields,
	sharedtags.NTitle:       {sharedtags.NTitle, sharedtags.NOriginalTitle},
}

// nfoDescriptionFields lists the NFO description tags.
var nfoDescriptionFields = []string{sharedtags.NPlot, sharedtags.NOutline, sharedtags.NDescription}

// ExpandGroups returns a copy of the operations with each operation on a group replaced by one per field in it.
//
// The operations it's called on are left unchanged.
func (m *MetaOps) ExpandGroups(groups map[string][]string) *MetaOps {
	out := *m

	// Sets.
	out.SetFields = make([]MetaSetField, 0, len(m.SetFields))
	for _, op := range m.SetFields {
		for _, field := range fieldsCovered(op.Field, groups) {
			op.Field = field
			out.SetFields = append(out.SetFields, op)
		}
	}

	// Prefixes.
	out.Prefixes = make([]MetaPrefix, 0, len(m.Prefixes))
	for _, op := range m.Prefixes {
		for _, field := range fieldsCovered(op.Field, groups) {
			op.Field = field
			out.Prefixes = append(out.Prefixes, op)
		}
	}

	// Suffixes.
	out.Suffixes = make([]MetaSuffix, 0, len(m.Suffixes))
	for _, op := range m.Suffixes {
		for _, field := range fieldsCovered(op.Field, groups) {
			op.Field = field
			out.Suffixes = append(out.Suffixes, op)
		}
	}

	// Replaces.
	out.Replaces = make([]MetaReplace, 0, len(m.Replaces))
	for _, op := range m.Replaces {
		for _, field := range fieldsCovered(op.Field, groups) {
			op.Field = field
			out.Replaces = append(out.Replaces, op)
		}
	}

	// Prefix replaces.
	out.ReplacePrefixes = make([]MetaReplacePrefix, 0, len(m.ReplacePrefixes))
	for _, op := range m.ReplacePrefixes {
		for _, field := range fieldsCovered(op.Field, groups) {
			op.Field = field
			out.ReplacePrefixes = append(out.ReplacePrefixes, op)
		}
	}

	// Suffix replaces.
	out.ReplaceSuffixes = make([]MetaReplaceSuffix, 0, len(m.ReplaceSuffixes))
	for _, op := range m.ReplaceSuffixes {
		for _, field := range fieldsCovered(op.Field, groups) {
			op.Field = field
			out.ReplaceSuffixes = append(out.ReplaceSuffixes, op)
		}
	}

	// Regex replaces.
	out.RegexReplaces = make([]MetaRegexReplace, 0, len(m.RegexReplaces))
	for _, op := range m.RegexReplaces {
		for _, field := range fieldsCovered(op.Field, groups) {
			op.Field = field
			out.RegexReplaces = append(out.RegexReplaces, op)
		}
	}

	// Cuts.
	out.Cuts = make([]MetaCut, 0, len(m.Cuts))
	for _, op := range m.Cuts {
		for _, field := range fieldsCovered(op.Field, groups) {
			op.Field = field
			out.Cuts = append(out.Cuts, op)
		}
	}

	// Copies, where only the destination can name a group.
	out.CopyToFields = make([]CopyToField, 0, len(m.CopyToFields))
	for _, op := range m.CopyToFields {
		for _, field := range fieldsCovered(op.Dest, groups) {
			op.Dest = field
			out.CopyToFields = append(out.CopyToFields, op)
		}
	}

	// Pastes, where only the field pasted into can name a group.
	out.PasteFromFields = make([]PasteFromField, 0, len(m.PasteFromFields))
	for _, op := range m.PasteFromFields {
		for _, field := range fieldsCovered(op.Field, groups) {
			op.Field = field
			out.PasteFromFields = append(out.PasteFromFields, op)
		}
	}

	// Date tags, which are keyed by field.
	out.DateTags = expandGroupKeys(m.DateTags, groups)
	out.DeleteDateTags = expandGroupKeys(m.DeleteDateTags, groups)
	return &out
}

// fieldsCovered returns the fields an operation on field edits: the group's fields if it names a group, else itself.
func fieldsCovered(field string, groups map[string][]string) []string {
	if groupFields, isGroup := groups[field]; isGroup {
		return groupFields
	}
	return []string{field}
}

// expandGroupKeys returns a copy of entries with each group key swapped for the fields that group covers.
//
// For example, the date tags from "all-titles:date-tag:prefix:ymd" and "fulltitle:date-tag:suffix:ymd":
//
//	before: all-titles → prefix, fulltitle → suffix
//	after:  title → prefix, fulltitle → suffix, subtitle → prefix
//
// Where two entries reach the same field, the more specific one wins, which is why fulltitle keeps its suffix.
// A single field beats any group, and a smaller group beats a larger one ("title" beats "all-titles").
func expandGroupKeys[V any](entries map[string]V, groups map[string][]string) map[string]V {
	out := make(map[string]V, len(entries))

	// Collect the keys naming groups.
	var groupKeys []string
	for key := range entries {
		if _, isGroup := groups[key]; isGroup {
			groupKeys = append(groupKeys, key)
		}
	}

	// Order them largest group first, so each smaller group overwrites the larger ones before it.
	slices.SortFunc(groupKeys, func(a, b string) int {
		if len(groups[a]) != len(groups[b]) {
			return len(groups[b]) - len(groups[a])
		}
		return strings.Compare(a, b) // Same size: alphabetical, so the order is the same every run.
	})

	// Expand the groups.
	for _, key := range groupKeys {
		for _, field := range groups[key] {
			out[field] = entries[key]
		}
	}

	// Single fields last, so they overwrite any group's entry.
	for key, value := range entries {
		if _, isGroup := groups[key]; !isGroup {
			out[key] = value
		}
	}
	return out
}
