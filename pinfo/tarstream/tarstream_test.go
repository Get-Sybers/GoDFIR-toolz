package tarstream

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
)

func mkTar(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	mod := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for name, body := range map[string]string{
		"var/log/wtmp":   "one",
		"var/log/wtmp.1": "two",
	} {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), ModTime: mod, Typeflag: tar.TypeReg})
		tw.Write([]byte(body))
	}
	tw.WriteHeader(&tar.Header{Name: "var/log", Typeflag: tar.TypeDir, Mode: 0o755, ModTime: mod})
	tw.Close()
	return &buf
}

func TestEach(t *testing.T) {
	got := map[string]string{}
	err := Each(mkTar(t), func(e Entry) error {
		b, err := io.ReadAll(e.R)
		if err != nil {
			return err
		}
		got[e.Name] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["var/log/wtmp"] != "one" || got["var/log/wtmp.1"] != "two" {
		t.Fatalf("entries: %v", got)
	}
}

func TestEachCallbackError(t *testing.T) {
	boom := errors.New("boom")
	err := Each(mkTar(t), func(e Entry) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}
