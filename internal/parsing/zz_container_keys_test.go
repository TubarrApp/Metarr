package parsing

import (
	"testing"

	"github.com/TubarrApp/gocommon/sharedtags"
)

// TestGetContainerKeys tests the tag a field is written as in each kind of container.
func TestGetContainerKeys(t *testing.T) {
	for _, c := range []struct {
		key, ext, want string
	}{
		// MP4 family takes the iTunes tags.
		{sharedtags.JShow, ".mp4", sharedtags.MP4Show},
		{sharedtags.JAlbumArtist, ".m4v", sharedtags.MP4AlbumArtist},
		{sharedtags.JTrackNumber, ".f4v", sharedtags.MP4Track},
		{sharedtags.JActor, ".mp4", ""},
		{sharedtags.JTitle, ".MP4", sharedtags.MP4Title},

		// MOV and 3GP take fewer, putting the description in the comment.
		{sharedtags.JDescription, ".mov", sharedtags.MOVComment},
		{sharedtags.JComment, ".mov", ""},
		{sharedtags.JSynopsis, ".mov", ""},
		{sharedtags.JAuthor, ".mov", ""},
		{sharedtags.JAuthor, ".3gp", sharedtags.ThreeGPAuthor},
		{sharedtags.JYear, ".3g2", sharedtags.ThreeGPDate},

		// AVI only takes INFO tags FFmpeg writes.
		{sharedtags.JKeywords, ".avi", sharedtags.AVIKeywords},
		{sharedtags.JActor, ".avi", ""},
		{sharedtags.JYear, ".avi", ""},

		// Containers taking any key keep the field's own name where there's no standard one.
		{sharedtags.JActor, ".mkv", sharedtags.MatroskaActor},
		{sharedtags.JPerformer, ".webm", sharedtags.MatroskaLeadPerformer},
		{sharedtags.JSeasonTitle, ".mkv", sharedtags.JSeasonTitle},
		{sharedtags.JComment, ".ogv", ""},
		{sharedtags.JShow, ".ogm", sharedtags.JShow},
		{sharedtags.JArtist, ".wmv", sharedtags.ASFAuthor},
		{sharedtags.JAuthor, ".asf", ""},
		{sharedtags.JShow, ".flv", sharedtags.JShow},

		// Containers with no or few tags.
		{sharedtags.JTitle, ".ts", sharedtags.TSServiceName},
		{sharedtags.JDescription, ".mts", ""},
		{sharedtags.JCopyright, ".rmvb", sharedtags.RMCopyright},
		{sharedtags.JTitle, ".vob", ""},
	} {
		if got := GetContainerKeys(c.key, c.ext); got != c.want {
			t.Errorf("GetContainerKeys(%q, %q) = %q, want %q", c.key, c.ext, got, c.want)
		}
	}
}
