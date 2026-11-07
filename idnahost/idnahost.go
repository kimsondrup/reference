// Package idnahost parses references whose registry host is an
// internationalized domain name, such as "bücher.example/app".
//
// Registries, and [reference.ParseNormalizedNamed], only accept the ASCII
// (punycode) form of such a host, "xn--bcher-kva.example". This package
// normalizes the host with IDNA before parsing. It is a separate module, so
// that the reference package itself does not depend on golang.org/x/net.
package idnahost

import (
	"strings"
	"unicode/utf8"

	"github.com/distribution/reference"
	"golang.org/x/net/idna"
)

// profile normalizes a domain with IDNA. All options are set explicitly,
// because [idna.Lookup] "may change over time" and does not check the DNS
// length limits.
var profile = idna.New(
	// Map and lowercase as for a lookup (UTS #46), allow only letters, digits
	// and '-' (StrictDomainName), and validate labels (the parts of the domain
	// between dots) and joiners.
	idna.MapForLookup(),
	// Keep "ß" and other deviation characters instead of mapping them as in
	// IDNA 2003 ("faß" stays distinct from "fass").
	idna.Transitional(false),
	// Reject labels that mix left-to-right and right-to-left text.
	idna.BidiRule(),
	// Labels of at most 63 bytes, and at most 253 bytes in total.
	idna.VerifyDNSLength(true),
	// Allow "--" as the third and fourth characters, as browsers do, because
	// real hosts such as YouTube CDN nodes use it. A '-' at the start or end of
	// a label is checked by validHyphens.
	idna.CheckHyphens(false),
)

// ParseNormalizedNamed is like [reference.ParseNormalizedNamed], but first
// normalizes the registry host with IDNA (the standard for domain names in any
// script). IDNA converts a Unicode domain name to its standard ASCII form,
// called punycode, which starts with "xn--", and also normalizes the rest of
// the host, for example by lowercasing letters:
//
//	"bücher.example/app"       -> registry "xn--bcher-kva.example"
//	"Bücher.Example/app"       -> registry "xn--bcher-kva.example"
//	"Registry.Example.COM/app" -> registry "registry.example.com"
//	"bücher。example:5000/app" -> registry "xn--bcher-kva.example:5000"
//	"[::1]:5000/app"           -> registry "[::1]:5000" (unchanged)
//
// The part before the first '/' is normalized if, after IDNA, it has a '.' or a
// port (':'). IDNA decides which characters are dots: besides '.', it reads
// full stops such as the ideographic "。" as dots. A part without a dot or port,
// such as "Foo" in "Foo/bar" or "bücher" in "bücher/app", is not a host and is
// left as written. IPv6 hosts in brackets and the port are also left as
// written.
//
// The IDNA rules for the host:
//   - after normalizing, the only ASCII characters allowed are letters, digits
//     and '-';
//   - a label (a part of the domain between dots) must not start or end with
//     '-';
//   - a label is at most 63 bytes, and the domain at most 253 bytes;
//   - non-ASCII characters must be allowed by IDNA, and left-to-right and
//     right-to-left letters must not be mixed.
//
// "--" as the third and fourth characters of a label is allowed, as in
// browsers, because real hosts such as YouTube CDN nodes use it.
//
// If IDNA rejects the host, the input is parsed unchanged. This matches how
// [reference.ParseNormalizedNamed] treats an invalid host:
// "foo_bar.example/app" has the domain "" and the path "foo_bar.example/app".
// So an ASCII host that only IDNA rejects, for example one longer than 253
// bytes, is kept as written, and the error for a rejected Unicode host is the
// one [reference.ParseNormalizedNamed] returns for it.
//
// The returned reference's String() parses back to the same reference with
// [reference.ParseNormalizedNamed].
func ParseNormalizedNamed(s string) (reference.Named, error) {
	if first, rest, ok := strings.Cut(s, "/"); ok {
		if host, ok := normalizeHost(first); ok {
			return reference.ParseNormalizedNamed(host + "/" + rest)
		}
	}
	return reference.ParseNormalizedNamed(s)
}

// normalizeHost normalizes first, the part before the first '/', with IDNA.
// It returns false if first is an IPv6 host, is not valid UTF-8 (which cannot
// be part of a valid host), is rejected by IDNA, or has no '.' and no port
// after IDNA, so it is not a host.
func normalizeHost(first string) (string, bool) {
	if strings.HasPrefix(first, "[") || !utf8.ValidString(first) {
		return "", false
	}
	domain, port := first, ""
	if i := strings.IndexByte(first, ':'); i >= 0 {
		domain, port = first[:i], first[i:]
	}
	ascii, err := profile.ToASCII(domain)
	if err != nil || !validHyphens(ascii) || !(strings.Contains(ascii, ".") || port != "") {
		return "", false
	}
	return ascii + port, true
}

// validHyphens checks that no label of the Unicode form of ascii starts or
// ends with '-'.
func validHyphens(ascii string) bool {
	unicode, err := profile.ToUnicode(ascii)
	if err != nil {
		return false
	}
	for _, label := range strings.Split(unicode, ".") {
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
	}
	return true
}
