package normalize

import (
	"strings"
	"testing"
	"time"

	"github.com/saharak/kardee-go/app/config"
	"github.com/saharak/kardee-go/app/models"
)

func TestExtract_CashbackWithTiers(t *testing.T) {
	c := Extract("ช้อปของแต่งบ้าน รับเครดิตเงินคืนรวมสูงสุด 78,700 บาท*",
		"ใช้จ่ายครบ 3,000 บาท รับเครดิตเงินคืน 5%\nใช้จ่ายครบ 30,000 บาท รับเครดิตเงินคืน 10% สูงสุด 3,000 บาท\nลงทะเบียนผ่าน UCHOOSE\nไม่ชำระเต็มจำนวนเสียดอกเบี้ย 16% ต่อปี")
	if c.BenefitType != models.BenefitCashback {
		t.Errorf("type = %q", c.BenefitType)
	}
	if c.Percent == nil || *c.Percent != 10 || !c.PercentTiers {
		t.Errorf("percent = %v tiers=%v, want 10 with tiers (16%% per year ignored)", c.Percent, c.PercentTiers)
	}
	if c.MinSpend == nil || *c.MinSpend != 3000 {
		t.Errorf("min = %v, want lowest tier 3000", c.MinSpend)
	}
	if c.MaxBenefit == nil || *c.MaxBenefit != 78700 {
		t.Errorf("max = %v", c.MaxBenefit)
	}
	if !c.NeedsRegister {
		t.Error("needs register")
	}
}

func TestExtract_InterestWarningIsNotABenefit(t *testing.T) {
	c := Extract("โปรโมชั่นที่ Sukishi", "รับคะแนน 2 เท่า\nบัตรเครดิต : ใช้เท่าที่จำเป็นและชำระคืนได้เต็มจำนวนตามกำหนด จะได้ไม่เสียดอกเบี้ย 16%\u00a0ต่อปี\nUse when necessary to avoid a 16% interest rate per annum.")
	if c.Percent != nil {
		t.Fatalf("percent = %v, the interest warning is not a benefit", *c.Percent)
	}
}

func TestExtract_InstallmentIsNotAPercentBenefit(t *testing.T) {
	c := Extract("ใหม่ iPhone Duo ผ่อน 0% สูงสุด 10 เดือน", "")
	if c.BenefitType != models.BenefitInstallment {
		t.Errorf("type = %q", c.BenefitType)
	}
	if c.Percent != nil {
		t.Errorf("percent = %v, the 0%% of an installment is not a benefit", *c.Percent)
	}
	if c.InstallMonths == nil || *c.InstallMonths != 10 {
		t.Errorf("months = %v", c.InstallMonths)
	}
}

func TestExtract_DiscountCodeAndNoMinimum(t *testing.T) {
	c := Extract("โปรโมชั่น Emirates รับส่วนลดสูงสุด 4,900 บาท กับบัตรเครดิต KTC",
		"เพียงใส่โค้ดส่วนลด (Promo Code) : THKTC25 เมื่อจองผ่านเว็บไซต์\nไม่มียอดใช้จ่ายขั้นต่ำ\nไม่ต้องลงทะเบียน")
	if c.BenefitType != models.BenefitDiscount || c.PromoCode != "THKTC25" {
		t.Errorf("type=%q code=%q", c.BenefitType, c.PromoCode)
	}
	if c.MinSpend == nil || *c.MinSpend != 0 {
		t.Errorf("min = %v, want 0", c.MinSpend)
	}
	if c.NeedsRegister {
		t.Error("ไม่ต้องลงทะเบียน means no registration")
	}
}

func TestExtract_English(t *testing.T) {
	c := Extract("Get 15% cashback up to THB 1,500", "Minimum spend of THB 2,000 per sales slip. Register via app.")
	if c.BenefitType != models.BenefitCashback || c.Percent == nil || *c.Percent != 15 {
		t.Fatalf("%+v", c)
	}
	if c.MinSpend == nil || *c.MinSpend != 2000 || c.MaxBenefit == nil || *c.MaxBenefit != 1500 || !c.NeedsRegister {
		t.Fatalf("min=%v max=%v reg=%v", c.MinSpend, c.MaxBenefit, c.NeedsRegister)
	}
}

func TestSummaries_BothLanguages(t *testing.T) {
	end := time.Date(2026, 12, 31, 23, 59, 59, 0, config.Thai)
	p := &models.Promotion{TitleTH: "x", EndAt: &end}
	Apply(p, Conditions{
		BenefitType: models.BenefitCashback, Percent: ptr(10.0), MinSpend: ptr(3000), MaxBenefit: ptr(500), NeedsRegister: true,
	})
	if want := "เครดิตเงินคืน 10% เมื่อใช้จ่ายครบ 3,000 บาท รับสูงสุด 500 บาท · ต้องลงทะเบียน · ถึง 31 ธ.ค. 69"; p.SummaryTH != want {
		t.Errorf("TH\n got %q\nwant %q", p.SummaryTH, want)
	}
	if want := "10% cashback on spend of 3,000 THB or more, up to 500 THB · registration required · until 31 Dec 2026"; p.SummaryEN != want {
		t.Errorf("EN\n got %q\nwant %q", p.SummaryEN, want)
	}
}

func TestSummary_NothingFoundUsesDescription(t *testing.T) {
	p := &models.Promotion{TitleTH: "ลุ้นรับบัตรคอนเสิร์ต", DescriptionTH: "ร่วมสนุก ลุ้นรับบัตรคอนเสิร์ต"}
	Apply(p, Conditions{BenefitType: models.BenefitOther})
	if !strings.HasPrefix(p.SummaryTH, "ร่วมสนุก") || p.SummaryEN != "See details" {
		t.Errorf("th=%q en=%q", p.SummaryTH, p.SummaryEN)
	}
}

func ptr[T any](v T) *T { return &v }

// Redeeming points is not free money: these must not rank as cashback or discounts.
func TestExtract_PointsRedemptionIsPoints(t *testing.T) {
	for _, s := range []string{
		"แลก 999 คะแนน KTC FOREVER รับส่วนลดบุฟเฟ่ต์ 210 บาท/ท่าน",
		"แลกคะแนนรับเครดิตเงินคืน 100 บาท เมื่อใช้ 800 คะแนน KTC FOREVER",
		"แลกพอยต์ลดบุฟเฟต์ 120 บาท* ที่ร้าน Kouen Premium Buffet",
		"399 คะแนน KTC FOREVER แลกรับ e-Coupon ส่วนลด มูลค่า 50 บาท",
	} {
		if c := Extract(s, ""); c.BenefitType != models.BenefitPoints {
			t.Errorf("%q → %s", s, c.BenefitType)
		}
	}
	if c := Extract("รับเครดิตเงินคืน 5% เมื่อใช้จ่ายครบ 3,000 บาท พร้อมรับคะแนนสะสม", ""); c.BenefitType != models.BenefitCashback {
		t.Errorf("earning points is still cashback, got %s", c.BenefitType)
	}
}

func TestExtract_MinSpendEvery(t *testing.T) {
	c := Extract("เติมน้ำมันครบทุก 700 บาท/สลิป รับเครดิตเงินคืนรวมสูงสุด 5%", "")
	if c.MinSpend == nil || *c.MinSpend != 700 {
		t.Errorf("min = %v", c.MinSpend)
	}
}
