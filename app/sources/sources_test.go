package sources

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/saharak/kardee-go/app/config"
)

func TestParseThaiPeriod(t *testing.T) {
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, config.Thai) }
	cases := []struct {
		in         string
		start, end time.Time // end is the start of the last day
	}{
		{"12 ก.ย. 69 – 31 ธ.ค. 69", day(2026, 9, 12), day(2026, 12, 31)},
		{"1 ก.ย. 68 - 31 ธ.ค. 69", day(2025, 9, 1), day(2026, 12, 31)},
		{"1 - 31 ธ.ค. 2569", day(2026, 12, 1), day(2026, 12, 31)},
		{"15 ธ.ค. - 15 ม.ค. 70", day(2026, 12, 15), day(2027, 1, 15)},
	}
	for _, c := range cases {
		s, e := ParseThaiPeriod(c.in)
		if s == nil || e == nil || !s.Equal(c.start) || !e.Equal(c.end.Add(24*time.Hour-time.Second)) {
			t.Errorf("%q -> %v, %v", c.in, s, e)
		}
	}
	if s, e := ParseThaiPeriod("ตลอดปี"); s != nil || e != nil {
		t.Error("no date in text gives nil")
	}
}

func TestMapCategory(t *testing.T) {
	for in, want := range map[string]string{
		"dining-restaurants": "dining", "air-ticket-hotels-travel": "travel", "zero-pay-plan": "installment",
		"household": "home", "apply-credit-card-loan-offers": "new-card", "something": "other",
	} {
		if got := mapCategory(in); got != want {
			t.Errorf("%s -> %s, want %s", in, got, want)
		}
	}
}

func TestKTCListingsFrom_PairsLanguagesAndDropsOld(t *testing.T) {
	now := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	set := urlSet{}
	add := func(loc, mod string) {
		set.URLs = append(set.URLs, struct {
			Loc     string `xml:"loc"`
			LastMod string `xml:"lastmod"`
		}{loc, mod})
	}
	add("https://www.ktc.co.th/promotion/air-ticket-hotels-travel/air-tickets/emirates", "2026-11-08T10:00:00Z")
	add("https://www.ktc.co.th/en/promotion/air-ticket-hotels-travel/air-tickets/emirates", "2026-11-08T11:00:00Z")
	add("https://www.ktc.co.th/promotion/dining-restaurants/restaurants/khao", "2025-01-01T00:00:00Z") // old
	add("https://www.ktc.co.th/promotion/dining-restaurants", "2026-11-08T00:00:00Z")                  // category page
	k := &KTC{MaxAge: 180 * 24 * time.Hour}
	got := k.listingsFrom([]urlSet{set}, now)
	if len(got) != 1 {
		t.Fatalf("got %d listings: %+v", len(got), got)
	}
	l := got[0]
	if l.ExternalID != "/promotion/air-ticket-hotels-travel/air-tickets/emirates" || l.Category != "travel" ||
		!strings.HasSuffix(l.URLEN, "/en/promotion/air-ticket-hotels-travel/air-tickets/emirates") || l.Version != "2026-11-08T11:00:00Z" {
		t.Fatalf("%+v", l)
	}
}

func TestParseKTCPage(t *testing.T) {
	rsc := `x"containRegister":true,"code":"$undefined","detail":{"id":10721,"title":"Emirates","startDate":"2026-11-08T17:00:00.000Z","endDate":"2027-02-08T16:59:59.000Z","meta":{"title":"โปรโมชั่น Emirates","description":"รับส่วนลดสูงสุด 4,900 บาท"},` +
		`"children":[{"text":"รับส่วนลดสูงสุด 4,900 บาท"},{"text":"ใส่โค้ด \"THKTC25\""},{"text":"This site is protected by reCAPTCHA"},{"text":"footer"}]}`
	enc, _ := json.Marshal(rsc)
	page := `<script>self.__next_f.push([1,` + string(enc) + `])</script>`
	p, err := parseKTCPage(page)
	if err != nil {
		t.Fatal(err)
	}
	if p.title != "โปรโมชั่น Emirates" || p.desc != "รับส่วนลดสูงสุด 4,900 บาท" || p.start == nil || p.end == nil || p.register == nil || !*p.register {
		t.Fatalf("%+v", p)
	}
	if p.text != "รับส่วนลดสูงสุด 4,900 บาท\nใส่โค้ด \"THKTC25\"" {
		t.Fatalf("text = %q", p.text)
	}
}

func TestParseTTBDetail_BothLanguages(t *testing.T) {
	data := `{"props":{"pageProps":{"content":{"title_th":"ไทย","title_en":"English","detail_th":"<p>รับเงินคืน 5%</p><p>ครบ 1,000 บาท</p>","detail_en":"<p>5% cashback</p>","start_date":"2026-09-09T19:35:00+07:00","end_date":"2026-12-31T19:35:00+07:00"}}}}`
	d, err := parseTTBDetail([]byte(`<script id="__NEXT_DATA__" type="application/json">` + data + `</script>`))
	if err != nil {
		t.Fatal(err)
	}
	if d.TitleEN != "English" || d.TextTH != "รับเงินคืน 5%\nครบ 1,000 บาท" || d.TextEN != "5% cashback" || d.StartAt == nil || d.EndAt == nil {
		t.Fatalf("%+v", d)
	}
}

func TestTTBListing_EnglishURLOnlyWhenTranslated(t *testing.T) {
	l := ttbListing(ttbItem{ID: 7, TitleTH: "ไทย", PeriodTH: "1 พ.ย. 69 - 30 พ.ย. 69", CanonicalURL: "promotion/credit-card/dining/x"}, "dining")
	if l.URLEN != "" || l.URLTH != "https://www.ttbbank.com/th/promotion/credit-card/dining/x" || l.EndAt == nil || l.Category != "dining" {
		t.Fatalf("%+v", l)
	}
}

func TestParseKrungsriList(t *testing.T) {
	page := `<script>var promotionData = [` +
		`{"NodeId":"4222","HiddenPromotion":"False","Title":"ช้อปของแต่งบ้าน","OG_Description":"รับเงินคืน","TypePromotion":"household|shopping","CardType":"บัตร A|บัตร B","Text_datetime":"1 ต.ค. 69 – 31 ธ.ค. 69","DocumentPublishTo":"2026-12-31T23:59:59","DocumentModifiedWhen":"2026-10-01T10:00:00","AbsoluteUrl":"https://www.krungsricard.com/th/promotion/household-all","BannerPC":"/getattachment/x.webp"},` +
		`{"NodeId":"1","HiddenPromotion":"True","Title":"hidden","AbsoluteUrl":"https://x"}` +
		`];var other = 1;</script>`
	got, err := parseKrungsriList(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d, hidden ones are skipped", len(got))
	}
	l := got[0]
	if l.ExternalID != "4222" || l.Category != "home" || len(l.Cards) != 2 || l.StartAt == nil || l.EndAt == nil || l.Version == "" ||
		l.ImageURL != "https://www.krungsricard.com/getattachment/x.webp" {
		t.Fatalf("%+v", l)
	}
}

func TestKrungsriText_CutsHeaderAndFooter(t *testing.T) {
	page := `<div>เมนู</div><div>โปรโมชัน</div><h1>ช้อปของแต่งบ้าน รับเงินคืน*</h1><p>ครบ 3,000 บาท</p><p>1 ต.ค. 69 – 31 ธ.ค. 69</p><div>บัตรเครดิตที่ร่วมรายการ</div><div>เกี่ยวกับเรา</div>`
	got := krungsriText(page, "ช้อปของแต่งบ้าน รับเงินคืน*")
	if got != "ช้อปของแต่งบ้าน รับเงินคืน*\nครบ 3,000 บาท\n1 ต.ค. 69 – 31 ธ.ค. 69" {
		t.Fatalf("%q", got)
	}
}

func TestHTMLText(t *testing.T) {
	got := HTMLText("<script>x()</script><p>a&amp;b</p><br><li>  c&nbsp;d </li>")
	if got != "a&b\nc d" {
		t.Fatalf("%q", got)
	}
}

// Newer ttb pages leave detail_th empty and put the text in nested payload components.
func TestParseTTBDetail_PayloadComponents(t *testing.T) {
	data := `{"props":{"pageProps":{"content":{"title_th":"ไทย","detail_th":"","payload":{
		"components":[{"name":"banner","data":{},"components":[
			{"data":{"title":{"th":"ช้อปที่ Power Buy","en":"Shop at Power Buy"}},"components":[
				{"data":{"content":{"th":"<div>แบ่งชำระ 0%</div><ul><li>นาน 10 เดือน</li></ul>","en":"<p>0% for 10 months</p>"}}}
			]}
		]}],
		"payload":{"components":[{"data":{"content":{"th":"<p>ซ้ำ</p>"}}}]}
	}}}}}`
	d, err := parseTTBDetail([]byte(`<script id="__NEXT_DATA__" type="application/json">` + data + `</script>`))
	if err != nil {
		t.Fatal(err)
	}
	if d.TextTH != "ช้อปที่ Power Buy\nแบ่งชำระ 0%\nนาน 10 เดือน" {
		t.Errorf("th = %q", d.TextTH)
	}
	if d.TextEN != "Shop at Power Buy\n0% for 10 months" {
		t.Errorf("en = %q", d.TextEN)
	}
}
