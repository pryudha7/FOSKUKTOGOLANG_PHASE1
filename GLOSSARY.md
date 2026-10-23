# FOS KUK

Bank Nano Syariah's Financing Origination System for the KUK segment: it takes a small-business financing request from first submission through checks, assessment and approval to disbursement or rejection. Canonical terms are English; the Indonesian label used in the UI is given in parentheses.

## Financing

**Financing** (_Pembiayaan_):
Sharia-compliant provision of funds by the bank to a Customer.
_Avoid_: Loan, Kredit, Credit

**KUK**:
The small-business financing segment this system serves, covering both Individual and Business Entity Customers. Commercial and Gold are separate segments.
_Avoid_: using KUK as a synonym for Financing

**Financing Case**:
One Customer's request for Financing, tracked from submission until it is disbursed or rejected.
_Avoid_: Application, Pengajuan, Loan, Loan Financing

**Case Status**:
The current condition of a Financing Case within its stage, such as pending pre-screening or rejected at RAC.
_Avoid_: status (on its own)

**Phone Verification**:
A child case of a Financing Case for verifying information by telephone.

**Recommendation** (_Rekomendasi_):
A recorded judgement on a Financing Case by an officer or the System, with its type, decision and notes.

**Requested Amount** (_Total Pembiayaan yang Diajukan_):
The Financing amount the Customer asks for.
_Avoid_: plafon, limit

**Recommended Amount** (_Total Pembiayaan yang Direkomendasi_):
The Financing amount an officer recommends in a Recommendation.

**Approved Amount** (_Total Pembiayaan yang Disetujui_):
The Financing amount finally approved for the Financing Case.

**Tenor** (_Jangka Waktu_):
The length of the Financing; like the amount and Margin Rate, it is requested, recommended and approved.
_Avoid_: term, duration

**Reopen**:
Resuming a rejected Financing Case so that its unfinished checks continue, overriding its Check Rejection.

**Total Obligor**:
A Customer's existing financing plus the amount requested in the Financing Case.
_Avoid_: Exposure

## Products

**Product** (_Produk_):
A financing offering that fixes the limit, tenor and Contract available to a Financing Case.

**Contract** (_Akad_):
The Sharia contract under which the Financing is given, such as Murabahah or Qardh.
_Avoid_: contract type, loan agreement

**Margin Rate** (_Margin / Ujrah_):
The bank's return on the Financing, called Margin or Ujrah depending on the Contract.
_Avoid_: interest, interest rate

**Financing Simulation**:
The calculation of instalments from a Requested Amount, Tenor and Margin Rate.
_Avoid_: Credit Simulation

**Submission Type** (_Jenis Pengajuan_):
The kind of financing request a Financing Case represents.

**Take Over**:
A Submission Type in which the Financing replaces the Customer's existing financing at another bank.

## Customers

**Customer** (_Nasabah_):
The party who requests the Financing in a Financing Case.
_Avoid_: Debtor, Debitur, Client

**Customer Code**:
The identifier of the Customer as recorded on one Financing Case; the same person applying twice has two Customer Codes.
_Avoid_: Customer Unique ID

**Customer Type** (_Tipe Nasabah_):
Whether the Customer is an Individual or a Business Entity.

**Individual** (_Perorangan_):
A Customer Type for a natural person.
_Avoid_: INDIVIDU, personal

**Business Entity** (_Badan Usaha_):
A Customer Type for a legally constituted business.
_Avoid_: CORPORATE, business, company

**Spouse** (_Pasangan_):
The husband or wife of an Individual Customer.

**Close Relative** (_Keluarga Dekat_):
A relative of an Individual Customer who does not live in the Customer's household, recorded as an emergency contact.

**Company Officer** (_Pengurus_):
A person holding a formal role in a Business Entity Customer.

## People

**Branch Officer**:
The Account Officer or Senior Account Officer who created a Financing Case and carries it through data entry.
_Avoid_: AO, SAO, AO/SAO, branch officer pembuat case

**Team Lead**:
The Branch Officer's direct superior, who reviews the Branch Officer's work on a Financing Case.
_Avoid_: TL

## Stages

**Financing Submission**:
The first stage, in which the Branch Officer records the Customer and the request and the Pre-Screening is carried out.

**Financing Application**:
The stage in which the Branch Officer completes the Hot Prospect data, performs the Branch Officer Survey, and the case is checked against RAC and appraised.
_Avoid_: Application (on its own)

**Financing Checker**:
The stage in which the Team Lead checks the documents and recommends the Financing Case.

**Analyst Survey**:
The stage following Financing Checker in which an analyst surveys the Customer.
_Avoid_: Survey Analyst, Survey (on its own)

**Recommendation Stage**:
The stage following Analyst Survey.
_Avoid_: Recommendation (for the stage)

**Pre-Disbursement**:
The stage preparing a Financing Case for disbursement.

**FAC Checker**:
The stage following Pre-Disbursement.

**Disbursement**:
The final stage, in which the Financing is paid out.

**Rejected**:
The terminal stage of a Financing Case that has been rejected.

**Financing Revision**:
The stage in which a Financing Case is sent back for revision.

## Pre-Screening

**Pre-Screening Submission**:
The Branch Officer's entry of the Customer's identity data and supporting documents for pre-screening.
_Avoid_: Prescreening, PreScreening

**Underlying Document** (_Dokumen Underlying_):
A document evidencing the Customer's identity or legal standing, such as KTP, NPWP or deed of establishment, required for pre-screening.
_Avoid_: Document Underlying

**Pre-Screening Review**:
The Team Lead's judgement of whether the Pre-Screening Submission is Sesuai (matches) or Tidak Sesuai (does not match); Tidak Sesuai sends it back to the Branch Officer.
_Avoid_: Pre Screening, FAC decision

**Pre-Screening Check**:
The automated checks, run in order after a Sesuai Pre-Screening Review, of the Customer and any Spouse against Dukcapil, DHN, the Rejected List, SLIK and Pefindo; not every check applies to every party or Customer Type, and Company Officers are not checked.
_Avoid_: Pre-Screening API, Prescreening run

**Check Rejection**:
The outcome of a Pre-Screening Check in which the Customer fails a criterion, sending the Financing Case to Rejected.
_Avoid_: Fail, Failure

**Check Failure**:
The outcome of a Pre-Screening Check that could not complete, returning the Financing Case to the Team Lead to retry.
_Avoid_: Reject, Rejection

**Manual SLIK Entry** (_Input Manual_):
The Branch Officer's upload of a SLIK report by hand when the SLIK result does not arrive in time.
_Avoid_: SLIK Bypass

## External checks

**Dukcapil**:
The national civil registry, used to verify a person's identity.

**DHN** (_Daftar Hitam Nasional_):
The national blacklist of bad-cheque issuers.

**Rejected List**:
The bank's own register of Customers and Spouses whose Financing Case received a Check Rejection.
_Avoid_: Rejection List, blacklist

**SLIK**:
The OJK credit bureau report of a party's financing history across banks.

**Pefindo**:
A private credit bureau that assigns a party a risk grade.
_Avoid_: IDScore

**Collectibility** (_Kolektabilitas_):
The quality of a party's repayment on a facility, graded 1 (current) to 5 (loss).
_Avoid_: Kol, kolektibilitas

**Days Past Due** (_Hari Tunggakan_):
The number of days a repayment on a facility is overdue.
_Avoid_: HT, DPD

**RAC** (_Risk Acceptance Criteria_):
The bank's rules for whether a Financing Case falls within its risk appetite.

## Collateral

**Main Collateral** (_Agunan Utama_):
The asset securing the Financing; land and building as Main Collateral requires an Appraisal.
_Avoid_: Agunan, Jaminan, Guarantee (on their own)

**Supporting Collateral** (_Jaminan Pendukung_):
Additional security given alongside the Main Collateral.

**Appraisal**:
The valuation of a land-and-building Main Collateral during Financing Application.

## Funnel

**Pipeline**:
The earliest funnel stage of a Financing Case, recording the Submission Type, Product and any take-over bank.

**Prospect**:
The funnel stage recording the Customer's identity, and for an Individual, their Spouse and Close Relative.

**Hot Prospect**:
The funnel stage recording the full request: the Customer's business, other financing, requested facility, qualitative assessment, collateral and documents.

## Surveys

**Branch Officer Survey**:
The Branch Officer's assessment of the Customer's finances, through an income statement and balance sheet, during Financing Application.
_Avoid_: Survey Account Officer, Survey (on its own)
