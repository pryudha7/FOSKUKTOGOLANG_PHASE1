package cases

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// The normalized parts of a Pefindo report (§9.35-9.39, DEC-139): the
// debtor, facilities, inquiries, inquiry summary and identity history,
// written with pefindo_report (DB-RULE-075, DB-RULE-112). They hold personal
// data and are never logged (DB-RULE-077).

// colKind is how a report value becomes a column value (DB-RULE-113; agreed
// for ticket 35: unreadable or out of range is NULL).
type colKind int

const (
	colText colKind = iota
	colInt
	colBigint
	colAmount
	colDate
	colTimestamp
	colJSON
)

// sqlCastOf is the parameter cast of each kind in an insert.
var sqlCastOf = map[colKind]string{colText: "varchar", colInt: "integer", colBigint: "bigint", colAmount: "numeric",
	colDate: "date", colTimestamp: "timestamp", colJSON: "jsonb"}

// reportColumn is a column of a detail table and the report field it comes
// from: "" when §9 names none yet, stored NULL until a Get Report sample
// shows it (ticket 35 Comment 1, open domain question).
type reportColumn struct {
	name, source string
	kind         colKind
}

// maxAmount bounds NUMERIC(18,2): 16 digits before the point.
var maxAmount = new(big.Rat).SetFloat64(1e16)

// value reads the column from a report element: nil when absent, null,
// unreadable or outside the column.
func (c reportColumn) value(m map[string]any) any {
	if c.source == "" {
		return nil
	}
	v, ok := m[c.source]
	if !ok || v == nil {
		return nil
	}
	if c.kind == colJSON {
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return string(b)
	}
	t, ok := scalarText(v)
	if !ok {
		return nil
	}
	switch c.kind {
	case colText:
		return t
	case colInt:
		return wholeNumber(t, math.MinInt32, math.MaxInt32)
	case colBigint:
		return wholeNumber(t, math.MinInt64, math.MaxInt64)
	case colAmount:
		// The range is checked on the rounded value, which is what is stored.
		r, text, ok := decimal2(t)
		if !ok || new(big.Rat).Abs(r).Cmp(maxAmount) >= 0 {
			return nil
		}
		return text
	case colDate:
		// Exactly yyyy-MM-dd (DB-RULE-113).
		d, err := time.Parse("2006-01-02", t)
		if err != nil {
			return nil
		}
		return d
	case colTimestamp:
		if ts := reportTime(t); ts != nil {
			return *ts
		}
	}
	return nil
}

// scalarText is a string or number as text; ok is false for anything else
// (agreed for ticket 35: VARCHAR columns take numbers as text).
func scalarText(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x), true
	case json.Number:
		return x.String(), true
	}
	return "", false
}

// decimal2 is a decimal rounded to two places, as a number and as text;
// ok is false when s is not a number.
func decimal2(s string) (rounded *big.Rat, text string, ok bool) {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(s))
	if !ok {
		return nil, "", false
	}
	text = r.FloatString(2)
	rounded, _ = new(big.Rat).SetString(text)
	return rounded, text, true
}

// wholeNumber reads an integer within [lo, hi], also written with a zero
// fraction (600.0); nil otherwise.
func wholeNumber(t string, lo, hi int64) any {
	if n, err := strconv.ParseInt(t, 10, 64); err == nil {
		if n < lo || n > hi {
			return nil
		}
		return n
	}
	r, ok := new(big.Rat).SetString(t)
	if !ok || !r.IsInt() || !r.Num().IsInt64() {
		return nil
	}
	if n := r.Num().Int64(); n >= lo && n <= hi {
		return n
	}
	return nil
}

// reportDetail is a detail table and where its rows come from in the report.
type reportDetail struct {
	table, source string
	columns       []reportColumn
	sequenced     bool // one row per array element, numbered from 1
}

var reportDetails = []reportDetail{
	{"pefindo_debtor", "debitur", pefindoDebtorColumns, false},
	{"pefindo_facility", "fasilitas", pefindoFacilityColumns, true},
	{"pefindo_inquiry", "permintaan_data", pefindoInquiryColumns, true},
	{"pefindo_inquiry_summary", "summary_permintaan_data", pefindoInquirySummaryColumns, true},
	{"pefindo_identity_history", "riwayat_identitas_debitur", pefindoIdentityHistoryColumns, true},
}

// deleteReportDetails removes the detail rows of the subject's report in tx,
// before the report itself is replaced (DB-RULE-074, DB-RULE-112).
func deleteReportDetails(ctx context.Context, tx pgx.Tx, caseID, subject string) error {
	for _, d := range reportDetails {
		if _, err := tx.Exec(ctx, `DELETE FROM fos.`+d.table+` WHERE pefindo_report_id IN
			(SELECT pefindo_report_id FROM fos.pefindo_report WHERE case_id = $1 AND subject_type = $2)`, caseID, subject); err != nil {
			return err
		}
	}
	return nil
}

// storeReportDetails writes the report's detail rows in tx. A null or empty
// debitur makes no debtor row; every array element makes a row, an element
// that is not an object one with its sequence number only (ticket 35
// Comment 1).
func storeReportDetails(ctx context.Context, tx pgx.Tx, reportID int64, raw json.RawMessage) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber() // keeps long identifiers exact
	var report map[string]any
	if err := dec.Decode(&report); err != nil || report == nil {
		return nil // not an object: only report_json keeps it
	}
	for _, d := range reportDetails {
		if !d.sequenced {
			if m, ok := report[d.source].(map[string]any); ok && len(m) > 0 {
				if err := insertDetail(ctx, tx, d, reportID, 0, m); err != nil {
					return err
				}
			}
			continue
		}
		items, _ := report[d.source].([]any)
		for i, it := range items {
			m, _ := it.(map[string]any)
			if err := insertDetail(ctx, tx, d, reportID, i+1, m); err != nil {
				return err
			}
		}
	}
	return nil
}

func insertDetail(ctx context.Context, tx pgx.Tx, d reportDetail, reportID int64, seq int, m map[string]any) error {
	names := []string{"pefindo_report_id"}
	params := []string{"$1"}
	args := []any{reportID}
	if d.sequenced {
		names, params, args = append(names, "sequence_number"), append(params, "$2"), append(args, seq)
	}
	for _, c := range d.columns {
		args = append(args, c.value(m))
		names = append(names, c.name)
		params = append(params, fmt.Sprintf("$%d::%s", len(args), sqlCastOf[c.kind]))
	}
	_, err := tx.Exec(ctx, `INSERT INTO fos.`+d.table+` (`+strings.Join(names, ", ")+`) VALUES (`+strings.Join(params, ", ")+`)`, args...)
	return err
}
