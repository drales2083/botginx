package core

// botSignatures are case-insensitive substrings marking a non-human client.
var botSignatures = []string{
	"bot", "spider", "crawler", "slurp", "curl/", "wget/", "python-requests",
	"python-urllib", "go-http-client", "java/", "okhttp", "libwww", "httpclient",
	"headlesschrome", "phantomjs", "scrapy", "monitor", "uptime", "pingdom",
	"proofpoint", "mimecast", "barracuda", "symantec", "microsoft-cryptoapi",
	"googlebot", "bingbot", "yandex", "ahrefs", "semrush", "facebookexternalhit",
}

// proxySignatures mark an image proxy (not Apple MPP, which has its own bucket).
var proxySignatures = []string{
	"googleimageproxy", // Gmail
	"ggpht.com",
	"yahoomailproxy",
}

// appleMPPSignatures mark an Apple Mail Privacy Protection fetch. UA-only
// detection is weak; CIDR-based detection is a documented later upgrade.
var appleMPPSignatures = []string{
	"maprivacyproxy",
}

// prefetchSignatures mark a known prefetch agent.
var prefetchSignatures = []string{
	"prefetch",
}
