package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"fos/internal/auth"
	"fos/internal/cases"
)

// regionJSON is the cascade of a form: codes of Provinsi, Kota, Kecamatan
// and Kelurahan.
type regionJSON struct {
	ProvinceCode    string `json:"provinceCode"`
	CityCode        string `json:"cityCode"`
	DistrictCode    string `json:"districtCode"`
	SubdistrictCode string `json:"subdistrictCode"`
}

// addressJSON is a Data Pribadi address block; numbers are digits.
type addressJSON struct {
	Address            string `json:"address"`
	MobilePhone        string `json:"mobilePhone"`
	Phone              string `json:"phone"`
	RT                 string `json:"rt"`
	RW                 string `json:"rw"`
	YearsOfStay        string `json:"yearsOfStay"`
	MonthsOfStay       string `json:"monthsOfStay"`
	HouseOwnershipCode string `json:"houseOwnershipCode"`
	regionJSON
}

func (a addressJSON) toAddress() cases.Address {
	return cases.Address{Address: a.Address, MobilePhone: a.MobilePhone, Phone: a.Phone, RT: a.RT, RW: a.RW,
		YearsOfStay: a.YearsOfStay, MonthsOfStay: a.MonthsOfStay, HouseOwnershipCode: a.HouseOwnershipCode, Region: cases.Region(a.regionJSON)}
}

func toAddressJSON(a cases.Address) addressJSON {
	return addressJSON{a.Address, a.MobilePhone, a.Phone, a.RT, a.RW, a.YearsOfStay, a.MonthsOfStay, a.HouseOwnershipCode, regionJSON(a.Region)}
}

type personalJSON struct {
	IDLifetime                bool        `json:"idLifetime"`
	Nationality               string      `json:"nationality"`
	Email                     string      `json:"email"`
	NPWP                      string      `json:"npwp"`
	JobCode                   string      `json:"jobCode"`
	Workplace                 string      `json:"workplace"`
	GenderCode                string      `json:"genderCode"`
	WorkplaceAddress          string      `json:"workplaceAddress"`
	GrossIncome               string      `json:"grossIncome"`
	IncomeSourceCode          string      `json:"incomeSourceCode"`
	RelationshipWithBankCode  string      `json:"relationshipWithBankCode"`
	CustomerCategoryCode      string      `json:"customerCategoryCode"`
	WorkplaceSectorCode       string      `json:"workplaceSectorCode"`
	BirthPlace                string      `json:"birthPlace"`
	WorkplaceBusinessTypeCode string      `json:"workplaceBusinessTypeCode"`
	EKTP                      addressJSON `json:"ektp"`
	SameAsEKTP                bool        `json:"sameAsEktp"`
	Domicile                  addressJSON `json:"domicile"`
	MailingAddressCode        string      `json:"mailingAddressCode"`
	EducationCode             string      `json:"educationCode"`
	MaritalStatusCode         string      `json:"maritalStatusCode"`
	MotherMaidenName          string      `json:"motherMaidenName"`
	DependentsCode            string      `json:"dependentsCode"`
}

func (f personalJSON) toForm() cases.PersonalForm {
	return cases.PersonalForm{
		IDLifetime: f.IDLifetime, Nationality: f.Nationality, Email: f.Email, NPWP: f.NPWP, JobCode: f.JobCode, Workplace: f.Workplace,
		GenderCode: f.GenderCode, WorkplaceAddress: f.WorkplaceAddress, GrossIncome: f.GrossIncome, IncomeSourceCode: f.IncomeSourceCode,
		RelationshipWithBankCode: f.RelationshipWithBankCode, CustomerCategoryCode: f.CustomerCategoryCode,
		WorkplaceSectorCode: f.WorkplaceSectorCode, BirthPlace: f.BirthPlace, WorkplaceBusinessTypeCode: f.WorkplaceBusinessTypeCode,
		EKTP: f.EKTP.toAddress(), SameAsEKTP: f.SameAsEKTP, Domicile: f.Domicile.toAddress(), MailingAddressCode: f.MailingAddressCode,
		EducationCode: f.EducationCode, MaritalStatusCode: f.MaritalStatusCode, MotherMaidenName: f.MotherMaidenName, DependentsCode: f.DependentsCode,
	}
}

func toPersonalJSON(f cases.PersonalForm) personalJSON {
	return personalJSON{
		f.IDLifetime, f.Nationality, f.Email, f.NPWP, f.JobCode, f.Workplace, f.GenderCode, f.WorkplaceAddress, f.GrossIncome,
		f.IncomeSourceCode, f.RelationshipWithBankCode, f.CustomerCategoryCode, f.WorkplaceSectorCode, f.BirthPlace,
		f.WorkplaceBusinessTypeCode, toAddressJSON(f.EKTP), f.SameAsEKTP, toAddressJSON(f.Domicile), f.MailingAddressCode,
		f.EducationCode, f.MaritalStatusCode, f.MotherMaidenName, f.DependentsCode,
	}
}

type spouseJSON struct {
	Name          string `json:"name"`
	NIK           string `json:"nik"`
	BirthDate     string `json:"birthDate"`
	BirthPlace    string `json:"birthPlace"`
	JobCode       string `json:"jobCode"`
	WorkDuration  string `json:"workDuration"`
	OfficeAddress string `json:"officeAddress"`
	Phone         string `json:"phone"`
	MobilePhone   string `json:"mobilePhone"`
	regionJSON
	EducationCode string `json:"educationCode"`
	PrenupCode    string `json:"prenupCode"`
}

type familyJSON struct {
	Name             string `json:"name"`
	RelationshipCode string `json:"relationshipCode"`
	Address          string `json:"address"`
	HomePhone        string `json:"homePhone"`
	OfficePhone      string `json:"officePhone"`
	regionJSON
	MobilePhone   string `json:"mobilePhone"`
	EducationCode string `json:"educationCode"`
}

func toOptionMapJSON(m map[string][]cases.Option) map[string][]optionJSON {
	out := make(map[string][]optionJSON, len(m))
	for k, opts := range m {
		out[k] = toOptionsJSON(opts)
	}
	return out
}

type personalPageJSON struct {
	NIK              string                  `json:"nik"`
	Name             string                  `json:"name"`
	BirthDate        string                  `json:"birthDate"`
	ExistingCustomer bool                    `json:"existingCustomer"`
	Values           personalJSON            `json:"values"`
	Sector           *optionJSON             `json:"sector"`
	Options          map[string][]optionJSON `json:"options"`
	Editable         bool                    `json:"editable"`
}

func toPersonalPageJSON(p cases.PersonalPage) personalPageJSON {
	out := personalPageJSON{p.NIK, p.Name, p.BirthDate, p.ExistingCustomer, toPersonalJSON(p.Values), nil, toOptionMapJSON(p.Options), p.Editable}
	if p.Sector != nil {
		out.Sector = &optionJSON{p.Sector.Code, p.Sector.Description}
	}
	return out
}

type spousePageJSON struct {
	Married           bool                    `json:"married"`
	Values            spouseJSON              `json:"values"`
	Options           map[string][]optionJSON `json:"options"`
	Editable          bool                    `json:"editable"`
	Today             string                  `json:"today"`
	EarliestBirthDate string                  `json:"earliestBirthDate"`
}

func toSpousePageJSON(p cases.SpousePage) spousePageJSON {
	v := p.Values
	return spousePageJSON{
		Married: p.Married, Options: toOptionMapJSON(p.Options), Editable: p.Editable, Today: p.Today, EarliestBirthDate: p.EarliestBirthDate,
		Values: spouseJSON{v.Name, v.NIK, v.BirthDate, v.BirthPlace, v.JobCode, v.WorkDuration, v.OfficeAddress, v.Phone, v.MobilePhone,
			regionJSON(v.Region), v.EducationCode, v.PrenupCode},
	}
}

type familyPageJSON struct {
	Values   familyJSON              `json:"values"`
	Options  map[string][]optionJSON `json:"options"`
	Editable bool                    `json:"editable"`
}

func toFamilyPageJSON(p cases.FamilyPage) familyPageJSON {
	v := p.Values
	return familyPageJSON{
		Values:  familyJSON{v.Name, v.RelationshipCode, v.Address, v.HomePhone, v.OfficePhone, regionJSON(v.Region), v.MobilePhone, v.EducationCode},
		Options: toOptionMapJSON(p.Options), Editable: p.Editable,
	}
}

// submitProspect handles POST .../assignments/{id}/{personal|spouse|family}?to=:
// Next, Back, a breadcrumb step or Create on Prospect Data Entry
// (FR-APP-012 to FR-APP-014). submit decodes the form's own fields only
// (BR-UI-010) and moves.
func (s *server) submitProspect(ep endpoint,
	submit func(ctx context.Context, dec *json.Decoder, sess auth.Session, caseID string, id int64, to string) (int64, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.session(w, r, ep)
		if !ok {
			return
		}
		caseID, id, ok := pathAssignment(r)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		next, err := submit(r.Context(), dec, sess, caseID, id, r.URL.Query().Get("to"))
		var bad badBody
		if errors.As(err, &bad) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeMoveResult(w, next, err)
	}
}

// badBody is a request body that is not the form, e.g. with another
// form's field (BR-UI-010): 400.
type badBody struct{ error }

func (s *server) submitPersonal(ep endpoint) http.HandlerFunc {
	return s.submitProspect(ep, func(ctx context.Context, dec *json.Decoder, sess auth.Session, caseID string, id int64, to string) (int64, error) {
		var f personalJSON
		if err := dec.Decode(&f); err != nil {
			return 0, badBody{err}
		}
		return s.cases.SubmitPersonal(ctx, sess, caseID, id, to, f.toForm())
	})
}

func (s *server) submitSpouse(ep endpoint) http.HandlerFunc {
	return s.submitProspect(ep, func(ctx context.Context, dec *json.Decoder, sess auth.Session, caseID string, id int64, to string) (int64, error) {
		var f spouseJSON
		if err := dec.Decode(&f); err != nil {
			return 0, badBody{err}
		}
		return s.cases.SubmitSpouse(ctx, sess, caseID, id, to, cases.SpouseForm{
			Name: f.Name, NIK: f.NIK, BirthDate: f.BirthDate, BirthPlace: f.BirthPlace, JobCode: f.JobCode, WorkDuration: f.WorkDuration,
			OfficeAddress: f.OfficeAddress, Phone: f.Phone, MobilePhone: f.MobilePhone, Region: cases.Region(f.regionJSON),
			EducationCode: f.EducationCode, PrenupCode: f.PrenupCode,
		})
	})
}

func (s *server) submitFamily(ep endpoint) http.HandlerFunc {
	return s.submitProspect(ep, func(ctx context.Context, dec *json.Decoder, sess auth.Session, caseID string, id int64, to string) (int64, error) {
		var f familyJSON
		if err := dec.Decode(&f); err != nil {
			return 0, badBody{err}
		}
		return s.cases.SubmitFamily(ctx, sess, caseID, id, to, cases.FamilyForm{
			Name: f.Name, RelationshipCode: f.RelationshipCode, Address: f.Address, HomePhone: f.HomePhone, OfficePhone: f.OfficePhone,
			Region: cases.Region(f.regionJSON), MobilePhone: f.MobilePhone, EducationCode: f.EducationCode,
		})
	})
}

// regions handles GET .../assignments/{id}/regions?type=&parent=: the units
// of a cascading region under their parent (FR-APP-005 rules 5 and 6).
func (s *server) regions(ep endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.session(w, r, ep)
		if !ok {
			return
		}
		caseID, id, ok := pathAssignment(r)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		q := r.URL.Query()
		opts, err := s.cases.Regions(r.Context(), sess, caseID, id, q.Get("type"), q.Get("parent"))
		if writeAssignmentError(w, "regions", err) {
			return
		}
		type regionOptionJSON struct {
			Code        string `json:"code"`
			Description string `json:"description"`
			Value1      string `json:"value1"`
		}
		out := make([]regionOptionJSON, 0, len(opts))
		for _, o := range opts {
			out = append(out, regionOptionJSON(o))
		}
		writeJSON(w, http.StatusOK, struct {
			Options []regionOptionJSON `json:"options"`
		}{out})
	}
}

// economicSectors handles GET .../assignments/{id}/economic-sectors?q=: the
// Bidang Usaha Tempat Bekerja autocomplete (BR-UI-005).
func (s *server) economicSectors(ep endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.session(w, r, ep)
		if !ok {
			return
		}
		caseID, id, ok := pathAssignment(r)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		opts, err := s.cases.EconomicSectors(r.Context(), sess, caseID, id, r.URL.Query().Get("q"))
		if writeAssignmentError(w, "economic sectors", err) {
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Options []optionJSON `json:"options"`
		}{toOptionsJSON(opts)})
	}
}
