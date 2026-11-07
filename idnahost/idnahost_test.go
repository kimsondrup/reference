package idnahost

import (
	"strings"
	"testing"

	"github.com/distribution/reference"
)

// checkNormalized checks a successful parse, and that the result parses back to
// the same reference with reference.ParseNormalizedNamed.
func checkNormalized(t *testing.T, input, domain, path string) {
	t.Helper()
	named, err := ParseNormalizedNamed(input)
	if err != nil {
		t.Fatalf("ParseNormalizedNamed(%q): %v", input, err)
	}
	if got := reference.Domain(named); got != domain {
		t.Errorf("ParseNormalizedNamed(%q): domain %q, expected %q", input, got, domain)
	}
	if got := reference.Path(named); got != path {
		t.Errorf("ParseNormalizedNamed(%q): path %q, expected %q", input, got, path)
	}
	reparsed, err := reference.ParseNormalizedNamed(named.String())
	if err != nil || reparsed.String() != named.String() {
		t.Errorf("ParseNormalizedNamed(%q) gives %q, but reference.ParseNormalizedNamed parses that as %v, %v", input, named.String(), reparsed, err)
	}
}

// checkSameError checks that input is rejected with the same error as
// reference.ParseNormalizedNamed gives.
func checkSameError(t *testing.T, input string) {
	t.Helper()
	_, err := ParseNormalizedNamed(input)
	_, want := reference.ParseNormalizedNamed(input)
	if err == nil || want == nil || err.Error() != want.Error() {
		t.Errorf("ParseNormalizedNamed(%q): got error %v, expected %v", input, err, want)
	}
}

// ASCII hosts are normalized by IDNA too: letters are lowercased. Parts that
// are not a host, IPv6 hosts and hosts that IDNA rejects are kept as written,
// so the result is the same as from reference.ParseNormalizedNamed.
func TestASCII(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ input, domain, path string }{
		{"Registry.Example.COM/app", "registry.example.com", "app"},
		{"Registry.Example.COM:5000/app", "registry.example.com:5000", "app"},
		{"127.1/app", "127.1", "app"},
		// "--" as the third and fourth characters is allowed.
		{"ab--c.example/app", "ab--c.example", "app"},
		{"ls---x.example/app", "ls---x.example", "app"},
		{"[2001:DB8:0:0::1]:5000/app", "[2001:DB8:0:0::1]:5000", "app"},
		// IDNA rejects '_', so this is not a host, as in
		// reference.ParseNormalizedNamed.
		{"foo_bar.example/app", "", "foo_bar.example/app"},
		// No '.' and no port, so not normalized: "Foo" stays a host, as in
		// reference.ParseNormalizedNamed. Lowercased, it would become the Docker
		// Hub repository "foo/bar".
		{"Foo/bar", "Foo", "bar"},
		{"busybox", "docker.io", "library/busybox"},
		{"foo/bar", "docker.io", "foo/bar"},
		{"localhost/app", "localhost", "app"},
		{"index.docker.io/app", "docker.io", "library/app"},
	} {
		checkNormalized(t, tc.input, tc.domain, tc.path)
	}
}

func TestUnicodeHost(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ input, domain, path string }{
		{"bücher.example/app", "xn--bcher-kva.example", "app"},
		// ASCII labels are lowercased as well.
		{"Bücher.Example:5000/app", "xn--bcher-kva.example:5000", "app"},
		{"MyReg.bücher.example/app", "myreg.xn--bcher-kva.example", "app"},
		{"ls---x.bücher.example/app", "ls---x.xn--bcher-kva.example", "app"},
		// "--" as the third and fourth characters is allowed, as in browsers.
		{"bü--c.example/app", "xn--b--c-0ra.example", "app"},
		{"bücher。example/app", "xn--bcher-kva.example", "app"},
		{"bücher．example/app", "xn--bcher-kva.example", "app"},
		{"bücher:5000/app", "xn--bcher-kva:5000", "app"},
		// The Unicode forms of the punycode domains in reference_test.go.
		{"☃.com/myimage", "xn--n3h.com", "myimage"},
		{"🐳.com/myimage", "xn--7o8h.com", "myimage"},
		// Normalization done by IDNA:
		// "ß" is kept ("faß"); older IDNA (2003) turned it into "ss". Uppercase
		// Greek is lowercased ("σασ").
		{"faß.de/app", "xn--fa-hia.de", "app"},
		{"ΣΑΣ.gr/app", "xn--mxa9ab.gr", "app"},
		// Decomposed Unicode (NFD): "u" followed by a combining diaeresis gives
		// the same result as "ü".
		{"bücher.example/app", "xn--bcher-kva.example", "app"},
		// Fullwidth letters are turned into ASCII.
		{"ｅｘａｍｐｌｅ.com/app", "example.com", "app"},
		// A right-to-left (Hebrew) label.
		{"שלום.example/app", "xn--9dbne9b.example", "app"},
		// A soft hyphen is invisible and removed by IDNA.
		{"exam­ple.cöm/app", "example.xn--cm-fka", "app"},
		// A look-alike letter (Cyrillic "а") gives a different host than
		// "apple.com".
		{"аpple.com/app", "xn--pple-43d.com", "app"},
		{"ü--c.example/app", "xn----c-goa.example", "app"},
		// A valid punycode label is kept.
		{"xn--bcher-kva.bücher.example/app", "xn--bcher-kva.xn--bcher-kva.example", "app"},
		// A tag is kept.
		{"Bücher.example:5000/app:v1", "xn--bcher-kva.example:5000", "app"},
	} {
		checkNormalized(t, tc.input, tc.domain, tc.path)
	}
}

// When IDNA rejects the host, the input is parsed unchanged, so the error is
// the same as from reference.ParseNormalizedNamed.
func TestInvalidUnicodeHost(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		// A label must not start or end with '-', and ASCII characters other
		// than letters, digits and '-' are not allowed.
		"-bücher.example/app",
		"bücher-.example/app",
		"bü_cher.example/app",
		"bad hüst.example/app",
		// Zero width joiner and non-joiner.
		"‍.com/app",
		"exa‌mple.cöm/app",
		// Mixing left-to-right and right-to-left letters in one label (Bidi
		// rule).
		"aשלום.example/app",
		// Not a valid host or port.
		"bücher.example./app",
		"bücher..example/app",
		" bücher.example/app",
		"bücher.example\n/app",
		"bü\tcher.example/app",
		"user@bücher.example/app",
		"bücher.example：5000/app",
		"bücher.example:５０００/app",
		"bücher.example:port/app",
		"[bü::1]:5000/app",
		// Invalid UTF-8 is not passed to IDNA.
		"bücher.example\xff/app",
		"\xc3.bücher.example/app",
		// Percent-encoding is not decoded.
		"b%C3%BCcher.example/app",
		// IDNA maps U+2100 (℀) to "a/c" and U+FE55 (small colon) to ':'. Only
		// letters, digits and '-' are allowed, so these are rejected and can't
		// change where the host ends.
		"bücher.ex℀ample/app",
		"bücher.example﹕5000/app",
		// A label longer than 63 bytes (for the whole domain, see
		// TestDomainLength).
		strings.Repeat("ü", 70) + ".example/app",
		// "xn--" followed by non-ASCII characters, and invalid punycode next to
		// a Unicode label.
		"xn--kitty-點看pd34d.com/app",
		"xn--not-valid.bücher.example/app",
		// Only the host may contain Unicode. The repository and tag must be ASCII.
		"bücher.example/bücher",
		"bücher.example/app:tagé",
		// Without a '.' or ':' after IDNA, the first component is not a host, so
		// it is left as written and the non-ASCII path is rejected.
		"bücher/app",
		// The repository is checked as usual.
		"bücher.example/App",
		"-bücher.example/App",
	} {
		checkSameError(t, input)
	}
}

// A host that is only dots fails the same way with '.' and "。", and with the
// same error as reference.ParseNormalizedNamed gives for '.'.
func TestOnlyDots(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ ascii, unicode string }{
		{".", "。"},
		{"./app", "。/app"},
		{".:5000/app", "。:5000/app"},
		{"../app", "。。/app"},
	} {
		_, want := reference.ParseNormalizedNamed(tc.ascii)
		if want == nil {
			t.Fatalf("reference.ParseNormalizedNamed(%q) should fail", tc.ascii)
		}
		for _, input := range []string{tc.ascii, tc.unicode} {
			if _, err := ParseNormalizedNamed(input); err == nil || err.Error() != want.Error() {
				t.Errorf("ParseNormalizedNamed(%q): got error %v, expected %v", input, err, want)
			}
		}
	}
}

// A domain is at most 253 bytes. An ASCII host that is longer is not
// normalized, but kept as written, as reference.ParseNormalizedNamed accepts
// it.
func TestDomainLength(t *testing.T) {
	t.Parallel()
	labels := strings.Repeat(strings.Repeat("a", 63)+".", 3)
	ascii := func(n int) string { return "xn--bcher-kva." + labels + strings.Repeat("a", n) }
	unicode := func(n int) string { return "bücher." + labels + strings.Repeat("a", n) + "/app" }
	if len(ascii(47)) != 253 {
		t.Fatalf("the test domain should be 253 bytes, got %d", len(ascii(47)))
	}
	checkNormalized(t, unicode(47), ascii(47), "app")
	checkSameError(t, unicode(48))
	long := "Upper-case-abc." + labels + strings.Repeat("a", 47)
	if len(long) != 254 {
		t.Fatalf("the long domain should be 254 bytes, got %d", len(long))
	}
	checkNormalized(t, long+"/app", long, "app")
}

// Every combination of these labels is either rejected or gives a reference
// that parses back to the same reference with reference.ParseNormalizedNamed.
func TestAcceptedReferencesParseBack(t *testing.T) {
	t.Parallel()
	labels := []string{
		"bücher", "Bücher", "faß", "ΣΑΣ", "ü--c", "bü--c", "ls---x", "xn--bcher-kva",
		"xn--not-valid", "☃", "שלום", "aשלום", "ｅｘａｍｐｌｅ", "exam\u00ADple", "\u200D",
		"a", "localhost", "",
	}
	accepted := 0
	for _, a := range labels {
		for _, b := range labels {
			for _, dot := range []string{".", "\u3002"} {
				for _, rest := range []string{"/app", ":5000/app", "/App", "/app:v1"} {
					input := a + dot + b + rest
					named, err := ParseNormalizedNamed(input)
					if err != nil {
						continue
					}
					accepted++
					reparsed, err := reference.ParseNormalizedNamed(named.String())
					if err != nil || reparsed.String() != named.String() {
						t.Errorf("ParseNormalizedNamed(%q) gives %q, but reference.ParseNormalizedNamed parses that as %v, %v", input, named.String(), reparsed, err)
					}
				}
			}
		}
	}
	if accepted < 100 {
		t.Errorf("only %d inputs were accepted", accepted)
	}
}
