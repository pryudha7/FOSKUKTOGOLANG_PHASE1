package cases

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"fos/internal/auth"
)

// QCA Data, the fourth Hot Prospect step (FR-APP-020, UI-033): ten fixed
// questions, question n answered from BMV QCA<n> (DEC-150).
const qcaQuestions = 10

// QCAPage is UI-033: the saved answer code of each question ("" when not
// answered) and its options. A saved answer whose code is now Inactive is
// added to its question's options so its description still shows
// (BR-UI-012, agreed comment 4).
type QCAPage struct {
	Answers []string
	PageOptions
	FacilityRequested, Editable bool
}

func qcaType(n int) string { return "QCA" + strconv.Itoa(n) }

// qcaKey is the field key of question n, also its options' key.
func qcaKey(n int) string { return "qca" + strconv.Itoa(n) }

// savedQCAAnswers are the case's answer codes by question, "" when not
// answered.
func savedQCAAnswers(ctx context.Context, q querier, caseID string) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT question_number, answer_code FROM fos.case_qca_answer WHERE case_id = $1`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	answers := make([]string, qcaQuestions)
	for rows.Next() {
		var n int16
		var code string
		if err := rows.Scan(&n, &code); err != nil {
			return nil, err
		}
		if n >= 1 && int(n) <= qcaQuestions {
			answers[n-1] = code
		}
	}
	return answers, rows.Err()
}

func (s *Service) qcaPage(ctx context.Context, caseID string, editable bool) (QCAPage, error) {
	answers, err := savedQCAAnswers(ctx, s.db, caseID)
	if err != nil {
		return QCAPage{}, err
	}
	types := map[string]string{}
	for n := 1; n <= qcaQuestions; n++ {
		types[qcaKey(n)] = qcaType(n)
	}
	page := QCAPage{Answers: answers, PageOptions: s.pageOptions(ctx, types), Editable: editable}
	for i, code := range answers {
		key := qcaKey(i + 1)
		if code == "" || slices.ContainsFunc(page.Options[key], func(o Option) bool { return o.Code == code }) {
			continue
		}
		saved, err := s.bmvDescription(ctx, qcaType(i+1), code)
		if err != nil {
			return QCAPage{}, err
		}
		page.Options[key] = append(page.Options[key], *saved)
	}
	page.FacilityRequested, err = facilityRequested(ctx, s.db, caseID)
	return page, err
}

// SubmitQCA validates and stores the ten QCA answers (FR-APP-020 Process 2,
// Validation, DEC-151): unanswered is ERR-010 "Pertanyaan <n> wajib diisi",
// a code outside BMV QCA<n> ERR-037 unless it is the question's saved
// answer (agreed comment 4). The case's rows are replaced by the answered
// ones (DB-RULE-126; on Save possibly fewer than ten, agreed comment 1),
// then the form moves to `to` or stays on Save.
func (s *Service) SubmitQCA(ctx context.Context, sess auth.Session, caseID string, id int64, to string, answers []string) (int64, error) {
	if len(answers) != qcaQuestions {
		return 0, ErrUnsupported
	}
	return s.moveHotProspect(ctx, sess, caseID, id, AssignmentQCAData, to, func(c *fieldCheck) error {
		saved, err := savedQCAAnswers(c.ctx, c.tx, caseID)
		if err != nil {
			return err
		}
		for i, a := range answers {
			answers[i] = strings.TrimSpace(a)
			if answers[i] != "" && answers[i] == saved[i] {
				continue
			}
			c.code(qcaKey(i+1), "Pertanyaan "+strconv.Itoa(i+1), qcaType(i+1), answers[i], true)
		}
		if c.err != nil || len(c.errs) > 0 {
			return c.err
		}
		if _, err := c.tx.Exec(c.ctx, `DELETE FROM fos.case_qca_answer WHERE case_id = $1`, caseID); err != nil {
			return err
		}
		for i, a := range answers {
			if a == "" {
				continue
			}
			if _, err := c.tx.Exec(c.ctx, `INSERT INTO fos.case_qca_answer (case_id, question_number, answer_code) VALUES ($1, $2, $3)`,
				caseID, i+1, a); err != nil {
				return err
			}
		}
		return nil
	})
}
