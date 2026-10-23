// SoT names the UI compares against (UI text, untranslated).
export const assignmentNames = {
  preScreeningSubmission: "Pre-Screening Submission",
  underlyingDocumentUpload: "Document Underlying Upload",
  preScreeningReview: "Pre Screening", // the Team Lead's Pre-Screening Review
} as const;

// The Tipe Nasabah description whose form exists today (Badan Usaha:
// ticket 31; other types: TBD-029a).
export const customerTypeIndividual = "Perorangan";

// Shown when a request fails for a reason other than validation (ERR-003).
export const unavailableMessage = "Layanan sedang tidak tersedia. Silakan coba beberapa saat lagi.";
