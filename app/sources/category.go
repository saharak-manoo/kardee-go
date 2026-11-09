package sources

import "strings"

// categoryRules maps words seen in a bank's category slug or name to our keys.
// First match wins, so more specific words come first.
var categoryRules = []struct {
	key   string
	words []string
}{
	{"installment", []string{"installment", "zero-pay", "0%", "ผ่อน"}},
	{"new-card", []string{"apply", "new-card", "สมัคร"}},
	{"points", []string{"point", "redeem", "forever", "reward", "คะแนน", "พอยต์"}},
	{"dining", []string{"dining", "restaurant", "food", "ร้านอาหาร"}},
	{"travel", []string{"travel", "hotel", "air", "flight", "trip", "japan", "ท่องเที่ยว", "โรงแรม"}},
	{"online", []string{"online", "e-commerce", "shopee", "lazada", "ออนไลน์"}},
	{"auto", []string{"auto", "car", "fuel", "gas", "oil", "รถ", "น้ำมัน"}},
	{"health", []string{"health", "beauty", "hospital", "wellness", "medical", "fitness", "สุขภาพ", "ความงาม"}},
	{"home", []string{"home", "household", "furniture", "บ้าน"}},
	{"entertain", []string{"entertain", "movie", "concert", "lifestyle", "บันเทิง"}},
	{"shopping", []string{"shopping", "department", "fashion", "everyday", "supermarket", "ช้อป"}},
}

// mapCategory turns a bank's category text into our key ("other" when nothing fits).
func mapCategory(s string) string {
	s = strings.ToLower(s)
	for _, r := range categoryRules {
		for _, w := range r.words {
			if strings.Contains(s, w) {
				return r.key
			}
		}
	}
	return "other"
}
