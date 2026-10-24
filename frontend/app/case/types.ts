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

// One Connect API Log row as UI-020 shows it: submittedAt is Waktu Connect,
// dd/MM/yy HH:mm in the operator's timezone; at (RFC 3339) sorts it.
export type APILogEntry = { name: string; at: string; submittedAt: string; status: string; request: string; response: string };

// UI-021 read-only field; indent puts it under Params.
export type APIDataField = { label: string; value: string; indent?: boolean };

// One Pre-Screening API Data sub-accordion (FR-PSAPI-005).
export type APIDataProcess = {
  name: string;
  rejected: boolean;
  rejectedOnly: boolean; // Rejected List: the banner only
  request: APIDataField[];
  response: APIDataField[];
  errors: string[];
  results: APIDataField[][];
  documentId?: string;
  documentName?: string;
  documentKind?: "slik" | "pefindo"; // View SLIK or Download PDF
};

// The History Details card (UI-013): API Log History and Pre-Screening API Data.
export type HistoryCard = {
  logs: APILogEntry[];
  truncateChars: number;
  truncateLines: number;
  processes: APIDataProcess[];
};

// UI-023 Waiting SLIK: whole minutes left before the SLIK SLA when opened.
export type WaitingSLIKPage = {
  editable: boolean;
  dueMinutes: number;
};

// One subject's Check SLIK Status answer.
export type SLIKStatus = { subject: "CUSTOMER" | "SPOUSE"; message: string };

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
  // How the log values are cut on display (with retry or slikTimeout).
  truncateChars: number;
  truncateLines: number;
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
// Numbered sections name card n (from 1) of a list: Data Pengurus
// (company-officer-<n>, FR-APP-005), Data Pembiayaan Lain
// (other-financing-<n>, FR-APP-017), Agunan Utama and Jaminan Pendukung
// (main-collateral-<n>, support-collateral-<n>, FR-APP-021). sectionNumber
// reads n back, or null for another kind of section.
export type NumberedSection = "company-officer" | "other-financing" | "main-collateral" | "support-collateral";
export type Section = "form" | "customer" | "spouse" | "upload" | "slik" | "documents" | "ektp" | "domicile" | `${NumberedSection}-${number}`;

export const numberedSection = (prefix: NumberedSection, n: number): Section => `${prefix}-${n}`;
export const companyOfficerSection = (n: number): Section => numberedSection("company-officer", n);
export const otherFinancingSection = (n: number): Section => numberedSection("other-financing", n);
export function sectionNumber(prefix: NumberedSection, section: Section): number | null {
  const m = new RegExp(`^${prefix}-(\\d+)$`).exec(section);
  return m ? Number(m[1]) : null;
}

// Data Usaha Nasabah (FR-APP-016); numbers are digits.
export type BusinessDataForm = Region & {
  businessField: string;
  businessCategoryCode: string;
  businessAgeMonths: string;
  employeeCount: string;
  businessLocationCode: string;
  address: string;
  houseOwnershipCode: string;
  yearsOfStay: string;
  monthsOfStay: string;
  phone: string;
  rt: string;
  rw: string;
};

// facilityRequested: Next on Data Permohonan Fasilitas has recorded its
// recommendation, so the steps after it open by breadcrumb.
export type BusinessDataPage = { values: BusinessDataForm; options: Record<string, Option[]>; failedOptions: string[]; facilityRequested: boolean; editable: boolean };

// One Data Pembiayaan Lain card (FR-APP-017): amounts and counts as digits,
// marginRate with a dot ("21.00").
export type OtherFinancingCard = {
  bankName: string;
  financingType: string;
  financingAmount: string;
  outstandingAmount: string;
  tenorMonths: string;
  marginRate: string;
  slikCollect: string;
  purpose: string;
};

export type OtherFinancingPage = { values: { otherFinancings: OtherFinancingCard[] }; slikAvailable: boolean; facilityRequested: boolean; editable: boolean };

// Data Permohonan Fasilitas (FR-APP-018): amounts and months as digits,
// marginRate with a dot ("12.00000"); the totals are the server's.
export type FacilityRequestForm = {
  purposeCode: string;
  requestedAmount: string;
  tenureMonths: string;
  existingFinancingAmount: string;
  marginTypeCode: string;
  marginRate: string;
  deviationNote: string;
  mitigationNote: string;
  notes: string;
};

// UI-032 with Tipe Akad, what the Financing Simulation names, and whether
// the steps after it open by breadcrumb.
export type FacilityRequestPage = {
  values: FacilityRequestForm;
  contractType: string;
  productName: string;
  applicantName: string;
  facilityRequested: boolean;
  options: Record<string, Option[]>;
  failedOptions: string[];
  editable: boolean;
};

// One Collateral Data card (FR-APP-021): codes, texts, amounts and years
// as digits, dates as yyyy-mm-dd.
export type CollateralCard = {
  typeCode: string;
  buildingFormCode: string;
  ownershipTypeCode: string;
  ownerName: string;
  shmNumber: string;
  shgbNumber: string;
  shgbValidUntil: string;
  bpkbNumber: string;
  fiduciaryNumber: string;
  location: string;
  landArea: string;
  dueDate: string;
  physicalValue: string;
  objectValue: string;
  depositAmount: string;
  vehicleBrand: string;
  vehicleYear: string;
  remarks: string;
  collateralStatusCode: string;
  collateralTypeText: string;
  bindingType: string;
  bindingDate: string;
  appraisalDate: string;
  kjppAppraisalDate: string;
  njopValue: string;
  kjppValue: string;
  kjppAppraiserName: string;
  pariPassuStatusCode: string;
  pariPassuPercent: string;
  jointFinancingCode: string;
  insuredCode: string;
};

// UI-034 Collateral Data: the cards of each section, the card limit and
// the operator's today.
export type CollateralPage = {
  values: { main: CollateralCard[]; support: CollateralCard[] };
  options: Record<string, Option[]>;
  failedOptions: string[];
  maxCards: number;
  today: string;
  facilityRequested: boolean;
  editable: boolean;
};

// UI-033 QCA Data: the answer code of questions 1 to 10, "" when not
// answered; options keyed qca1 to qca10.
export type QCAPage = {
  values: { answers: string[] };
  options: Record<string, Option[]>;
  failedOptions: string[];
  facilityRequested: boolean;
  editable: boolean;
};

// The Financing Simulation of the values on screen: installments in whole
// rupiah, margins with five decimals; all zero until ready (DEC-146).
export type FinancingSimulation = {
  ready: boolean;
  installmentFlat: string;
  installmentEffective: string;
  marginFlat: string;
  marginEffective: string;
};

// The customer's SLIK facilities for Isi Data dari SLIK: amounts and margin
// as SLIK has them (two decimals) and the card each fills.
export type SLIKFacility = { financingAmount: string; outstandingAmount: string; marginRate: string; card: OtherFinancingCard };
export type SLIKFacilityGroup = { title: string; facilities: SLIKFacility[] };

// A validation failure returned with 422 by a submit: key is the stable
// field key, field the label shown in the banner.
export type FieldError = { section: Section; key: string; field: string; code: string; message: string };

// A file item of UI-024: name is the link text, caption the line under it,
// fileName the download's name.
export type DocumentItem = { fileId: string; fileName: string; name: string; caption: string };

// UI-024 Pipeline Data Entry (FR-APP-004): the case's values (codes; Nama
// Bank as typed) and the dropdown options.
export type PipelinePage = {
  branchCode: string; // empty when the Branch is not Active
  branchName: string;
  customerType: string;
  submissionType: string;
  productCode: string;
  bankName: string;
  submissionTypeOptions: Option[];
  productOptions: Option[]; // description is the product name
  identity: { label: string; file: DocumentItem | null }; // KTP, or NPWP Badan Usaha
  bankLetter: DocumentItem | null;
  editable: boolean;
};

// The region cascade of a form: codes of Provinsi, Kota, Kecamatan, Kelurahan.
export type Region = { provinceCode: string; cityCode: string; districtCode: string; subdistrictCode: string };

// A Data Pribadi address block (UI-026); numbers are digits.
export type AddressForm = Region & {
  address: string;
  mobilePhone: string;
  phone: string;
  rt: string;
  rw: string;
  yearsOfStay: string;
  monthsOfStay: string;
  houseOwnershipCode: string;
};

// Data Pribadi (FR-APP-012) as sent and saved: BMV codes, digits for numbers.
export type IndividualDataForm = {
  idLifetime: boolean;
  nationality: string;
  email: string;
  npwp: string;
  jobCode: string;
  workplace: string;
  genderCode: string;
  workplaceAddress: string;
  grossIncome: string;
  incomeSourceCode: string;
  relationshipWithBankCode: string;
  customerCategoryCode: string;
  workplaceSectorCode: string;
  birthPlace: string;
  workplaceBusinessTypeCode: string;
  ektp: AddressForm;
  sameAsEktp: boolean;
  domicile: AddressForm;
  mailingAddressCode: string;
  educationCode: string;
  maritalStatusCode: string;
  motherMaidenName: string;
  dependentsCode: string;
};

export type IndividualDataPage = {
  nik: string;
  name: string;
  birthDate: string; // yyyy-mm-dd
  existingCustomer: boolean;
  values: IndividualDataForm;
  sector: Option | null; // the saved Bidang Usaha Tempat Bekerja
  options: Record<string, Option[]>;
  failedOptions: string[]; // dropdowns that could not be loaded (ERR-039)
  editable: boolean;
};

// Data Pasangan (FR-APP-013); birthDate is yyyy-mm-dd.
export type SpouseForm = Region & {
  name: string;
  nik: string;
  birthDate: string;
  birthPlace: string;
  jobCode: string;
  workDuration: string;
  officeAddress: string;
  phone: string;
  mobilePhone: string;
  educationCode: string;
  prenupCode: string;
};

export type SpousePage = {
  married: boolean;
  values: SpouseForm;
  options: Record<string, Option[]>;
  failedOptions: string[];
  editable: boolean;
  today: string;
  earliestBirthDate: string;
};

// Data Keluarga Dekat (FR-APP-014).
export type CloseRelativeForm = Region & {
  name: string;
  relationshipCode: string;
  address: string;
  homePhone: string;
  officePhone: string;
  mobilePhone: string;
  educationCode: string;
};

export type CloseRelativePage = { values: CloseRelativeForm; options: Record<string, Option[]>; failedOptions: string[]; editable: boolean };

// One Data Pengurus card, a Company Officer (FR-APP-002); birthDate is yyyy-mm-dd.
export type CompanyOfficer = { roleCode: string; nik: string; name: string; birthDate: string };

// The Badan Usaha form (FR-APP-005); goPublic is the BMV description.
export type BusinessEntityForm = Region & {
  businessEntityCode: string;
  placeOfEstablishment: string;
  deedNumber: string;
  lastDeedNumber: string;
  lastDeedDate: string;
  phone: string;
  mobilePhone: string;
  email: string;
  economicSectorCode: string;
  relationshipWithBankCode: string;
  businessTypeCode: string;
  customerCategoryCode: string;
  address: string;
  goPublic: string;
  businessGroupName: string;
  companyOfficers: CompanyOfficer[];
};

export type BusinessEntityPage = {
  npwp: string;
  name: string;
  establishmentDate: string;
  existingCustomer: boolean;
  values: BusinessEntityForm;
  sector: Option | null;
  category: Option | null;
  options: Record<string, Option[]>;
  failedOptions: string[];
  editable: boolean;
  today: string;
  earliestBirthDate: string;
  latestDeedDate: string;
};

export type Assignment = {
  assignmentId: number;
  name: string;
  status: string;
  // The page of the assignment by its name (CaseView): Pre-Screening
  // Submission, Document Underlying Upload, Pre Screening, Waiting SLIK,
  // Pipeline Data Entry, Prospect and Hot Prospect Data Entry.
  page: PreScreeningPage | UnderlyingPage | ReviewPage | WaitingSLIKPage | PipelinePage | IndividualDataPage | SpousePage | CloseRelativePage | BusinessEntityPage | BusinessDataPage | OtherFinancingPage | FacilityRequestPage | QCAPage | CollateralPage | null;
};
