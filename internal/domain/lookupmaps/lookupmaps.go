package lookupmaps

import "metarr/internal/domain/consts"

// AllVidExtensions is a map of found video extensions.
var (
	AllVidExtensions = map[string]bool{
		consts.Ext3G2: false, consts.Ext3GP: false, consts.ExtAVI: false, consts.ExtF4V: false, consts.ExtFLV: false,
		consts.ExtM4V: false, consts.ExtMKV: false, consts.ExtMOV: false, consts.ExtMP4: false, consts.ExtMPEG: false,
		consts.ExtMPG: false, consts.ExtMTS: false, consts.ExtOGM: false, consts.ExtOGV: false, consts.ExtRM: false,
		consts.ExtRMVB: false, consts.ExtTS: false, consts.ExtVOB: false, consts.ExtWEBM: false, consts.ExtWMV: false,
		consts.ExtASF: false,
	}
)

// AllMetaExtensions is a map of found metafile extensions.
var (
	AllMetaExtensions = map[string]bool{
		consts.MExtJSON: false, consts.MExtNFO: false,
	}
)
