package main

import "testing"

func mustParse(t *testing.T, s string, opts ...Option) string {
	t.Helper()
	out, err := Parse(s, opts...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return out
}

func TestParseBasic(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"  hello  ", "hello"},
		{"\t foo \t", "foo"},
		{"", ""},
		{"  ", ""},
	}
	for _, c := range cases {
		if got := mustParse(t, c.in); got != c.want {
			t.Errorf("Parse(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCollapseSpaces(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"foo  bar", "foo bar"},
		{"a  b  c", "a b c"},
		{"  lots   of   spaces  ", "lots of spaces"},
		{"no change", "no change"},
	}
	opt := WithCollapseSpaces()
	for _, c := range cases {
		if got := mustParse(t, c.in, opt); got != c.want {
			t.Errorf("collapse(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStripTimestamps(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"[19:54:47] hello", "hello"},
		{"hello [19:54:47]", "hello"},
		{"[2024-01-02T10:00:00] message here", "message here"},
		{"no timestamp", "no timestamp"},
	}
	opt := WithStripTimestamps()
	for _, c := range cases {
		if got := mustParse(t, c.in, opt); got != c.want {
			t.Errorf("stripTs(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStripBorders(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"│ hello │", "hello"},
		{"┃ world ┃", "world"},
		{"| text |", "text"},
		{"─── foo ───", "foo"},
		{"normal text", "normal text"},
		// corner box chars
		{"└─ foo ─┘", "foo"},
		// multiline: trailing terminal UI artifacts are stripped
		{
			"First line\nSecond line\n\n🔓︎\n└──────────────────────────────┘\n:rooms",
			"First line\nSecond line",
		},
		// multiline: leading artifact lines are stripped too
		{
			"│\nActual content\nMore content\n\n",
			"Actual content\nMore content",
		},
	}
	opt := WithStripBorders()
	for _, c := range cases {
		if got := mustParse(t, c.in, opt); got != c.want {
			t.Errorf("stripBorders(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCutset(t *testing.T) {
	opt := WithCutset("\"'")
	cases := []struct {
		in   string
		want string
	}{
		{`"hello"`, "hello"},
		{`'world'`, "world"},
		{`"mixed'`, "mixed"},
		{"no quotes", "no quotes"},
	}
	for _, c := range cases {
		if got := mustParse(t, c.in, opt); got != c.want {
			t.Errorf("cutset(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMaxLen(t *testing.T) {
	opt := WithMaxLen(10)
	cases := []struct {
		in   string
		want string
	}{
		{"short", "short"},
		{"exactly ten!", "exactly…"},
		{"word boundary test here", "word…"},
		{"nospacehere!!!", "nospaceher…"},
	}
	for _, c := range cases {
		if got := mustParse(t, c.in, opt); got != c.want {
			t.Errorf("maxLen(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExtractCode(t *testing.T) {
	opt := WithExtractCode()
	cases := []struct {
		in   string
		want string
	}{
		// pure-digit codes
		{"Your code is 483921", "483921"},
		{"Use code 12345 to login", "12345"},
		{"OTP: 999999", "999999"},
		{"  483921  ", "483921"},
		// alphanumeric codes (at least one digit)
		{"token: aB3d4e", "aB3d4e"},
		{"ref A1B2C", "A1B2C"},
		// length boundaries — too short
		{"too short 123", ""},
		{"too short 1234", ""},
		// length boundaries — too long
		{"too long 1234567", ""},
		{"too long aB3d4e5", ""},
		// all letters, no digit — not a code
		{"no digits here", ""},
		{"word abcde has no digit", ""},
		{"word abcdef has no digit", ""},
	}
	for _, c := range cases {
		if got := mustParse(t, c.in, opt); got != c.want {
			t.Errorf("extractCode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExtractCodeUniqueness(t *testing.T) {
	cases := []struct {
		desc string
		in   string
		want string
	}{
		// pure-digit uniqueness
		{"same code twice → extract", "802549 your code is 802549", "802549"},
		{"same code twice in different forms → extract", "Code: 123456 #123456", "123456"},
		{"two 6-digit codes → nothing", "codes: 111111 and 222222", ""},
		{"two 5-digit codes → nothing", "12345 and 67890", ""},
		{"one 5-digit + one 6-digit → nothing", "12345 and 123456", ""},
		{"one 5-digit + one 9-digit → extract 5-digit", "12345 and 123456789", "12345"},
		{"one 6-digit + one 9-digit → extract 6-digit", "123456 and 123456789", "123456"},
		{"only 9-digit → nothing", "123456789", ""},
		{"single 5-digit → extract", "code: 12345", "12345"},
		{"single 6-digit → extract", "code: 123456", "123456"},
		// alphanumeric uniqueness
		{"two alphanumeric codes → nothing", "aB3d4e and xY5z6w", ""},
		{"alphanumeric + pure-digit → nothing", "aB3d4e and 123456", ""},
		{"single alphanumeric → extract", "token aB3d4e here", "aB3d4e"},
		// all-letter words don't count as codes
		{"alphanumeric + all-letter word → extract alphanumeric", "aB3d4e hello world", "aB3d4e"},
		{"pure-digit + all-letter 5-char word → extract digit", "12345 hello", "12345"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			got := mustParse(t, c.in, WithExtractCode())
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestExtractURL(t *testing.T) {
	opt := WithExtractURL()
	cases := []struct {
		in   string
		want string
	}{
		// existing: explicit protocol
		{"check https://example.com now", "https://example.com"},
		{"https://a.com and https://b.com/path", "https://a.com\nhttps://b.com/path"},
		{"no url here", ""},
		{"http://insecure.org/page?q=1", "http://insecure.org/page?q=1"},
		{"  https://trimmed.com  ", "https://trimmed.com"},
		// www without protocol
		{"visit www.example.com today", "www.example.com"},
		{"www.foo.org/bar and www.baz.net", "www.foo.org/bar\nwww.baz.net"},
		// bare domain — popular TLD, no path needed
		{"see example.com for info", "example.com"},
		{"example.org", "example.org"},
		{"example.de", "example.de"},
		{"example.fi and example.dk", "example.fi\nexample.dk"},
		// bare domain with path/query (no protocol, no www)
		{"see example.com/docs for more", "example.com/docs"},
		{"go to example.com/search?q=hi", "example.com/search?q=hi"},
		// no false positive on words with similar suffix
		{"example.company details", ""},
		// trailing punctuation trimmed
		{"see https://example.com.", "https://example.com"},
		{"at https://example.com, thanks", "https://example.com"},
		{"(https://example.com)", "https://example.com"},
		{"[https://example.com]", "https://example.com"},
		// balanced parens in path preserved
		{"https://example.com/foo(bar) end", "https://example.com/foo(bar)"},
	}
	for _, c := range cases {
		if got := mustParse(t, c.in, opt); got != c.want {
			t.Errorf("extractURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExtractCodeIgnoresURLs(t *testing.T) {
	opt := WithExtractCode()
	cases := []struct {
		desc string
		in   string
		want string
	}{
		{"token in URL not extracted", "visit https://example.com/ab1c2d for info", ""},
		{"code outside URL extracted, token in URL ignored", "code 48291 see https://x.com/ab1c2d", "48291"},
		{"two codes but one in URL → single real code extracted", "https://x.com/12345 and real code 67890", "67890"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			got := mustParse(t, c.in, opt)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestExtractCodeMutuallyExclusive(t *testing.T) {
	combos := [][]Option{
		{WithExtractCode(), WithCollapseSpaces()},
		{WithExtractCode(), WithStripTimestamps()},
		{WithExtractCode(), WithStripBorders()},
		{WithExtractCode(), WithCutset("\"")},
		{WithExtractCode(), WithMaxLen(10)},
		{WithExtractCode(), WithCollapseSpaces(), WithStripTimestamps(), WithStripBorders()},
		{WithExtractCode(), WithExtractURL()},
		{WithExtractURL(), WithCollapseSpaces()},
		{WithExtractURL(), WithMaxLen(10)},
	}
	for _, opts := range combos {
		_, err := Parse("code 123456", opts...)
		if err == nil {
			t.Errorf("expected error when combining extract-code with other opts, got nil")
		}
	}
}
