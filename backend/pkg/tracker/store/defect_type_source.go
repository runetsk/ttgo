package store

import (
	"fmt"
	"time"
	"ttgo/pkg/tracker/models"

	"gorm.io/gorm"
)

// This file is the ONLY place allowed to spell a run_results.defect_type write (see
// TestEveryDefectTypeWriterGoesThroughTheHelper): the helper every non-auto writer uses, and
// auto-apply itself.

// autoLabelChunk bounds how many result ids one auto-apply statement binds.
const autoLabelChunk = 500

// DefectTypePatch is the JSON-safe part of a non-auto defect_type write — the value and its
// source ("human" for a person's explicit choice, "" for the automatic default and non-failure
// clears) — for live result patches.
func DefectTypePatch(value string, explicit bool) map[string]interface{} {
	source := ""
	if explicit {
		source = models.DefectTypeSourceHuman
	}
	return map[string]interface{}{"defect_type": value, "defect_type_source": source}
}

// confirmsAILabel is R10's provenance: true when the label this write replaces was set by
// auto-apply. Every right-hand side of an UPDATE's SET sees the row as it was before the update,
// so it reads the old defect_type_source even though the same statement rewrites it.
func confirmsAILabel() interface{} {
	return gorm.Expr("defect_type_source = ?", models.DefectTypeSourceAI)
}

// HumanDefectTypeFields is the one way to write run_results.defect_type outside auto-apply
// (spec R2, header amendment 1): the value, its source (DefectTypePatch) and
// suggested_auto_applied (R10) in the same update, so a label set by auto-apply never survives a
// person's decision, the status-change default or the non-failure clear, and the accuracy gate can
// tell a Confirm from an independent decision. Callers merge it into their update map.
func HumanDefectTypeFields(value string, explicit bool) map[string]interface{} {
	f := DefectTypePatch(value, explicit)
	f["suggested_auto_applied"] = confirmsAILabel()
	return f
}

// withDefectTypeSource is the backstop behind HumanDefectTypeFields for the generic run-result
// writers: an update map that writes defect_type without saying where it came from is treated as
// a non-explicit write. Anything else passes through unchanged.
func withDefectTypeSource(updates interface{}) interface{} {
	m, ok := updates.(map[string]interface{})
	if !ok {
		return updates
	}
	if _, writes := m["defect_type"]; !writes {
		return updates
	}
	if _, has := m["defect_type_source"]; has {
		return updates
	}
	out := make(map[string]interface{}, len(m)+2)
	for k, v := range m {
		out[k] = v
	}
	out["defect_type_source"] = ""
	out["suggested_auto_applied"] = confirmsAILabel()
	return out
}

// conclusiveDefectTypes are the only labels auto-apply writes — never the untriaged default.
var conclusiveDefectTypes = map[string]bool{"product_bug": true, "automation_bug": true, "system_issue": true}

// autoLabelChunks splits ids into statement-sized chunks.
func autoLabelChunks(ids []string) [][]string {
	var out [][]string
	for len(ids) > 0 {
		n := autoLabelChunk
		if len(ids) < n {
			n = len(ids)
		}
		out = append(out, ids[:n])
		ids = ids[n:]
	}
	return out
}

// ApplyAutoDefectType labels failing results with a qualifying TypeSafe decision's suggestion and
// marks the label as set by AI (spec §3.3, R3, header amendment 1). It writes only where no person
// has decided (decided_at IS NULL) and the current label is either auto-apply's own (a re-analysis
// follows the new decision) or the untriaged default nobody chose (” source with ” or
// 'to_investigate'). A person's label — any 'human' source, including an explicit
// "to investigate" — is never overwritten, and a person's write in between wins because it sets
// the 'human' source. It never writes decided_at or the snapshot columns. Returns how many rows it
// labelled.
func (s *Store) ApplyAutoDefectType(resultIDs []string, value string) (int64, error) {
	if !conclusiveDefectTypes[value] {
		return 0, fmt.Errorf("auto-apply writes only a conclusive defect type, not %q", value)
	}
	now := time.Now()
	var total int64
	for _, ids := range autoLabelChunks(resultIDs) {
		res := s.db.Model(&models.RunResult{}).
			Where("id IN ? AND status IN ? AND decided_at IS NULL AND (defect_type_source = ? OR (defect_type_source = '' AND defect_type IN ?))",
				ids, models.FailureStatuses, models.DefectTypeSourceAI, []string{"", "to_investigate"}).
			Updates(map[string]interface{}{
				"defect_type":        value,
				"defect_type_source": models.DefectTypeSourceAI,
				"updated_at":         now,
			})
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
	}
	return total, nil
}

// ResetAutoDefectType puts AI-set labels back to the untriaged default when a newer analysis no
// longer qualifies (spec §3.5, R3). Human labels are never touched: the WHERE matches only rows
// whose label auto-apply wrote and no person has decided since. Returns how many rows it reset.
func (s *Store) ResetAutoDefectType(resultIDs []string) (int64, error) {
	now := time.Now()
	var total int64
	for _, ids := range autoLabelChunks(resultIDs) {
		res := s.db.Model(&models.RunResult{}).
			Where("id IN ? AND defect_type_source = ? AND decided_at IS NULL", ids, models.DefectTypeSourceAI).
			Updates(map[string]interface{}{
				"defect_type":        "to_investigate",
				"defect_type_source": "",
				"updated_at":         now,
			})
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
	}
	return total, nil
}
