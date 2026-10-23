// Shapes returned by the case API (backend/internal/app/cases.go).

export type Person = { userId: string; name: string; at: string };

export type StageItem = { name: string; current: boolean; entered: boolean };

export type LifecycleStage = {
  stage: string;
  entered: boolean;
  processes: { name: string; started: boolean }[];
};

export type CaseFrame = {
  caseId: string;
  caseType: string;
  caseTypeLabel: string;
  priority: number;
  status: string;
  created: Person;
  updated: Person;
  stages: StageItem[];
  lifecycle: LifecycleStage[];
  attachmentCount: number;
  attachments: Attachment[]; // newest first
};

// FR-CASE-006 panel item; uploadedBy is the uploader's full name.
export type Attachment = {
  fileId: string;
  fileName: string;
  categoryLabel: string;
  uploadedBy: string;
  createdAt: string;
};

export type Option = { code: string; description: string };

// birthDate is yyyy-mm-dd.
export type PersonForm = { nik: string; name: string; birthDate: string };

export type PreScreeningPage = {
  branchName: string;
  branchError: { code: string; message: string } | null;
  customerType: string;
  customerTypeOptions: Option[];
  customer: PersonForm; // Data Pribadi
  spouse: PersonForm; // Data Pasangan
  // False for an Authors look at someone else's assignment: read-only.
  editable: boolean;
  // The operator's today and the earliest birth date, yyyy-mm-dd.
  today: string;
  earliestBirthDate: string;
  // The latest Pre-Screening Review decision once the case has one (FR-APP-010 rule 2).
  notesBanner: { decision: string; notes: string } | null;
};

// One entry of History Pre-screening Notes; submittedAt is dd/MM/yy H:mm in
// the operator's timezone, decidedBy a full name.
export type Recommendation = { decidedBy: string; submittedAt: string; decision: string; notes: string };

// One Connect API Log row (UI-020); submittedAt is dd/MM/yy H:mm.
export type APILogEntry = { name: string; submittedAt: string; status: string; request: string; response: string };

// UI-017 for the Team Lead.
export type ReviewPage = {
  decisionOptions: Option[];
  history: Recommendation[]; // oldest first
  editable: boolean;
  // Set after a Check Failure: the Retry view, without decision fields.
  retry: { logs: APILogEntry[] } | null;
  // Set after Manual SLIK Entry (Timeout SLIK variant): the SLIK log rows
  // above the history and the decision fields.
  slikTimeout: { logs: APILogEntry[] } | null;
};

export type StoredFile = { fileId: string; fileName: string };

// One accordion of UI-011: code is the stored codename, label is shown.
export type DocumentCategory = {
  code: string;
  label: string;
  documents: { documentType: string; mandatory: boolean; file: StoredFile | null }[];
};

// SLIK Request Data (UI-011, Perorangan): BMV codes and Alamat Nasabah.
export type SLIKRequest = {
  requestPurpose: string;
  facilityOption: string;
  address: string;
  searchType: string;
  segmentation: string;
};

export type UnderlyingPage = {
  customerType: string;
  memoNumber: string; // empty until it can be formed (FR-APP-008)
  // Perorangan only: the saved values and each dropdown's options.
  slik: {
    values: SLIKRequest;
    options: Record<"requestPurpose" | "facilityOption" | "searchType" | "segmentation", Option[]>;
  } | null;
  totalLimitMb: number;
  categories: DocumentCategory[];
  listError: { code: string; message: string } | null; // ERR-027
  editable: boolean;
};

// "documents" errors (ERR-026) belong to a document row, keyed
// "<category>/<document type>", and are not banner items.
export type Section = "form" | "customer" | "spouse" | "upload" | "slik" | "documents";

// A validation failure returned with 422 by a submit: key is the stable
// field key, field the label shown in the banner.
export type FieldError = { section: Section; key: string; field: string; code: string; message: string };

export type Assignment = {
  assignmentId: number;
  name: string;
  status: string;
  // PreScreeningPage on Pre-Screening Submission, UnderlyingPage on
  // Document Underlying Upload.
  page: PreScreeningPage | UnderlyingPage | ReviewPage | null;
};
