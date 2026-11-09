package normalize

import (
	"slices"
	"testing"

	"github.com/saharak/kardee-go/app/models"
)

func matchKeys(ms []MerchantMatch) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Key
	}
	return out
}

func TestMerchants_Bangchak(t *testing.T) {
	got := ExtractMerchants(testCatalog(t), "เติมน้ำมันสุดคุ้มที่ปั๊มบางจาก")
	if len(got) != 1 || got[0] != (MerchantMatch{"bangchak", models.MatchMerchant}) {
		t.Errorf("got %+v", got)
	}
}

func TestMerchants_GenericFuelStation(t *testing.T) {
	got := ExtractMerchants(testCatalog(t), "เติมน้ำมันครบทุก 700 บาท/สลิป รับเครดิตเงินคืน 5% ที่ปั๊มที่ร่วมรายการ")
	want := []string{"bangchak", "ptt-station", "shell", "caltex", "pt", "susco"}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for _, m := range got {
		if !slices.Contains(want, m.Key) || m.Match != models.MatchCategory {
			t.Errorf("unexpected %+v", m)
		}
	}
}

func TestMerchants_ShortAliasNeedsBoundary(t *testing.T) {
	c := testCatalog(t)
	if k := matchKeys(ExtractMerchants(c, "เติมที่ PTT Station")); !slices.Contains(k, "ptt-station") || slices.Contains(k, "pt") {
		t.Errorf("PTT Station → %v", k)
	}
	if k := matchKeys(ExtractMerchants(c, "รหัส SPTX ใช้ได้")); len(k) != 0 {
		t.Errorf("SPTX → %v", k)
	}
	if k := matchKeys(ExtractMerchants(c, "ปั๊มพีทีที")); !slices.Contains(k, "ptt-station") || slices.Contains(k, "pt") {
		t.Errorf("พีทีที → %v", k)
	}
}

func TestMerchants_Several(t *testing.T) {
	k := matchKeys(ExtractMerchants(testCatalog(t), "11.11 Shopee และ Lazada"))
	if !slices.Equal(k, []string{"shopee", "lazada"}) {
		t.Errorf("got %v", k)
	}
}

func TestExtract_AmountOff(t *testing.T) {
	c := Extract("รับส่วนลดทันที 200 บาท เมื่อใช้จ่ายครบ 2,000 บาท", "")
	if c.AmountOff == nil || *c.AmountOff != 200 {
		t.Errorf("amount off = %v", c.AmountOff)
	}
	if c.MinSpend == nil || *c.MinSpend != 2000 {
		t.Errorf("min = %v", c.MinSpend)
	}
	if c := Extract("เครดิตเงินคืนสูงสุด 500 บาท", ""); c.AmountOff != nil {
		t.Errorf("cap read as amount off: %v", *c.AmountOff)
	}
}
