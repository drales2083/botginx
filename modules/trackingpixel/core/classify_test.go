package core

import "testing"

func TestClassify(t *testing.T) {
	chrome := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0 Safari/537.36"
	cases := []struct {
		name, ua, method string
		want             Classification
	}{
		{"head is prefetch", chrome, "HEAD", ClassPrefetch},
		{"google image proxy", "Mozilla/5.0 (via ggpht.com GoogleImageProxy)", "GET", ClassProxied},
		{"curl is bot", "curl/8.4.0", "GET", ClassBot},
		{"python is bot", "python-requests/2.31.0", "GET", ClassBot},
		{"googlebot is bot", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", "GET", ClassBot},
		{"go client is bot", "Go-http-client/1.1", "GET", ClassBot},
		{"normal chrome is human", chrome, "GET", ClassHuman},
		{"safari is human", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148 Safari/604.1", "GET", ClassHuman},
		{"empty ua is bot", "", "GET", ClassBot},
		{"apple mpp signature", "Mozilla/5.0 (Macintosh) MaPrivacyProxy/1.0", "GET", ClassAppleMPP},
		{"prefetch signature", "Mozilla/5.0 Prefetch-Agent/1.0", "GET", ClassPrefetch},
		{"head beats proxy", "Mozilla/5.0 (via ggpht.com GoogleImageProxy)", "HEAD", ClassPrefetch},
		{"proxy beats bot", "bot (via ggpht.com GoogleImageProxy)", "GET", ClassProxied},
		{"apple mpp beats bot", "bot maprivacyproxy", "GET", ClassAppleMPP},
		{"whitespace ua is bot", "   ", "GET", ClassBot},
		{"mixed case curl is bot", "CURL/8.4.0", "GET", ClassBot},
		{"yahoo mail proxy", "YahooMailProxy", "GET", ClassProxied},
		{"real gmail proxy ua", "Mozilla/5.0 (Windows NT 5.1; rv:11.0) Gecko Firefox/11.0 (via ggpht.com GoogleImageProxy)", "GET", ClassProxied},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Classify(c.ua, c.method); got != c.want {
				t.Errorf("Classify(%q,%q) = %v, want %v", c.ua, c.method, got, c.want)
			}
		})
	}
}
