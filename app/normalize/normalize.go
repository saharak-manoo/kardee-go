// Package normalize pulls the numbers people care about out of a promotion's text
// (benefit type, percent, minimum spend, cap, installment months, registration, code)
// and writes a one-line summary in Thai and English from them.
//
// It is rule based (regular expressions over Thai and English wording). Promotions
// with several tiers are reduced to the headline: highest percent, highest cap,
// lowest minimum spend. Anything not found stays nil and is left out of the summary.
package normalize

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/saharak/kardee-go/app/config"
	"github.com/saharak/kardee-go/app/models"
)

// Conditions are the extracted values.
type Conditions struct {
	BenefitType   string
	Percent       *float64
	PercentTiers  bool // more than one percent in the text: the summary says "up to"
	MinSpend      *int
	MaxBenefit    *int
	AmountOff     *int // fixed baht off
	InstallMonths *int
	NeedsRegister bool
	PromoCode     string
}

const num = `(\d{1,3}(?:,\d{3})+|\d+)`

var (
	percentRe    = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)
	percentAfter = regexp.MustCompile(`(?i)^[\s\x{00a0}]*(ต่อปี|per annum|p\.a\.|ต่อเดือน|interest|ดอกเบี้ย)`)
	maxRe        = []*regexp.Regexp{
		regexp.MustCompile(`สูงสุด(?:รวม)?\s*(?:ถึง)?\s*` + num + `\s*บาท`),
		regexp.MustCompile(`(?i)(?:up to|maximum(?: of)?|max\.?)\s*(?:THB|฿)?\s*` + num + `\s*(?:THB|baht|฿)?`),
	}
	minRe = []*regexp.Regexp{
		regexp.MustCompile(`(?:ครบทุก|ครบ|ขั้นต่ำ|ตั้งแต่|ทุกยอด(?:ใช้จ่าย)?|ทุกๆ)\s*` + num + `\s*บาท`),
		regexp.MustCompile(`(?i)(?:minimum(?: spend(?:ing)?)?(?: of)?|spend(?:ing)?(?: of)?|every)\s*(?:THB|฿)?\s*` + num + `\s*(?:THB|baht|฿)?\s*(?:or more|and above)?`),
	}
	amountOffRe = []*regexp.Regexp{
		regexp.MustCompile(`ลด(?:ทันที)?\s*` + num + `\s*บาท`),
		regexp.MustCompile(`(?i)(?:THB|฿)\s*` + num + `\s*off\b`),
		regexp.MustCompile(`(?i)` + num + `\s*(?:THB|baht)\s*off\b`),
	}
	perMonthRe = regexp.MustCompile(`^\s*(?:/\s*เดือน|ต่อเดือน|per month|/\s*month)`)
	// Spending points for a discount or cashback ("แลก 999 คะแนน รับส่วนลด 210 บาท").
	redeemRe  = regexp.MustCompile(`(?i)แลก\s*(?:รับ)?\s*(?:[\d,]+\s*)?(?:คะแนน|พอยต์|point)|(?:คะแนน|พอยต์)[^\n]{0,40}แลก|redeem`)
	noMinRe   = regexp.MustCompile(`(?i)ไม่มียอด(?:ใช้จ่าย)?ขั้นต่ำ|no minimum spend`)
	installRe = []*regexp.Regexp{
		regexp.MustCompile(`ผ่อน\s*0\s*%[^\d]{0,40}?(\d{1,2})\s*(?:เดือน|งวด)`),
		regexp.MustCompile(`(?i)0\s*%\s*instal{1,2}ment\D{0,40}?(\d{1,2})\s*months?`), //nolint:misspell // matches "installment" and British "instalment"
	}
	registerRe   = regexp.MustCompile(`(?i)ลงทะเบียน|register`)
	noRegisterRe = regexp.MustCompile(`(?i)ไม่ต้องลงทะเบียน|no registration|without registration`)
	codeRe       = regexp.MustCompile(`(?i)(?:promo\s*code|โค้ด(?:ส่วนลด)?|code)\s*(?:\))?\s*[:：]?\s*([A-Z0-9]{4,20})\b`)
)

var benefitWords = []struct {
	key   string
	words []string
}{
	{models.BenefitCashback, []string{"เครดิตเงินคืน", "เงินคืน", "cashback", "cash back"}},
	{models.BenefitInstallment, []string{"ผ่อน 0%", "ผ่อน0%", "0% install", "0% instal"}}, //nolint:misspell // "0% instal" also catches British "instalment"
	{models.BenefitDiscount, []string{"ส่วนลด", "ลดทันที", "ลด ", "discount", "% off"}},
	{models.BenefitPoints, []string{"คะแนน", "พอยต์", "point", "แลก"}},
	{models.BenefitGift, []string{"รับฟรี", "ของสมนาคุณ", "ของรางวัล", "ลุ้น", "free", "gift"}},
}

// Extract reads the conditions. headline is the title and short description (used first
// for the benefit type); body is the full conditions text.
func Extract(headline, body string) Conditions {
	all := headline + "\n" + body
	c := Conditions{BenefitType: benefitType(headline)}
	if redeemRe.MatchString(redeemScope(headline, body)) {
		c.BenefitType = models.BenefitPoints
	}
	if c.BenefitType == models.BenefitOther {
		c.BenefitType = benefitType(body)
	}

	seen := map[float64]bool{}
	for _, m := range percentRe.FindAllStringSubmatchIndex(all, -1) {
		v, err := strconv.ParseFloat(all[m[2]:m[3]], 64)
		if err != nil || v <= 0 || v > 100 || percentAfter.MatchString(all[m[1]:]) {
			continue
		}
		// "ผ่อน 0%" and "ดอกเบี้ย 16% ต่อปี" (the legal warning on every page) are not benefits.
		before := strings.ToLower(all[max(0, m[0]-40):m[0]])
		if strings.Contains(before, "ผ่อน") || strings.Contains(before, "installment") ||
			strings.Contains(before, "ดอกเบี้ย") || strings.Contains(before, "interest") {
			continue
		}
		seen[v] = true
		if c.Percent == nil || v > *c.Percent {
			c.Percent = &v
		}
	}
	c.PercentTiers = len(seen) > 1

	c.MaxBenefit = pick(all, maxRe, true)
	c.AmountOff = amountOff(all)
	if noMinRe.MatchString(all) {
		zero := 0
		c.MinSpend = &zero
	} else {
		c.MinSpend = pick(all, minRe, false)
	}
	if n := pick(all, installRe, true); n != nil && *n > 0 && *n <= 48 {
		c.InstallMonths = n
	}
	c.NeedsRegister = registerRe.MatchString(all) && !noRegisterRe.MatchString(all)
	// Codes are written in capitals ("THKTC25"); a lowercase match is just a word after "code".
	if m := codeRe.FindStringSubmatch(all); len(m) > 1 && strings.ToUpper(m[1]) == m[1] {
		c.PromoCode = m[1]
	}
	return c
}

func benefitType(s string) string {
	low := strings.ToLower(s)
	for _, b := range benefitWords {
		for _, w := range b.words {
			if strings.Contains(low, w) {
				return b.key
			}
		}
	}
	return models.BenefitOther
}

// pick returns the highest (or lowest) number matched by any of res.
func pick(s string, res []*regexp.Regexp, highest bool) *int {
	var out *int
	for _, re := range res {
		for _, m := range re.FindAllStringSubmatch(s, -1) {
			n, err := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
			if err != nil || n <= 0 {
				continue
			}
			if out == nil || highest && n > *out || !highest && n < *out {
				v := n
				out = &v
			}
		}
	}
	return out
}

// Apply copies the conditions onto p and rebuilds both summaries.
func Apply(p *models.Promotion, c Conditions) {
	p.BenefitType = c.BenefitType
	p.Percent = c.Percent
	p.MinSpend = c.MinSpend
	p.MaxBenefit = c.MaxBenefit
	p.AmountOff = c.AmountOff
	p.InstallMonths = c.InstallMonths
	p.NeedsRegister = c.NeedsRegister
	p.PromoCode = c.PromoCode
	p.SummaryTH = SummaryTH(p, c.PercentTiers)
	p.SummaryEN = SummaryEN(p, c.PercentTiers)
}

var thaiMonthShort = []string{"", "ม.ค.", "ก.พ.", "มี.ค.", "เม.ย.", "พ.ค.", "มิ.ย.", "ก.ค.", "ส.ค.", "ก.ย.", "ต.ค.", "พ.ย.", "ธ.ค."}

func thaiDate(t time.Time) string {
	t = t.In(config.Thai)
	return fmt.Sprintf("%d %s %02d", t.Day(), thaiMonthShort[t.Month()], (t.Year()+543)%100)
}

func baht(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func pct(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) + "%" }

// SummaryTH: "เครดิตเงินคืน 10% เมื่อใช้จ่ายครบ 3,000 บาท สูงสุด 500 บาท · ต้องลงทะเบียน · ถึง 31 ธ.ค. 69"
func SummaryTH(p *models.Promotion, tiers bool) string {
	var parts []string
	head := models.BenefitTypes[p.BenefitType].TH
	if p.BenefitType == models.BenefitInstallment && p.InstallMonths != nil {
		head = fmt.Sprintf("ผ่อน 0%% สูงสุด %d เดือน", *p.InstallMonths)
	} else if p.Percent != nil {
		if tiers {
			head += " สูงสุด " + pct(*p.Percent)
		} else {
			head += " " + pct(*p.Percent)
		}
	}
	if p.MinSpend != nil {
		if *p.MinSpend == 0 {
			head += " ไม่มียอดขั้นต่ำ"
		} else {
			head += " เมื่อใช้จ่ายครบ " + baht(*p.MinSpend) + " บาท"
		}
	}
	if p.MaxBenefit != nil {
		head += " รับสูงสุด " + baht(*p.MaxBenefit) + " บาท"
	}
	if p.BenefitType == models.BenefitOther && p.Percent == nil && p.MinSpend == nil && p.MaxBenefit == nil {
		head = firstLine(p.DescriptionTH, p.TitleTH)
	}
	parts = append(parts, head)
	if p.InstallMonths != nil && p.BenefitType != models.BenefitInstallment {
		parts = append(parts, fmt.Sprintf("ผ่อน 0%% %d เดือน", *p.InstallMonths))
	}
	if p.NeedsRegister {
		parts = append(parts, "ต้องลงทะเบียน")
	}
	if p.PromoCode != "" {
		parts = append(parts, "โค้ด "+p.PromoCode)
	}
	if p.EndAt != nil {
		parts = append(parts, "ถึง "+thaiDate(*p.EndAt))
	}
	return strings.Join(parts, " · ")
}

// SummaryEN: "10% cashback on spend of 3,000 THB, up to 500 THB · registration required · until 31 Dec 2026"
func SummaryEN(p *models.Promotion, tiers bool) string {
	var parts []string
	label := strings.ToLower(models.BenefitTypes[p.BenefitType].EN)
	head := models.BenefitTypes[p.BenefitType].EN
	if p.BenefitType == models.BenefitInstallment && p.InstallMonths != nil {
		head = fmt.Sprintf("0%% installment up to %d months", *p.InstallMonths)
	} else if p.Percent != nil {
		if tiers {
			head = "Up to " + pct(*p.Percent) + " " + label
		} else {
			head = pct(*p.Percent) + " " + label
		}
	}
	if p.MinSpend != nil {
		if *p.MinSpend == 0 {
			head += ", no minimum spend"
		} else {
			head += " on spend of " + baht(*p.MinSpend) + " THB or more"
		}
	}
	if p.MaxBenefit != nil {
		head += ", up to " + baht(*p.MaxBenefit) + " THB"
	}
	if p.BenefitType == models.BenefitOther && p.Percent == nil && p.MinSpend == nil && p.MaxBenefit == nil {
		head = firstLine(p.DescriptionEN, p.TitleEN)
		if head == "" {
			head = "See details"
		}
	}
	parts = append(parts, head)
	if p.InstallMonths != nil && p.BenefitType != models.BenefitInstallment {
		parts = append(parts, fmt.Sprintf("0%% installment %d months", *p.InstallMonths))
	}
	if p.NeedsRegister {
		parts = append(parts, "registration required")
	}
	if p.PromoCode != "" {
		parts = append(parts, "code "+p.PromoCode)
	}
	if p.EndAt != nil {
		parts = append(parts, "until "+p.EndAt.In(config.Thai).Format("2 Jan 2006"))
	}
	return strings.Join(parts, " · ")
}

func firstLine(options ...string) string {
	for _, s := range options {
		s = strings.TrimSpace(strings.SplitN(s, "\n", 2)[0])
		if s != "" {
			if r := []rune(s); len(r) > 140 {
				s = string(r[:140]) + "…"
			}
			return s
		}
	}
	return ""
}

// amountOff is the highest fixed baht discount ("ลดทันที 200 บาท", "THB 200 off"),
// leaving out monthly amounts.
func amountOff(s string) *int {
	var out *int
	for _, re := range amountOffRe {
		for _, m := range re.FindAllStringSubmatchIndex(s, -1) {
			if perMonthRe.MatchString(s[m[1]:]) {
				continue
			}
			n, err := strconv.Atoi(strings.ReplaceAll(s[m[2]:m[3]], ",", ""))
			if err != nil || n <= 0 {
				continue
			}
			if out == nil || n > *out {
				v := n
				out = &v
			}
		}
	}
	return out
}

// redeemScope is where a points redemption is announced: the headline, or the start of
// the conditions when the headline is empty.
func redeemScope(headline, body string) string {
	if strings.TrimSpace(headline) != "" {
		return headline
	}
	r := []rune(body)
	return string(r[:min(len(r), 300)])
}
