package cases

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Field rules shared by the Prospect Data Entry forms (FR-APP-005 rules
// 5-9, FR-APP-012 to FR-APP-014): one error per field, in field order.

// tooLong is ERR-040 "<Nama Field> maksimal <n> karakter".
func tooLong(section, key, label string, max int) FieldError {
	return FieldError{section, key, label, "ERR-040", fmt.Sprintf("%s maksimal %d karakter", label, max)}
}

// regionLevels are the cascade, top first: BMV type, label and field key.
var regionLevels = []struct{ bmvType, label, key string }{
	{"PROVINCE", "Provinsi", "province"},
	{"CITY", "Kota", "city"},
	{"DISTRICT", "Kecamatan", "district"},
	{"SUBDISTRICT", "Kelurahan", "subdistrict"},
}

func (r Region) codes() []string {
	return []string{r.ProvinceCode, r.CityCode, r.DistrictCode, r.SubdistrictCode}
}

// fieldCheck collects a form's errors in field order (one per field) and
// keeps the first database error; section is the banner section of the
// fields being checked. lenient is Save on Hot Prospect: no field is
// required, formats and ranges still are checked (FR-APP-016 Process 4).
type fieldCheck struct {
	ctx     context.Context
	tx      pgx.Tx
	section string
	lenient bool
	errs    []FieldError
	err     error
}

func (c *fieldCheck) add(key, label, code, message string) {
	c.errs = append(c.errs, FieldError{c.section, key, label, code, message})
}

func (c *fieldCheck) addError(e *FieldError) {
	if e != nil {
		c.errs = append(c.errs, *e)
	}
}

// invalid is ERR-037 on a field (BR-UI-007).
func (c *fieldCheck) invalid(key, label string) {
	c.errs = append(c.errs, invalid(c.section, key, label))
}

// filled reports ERR-010 for an empty mandatory value; false when empty.
func (c *fieldCheck) filled(key, label, v string, mandatory bool) bool {
	if v != "" {
		return true
	}
	if mandatory && !c.lenient {
		c.errs = append(c.errs, required(c.section, key, label))
	}
	return false
}

// text is a free text of at most max characters (ERR-040).
func (c *fieldCheck) text(key, label, v string, mandatory bool, max int) {
	if c.filled(key, label, v, mandatory) && utf8.RuneCountInString(v) > max {
		c.errs = append(c.errs, tooLong(c.section, key, label, max))
	}
}

// code is a value that must be an Active row of a BMV type (BR-UI-007).
func (c *fieldCheck) code(key, label, bmvType, v string, mandatory bool) {
	if !c.filled(key, label, v, mandatory) || c.err != nil {
		return
	}
	known, err := activeCode(c.ctx, c.tx, bmvType, v)
	switch {
	case err != nil:
		c.err = err
	case !known:
		c.invalid(key, label)
	}
}

var (
	phonePattern = regexp.MustCompile(`^[0-9+\-]+$`)
	digits       = regexp.MustCompile(`^[0-9]+$`)
	// Letters and spaces only (Tempat Lahir).
	placePattern = regexp.MustCompile(`^[\p{L} ]+$`)
)

// phone is FR-APP-005 rule 8: 0-9, "+" and "-" only, one digit at least, at
// most 15 digits and 20 characters.
func (c *fieldCheck) phone(key, label, v string, mandatory bool) {
	if !c.filled(key, label, v, mandatory) {
		return
	}
	n := 0
	for _, r := range v {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	if !phonePattern.MatchString(v) || n == 0 || n > 15 || len(v) > 20 {
		c.invalid(key, label)
	}
}

// email is FR-APP-005 rule 9: optional, one "@", local part and a domain
// with a dot, no spaces, at most 254 characters.
func (c *fieldCheck) email(key, label, v string) {
	if !c.filled(key, label, v, false) {
		return
	}
	local, domain, ok := strings.Cut(v, "@")
	switch {
	case utf8.RuneCountInString(v) > 254:
		c.errs = append(c.errs, tooLong(c.section, key, label, 254))
	case !ok || local == "" || domain == "" || strings.Contains(domain, "@") || !strings.Contains(domain, ".") || strings.ContainsAny(v, " \t"):
		c.invalid(key, label)
	}
}

// npwp is the Pre-Screening NPWP rule: digits only (ERR-011), 15 to 22 of
// them (ERR-012).
func (c *fieldCheck) npwp(key, label, v string) {
	switch {
	case !c.filled(key, label, v, true):
	case !digits.MatchString(v):
		c.add(key, label, "ERR-011", "Harap masukkan angka saja")
	case len(v) < 15 || len(v) > 22:
		c.add(key, label, "ERR-012", "NPWP minimal 15 digit maksimal 22 digit")
	}
}

// number is a whole number from 0 to max (FR-APP-012 rule 10); over max or
// not a whole number is ERR-037.
func (c *fieldCheck) number(key, label, v string, max int, mandatory bool) {
	if !c.filled(key, label, v, mandatory) {
		return
	}
	n, err := strconv.Atoi(v)
	if !digits.MatchString(v) || err != nil || n > max {
		c.invalid(key, label)
	}
}

// money is a whole rupiah amount, 0 to 999.999.999.999.999 (BR-UI-009,
// FR-APP-012 rule 5, FR-APP-017 Field 2-3).
func (c *fieldCheck) money(key, label, v string, mandatory bool) {
	if c.filled(key, label, v, mandatory) && (!digits.MatchString(v) || len(v) > 15) {
		c.invalid(key, label)
	}
}

// percentPattern is up to three whole digits and two decimals, dot-separated.
var percentPattern = regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,2})?$`)

// percent is an optional rate from 0 to 100 with at most two decimals
// (BR-UI-013, Margin / Ujrah).
func (c *fieldCheck) percent(key, label, v string) {
	if !c.filled(key, label, v, false) {
		return
	}
	rate, err := strconv.ParseFloat(v, 64)
	if !percentPattern.MatchString(v) || err != nil || rate > 100 {
		c.invalid(key, label)
	}
}

// positive is a whole number from 1 to max (FR-APP-018: Pembiayaan yang di
// ajukan, Jangka Waktu); 0 or over max is ERR-037.
func (c *fieldCheck) positive(key, label, v string, max int64, mandatory bool) {
	if !c.filled(key, label, v, mandatory) {
		return
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if !digits.MatchString(v) || err != nil || n < 1 || n > max {
		c.invalid(key, label)
	}
}

// marginPattern is up to three whole digits and five decimals, dot-separated.
var marginPattern = regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,5})?$`)

// margin is a rate from 0 to 100 with at most five decimals (BR-UI-014,
// Margin / Ujrah of Data Permohonan Fasilitas).
func (c *fieldCheck) margin(key, label, v string, mandatory bool) {
	if !c.filled(key, label, v, mandatory) {
		return
	}
	rate, err := strconv.ParseFloat(v, 64)
	if !marginPattern.MatchString(v) || err != nil || rate > 100 {
		c.invalid(key, label)
	}
}

// rtRW is RT or RW: digits only, at most 3, leading zeros kept.
func (c *fieldCheck) rtRW(key, label, v string) {
	switch {
	case !c.filled(key, label, v, true):
	case !digits.MatchString(v):
		c.invalid(key, label)
	case len(v) > 3:
		c.errs = append(c.errs, tooLong(c.section, key, label, 3))
	}
}

// birthPlace is Tempat Lahir: letters and spaces, at most 50.
func (c *fieldCheck) birthPlace(key, label, v string) {
	switch {
	case !c.filled(key, label, v, true):
	case utf8.RuneCountInString(v) > 50:
		c.errs = append(c.errs, tooLong(c.section, key, label, 50))
	case !placePattern.MatchString(v):
		c.invalid(key, label)
	}
}

// resolvedRegion is a checked region with what the server works out:
// Dati II (the Kota's Value1) and Kode Pos (DB-RULE-096).
type resolvedRegion struct {
	Region
	regency, postal string
}

// region checks the cascade (FR-APP-005 rules 5 and 6): each unit must be an
// Active row under the unit above it. Below an empty unit a filled one is
// ERR-037 when the region is optional (when mandatory the empty unit is
// already ERR-010); below an invalid one nothing is judged again, but an
// empty mandatory unit is still ERR-010.
func (c *fieldCheck) region(r Region, mandatory bool) resolvedRegion {
	const (
		fine = iota
		missing
		broken
	)
	out := resolvedRegion{Region: r}
	parent, above := "", fine
	for i, code := range r.codes() {
		level := regionLevels[i]
		if !c.filled(level.key, level.label, code, mandatory) {
			above = missing
			continue
		}
		if above == missing && !mandatory {
			c.invalid(level.key, level.label)
		}
		if above != fine {
			continue
		}
		var value1 *string
		err := c.tx.QueryRow(c.ctx, `
			SELECT value1 FROM fos.business_mapping_value
			WHERE type = $1 AND sub_type = $2 AND code = $3 AND `+activeStatus, level.bmvType, parent, code).Scan(&value1)
		switch {
		case isNotFound(err):
			c.invalid(level.key, level.label)
			above = broken
			continue
		case err != nil:
			c.err = err
			return out
		}
		if level.bmvType == "CITY" {
			out.regency = value(value1)
		}
		parent = code
	}
	if above == fine && r.SubdistrictCode != "" {
		err := c.tx.QueryRow(c.ctx, `
			SELECT code FROM fos.business_mapping_value WHERE type = 'POSTALCODE' AND sub_type = $1 AND `+activeStatus+`
			ORDER BY sort_order, code LIMIT 1`, r.SubdistrictCode).Scan(&out.postal)
		if err != nil && !isNotFound(err) {
			c.err = err
		}
	}
	return out
}

// address checks a Data Pribadi address block in its layout order.
func (c *fieldCheck) address(a Address, addressLabel string, mailing *string) resolvedRegion {
	c.text("address", addressLabel, a.Address, true, 350)
	c.phone("mobilePhone", "Nomor Handphone", a.MobilePhone, true)
	c.phone("phone", "Nomor Telepon", a.Phone, true)
	if mailing == nil {
		c.rtRW("rt", "RT", a.RT)
		c.rtRW("rw", "RW", a.RW)
		c.number("yearsOfStay", "Lama Tinggal (Tahun)", a.YearsOfStay, 99, true)
		c.number("monthsOfStay", "Lama Tinggal (Bulan)", a.MonthsOfStay, 11, true)
		c.code("houseOwnership", "Status Kepemilikan Rumah", "HOUSE_OWNERSHIP_STATUS", a.HouseOwnershipCode, true)
	} else {
		// Domisili: Bulan, Tahun and Alamat Surat Menyurat come first (Gambar 52).
		c.number("monthsOfStay", "Lama Tinggal (Bulan)", a.MonthsOfStay, 11, true)
		c.number("yearsOfStay", "Lama Tinggal (Tahun)", a.YearsOfStay, 99, true)
		c.code("mailingAddress", "Alamat Surat Menyurat", "MAILING_ADDRESS", *mailing, true)
		c.code("houseOwnership", "Status Kepemilikan Rumah", "HOUSE_OWNERSHIP_STATUS", a.HouseOwnershipCode, true)
		c.rtRW("rt", "RT", a.RT)
		c.rtRW("rw", "RW", a.RW)
	}
	return c.region(a.Region, true)
}

// pastDate is an optional date (yyyy-mm-dd) before today, with no lower
// bound (FR-APP-005 rule 7); today or later, or no real date, is ERR-037.
func (c *fieldCheck) pastDate(key, label, v string, today time.Time) {
	if !c.filled(key, label, v, false) {
		return
	}
	d, err := time.Parse(dateISO, v)
	if err != nil || !d.Before(today) {
		c.invalid(key, label)
	}
}

// description is a mandatory value that must be the description of an
// Active row of a BMV type, for a field stored by description (Go Public,
// §9.6).
func (c *fieldCheck) description(key, label, bmvType, v string) {
	if !c.filled(key, label, v, true) || c.err != nil {
		return
	}
	opts, err := optionsIn(c.ctx, c.tx, bmvType)
	if err != nil {
		c.err = err
		return
	}
	for _, o := range opts {
		if o.Description == v {
			return
		}
	}
	c.invalid(key, label)
}
