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
};

export type StoredFile = { fileId: string; fileName: string };

// One accordion of UI-011: code is the stored codename, label is shown.
export type DocumentCategory = {
  code: string;
  label: string;
  documents: { documentType: string; mandatory: boolean; file: StoredFile | null }[];
};

export type UnderlyingPage = {
  customerType: string;
  memoNumber: string; // empty until it can be formed (FR-APP-008)
  totalLimitMb: number;
  categories: DocumentCategory[];
  listError: { code: string; message: string } | null; // ERR-027
  editable: boolean;
};

export type Section = "form" | "customer" | "spouse" | "upload";

// A validation failure returned with 422 by a submit: key is the stable
// field key, field the label shown in the banner.
export type FieldError = { section: Section; key: string; field: string; code: string; message: string };

export type Assignment = {
  assignmentId: number;
  name: string;
  status: string;
  // PreScreeningPage on Pre-Screening Submission, UnderlyingPage on
  // Document Underlying Upload.
  page: PreScreeningPage | UnderlyingPage | null;
};
