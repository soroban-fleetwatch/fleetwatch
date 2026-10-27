package follow

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func gzipBytes(t testing.TB, raw []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	w := gzip.NewWriter(&out)
	if _, err := w.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func archiveFixture(t testing.TB) (*Store, string, []byte) {
	t.Helper()
	b, all := chain(t)
	path := filepath.Join(t.TempDir(), "follow.db")
	s := openState(t, path, b)
	t.Cleanup(func() { s.Close() })
	for i := 0; i < len(all); i += 8 {
		if _, err := s.Commit(context.Background(), batch(b.Snapshot.Network, all[i:i+8])); err != nil {
			t.Fatal(err)
		}
	}
	destination := filepath.Join(t.TempDir(), "evidence.ndjson.gz")
	if _, err := s.ExportArchive(context.Background(), destination); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	return s, path, raw
}

func TestFollowArchiveRoundTripDeterminismAndNoStateChanges(t *testing.T) {
	s, path, raw := archiveFixture(t)
	before, _ := os.ReadFile(path)
	receipt, err := VerifyArchive(context.Background(), bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if receipt.ArchiveSHA256 != hex.EncodeToString(digest[:]) || receipt.RetainedLedgers != 40 || receipt.ReconstructedFindings != 2 || receipt.BaselineLedger != 99 || receipt.ProcessedThrough != 139 || receipt.BaselineDigest != s.Head().Projection.OriginDigest || receipt.BaselineHeaderHash != s.Head().Projection.OriginHeaderHash || !receipt.Offline || receipt.FinalVerdict != "UNDETERMINED" || receipt.SourceAuthentication != "UNVERIFIED" || receipt.SemanticValidation != "REPLAYED_FROM_ORIGINAL_ASSERTION" {
		t.Fatal(receipt)
	}
	destination := filepath.Join(t.TempDir(), "second.gz")
	repeated, err := s.ExportArchive(context.Background(), destination)
	if err != nil || !reflect.DeepEqual(repeated, receipt) {
		t.Fatal(repeated, err)
	}
	second, _ := os.ReadFile(destination)
	after, _ := os.ReadFile(path)
	if !bytes.Equal(raw, second) || !bytes.Equal(before, after) || len(eventBytes(t, s)) == 0 {
		t.Fatal("archive changed source or is not deterministic")
	}
	info, _ := os.Stat(destination)
	if info.Mode().Perm() != 0600 {
		t.Fatal("archive is not private")
	}
}

func TestFollowArchiveRejectsTamperingEvenWithRecomputedDigest(t *testing.T) {
	_, _, raw := archiveFixture(t)
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	gz.Close()
	for _, mode := range []string{"remove", "duplicate", "reorder", "metadata", "applied", "head", "origin", "digest", "extra", "null", "unknown", "duplicate-key", "terminator", "missing-trailer"} {
		t.Run(mode, func(t *testing.T) {
			lines := bytes.Split(bytes.TrimSuffix(plain, []byte{'\n'}), []byte{'\n'})
			rows := make([]archiveLine, len(lines))
			for i, line := range lines {
				if json.Unmarshal(line, &rows[i]) != nil {
					t.Fatal("bad fixture")
				}
			}
			switch mode {
			case "remove":
				rows = append(rows[:8], rows[9:]...)
			case "duplicate":
				rows = append(rows[:8], append([]archiveLine{rows[7]}, rows[8:]...)...)
			case "reorder":
				rows[8], rows[9] = rows[9], rows[8]
			case "metadata":
				rows[1].Record.Evidence.MetadataXDR = "broken"
			case "applied":
				rows[1].Record.Applied.Events[0].ID = "fabricated"
			case "head":
				rows[len(rows)-1].Trailer.Head.PendingFindings = 0
			case "origin":
				rows[0].Header.Baseline.HeaderHash = rows[1].Record.Applied.HeaderHash
			}
			var modified bytes.Buffer
			hash := sha256.New()
			for _, row := range rows[:len(rows)-1] {
				line := append(encode(row), '\n')
				modified.Write(line)
				hash.Write(line)
			}
			rows[len(rows)-1].Trailer.ContentSHA256 = hex.EncodeToString(hash.Sum(nil))
			if mode == "digest" {
				rows[len(rows)-1].Trailer.ContentSHA256 = "wrong"
			}
			if mode != "missing-trailer" {
				modified.Write(append(encode(rows[len(rows)-1]), '\n'))
			}
			content := modified.Bytes()
			switch mode {
			case "extra":
				content = append(content, []byte("{}\n")...)
			case "null":
				content = []byte("null\n")
			case "unknown":
				content = bytes.Replace(content, []byte(`"schema_version":1`), []byte(`"unrecognized":1,"schema_version":1`), 1)
			case "duplicate-key":
				content = bytes.Replace(content, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_version":1`), 1)
			case "terminator":
				content = bytes.TrimSuffix(content, []byte{'\n'})
			}
			if _, err := VerifyArchive(context.Background(), bytes.NewReader(gzipBytes(t, content))); err == nil {
				t.Fatal("invalid archive accepted")
			}
		})
	}
	for _, bad := range [][]byte{raw[:len(raw)-1], append(append([]byte{}, raw...), []byte("garbage")...)} {
		if _, err := VerifyArchive(context.Background(), bytes.NewReader(bad)); err == nil {
			t.Fatal("truncated/trailing gzip data accepted")
		}
	}
}

func TestFollowArchiveFailurePublicationAndBounds(t *testing.T) {
	s, _, raw := archiveFixture(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "evidence.gz")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ExportArchive(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := VerifyArchive(ctx, bytes.NewReader(raw)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := s.exportArchive(context.Background(), path, func() error { return errors.New("publication interrupted") }); err == nil {
		t.Fatal("fault injection ignored")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("failed export published destination")
	}
	if _, err := s.exportArchive(context.Background(), path, func() error { return os.WriteFile(path, []byte("preserve"), 0600) }); err == nil {
		t.Fatal("racing destination overwritten")
	}
	prior, _ := os.ReadFile(path)
	if string(prior) != "preserve" {
		t.Fatal("existing output lost")
	}
	if _, err := s.ExportArchive(context.Background(), path); err == nil {
		t.Fatal("existing output accepted")
	}
	if err := os.Symlink(path, filepath.Join(dir, "link.gz")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExportArchive(context.Background(), filepath.Join(dir, "link.gz")); err == nil {
		t.Fatal("symlink accepted")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatal("temporary export leaked", entries)
	}
	for _, limits := range [][2]int64{{16, MaxArchiveBytes}, {MaxArchiveBytes, 1024}} {
		if _, err := verifyArchive(context.Background(), bytes.NewReader(raw), limits[0], limits[1]); err == nil {
			t.Fatal("compressed/expanded archive bound ignored")
		}
	}
	w := archiveWriter{ctx: context.Background(), w: io.Discard, n: MaxArchiveBytes - 1}
	if _, err := w.Write([]byte("xx")); !errors.Is(err, ErrCapacity) {
		t.Fatal("export byte bound ignored", err)
	}
}

func FuzzVerifyFollowArchive(f *testing.F) {
	_, _, valid := archiveFixture(f)
	f.Add(valid)
	f.Add(gzipBytes(f, []byte("null\n")))
	f.Add([]byte("not gzip"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<20 {
			t.Skip()
		}
		_, _ = verifyArchive(context.Background(), bytes.NewReader(raw), 1<<20, 2<<20)
	})
}

func TestFollowArchiveReadOnlyExistingStore(t *testing.T) {
	b, all := chain(t)
	path := filepath.Join(t.TempDir(), "follow.db")
	if s, err := OpenExisting(context.Background(), path, b, config().Limits()); err == nil {
		s.Close()
		t.Fatal("missing read-only state created")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("read-only open created file")
	}
	s := openState(t, path, b)
	if _, err := s.Commit(context.Background(), batch(b.Snapshot.Network, all[:8])); err != nil {
		t.Fatal(err)
	}
	s.Close()
	before, _ := os.ReadFile(path)
	s, err := OpenExisting(context.Background(), path, b, config().Limits())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Commit(context.Background(), batch(b.Snapshot.Network, all[8:9])); err == nil {
		t.Fatal("read-only store accepted write")
	}
	if _, err := s.ExportArchive(context.Background(), filepath.Join(t.TempDir(), "archive.gz")); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("read-only export changed database")
	}
}
