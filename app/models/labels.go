package models

// Label is a key's display text per language.
type Label struct {
	TH string `json:"th"`
	EN string `json:"en"`
}

// Get returns the label in lang (Thai fallback).
func (l Label) Get(lang string) string { return Text(lang, l.TH, l.EN) }

// Banks we read.
var Banks = map[string]Label{
	"ktc":      {TH: "เคทีซี (KTC)", EN: "KTC"},
	"kbank":    {TH: "กสิกรไทย", EN: "KBank"},
	"krungsri": {TH: "กรุงศรี", EN: "Krungsri"},
	"ttb":      {TH: "ทีทีบี", EN: "ttb"},
	"jcb":      {TH: "เจซีบี (JCB)", EN: "JCB"},
	"cardx":    {TH: "CardX (SCB)", EN: "CardX"},
	"aeon":     {TH: "อิออน", EN: "AEON"},
}

// Categories are our own keys; each source maps its categories onto them.
var Categories = map[string]Label{
	"dining":      {TH: "ร้านอาหาร", EN: "Dining"},
	"shopping":    {TH: "ช้อปปิ้ง", EN: "Shopping"},
	"online":      {TH: "ช้อปออนไลน์", EN: "Online shopping"},
	"travel":      {TH: "ท่องเที่ยว", EN: "Travel"},
	"installment": {TH: "ผ่อน 0%", EN: "0% installment"},
	"auto":        {TH: "รถยนต์และน้ำมัน", EN: "Car and fuel"},
	"health":      {TH: "สุขภาพและความงาม", EN: "Health and beauty"},
	"home":        {TH: "บ้าน", EN: "Home"},
	"entertain":   {TH: "บันเทิง", EN: "Entertainment"},
	"points":      {TH: "แลกคะแนน", EN: "Points"},
	"fuel":        {TH: "น้ำมัน", EN: "Fuel"},
	"new-card":    {TH: "สมัครบัตรใหม่", EN: "New card"},
	"other":       {TH: "อื่นๆ", EN: "Other"},
}

// BenefitTypes labels BenefitType keys.
var BenefitTypes = map[string]Label{
	BenefitCashback:    {TH: "เครดิตเงินคืน", EN: "Cashback"},
	BenefitDiscount:    {TH: "ส่วนลด", EN: "Discount"},
	BenefitPoints:      {TH: "คะแนน", EN: "Points"},
	BenefitInstallment: {TH: "ผ่อน 0%", EN: "0% installment"},
	BenefitGift:        {TH: "ของสมนาคุณ", EN: "Gift"},
	BenefitOther:       {TH: "อื่นๆ", EN: "Other"},
}
