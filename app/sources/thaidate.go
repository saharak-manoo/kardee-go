package sources

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/saharak/kardee-go/app/config"
)

var thaiMonths = map[string]time.Month{
	"ม.ค.": 1, "ก.พ.": 2, "มี.ค.": 3, "เม.ย.": 4, "พ.ค.": 5, "มิ.ย.": 6,
	"ก.ค.": 7, "ส.ค.": 8, "ก.ย.": 9, "ต.ค.": 10, "พ.ย.": 11, "ธ.ค.": 12,
	"มกราคม": 1, "กุมภาพันธ์": 2, "มีนาคม": 3, "เมษายน": 4, "พฤษภาคม": 5, "มิถุนายน": 6,
	"กรกฎาคม": 7, "สิงหาคม": 8, "กันยายน": 9, "ตุลาคม": 10, "พฤศจิกายน": 11, "ธันวาคม": 12,
}

// One date: day, optional Thai month, optional Buddhist year (2 or 4 digits).
var thaiDateRe = regexp.MustCompile(`(\d{1,2})\s*((?:ม\.ค\.|ก\.พ\.|มี\.ค\.|เม\.ย\.|พ\.ค\.|มิ\.ย\.|ก\.ค\.|ส\.ค\.|ก\.ย\.|ต\.ค\.|พ\.ย\.|ธ\.ค\.|มกราคม|กุมภาพันธ์|มีนาคม|เมษายน|พฤษภาคม|มิถุนายน|กรกฎาคม|สิงหาคม|กันยายน|ตุลาคม|พฤศจิกายน|ธันวาคม))?\s*(\d{4}|\d{2})?`)

// ParseThaiPeriod reads "12 ก.ย. 69 – 31 ธ.ค. 69", "1 - 31 ธ.ค. 2569" and the like.
// The start borrows month and year from the end when it leaves them out. The end is
// the last second of its day, Thai time. Either result may be nil.
func ParseThaiPeriod(s string) (start, end *time.Time) {
	s = strings.NewReplacer("–", "-", "—", "-", "ถึง", "-").Replace(s)
	parts := strings.SplitN(s, "-", 2)
	type d struct {
		day, year int
		month     time.Month
	}
	parse := func(p string) (d, bool) {
		m := thaiDateRe.FindStringSubmatch(p)
		if m == nil {
			return d{}, false
		}
		day, err := strconv.Atoi(m[1])
		if err != nil {
			return d{}, false
		}
		out := d{day: day, month: thaiMonths[m[2]]}
		if m[3] != "" {
			y, err := strconv.Atoi(m[3])
			if err != nil {
				return d{}, false
			}
			if y < 100 {
				y += 2500
			}
			out.year = y - 543
		}
		return out, true
	}
	mk := func(x d, endOfDay bool) *time.Time {
		if x.day < 1 || x.day > 31 || x.month == 0 || x.year < 2000 {
			return nil
		}
		t := time.Date(x.year, x.month, x.day, 0, 0, 0, 0, config.Thai)
		if endOfDay {
			t = t.Add(24*time.Hour - time.Second)
		}
		return &t
	}
	if len(parts) == 1 {
		if x, ok := parse(parts[0]); ok {
			return nil, mk(x, true)
		}
		return nil, nil
	}
	a, okA := parse(parts[0])
	b, okB := parse(parts[1])
	if okB {
		end = mk(b, true)
	}
	if okA {
		if a.month == 0 {
			a.month = b.month
		}
		if a.year == 0 {
			a.year = b.year
			if a.month > b.month { // "15 ธ.ค. - 15 ม.ค. 70"
				a.year--
			}
		}
		start = mk(a, false)
	}
	return start, end
}
