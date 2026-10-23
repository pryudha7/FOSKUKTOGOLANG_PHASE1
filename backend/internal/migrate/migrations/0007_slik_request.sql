-- Ticket 33: SLIK Request Data of a Perorangan case (UI-011). Not in §9.4;
-- agreed for TBD-039b. Codes are BMV codes of SLIK_TUJUANPERMINTAAN,
-- SLIK_PILIHANFASILITAS, SLIK_TIPEPENCARIAN and SLIK_SEGMENTASI; the address
-- is Alamat Nasabah, sent to SLIK as alamat_sesuai_identitas (FR-PSAPI-008).
ALTER TABLE fos.work
    ADD COLUMN slik_request_purpose_code VARCHAR,
    ADD COLUMN slik_facility_option_code VARCHAR,
    ADD COLUMN slik_identity_address     VARCHAR,
    ADD COLUMN slik_search_type_code     VARCHAR,
    ADD COLUMN slik_segmentation_code    VARCHAR;
