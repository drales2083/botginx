// pkg/customizer/types.go
package customizer

type Customization struct {
	// Background
	BgColor          string `json:"bgColor"`
	BgColorSecondary string `json:"bgColorSecondary"`
	GradientEnabled  bool   `json:"gradientEnabled"`
	Pattern          string `json:"pattern"`
	PatternColor     string `json:"patternColor"`
	PatternOpacity   int    `json:"patternOpacity"`

	// Loader
	Loader               string `json:"loader"`
	LoaderColorPrimary   string `json:"loaderColorPrimary"`
	LoaderColorSecondary string `json:"loaderColorSecondary"`

	// Text
	Heading           string `json:"heading"`
	HeadingVisible    bool   `json:"headingVisible"`
	Subheading        string `json:"subheading"`
	SubheadingVisible bool   `json:"subheadingVisible"`
	Font              string `json:"font"`
	FontWeight        int    `json:"fontWeight"`
	TextColor         string `json:"textColor"`
	TextSize          int    `json:"textSize"`
	TextShadow        bool   `json:"textShadow"`

	// Image/Logo
	ImageMode    string `json:"imageMode"`
	ImageDataURL string `json:"imageDataUrl"`
	ImageSize    int    `json:"imageSize"`
	ImageOverlay int    `json:"imageOverlay"`

	// Layout
	ContentOrder string `json:"contentOrder"`
	TextAlign    string `json:"textAlign"`
	VPos         int    `json:"vPos"`
	Gap          int    `json:"gap"`
	PageTitle    string `json:"pageTitle"`
}

type LoaderDef struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type PatternDef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type FontDef struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Weights  []int  `json:"weights"`
}

type GenerateOptions struct {
	Customization   Customization
	RedirectURL     string   // single URL (legacy)
	RedirectURLs    []string // multiple URLs (random rotation)
	Delay           int
	Mode            string
	RandomizeSource bool // generate PHP with per-request randomization
	PassParams      bool // append query params and hash to destination URL
}
