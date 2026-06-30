package internal

import "strings"

// identClass is the single source of truth for the character class that makes
// up a source-code identifier across the whole toolkit.
//
// It is Unicode-aware: `\p{L}` (any Unicode letter) plus `\p{Nd}` (decimal
// digit) plus underscore. This is the Unicode equivalent of the ASCII `\w`
// (`[0-9A-Za-z_]`) — for ASCII identifiers the two match exactly, so enabling
// it is a pure superset with no behavior change on ASCII code; for the many
// languages that permit non-ASCII identifiers (Go, Python 3, Rust 1.53+, JS,
// C#, Java, Swift, Kotlin, Ruby, Scala, D, …) it additionally detects names
// like `func Привет()` or `type Café`.
//
// Language patterns in languages.json reference this via the `{IDENT}`
// placeholder, expanded at config-load time (see expandIdentPlaceholder).
// callgraph.go builds its call-site regex from it directly. Keeping one
// definition means the way an identifier is recognised can never drift between
// "where is X defined" and "who calls X".
const identClass = `[\p{L}\p{Nd}_]`

// identStart is the class of characters that may begin an identifier: a Unicode
// letter or underscore, but not a digit. Use identStart followed by identClass*
// when you need to reject numeric literals (e.g. distinguishing a function call
// `foo(` from `123(`); the language patterns themselves stay lenient and use
// `{IDENT}+` directly, matching the old `\w+` behaviour.
const identStart = `[\p{L}_]`

// expandIdentPlaceholder substitutes the `{IDENT}` token in a raw pattern with
// identClass. It is a no-op for patterns that don't use the token, so it is
// safe to apply to every pattern before compilation.
func expandIdentPlaceholder(pattern string) string {
	return strings.ReplaceAll(pattern, "{IDENT}", identClass)
}
