# clipkit

A small CLI that reads text from stdin, transforms it, and writes to stdout. Designed to work standalone or as a pipe processor in [mxctl](https://github.com/EugeneShtoka/mxctl).

## Install

```sh
go install github.com/EugeneShtoka/clipkit@latest
```

## Usage

```
clipkit [flags]
```

Reads lines from stdin, applies the requested transformations, and writes to stdout.

### Flags

| Flag | Description |
|------|-------------|
| `--collapse` | Collapse internal whitespace runs to a single space |
| `--cutset <chars>` | Trim the given characters from both ends |
| `--max-len <n>` | Truncate at word boundary to n runes (0 = off) |
| `--strip-timestamps` | Remove bracketed timestamps at start/end, e.g. `[19:54:47]` |
| `--strip-borders` | Remove box-drawing border characters (`│ ┃ ─ ━ \|`) from edges |
| `--extract-code` | Extract a single 5–6 char alphanumeric code; exits 2 if none found |
| `--extract-url` | Extract all URLs, one per line; exits 2 if none found |

Extraction flags (`--extract-code`, `--extract-url`) are mutually exclusive with each other and with all transformation flags.

### Examples

```sh
# Collapse whitespace
echo '  hello   world  ' | clipkit --collapse

# Strip terminal UI artifacts from a log
cat app.log | clipkit --strip-borders --strip-timestamps --collapse

# Extract an OTP code
echo 'Your code is 482910' | clipkit --extract-code

# Extract URLs (detects https://, www., bare domains, and popular TLDs)
echo 'Visit example.com or https://docs.example.org/guide' | clipkit --extract-url
```

## URL Detection

`--extract-url` detects URLs in three forms:

- Explicit protocol: `https://example.com`, `http://example.com`
- `www.` prefix: `www.example.com`
- Bare domain with a popular TLD (no protocol needed): `example.com`, `example.de`
- Any domain with a path/query/fragment: `example.xyz/path`

Trailing punctuation (`.`, `,`, `)`, etc.) is automatically trimmed from matched URLs.

Supported bare TLDs: all 27 EU country codes (`at`, `be`, `bg`, `hr`, `cy`, `cz`, `dk`, `ee`, `fi`, `fr`, `de`, `gr`, `hu`, `ie`, `it`, `lv`, `lt`, `lu`, `mt`, `nl`, `pl`, `pt`, `ro`, `sk`, `si`, `es`, `se`) plus `com`, `org`, `net`, `edu`, `gov`, `io`, `co`, `app`, `dev`, `ai`, `ru`, `il`, `us`, `uk`, `ca`, `au`, `no`.

## mxctl Pipe Mode

When called with `--config` and `--event`, clipkit operates in pipe mode for [mxctl](https://github.com/EugeneShtoka/mxctl). It reads a JSON object from stdin, transforms the `body` field, and writes the updated JSON to stdout. Exits 2 (silently stops the pipe chain) if extraction finds nothing.

```sh
echo '{"body":"OTP: 482910"}' | clipkit --config '{"extract_code":true}' --event '{}'
```

## License

[MIT](LICENSE)
