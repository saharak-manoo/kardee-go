package normalize

import (
	"testing"

	"github.com/saharak/kardee-go/app/models"
)

func TestPromotion_SetsFuelCategory(t *testing.T) {
	p := &models.Promotion{Bank: "ttb", Category: "auto", TitleTH: "เติมน้ำมันที่ปั๊มบางจาก"}
	rows := Promotion(testCatalog(t), p)
	if p.Category != "fuel" {
		t.Errorf("category = %s", p.Category)
	}
	if len(rows) != 1 || rows[0].MerchantKey != "bangchak" || rows[0].Match != models.MatchMerchant {
		t.Errorf("rows = %+v", rows)
	}
	if p.Eligibility.Confidence == "" {
		t.Error("eligibility not set")
	}
}
