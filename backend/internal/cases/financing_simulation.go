package cases

import (
	"math/big"
	"strconv"
	"strings"
)

// Financing Simulation arithmetic of Data Permohonan Fasilitas (FR-APP-018
// Perhitungan, agreed comment 11): P the amount, n the months, m the margin
// typed in percent. Computed with 256-bit floats; installments are rounded
// to whole rupiah and margins to five decimals, half up.

// Margin types (BMV MARGINTYPE, DEC-145).
const (
	MarginFlat      = "FLAT"
	MarginEffective = "EFFECTIVE"
)

// FinancingSimulation is the simulation on screen: both installments in whole
// rupiah and both margins with five decimals ("12.00000"). Ready is false,
// and every value zero, until the four inputs are filled and valid
// (DEC-146).
type FinancingSimulation struct {
	Ready                                 bool
	InstallmentFlat, InstallmentEffective string
	MarginFlat, MarginEffective           string
}

// noSimulation is the simulation before the four inputs are valid.
var noSimulation = FinancingSimulation{InstallmentFlat: "0", InstallmentEffective: "0", MarginFlat: "0.00000", MarginEffective: "0.00000"}

const simulationPrecision = 256

func decimal(s string) *big.Float {
	f, _, err := big.ParseFloat(s, 10, simulationPrecision, big.ToNearestEven)
	if err != nil {
		panic("financing simulation: " + err.Error())
	}
	return f
}

func num() *big.Float { return new(big.Float).SetPrec(simulationPrecision) }

func fromInt(i *big.Int) *big.Float { return num().SetInt(i) }

// roundHalfUp is x rounded to a whole number, halves up.
func roundHalfUp(x *big.Float) *big.Int {
	i, acc := num().Add(x, decimal("0.5")).Int(nil)
	// Int truncates towards zero; below zero that is one too high.
	if acc == big.Above {
		i.Sub(i, big.NewInt(1))
	}
	return i
}

// marginUnits is a margin in units of 0.00001, rounded half up.
func marginUnits(x *big.Float) *big.Int {
	return roundHalfUp(num().Mul(x, decimal("100000")))
}

func marginFromUnits(u *big.Int) *big.Float {
	return num().Quo(fromInt(u), decimal("100000"))
}

// formatMargin writes units of 0.00001 as "12.00000".
func formatMargin(u *big.Int) string {
	s := new(big.Int).Abs(u).String()
	if len(s) < 6 {
		s = strings.Repeat("0", 6-len(s)) + s
	}
	s = s[:len(s)-5] + "." + s[len(s)-5:]
	if u.Sign() < 0 {
		s = "-" + s
	}
	return s
}

func power(x *big.Float, n int) *big.Float {
	out, base := decimal("1"), num().Set(x)
	for ; n > 0; n >>= 1 {
		if n&1 == 1 {
			out.Mul(out, base)
		}
		base.Mul(base, base)
	}
	return out
}

// payment is the annuity installment of P over n months at monthly rate r
// (PMT), unrounded: P/n at r = 0.
func payment(r *big.Float, n int, p *big.Float) *big.Float {
	if r.Sign() == 0 {
		return num().Quo(p, decimal(strconv.Itoa(n)))
	}
	growth := power(num().Add(decimal("1"), r), n)
	top := num().Mul(num().Mul(p, r), growth)
	return top.Quo(top, num().Sub(growth, decimal("1")))
}

// monthlyRate is the monthly percent rate m/100/12.
func monthlyRate(m *big.Float) *big.Float {
	return num().Quo(m, decimal("1200"))
}

// effectiveInstallment is PMT(m/100/12, n, P) rounded.
func effectiveInstallment(m *big.Float, n int, p *big.Float) *big.Int {
	return roundHalfUp(payment(monthlyRate(m), n, p))
}

// flatInstallment is (P + P × m/100/12 × n) / n rounded.
func flatInstallment(m *big.Float, n int, p *big.Float) *big.Int {
	nf := decimal(strconv.Itoa(n))
	margin := num().Mul(num().Mul(p, monthlyRate(m)), nf)
	return roundHalfUp(num().Quo(num().Add(p, margin), nf))
}

// rate is RATE(n, -A, P): the monthly rate whose annuity of n months on P
// is A, found by bisection (the installment grows with the rate).
func rate(n int, a, p *big.Float) *big.Float {
	lo, hi := decimal("-0.99"), decimal("1")
	for payment(hi, n, p).Cmp(a) < 0 {
		hi.Mul(hi, decimal("2"))
	}
	for range 300 {
		mid := num().Quo(num().Add(lo, hi), decimal("2"))
		if payment(mid, n, p).Cmp(a) < 0 {
			lo = mid
		} else {
			hi = mid
		}
	}
	return num().Quo(num().Add(lo, hi), decimal("2"))
}

// simulate computes both sides from the side typed: Effective gives the
// effective installment and Margin Flat = 12 × ((A × n − P) / (P × n)) × 100;
// Flat gives the flat installment and Margin Efektif = RATE × 12 × 100 (0 at
// m = 0). The other installment comes from the rounded other margin
// (FR-APP-018 Perhitungan 1-3). amount, months and margin are valid.
func simulate(amount string, months int, marginType, margin string) FinancingSimulation {
	p, m := decimal(amount), decimal(margin)
	typed := marginUnits(m)
	var flat, effective *big.Int
	out := FinancingSimulation{Ready: true}
	if marginType == MarginEffective {
		a := effectiveInstallment(m, months, p)
		pn := num().Mul(p, decimal(strconv.Itoa(months)))
		extra := num().Sub(num().Mul(fromInt(a), decimal(strconv.Itoa(months))), p)
		flat = marginUnits(num().Mul(num().Quo(extra, pn), decimal("1200")))
		effective = typed
		out.InstallmentEffective = a.String()
		out.InstallmentFlat = flatInstallment(marginFromUnits(flat), months, p).String()
	} else {
		a := flatInstallment(m, months, p)
		flat = typed
		effective = new(big.Int)
		if m.Sign() != 0 {
			effective = marginUnits(num().Mul(rate(months, fromInt(a), p), decimal("1200")))
		}
		out.InstallmentFlat = a.String()
		out.InstallmentEffective = effectiveInstallment(marginFromUnits(effective), months, p).String()
	}
	out.MarginFlat, out.MarginEffective = formatMargin(flat), formatMargin(effective)
	return out
}
