package consts

// Colors
const (
	ColorReset       = "\033[0m"
	ColorRed         = "\033[91m"
	ColorGreen       = "\033[92m"
	ColorYellow      = "\033[93m"
	ColorBlue        = "\033[34m"
	ColorPurple      = "\033[35m"
	ColorCyan        = "\033[96m"
	ColorDimCyan     = "\x1b[36m"
	ColorWhite       = "\033[37m"
	ColorBrightBlack = "\x1b[90m"
	ColorDimWhite    = "\x1b[2;37m"
)

// Tags
const (
	ColorRedError      string = ColorRed + "[ERROR] " + ColorReset
	ColorGreenSuccess  string = ColorGreen + "[Success] " + ColorReset
	ColorYellowDebug   string = ColorYellow + "[Debug] " + ColorReset
	ColorYellowWarning string = ColorYellow + "[Warning] " + ColorReset
	ColorBlueInfo      string = ColorCyan + "[Info] " + ColorReset
)
