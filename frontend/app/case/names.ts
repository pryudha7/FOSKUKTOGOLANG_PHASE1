// SoT names the UI compares against (UI text, untranslated).
export const assignmentNames = {
  preScreeningSubmission: "Pre-Screening Submission",
  underlyingDocumentUpload: "Document Underlying Upload",
  preScreeningReview: "Pre Screening", // the Team Lead's Pre-Screening Review
  waitingSLIK: "SLIKWQFlowAct", // UI-023
  pipelineDataEntry: "Pipeline Data Entry", // UI-024
  individualData: "Data Pribadi", // UI-026
  spouseData: "Data Pasangan", // UI-027
  closeRelativeData: "Data Keluarga Dekat", // UI-028
  businessEntity: "Badan Usaha", // UI-025
  businessData: "Data Usaha Nasabah", // UI-030
  otherFinancingData: "Data Pembiayaan Lain", // UI-031
  facilityRequest: "Data Permohonan Fasilitas", // UI-032
  qcaData: "QCA Data", // UI-033
  collateralData: "Collateral Data", // UI-034
} as const;

// The Tipe Nasabah description whose form exists today (Badan Usaha:
// ticket 31; other types: TBD-029a).
export const customerTypeIndividual = "Perorangan";

// Shown when a request fails for a reason other than validation (ERR-003).
export const unavailableMessage = "Layanan sedang tidak tersedia. Silakan coba beberapa saat lagi.";
