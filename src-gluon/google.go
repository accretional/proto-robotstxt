package robotsgluon

// google.go — runs the C++ event-dump binary (tools/robots-dump, built to
// gen/bin/robots_dump) and parses its output, so tests and the CLI can
// compare google's deserialization of a robots.txt byte-for-byte against
// Events(). The dump format is one base64-armored TSV record per handler
// callback: KIND \t line \t base64(key) \t base64(value).

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// GoogleEvents runs dumpBin (the robots_dump binary) on robotsPath and
// returns google's parse-event stream.
func GoogleEvents(dumpBin string, robotsPath string) ([]Event, error) {
	out, err := exec.Command(dumpBin, robotsPath).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s %s: %w: %s", dumpBin, robotsPath, err, ee.Stderr)
		}
		return nil, fmt.Errorf("%s %s: %w", dumpBin, robotsPath, err)
	}
	return parseDump(out)
}

func parseDump(out []byte) ([]Event, error) {
	var events []Event
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || line == "START" || line == "END" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 4 {
			return nil, fmt.Errorf("robots_dump: bad record %q", line)
		}
		num, err := strconv.ParseInt(f[1], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("robots_dump: bad line number in %q: %w", line, err)
		}
		key, err := base64.StdEncoding.DecodeString(f[2])
		if err != nil {
			return nil, fmt.Errorf("robots_dump: bad key in %q: %w", line, err)
		}
		value, err := base64.StdEncoding.DecodeString(f[3])
		if err != nil {
			return nil, fmt.Errorf("robots_dump: bad value in %q: %w", line, err)
		}
		events = append(events, Event{
			Line:  int32(num),
			Kind:  EventKind(f[0]),
			Key:   string(key),
			Value: string(value),
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

// DiffEvents compares two event streams and returns a human-readable list of
// differences (empty = equal). Unknown-key comparison included; for the
// typed kinds the key column is ignored (google's handlers don't receive it).
func DiffEvents(gluon, google []Event) []string {
	var diffs []string
	n := max(len(gluon), len(google))
	for i := 0; i < n; i++ {
		switch {
		case i >= len(gluon):
			diffs = append(diffs, fmt.Sprintf("event %d: gluon <none> | google %s", i, google[i]))
		case i >= len(google):
			diffs = append(diffs, fmt.Sprintf("event %d: gluon %s | google <none>", i, gluon[i]))
		default:
			a, b := gluon[i], google[i]
			if a.Kind != b.Kind || a.Line != b.Line || a.Value != b.Value ||
				(a.Kind == Unknown && a.Key != b.Key) {
				diffs = append(diffs, fmt.Sprintf("event %d: gluon %s | google %s", i, a, b))
			}
		}
	}
	return diffs
}
