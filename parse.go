package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var timestampRe = regexp.MustCompile(
	`^\[[\d\s\-:\.T]+\]\s*|\s*\[[\d\s\-:\.T]+\]$`,
)

// codeRe matches any 5–6 character alphanumeric word.
// Matches are further filtered to require at least one digit.
var codeRe = regexp.MustCompile(`\b[a-zA-Z0-9]{5,6}\b`)

// popularTLDs lists TLDs where bare hostnames (no protocol, no www) are auto-detected.
// For all other TLDs a path/query/fragment is still required to reduce false positives.
var popularTLDs = []string{
	// generic
	"com", "org", "net", "edu", "gov", "io", "co", "app", "dev", "ai",
	// country — EU member states
	"at", "be", "bg", "hr", "cy", "cz", "dk", "ee", "fi",
	"fr", "de", "gr", "hu", "ie", "it", "lv", "lt", "lu",
	"mt", "nl", "pl", "pt", "ro", "sk", "si", "es", "se",
	// other popular country TLDs
	"ru", "il", "us", "uk", "ca", "au", "no",
}

// urlRe matches URLs: explicit https?://, www. prefix, bare domain with a popular TLD,
// or any domain with a path/query/fragment.
var urlRe = buildURLRe()

func buildURLRe() *regexp.Regexp {
	tlds := strings.Join(popularTLDs, "|")
	return regexp.MustCompile(
		`https?://\S+` +
			`|www\.\S+` +
			`|[a-zA-Z0-9][a-zA-Z0-9.-]*\.(?:` + tlds + `)\b(?:[/?#]\S*)?` +
			`|[a-zA-Z0-9][a-zA-Z0-9.-]*\.[a-zA-Z]{2,}[/?#]\S*`,
	)
}

// borderCharRe strips border characters from the start/end of a single line.
var borderCharRe = regexp.MustCompile(`^[\s│┃|─━└┘┌┐├┤┬┴┼╔╗╚╝╠╣╦╩╬]+|[\s│┃|─━└┘┌┐├┤┬┴┼╔╗╚╝╠╣╦╩╬]+$`)

// borderLineRe matches lines that contain no letters or digits (whitespace, box-drawing, emoji, symbols).
var borderLineRe = regexp.MustCompile(`^[^\p{L}\p{N}]*$`)

// vimCmdLineRe matches vim/terminal command lines like ":rooms", ":q!", etc.
var vimCmdLineRe = regexp.MustCompile(`^:[a-zA-Z!?]+\s*$`)

// trimURLPunct strips trailing punctuation that is unlikely to be part of a URL.
// It handles unbalanced closing parens/brackets iteratively.
func trimURLPunct(u string) string {
	for {
		prev := u
		u = strings.TrimRight(u, ".,;:!?\"'")
		for _, pair := range [][2]byte{{'(', ')'}, {'[', ']'}} {
			open, close := string(pair[0]), string(pair[1])
			for strings.HasSuffix(u, close) && strings.Count(u, open) < strings.Count(u, close) {
				u = u[:len(u)-1]
			}
		}
		if u == prev {
			break
		}
	}
	return u
}

func containsDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func isBorderLine(s string) bool {
	return borderLineRe.MatchString(s) || vimCmdLineRe.MatchString(s)
}

type options struct {
	collapseSpaces  bool
	cutset          string
	maxLen          int
	stripTimestamps bool
	stripBorders    bool
	extractCode     bool
	extractURL      bool
}

type Option func(*options)

func WithCollapseSpaces() Option  { return func(o *options) { o.collapseSpaces = true } }
func WithStripTimestamps() Option { return func(o *options) { o.stripTimestamps = true } }
func WithStripBorders() Option    { return func(o *options) { o.stripBorders = true } }
func WithExtractCode() Option     { return func(o *options) { o.extractCode = true } }
func WithExtractURL() Option      { return func(o *options) { o.extractURL = true } }

func WithCutset(cutset string) Option {
	return func(o *options) { o.cutset = cutset }
}

func WithMaxLen(n int) Option {
	return func(o *options) { o.maxLen = n }
}

func Parse(s string, opts ...Option) (string, error) {
	cfg := &options{}
	for _, o := range opts {
		o(cfg)
	}

	extracting := cfg.extractCode || cfg.extractURL
	transforming := cfg.collapseSpaces || cfg.cutset != "" || cfg.maxLen > 0 || cfg.stripTimestamps || cfg.stripBorders

	if cfg.extractCode && cfg.extractURL {
		return "", fmt.Errorf("--extract-code and --extract-url are mutually exclusive")
	}
	if extracting && transforming {
		name := "--extract-code"
		if cfg.extractURL {
			name = "--extract-url"
		}
		return "", fmt.Errorf("%s is mutually exclusive with all other flags", name)
	}

	s = strings.TrimSpace(s)

	if cfg.extractURL {
		var urls []string
		for _, u := range urlRe.FindAllString(s, -1) {
			if trimmed := trimURLPunct(u); trimmed != "" {
				urls = append(urls, trimmed)
			}
		}
		return strings.Join(urls, "\n"), nil
	}

	if cfg.extractCode {
		withoutURLs := urlRe.ReplaceAllString(s, "")
		var matches []string
		for _, m := range codeRe.FindAllString(withoutURLs, -1) {
			if containsDigit(m) {
				matches = append(matches, m)
			}
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
		return "", nil
	}

	if cfg.stripBorders {
		lines := strings.Split(s, "\n")

		// Strip leading/trailing lines that are purely UI artifacts.
		lo, hi := 0, len(lines)
		for lo < hi && isBorderLine(lines[lo]) {
			lo++
		}
		for hi > lo && isBorderLine(lines[hi-1]) {
			hi--
		}
		lines = lines[lo:hi]

		// Strip border characters from the edges of each remaining line.
		for i, line := range lines {
			lines[i] = strings.TrimSpace(borderCharRe.ReplaceAllString(line, ""))
		}

		s = strings.TrimSpace(strings.Join(lines, "\n"))
	}

	if cfg.cutset != "" {
		s = strings.Trim(s, cfg.cutset)
		s = strings.TrimSpace(s)
	}

	if cfg.stripTimestamps {
		s = timestampRe.ReplaceAllString(s, "")
		s = strings.TrimSpace(s)
	}

	if cfg.collapseSpaces {
		var b strings.Builder
		inSpace := false
		for _, r := range s {
			if unicode.IsSpace(r) {
				if !inSpace {
					b.WriteRune(' ')
					inSpace = true
				}
			} else {
				b.WriteRune(r)
				inSpace = false
			}
		}
		s = b.String()
	}

	if cfg.maxLen > 0 {
		runes := []rune(s)
		if len(runes) > cfg.maxLen {
			cut := cfg.maxLen
			for cut > 0 && runes[cut-1] != ' ' {
				cut--
			}
			if cut == 0 {
				cut = cfg.maxLen
			}
			s = strings.TrimRight(string(runes[:cut]), " ") + "…"
		}
	}

	return s, nil
}
