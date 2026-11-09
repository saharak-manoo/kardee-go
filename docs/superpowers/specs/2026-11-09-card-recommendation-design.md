# แนะนำโปรคุ้มสุดตามร้านค้าและบัตรที่ถือ — Design

วันที่: 2026-11-09 · สถานะ: รอ review

## 1. ทำไมต้องมี

ผู้ใช้ยืนอยู่หน้าร้าน (เช่น ปั๊มบางจาก) ถือบัตรอยู่หลายใบ (เช่น KBank The Passion, ttb so smart)
อยากรู้ทันทีว่า **ร้านนี้มีโปรอะไร ใบไหนคุ้มสุด** และถ้ามีบัตรที่ยังไม่มีแต่คุ้มกว่ามาก ก็อยากรู้ว่า
**ควรสมัครใบไหน**

ปัญหาคือโปรส่วนใหญ่ไม่ได้ใช้ได้ทุกใบ บางโปรเฉพาะบางใบ บางโปรเฉพาะเครือข่าย (JCB/Visa)
บางโปรยกเว้นบางใบ และโปรของเครือข่ายเอง (Visa/JCB) ใช้ได้ข้ามธนาคาร

## 2. เป้าหมาย / ไม่ทำ / ห้ามเกิด

**เป้าหมาย**
- API: ระบุร้านค้า → ได้โปรเรียงตามความคุ้ม (บาท)
- ระบุบัตรที่ถือเพิ่ม → ได้โปรที่ใช้ได้กับบัตรตัวเอง เรียงตามความคุ้ม พร้อมบอกว่าให้รูดใบไหน
- แนะนำบัตรที่ควรสมัคร ถ้ามีใบที่คุ้มกว่าใบที่ถืออย่างชัดเจน
- ระบบเครือข่ายกลาง: โปรของ Visa/JCB ใช้ได้กับบัตรทุกธนาคารที่ตรงเครือข่ายและระดับ
- ตารางบัตร (catalog) กลางที่ใช้ทั้งจับคู่ข้อความและให้ผู้ใช้เลือกบัตร

**ไม่ทำในรอบนี้**
- KBank และ Mastercard: เว็บมีระบบกันบอท (Akamai challenge / 403) — ไม่เลี่ยง ไม่ดึง
  บัตร KBank ยังอยู่ใน catalog เพื่อให้ได้โปรเครือข่าย (Visa/JCB) ตามระดับบัตร
- หน้าแอดมินกรอกโปรเอง (กสิกร ฯลฯ) — ออกแบบตารางให้รองรับไว้ แต่ยังไม่ทำ UI/API เขียน
- การรวมสิทธิ์ซ้อนกัน (โปรธนาคาร + โปรเครือข่ายในการรูดครั้งเดียว) — แสดงแยกกัน ไม่บวกรวม
- บัญชีผู้ใช้ / เก็บบัตรของผู้ใช้ฝั่ง server — client ส่งรายการบัตรมากับ request
- ค่าธรรมเนียมรายปี / เงื่อนไขรายได้ในการสมัครบัตร

**ห้ามเกิด (ถือว่าล้มเหลวแม้ระบบทำงาน)**
- แนะนำให้รูดบัตรที่ใช้โปรนั้นไม่ได้จริง — การ "แนะนำผิดใบ" แย่กว่า "ไม่เจอโปร"
  ถ้าอ่านเงื่อนไขบัตรไม่แน่ใจ ต้องติดธง `confidence: "low"` และบอกให้ตรวจเงื่อนไข ไม่ใช่เดา
- แต่งมูลค่าเป็นบาทขึ้นมาเองเมื่ออ่านจากเงื่อนไขไม่ได้
- แสดงโปรหมดอายุ

## 3. สถานะปัจจุบัน (ข้อมูลจริง ณ 2026-11-09)

- แหล่ง: KTC 2,754 / ttb 349 / กรุงศรี 559 โปร; KBank ปิด (bot wall)
- เงื่อนไขบัตร: มีแค่กรุงศรี (ฟิลด์ `CardType` จากเว็บ, 554/559) — KTC/ttb ไม่มีฟิลด์บัตร
  (ตรวจ `__NEXT_DATA__` ttb และ RSC ของ KTC แล้ว) ต้องอ่านจากข้อความ
- รูปแบบข้อความ: ttb ส่วนใหญ่ "บัตรเครดิต ttb" (= ทุกใบ), ระบุใบเฉพาะแทบมีแค่ ttb Disney;
  KTC มักเป็น "บัตร KTC ทุกประเภท ยกเว้น KTC CASH BACK, ROYAL ORCHID PLUS…" หรือ "KTC JCB ทุกประเภท"
- หน้าเงื่อนไขถูกอ่านแล้วน้อยมาก (KTC 45, ttb 3) — กำลังรัน backlog crawl ให้ครบ
- มูลค่า: 3,116/3,662 โปร `benefit_type = other`, มี `percent` ~300 โปร → ตัวดึงเงื่อนไขใน
  `normalize` ต้องเก่งขึ้นถึงจะจัดอันดับได้จริง (ดูข้อ 7)
- ร้านค้า: ไม่มีฟิลด์ร้านค้า มีแค่หมวด (`auto` = รถยนต์และน้ำมัน รวมกัน)
- แหล่งเครือข่าย: Visa (visa.co.th) robots อนุญาต ไม่มีด่าน, หน้าโปรแยกใน sitemap;
  JCB (specialoffers.jcb) เปิดได้ มีหมวดไทย; Mastercard 403 → ข้าม

## 4. ข้อมูล (ตารางใหม่)

ใช้ GORM AutoMigrate เหมือนเดิม (เพิ่มเท่านั้น ไม่เปลี่ยน type)

### `networks` — seed จากโค้ด
| field | หมายเหตุ |
|---|---|
| key | `visa`, `mastercard`, `jcb`, `unionpay`, `amex` |
| name_th / name_en | |
| tiers | ลำดับจากต่ำไปสูง เช่น visa: classic, gold, platinum, signature, infinite; mastercard: standard, gold, titanium, platinum, world, world-elite; jcb: standard, gold, platinum, ultimate |

ระดับเทียบกันได้ภายในเครือข่ายเดียวกันเท่านั้น ("Platinum ขึ้นไป" = index ≥ platinum)

### `cards` — seed จากไฟล์ `app/catalog/cards.yaml` (commit ใน repo, review ได้)
| field | หมายเหตุ |
|---|---|
| key | เช่น `ttb-so-smart` (ใช้ใน API) |
| bank | `ktc`, `ttb`, `krungsri`, `kbank`, … (ธนาคารที่ไม่มีแหล่งโปรก็ใส่ได้) |
| name_th / name_en | |
| aliases | ชื่อที่ใช้จับคู่กับข้อความ (ไทย/อังกฤษ/ตามที่ธนาคารเขียน เช่น ชื่อในฟิลด์ CardType ของกรุงศรี) |
| network / tier | |
| co_brand | เช่น `bangchak`, `homepro` (ไว้จับโปรที่ผูกกับบัตร co-brand) |
| apply_url | ลิงก์สมัคร (ใช้ตอนแนะนำสมัคร) |
| verified | false จนกว่าจะตรวจเครือข่าย/ระดับกับหน้าเว็บธนาคารแล้ว |

### `promotion_eligibility` — 1:N ต่อ promotion, สร้างโดย normalize
| field | หมายเหตุ |
|---|---|
| promotion_id | |
| scope | `bank_all` (ทุกใบของธนาคารเจ้าของโปร), `cards`, `network` |
| card_keys | ใช้เมื่อ scope=cards |
| network / min_tier | ใช้เมื่อ scope=network; `issuer` = ธนาคารใด (ว่าง = ทุกธนาคาร → โปรของเครือข่ายเอง) |
| excluded_card_keys | บัตรที่ถูกยกเว้น |
| confidence | `high` (จากฟิลด์โครงสร้าง เช่น CardType) / `medium` (ข้อความตรง pattern) / `low` (default เพราะไม่เจอเงื่อนไขบัตร หรือเจอชื่อที่ไม่อยู่ใน catalog) |
| unmatched | ชื่อบัตรที่เจอในข้อความแต่ไม่อยู่ใน catalog (ไว้เติม catalog) |

หลายแถวต่อโปร = OR กัน (เช่น "KTC JCB ทุกประเภท หรือ KTC X VISA SIGNATURE")

ช่อง `promotions.cards` แบบข้อความเดิม: เก็บต่อไว้เป็นข้อมูลดิบ ไม่ใช้ในการจับคู่

### `merchants` + `promotion_merchants`
| field | หมายเหตุ |
|---|---|
| key | เช่น `bangchak`, `ptt-station`, `shell` |
| name_th / name_en, aliases | aliases จับคู่ข้อความ (เช่น "บางจาก", "Bangchak", "BCP") |
| category | หมวดของเรา; เพิ่มหมวดย่อย `fuel` แยกจาก `auto` |

`promotion_merchants(promotion_id, merchant_key)` — 1 โปรหลายร้านได้

ร้านค้า seed จากไฟล์ `app/catalog/merchants.yaml` เริ่มจากปั๊มน้ำมันทุกเจ้า + ร้านที่มีโปรเยอะสุดใน
ข้อมูล (ดึงรายชื่อจากการนับคำในหัวข้อโปร)

**โปรที่ไม่ระบุร้าน แต่ระบุหมวด** (เช่น "เติมน้ำมันที่ปั๊มที่ร่วมรายการ"): ผูกกับหมวด `fuel` แทน
และนับเป็นผลลัพธ์ของทุกร้านในหมวดนั้น ติดธง `merchant_match: "category"` ให้ client แสดงว่า
"ตรวจสอบร้านที่ร่วมรายการ"

## 5. Eligibility: บัตร c ใช้โปร p ได้เมื่อ

มีแถว eligibility e ของ p อย่างน้อยหนึ่งแถวที่ผ่าน และ c ไม่อยู่ใน `e.excluded_card_keys`:
- `bank_all`: `c.bank == p.bank`
- `cards`: `c.key ∈ e.card_keys`
- `network`: `c.network == e.network` และ `tier_index(c) ≥ tier_index(e.min_tier)` และ
  (`e.issuer` ว่าง หรือ `c.bank == e.issuer`)

โปรที่ไม่มีแถว eligibility เลย = ถือเป็น `bank_all` แบบ `confidence: low`

## 6. มูลค่า (บาท) ของโปรที่ยอดใช้จ่าย A

| benefit | สูตร |
|---|---|
| cashback / discount แบบ % | `A < min_spend → 0` (แต่บอก `needs_min_spend`) ไม่งั้น `min(A × percent, max_benefit)` |
| ส่วนลดเป็นบาท (ฟิลด์ใหม่ `amount_off`) | `A ≥ min_spend ? amount_off : 0` |
| points | `points × baht_per_point` ของธนาคาร (ค่าตั้งใน config, ติดธง `estimated: true`); ธนาคารที่ยังไม่ตั้งอัตรา → กลุ่ม `no_value` |
| installment / gift / other / อ่านค่าไม่ได้ | ไม่มีมูลค่า → อยู่กลุ่ม `no_value` (ไม่จัดอันดับปนกับกลุ่มที่มีมูลค่า) |

`A` ไม่ส่งมา → ใช้ยอดตั้งต้นตามหมวด (fuel 1,500 / dining 1,000 / shopping 2,000 / อื่น 1,000)
และตอบกลับว่า `amount_assumed: true`

ต้องลงทะเบียน / ต้องใช้โค้ด: ไม่ลดคะแนน แต่ส่ง `needs_register`, `promo_code` ให้ client แสดง

## 7. งานดึงข้อมูลจากข้อความ (normalize)

1. **เงื่อนไขบัตร** (รัน per bank, ใช้ catalog):
   - กรุงศรี: map ชื่อในฟิลด์ `CardType` → card key ผ่าน aliases (`high`)
   - ข้อความ: หา pattern "เฉพาะบัตร…", "บัตร X ทุกประเภท", "ยกเว้น…/สงวนสิทธิ์ยกเว้น…", ชื่อเครือข่าย+ระดับ
     ("JCB Platinum หรือ JCB Ultimate ที่ออกในประเทศไทย" → network, `issuer` ว่าง)
   - ไม่เจออะไร → `bank_all` + `low`
2. **ร้านค้า**: จับ aliases ของ merchants ใน title/description/detail (แยกคำให้ถูก ไม่จับคำย่อยกลางคำ)
3. **มูลค่า**: ปรับตัวดึง percent / min_spend / max_benefit / ส่วนลดเป็นบาท ให้ครอบรูปแบบที่พบบ่อย
   วัดผลก่อน-หลังด้วยจำนวนโปรที่ได้ค่า (เป้า: โปรหมวด fuel และ dining ส่วนใหญ่มีมูลค่า)
4. รันซ้ำได้ทั้งหมด (idempotent) เมื่อ catalog เปลี่ยน — คำสั่ง `jobs -renormalize`

ทดสอบด้วยข้อความจริงจาก DB เป็น fixture (ตัวอย่างที่เห็นแล้ว: Crown Dish JCB, KTC ยกเว้น CASH BACK,
ttb Disney, กรุงศรีปั๊มน้ำมัน 5%)

## 8. แหล่งโปรเครือข่าย

Source ใหม่ `visa`, `jcb` ทำตามกฎเดิมของ `sources` (robots, polite client, ไม่เลี่ยงด่าน)
promotions.bank = `visa` / `jcb`; eligibility = `network` ไม่มี issuer + min_tier ตามที่โปรเขียน
หน้ารวมของทั้งคู่โหลดด้วย JS → ใช้ sitemap / หา endpoint ข้อมูลสาธารณะ ถ้าไม่มีทางที่ไม่ต้องเลี่ยงอะไร ให้ข้ามและรายงาน

## 9. API

ทุก endpoint รับ `lang` เหมือนเดิม

```
GET /v1/merchants?q=บางจาก&category=fuel     # ค้น/auto-complete ร้านค้า
GET /v1/cards?bank=ttb&q=smart               # catalog สำหรับให้ผู้ใช้เลือกบัตร
GET /v1/networks
GET /v1/recommend?merchant=bangchak&cards=ttb-so-smart,kbank-the-passion&amount=1500
```

`merchant` จำเป็น (key; ถ้าส่งชื่อมาให้ resolve ผ่าน aliases), `cards` ไม่บังคับ, `amount` ไม่บังคับ

```jsonc
{
  "lang": "th",
  "merchant": {"key": "bangchak", "name": "บางจาก"},
  "amount": 1500, "amount_assumed": false,
  // มี cards: โปรที่ใช้ได้กับบัตรที่ถือ เรียงตาม value มาก→น้อย; ไม่มี cards: โปรทั้งหมด
  "best": [
    {
      "value_baht": 75, "estimated": false,
      "use_card": {"key": "ttb-so-smart", "name": "ttb so smart"},   // null ถ้าไม่ส่ง cards
      "eligible_cards": ["…"],                                        // ไม่มี cards: บัตรใน catalog ที่ใช้ได้
      "confidence": "medium",
      "merchant_match": "merchant",   // หรือ "category"
      "needs_min_spend": null, "needs_register": true, "promo_code": "",
      "promotion": { /* PromotionView เดิม */ }
    }
  ],
  // มี cards เท่านั้น: บัตรที่ไม่ได้ถือแต่คุ้มกว่าใบที่ดีที่สุดที่ถือ
  "consider_applying": [
    {"card": {"key": "ktc-bangchak-visa-platinum", "apply_url": "…"},
     "value_baht": 150, "extra_vs_best_held": 75, "promotion": { }}
  ],
  "no_value": [ /* โปรที่ใช้ได้แต่คำนวณมูลค่าไม่ได้ (ผ่อน 0%, ของแถม…) */ ]
}
```

กติกาเรียง: `value_baht` ↓, แล้ว `confidence` (high > medium > low), แล้ววันหมดเขตใกล้สุดก่อน
โปรเดียวกันที่ถือได้หลายใบ → ตอบครั้งเดียว `use_card` = ใบแรกตามลำดับที่ผู้ใช้ส่งมา

`consider_applying`: แสดงเมื่อ `extra_vs_best_held ≥ max(50 บาท, 20% ของ best ที่ถือ)` สูงสุด 3 ใบ
ไม่เอาโปรที่ `confidence: low` และไม่เอาโปรสำหรับสมัครบัตรใหม่ (หมวด `new-card`) มาปน

บัตรที่ส่งมาแต่ไม่อยู่ใน catalog → ตอบ `unknown_cards: [...]` ไม่ error

## 10. Catalog ร่าง (ต้องตรวจ — `verified: false` ทั้งหมด)

เครือข่าย/ระดับข้างล่าง **เป็นร่าง** ส่วนที่มาจากชื่อกรุงศรีในข้อมูลจริงมีที่มาชัด; ใบอื่นต้องเช็คกับ
หน้าบัตรของธนาคารตอน implement ก่อนเปลี่ยนเป็น `verified: true`

| bank | card (ร่าง) | network / tier (ร่าง) | ที่มา |
|---|---|---|---|
| krungsri | กรุงศรี เจซีบี อัลติเมท | jcb / ultimate | ชื่อใน CardType |
| krungsri | กรุงศรี เจซีบี แพลทินัม | jcb / platinum | ชื่อใน CardType |
| krungsri | กรุงศรี แพลทินัม มาสเตอร์การ์ด | mastercard / platinum | ชื่อใน CardType |
| krungsri | กรุงศรี เอ็กซ์คลูซีฟ ซิกเนเจอร์ | visa / signature (เดา) | ชื่อใน CardType |
| krungsri | กรุงศรี ซิกเนเจอร์ | visa / signature (เดา) | ชื่อใน CardType |
| krungsri | กรุงศรี เลดี้ ไทเทเนี่ยม | mastercard / titanium (เดา) | ชื่อใน CardType |
| krungsri | กรุงศรี แพลทินัม, นาว แพลทินัม, ไพรเวท แบงก์กิ้ง, ดอกเตอร์การ์ด, วีซ่า/มาสเตอร์การ์ด | ต้องเช็ค | ชื่อใน CardType |
| krungsri | โฮมโปร วีซ่า แพลทินัม (co_brand homepro), เอไอเอ วีซ่า/แพลทินัม, สยาม ทาคาชิมายะ วีซ่า/เจซีบี/ไฟน์เนส, ทีซีซี พริวิเลจ | ตามชื่อ / ต้องเช็ค | ชื่อใน CardType |
| ktc | KTC JCB (ทุกระดับ), KTC VISA, KTC MASTERCARD | ตามชื่อ / tier ต้องเช็ค | ข้อความเงื่อนไข |
| ktc | KTC CASH BACK, KTC ROYAL ORCHID PLUS, KTC VISA CORPORATE, KTC GOVERNMENT SERVICES, KTC THAI RED CROSS | ต้องเช็ค | ข้อความ "ยกเว้น" |
| ktc | KTC X VISA SIGNATURE, KTC X WORLD REWARDS MASTERCARD, KTC บางจาก (co_brand bangchak) | ต้องเช็ค | ความรู้เดิม — ยืนยันกับเว็บ KTC |
| ttb | ttb so smart, ttb so fast, ttb so chill, ttb reserve (infinite/signature), ttb absolute, ttb Disney | ต้องเช็คทั้งหมด | ข้อความ (Disney) + ความรู้เดิม |
| kbank | KBank The Passion (ตามที่ผู้ใช้ยกตัวอย่าง), THE WISDOM, THE PREMIER, Platinum, Titanium, JCB Platinum | ต้องเช็คทั้งหมด | ความรู้เดิม — เว็บ KBank ดึงไม่ได้ ต้องกรอกมือ |

ขั้นตอนตอน implement: ดึงรายชื่อบัตรจากหน้า "บัตรเครดิต" สาธารณะของ KTC / ttb / กรุงศรี
(ถ้า robots อนุญาตและไม่มีด่าน) มาเทียบ แล้วรายงานว่าใบไหนยัง verify ไม่ได้

## 11. ลำดับงาน

1. รอ backlog crawl ให้ครบ (กำลังรัน)
2. ตาราง + seed: networks, cards.yaml, merchants.yaml
3. normalize: eligibility, merchants, มูลค่า + `jobs -renormalize` + เทสต์ด้วย fixture จริง
4. API: merchants, cards, networks, recommend + เทสต์
5. แหล่ง Visa, JCB
6. วัดผล: จำนวนโปร fuel/dining ที่มีมูลค่า, จำนวนชื่อบัตร `unmatched` → เติม catalog

## 12. Calls ที่ผมตัดสินใจเอง (ตรวจได้)

- ยอดตั้งต้นตามหมวดเมื่อไม่ส่ง `amount`
- เกณฑ์แนะนำสมัคร ≥ 50 บาท และ ≥ 20%, สูงสุด 3 ใบ
- ไม่รวมสิทธิ์ซ้อน (ธนาคาร + เครือข่าย) แสดงแยก
- โปรไม่ระบุร้านแต่ระบุหมวด นับเป็นผลลัพธ์ของทุกร้านในหมวดนั้น (ติดธง)
- คะแนนแปลงเป็นบาทด้วยอัตราตั้งค่าต่อธนาคาร และติดธง `estimated`
- ไม่มีเงื่อนไขบัตร = ทุกใบของธนาคาร แต่ `confidence: low` และไม่ใช้ใน `consider_applying`
- catalog เก็บเป็นไฟล์ YAML ใน repo (review ผ่าน git) แทนการแก้ใน DB
