-- Ticket 05 review: DB-RULE-029 and DB-RULE-015.

-- DB-RULE-029: stage_name is one of the §8.1 stages.
ALTER TABLE fos.case_stage_progress ADD CONSTRAINT case_stage_progress_stage_chk CHECK (stage_name IN (
    'Financing Submission', 'Financing Application', 'Financing Checker', 'Survey Analyst', 'Recommendation',
    'Pre-Disbursement', 'FAC Checker', 'Disbursement', 'Rejected', 'Financing Revision'));
ALTER TABLE fos.lifecycle_process ADD CONSTRAINT lifecycle_process_stage_chk CHECK (stage_name IN (
    'Financing Submission', 'Financing Application', 'Financing Checker', 'Survey Analyst', 'Recommendation',
    'Pre-Disbursement', 'FAC Checker', 'Disbursement'));

-- DB-RULE-015 (PROPOSED): in process Pre-Screening Submission a case has at
-- most one unfinished assignment at a time. Its assignments are
-- Pre-Screening Submission, Document Underlying Upload and Pre Screening (§8.1).
CREATE UNIQUE INDEX assignments_one_open_pre_screening_idx ON fos.assignments (case_id)
    WHERE assignment_status <> 'sudah selesai dikerjakan'
      AND assignment_name IN ('Pre-Screening Submission', 'Document Underlying Upload', 'Pre Screening');
