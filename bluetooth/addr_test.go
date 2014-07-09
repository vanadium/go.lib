package bluetooth

import "testing"

func TestParseValidAddr(t *testing.T) {
	testdata := []struct{ before, after string }{
		{"-1", "00:00:00:00:00:00-1"},
		{"0A:0B:33:55:C9:9a-8", "0a:0b:33:55:c9:9a-8"},
	}
	for _, d := range testdata {
		addr, err := parseAddress(d.before)
		if err != nil {
			t.Errorf("failed to parse %q: %v", d.before, err)
			continue
		}
		if got, want := addr.String(), d.after; got != want {
			t.Errorf("Got %q, want %q", got, want)
		}
	}
}

func TestParseInvalidAddr(t *testing.T) {
	testdata := []string{
		"00:01:02:03:04:05",      // missing channel
		"00:01:02:03:04:05:06-1", // invalid MAC addr
	}
	for _, d := range testdata {
		addr, err := parseAddress(d)
		if err == nil {
			t.Errorf("Got %q, want error for parseAddress(%q)", addr, d)
		}
	}
}
