package store

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// defectTypeWriterAllowlist: the only file that may spell a defect_type write — the R2 helper
// (HumanDefectTypeFields) and auto-apply itself (ApplyAutoDefectType, ResetAutoDefectType).
var defectTypeWriterAllowlist = map[string]bool{"pkg/tracker/store/defect_type_source.go": true}

// defectTypeWriterDirs are the packages that write run_results. internal/cli is left out (it
// builds HTTP request bodies, not updates), as is pkg/tracker/failureanalysis (its
// "defect_type" is a TypeSafe question id, and it never writes the database).
var defectTypeWriterDirs = []string{"internal/api", "pkg/tracker/store", "cmd"}

var (
	defectTypeWriteShapes = []*regexp.Regexp{
		regexp.MustCompile(`\[\s*"defect_type"\s*\]\s*=[^=]`),      // m["defect_type"] = v
		regexp.MustCompile(`"defect_type"\s*:`),                    // map literal key
		regexp.MustCompile(`\.Update\(\s*"defect_type"`),           // single-column Update
		regexp.MustCompile(`Updates\(\s*&?models\.RunResult\s*\{`), // struct update of a result
	}
	updateRunResultsSQL = regexp.MustCompile(`(?i)update\s+run_results\s+set\s`)
	setsDefectTypeSQL   = regexp.MustCompile(`(?i)\bdefect_type\s*=`)
	whereSQL            = regexp.MustCompile(`(?i)\bwhere\b`)
)

func stripLineComments(src string) string {
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		if j := strings.Index(l, "//"); j >= 0 {
			lines[i] = l[:j]
		}
	}
	return strings.Join(lines, "\n")
}

// writesDefectType reports the first defect_type write shape found in Go source, or "".
func writesDefectType(src string) string {
	src = stripLineComments(src)
	for _, re := range defectTypeWriteShapes {
		if re.MatchString(src) {
			return re.String()
		}
	}
	for _, loc := range updateRunResultsSQL.FindAllStringIndex(src, -1) {
		stmt := src[loc[1]:]
		if i := strings.IndexByte(stmt, '`'); i >= 0 {
			stmt = stmt[:i]
		}
		if w := whereSQL.FindStringIndex(stmt); w != nil {
			stmt = stmt[:w[0]]
		}
		if setsDefectTypeSQL.MatchString(stmt) {
			return "UPDATE run_results SET … defect_type ="
		}
	}
	return ""
}

func TestDefectTypeWriteShapesAreRecognised(t *testing.T) {
	for _, bad := range []string{
		`m["defect_type"] = "x"`,
		`u := map[string]interface{}{"defect_type": v}`,
		`db.Model(&models.RunResult{}).Update("defect_type", v)`,
		`db.Updates(&models.RunResult{DefectType: v})`,
		"db.Exec(`UPDATE run_results SET defect_type = 'x' WHERE id = ?`)",
	} {
		require.NotEmpty(t, writesDefectType(bad), bad)
	}
	for _, ok := range []string{
		`v := m["defect_type"].(string)`,
		`if m["defect_type"] == "x" {}`,
		"DefectType string `json:\"defect_type\" gorm:\"default:''\"`",
		"db.Exec(`UPDATE run_results SET suggested_engine = 'g' WHERE defect_type = 'x'`)",
		`// m["defect_type"] = v in a comment`,
		`m["suggested_defect_type"] = ""`,
	} {
		require.Empty(t, writesDefectType(ok), ok)
	}
}

// TestEveryDefectTypeWriterGoesThroughTheHelper is the R2 grep guard: a new writer of
// run_results.defect_type must use store.HumanDefectTypeFields (or the runs handlers'
// setDefectType, which wraps it), so defect_type_source is cleared in the same update and an
// AI-applied label never survives a person's or a default write.
func TestEveryDefectTypeWriterGoesThroughTheHelper(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	var offenders []string
	for _, dir := range defectTypeWriterDirs {
		base := filepath.Join(root, dir)
		if _, err := os.Stat(base); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if defectTypeWriterAllowlist[rel] {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if shape := writesDefectType(string(raw)); shape != "" {
				offenders = append(offenders, rel+" ("+shape+")")
			}
			return nil
		})
		require.NoError(t, err)
	}
	require.Empty(t, offenders, "write run_results.defect_type only through store.HumanDefectTypeFields "+
		"(runs: setDefectType) so defect_type_source is cleared in the same update (spec R2)")
}
