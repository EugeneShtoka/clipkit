package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func runWith(t *testing.T, input string, opts ...Option) string {
	t.Helper()
	var out strings.Builder
	if err := run(strings.NewReader(input), &out, opts, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return strings.TrimRight(out.String(), "\n")
}

func runJSON(t *testing.T, inputJSON, field string, extraction bool, opts ...Option) (string, error) {
	t.Helper()
	var out strings.Builder
	err := run(strings.NewReader(inputJSON), &out, opts, field, extraction)
	return strings.TrimRight(out.String(), "\n"), err
}

// ---- plain text mode -------------------------------------------------------

func TestRunPlainPassthrough(t *testing.T) {
	got := runWith(t, "  hello world  ")
	if got != "hello world" {
		t.Errorf("got %q", got)
	}
}

func TestRunPlainEmpty(t *testing.T) {
	got := runWith(t, "")
	if got != "" {
		t.Errorf("got %q", got)
	}
}

func TestRunPlainMultiLine(t *testing.T) {
	got := runWith(t, "  foo  \n  bar  ", WithCollapseSpaces())
	want := "foo\nbar"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---- JSON mode -------------------------------------------------------------

func TestRunJSONPassthrough(t *testing.T) {
	got, err := runJSON(t, `{"body":"  hello world  "}`, "body", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]string
	if e := json.Unmarshal([]byte(got), &obj); e != nil {
		t.Fatalf("output is not valid JSON: %v", e)
	}
	if obj["body"] != "hello world" {
		t.Errorf("body = %q, want %q", obj["body"], "hello world")
	}
}

func TestRunJSONExtractCode(t *testing.T) {
	got, err := runJSON(t, `{"body":"Your OTP is 482910"}`, "body", true, WithExtractCode())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "482910" {
		t.Errorf("got %q, want %q", got, "482910")
	}
}

func TestRunJSONNoMatchReturnsErrNoMatch(t *testing.T) {
	_, err := runJSON(t, `{"body":"no code here"}`, "body", true, WithExtractCode())
	if !errors.Is(err, errNoMatch) {
		t.Errorf("expected errNoMatch, got %v", err)
	}
}

func TestRunJSONExtractCodePlainOutput(t *testing.T) {
	// extraction with --json outputs just the value, not a JSON envelope
	input := `{"body":"OTP: 111222","sender":"@alice:example.com","room_name":"Work"}`
	got, err := runJSON(t, input, "body", true, WithExtractCode())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "111222" {
		t.Errorf("got %q, want %q", got, "111222")
	}
}

func TestRunJSONPreservesCustomFields(t *testing.T) {
	input := `{"body":"hello","urgency":"critical","custom_flag":true}`
	got, err := runJSON(t, input, "body", false, WithCollapseSpaces())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]json.RawMessage
	if e := json.Unmarshal([]byte(got), &obj); e != nil {
		t.Fatalf("output is not valid JSON: %v", e)
	}
	if string(obj["urgency"]) != `"critical"` {
		t.Errorf("urgency = %s, want preserved", obj["urgency"])
	}
	if string(obj["custom_flag"]) != `true` {
		t.Errorf("custom_flag = %s, want preserved", obj["custom_flag"])
	}
}

func TestRunJSONExtractURL(t *testing.T) {
	got, err := runJSON(t, `{"body":"check https://example.com now"}`, "body", true, WithExtractURL())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://example.com" {
		t.Errorf("got %q, want %q", got, "https://example.com")
	}
}

func TestRunJSONNoURLReturnsErrNoMatch(t *testing.T) {
	_, err := runJSON(t, `{"body":"no url here"}`, "body", true, WithExtractURL())
	if !errors.Is(err, errNoMatch) {
		t.Errorf("expected errNoMatch, got %v", err)
	}
}

func TestRunJSONCollapse(t *testing.T) {
	got, err := runJSON(t, `{"body":"  foo   bar  "}`, "body", false, WithCollapseSpaces())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]string
	json.Unmarshal([]byte(got), &obj)
	if obj["body"] != "foo bar" {
		t.Errorf("body = %q, want %q", obj["body"], "foo bar")
	}
}

func TestRunJSONMaxLen(t *testing.T) {
	got, err := runJSON(t, `{"body":"hello world"}`, "body", false, WithMaxLen(5))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]string
	json.Unmarshal([]byte(got), &obj)
	if obj["body"] != "hello…" {
		t.Errorf("body = %q, want %q", obj["body"], "hello…")
	}
}

func TestRunJSONEmptyFieldNoMatchOnExtract(t *testing.T) {
	_, err := runJSON(t, `{"sender":"@alice:example.com"}`, "body", true, WithExtractCode())
	if !errors.Is(err, errNoMatch) {
		t.Errorf("expected errNoMatch for missing field, got %v", err)
	}
}

func TestRunJSONCustomField(t *testing.T) {
	got, err := runJSON(t, `{"title":"  hello   world  ","other":"keep"}`, "title", false, WithCollapseSpaces())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]string
	if e := json.Unmarshal([]byte(got), &obj); e != nil {
		t.Fatalf("output is not valid JSON: %v", e)
	}
	if obj["title"] != "hello world" {
		t.Errorf("title = %q, want %q", obj["title"], "hello world")
	}
	if obj["other"] != "keep" {
		t.Errorf("other = %q, want preserved", obj["other"])
	}
}

// ---- flag validation -------------------------------------------------------
