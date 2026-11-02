package proto

import (
	"os"
	"strings"
	"testing"
)

func TestVersionMatchesClient(t *testing.T) {
	src, err := os.ReadFile("../../web/src/version.ts")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), `VERSION = '`+Version+`'`) {
		t.Fatalf("web/src/version.ts does not say VERSION = '%s'", Version)
	}
}
