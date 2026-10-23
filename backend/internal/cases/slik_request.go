package cases

import (
	"context"
)

// SLIKRequest is SLIK Request Data on UI-011 (Perorangan): BMV codes for the
// dropdowns and Alamat Nasabah (FR-PSAPI-008 sends them to SLIK).
type SLIKRequest struct {
	RequestPurpose string // Tujuan Permintaan, SLIK_TUJUANPERMINTAAN
	FacilityOption string // Pilihan Fasilitas, SLIK_PILIHANFASILITAS
	Address        string // Alamat Nasabah
	SearchType     string // Tipe Pencarian, SLIK_TIPEPENCARIAN
	Segmentation   string // Segmentasi, SLIK_SEGMENTASI
}

// SLIKSection is the SLIK Request Data section: the saved values and each
// dropdown's options.
type SLIKSection struct {
	Values  SLIKRequest
	Options SLIKOptions
}

type SLIKOptions struct {
	RequestPurpose, FacilityOption, SearchType, Segmentation []Option
}

// The BMV types of the dropdowns (§9.6).
const (
	bmvRequestPurpose = "SLIK_TUJUANPERMINTAAN"
	bmvFacilityOption = "SLIK_PILIHANFASILITAS"
	bmvSearchType     = "SLIK_TIPEPENCARIAN"
	bmvSegmentation   = "SLIK_SEGMENTASI"
)

func (s *Service) slikSection(ctx context.Context, caseID string) (SLIKSection, error) {
	var (
		sec                                                  SLIKSection
		purpose, facility, address, searchType, segmentation *string
	)
	if err := s.db.QueryRow(ctx, `
		SELECT slik_request_purpose_code, slik_facility_option_code, slik_identity_address,
		       slik_search_type_code, slik_segmentation_code
		FROM fos.work WHERE case_id = $1`, caseID).Scan(&purpose, &facility, &address, &searchType, &segmentation); err != nil {
		return SLIKSection{}, err
	}
	sec.Values = SLIKRequest{value(purpose), value(facility), value(address), value(searchType), value(segmentation)}
	for _, o := range []struct {
		bmvType string
		into    *[]Option
	}{
		{bmvRequestPurpose, &sec.Options.RequestPurpose},
		{bmvFacilityOption, &sec.Options.FacilityOption},
		{bmvSearchType, &sec.Options.SearchType},
		{bmvSegmentation, &sec.Options.Segmentation},
	} {
		opts, err := s.options(ctx, o.bmvType)
		if err != nil {
			return SLIKSection{}, err
		}
		*o.into = opts
	}
	return sec, nil
}
