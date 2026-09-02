package i18n

import (
	"context"
	"strings"
)

type Locale string

const (
	Thai    Locale = "th"
	English Locale = "en"
)

type contextKey struct{}

type Translation struct {
	TH string `json:"th"`
	EN string `json:"en"`
}

var catalog = map[string]Translation{
	"SERVICE_READY":                    {TH: "บริการ Domain Monitoring พร้อมใช้งาน", EN: "The Domain Monitoring service is running."},
	"INTERNAL_ERROR":                   {TH: "เกิดข้อผิดพลาดภายในระบบ", EN: "An internal error occurred."},
	"INVALID_JSON":                     {TH: "ข้อมูล JSON ไม่ถูกต้อง", EN: "The JSON payload is invalid."},
	"VALIDATION_FAILED":                {TH: "ข้อมูลไม่ผ่านการตรวจสอบ", EN: "The supplied data is invalid."},
	"UNAUTHORIZED":                     {TH: "กรุณาเข้าสู่ระบบ", EN: "Authentication is required."},
	"FORBIDDEN":                        {TH: "คุณไม่มีสิทธิ์ดำเนินการนี้", EN: "You do not have permission to perform this action."},
	"CSRF_INVALID":                     {TH: "โทเค็นป้องกัน CSRF ไม่ถูกต้อง", EN: "The CSRF token is invalid."},
	"INVALID_CREDENTIALS":              {TH: "อีเมลหรือรหัสผ่านไม่ถูกต้อง", EN: "The email or password is incorrect."},
	"DOMAIN_ALREADY_EXISTS":            {TH: "มีโดเมนนี้อยู่ในระบบแล้ว", EN: "The normalized domain already exists."},
	"DOMAIN_NOT_FOUND":                 {TH: "ไม่พบโดเมน", EN: "The domain was not found."},
	"VERSION_CONFLICT":                 {TH: "ข้อมูลถูกแก้ไขโดยผู้ใช้อื่น กรุณาโหลดใหม่", EN: "The record was changed by another user. Reload and try again."},
	"INVALID_DOMAIN":                   {TH: "รูปแบบโดเมนไม่ถูกต้อง", EN: "The domain format is invalid."},
	"READINESS_FAILED":                 {TH: "ระบบยังไม่พร้อมให้บริการ", EN: "The service is not ready."},
	"NOT_FOUND":                        {TH: "ไม่พบทรัพยากรที่ร้องขอ", EN: "The requested resource was not found."},
	"METHOD_NOT_ALLOWED":               {TH: "ไม่รองรับ HTTP method นี้", EN: "The HTTP method is not allowed."},
	"SESSION_CREATED":                  {TH: "เข้าสู่ระบบสำเร็จ", EN: "Signed in successfully."},
	"SESSION_REVOKED":                  {TH: "ออกจากระบบแล้ว", EN: "Signed out successfully."},
	"MONITOR_RUN_ACCEPTED":             {TH: "รับงานตรวจสอบโดเมนแล้ว", EN: "The domain monitoring run was accepted."},
	"ISP_CHECK_ACCEPTED":               {TH: "รับคำขอบังคับตรวจสอบ ISP แล้ว", EN: "The forced ISP check was accepted."},
	"MONITOR_RUN_NOT_FOUND":            {TH: "ไม่พบงานตรวจสอบ", EN: "The monitoring run was not found."},
	"DOMAIN_INACTIVE":                  {TH: "โดเมนนี้ไม่ได้อยู่ในสถานะพร้อมตรวจสอบ", EN: "The domain is not active for monitoring."},
	"IDEMPOTENCY_KEY_REQUIRED":         {TH: "ต้องระบุ Idempotency-Key ที่ถูกต้อง", EN: "A valid Idempotency-Key header is required."},
	"INVALID_HISTORY_WINDOW":           {TH: "ช่วงเวลาประวัติไม่ถูกต้อง", EN: "The monitoring history window is invalid."},
	"INVALID_INCIDENT_STATUS":          {TH: "สถานะ incident ไม่ถูกต้อง", EN: "The incident status filter is invalid."},
	"PROBE_REGISTERED":                 {TH: "ลงทะเบียนโหนดตรวจสอบแล้ว", EN: "The probe node was registered."},
	"PROBE_TOKEN_CREATED":              {TH: "สร้างโทเค็นลงทะเบียนโหนดแล้ว", EN: "The probe registration token was created."},
	"PROBE_REVOKED":                    {TH: "เพิกถอนโหนดตรวจสอบแล้ว", EN: "The probe node was revoked."},
	"PROBE_UNAUTHORIZED":               {TH: "การยืนยันตัวตนของโหนดไม่ถูกต้อง", EN: "Probe authentication failed."},
	"PROBE_FORBIDDEN":                  {TH: "โหนดนี้ถูกระงับหรือเวอร์ชันไม่รองรับ", EN: "The probe is revoked or incompatible."},
	"PROBE_INVALID_REQUEST":            {TH: "คำขอจากโหนดไม่ถูกต้อง", EN: "The probe request is invalid."},
	"PROBE_EXPIRED":                    {TH: "ข้อมูลยืนยันตัวตนหรืองานของโหนดหมดอายุ", EN: "The probe credential or job expired."},
	"PROBE_REPLAY":                     {TH: "คำขอหรือลายเซ็นนี้ถูกใช้แล้ว", EN: "The probe request or signature was already used."},
	"PROBE_SIGNATURE_INVALID":          {TH: "ลายเซ็นของโหนดไม่ถูกต้อง", EN: "The probe signature is invalid."},
	"PROBE_PAYLOAD_TOO_LARGE":          {TH: "ผลตรวจจากโหนดมีขนาดใหญ่เกินกำหนด", EN: "The probe result payload is too large."},
	"PROBE_CLOCK_SKEW":                 {TH: "เวลาของโหนดคลาดเคลื่อนเกินกำหนด", EN: "The probe clock skew exceeds policy."},
	"PROBE_NOT_FOUND":                  {TH: "ไม่พบโหนดหรืองานตรวจสอบ", EN: "The probe or probe job was not found."},
	"RDAP_RESULT_NOT_FOUND":            {TH: "ยังไม่มีผล RDAP สำหรับโดเมนนี้", EN: "No RDAP result exists for this domain."},
	"RDAP_RATE_LIMITED":                {TH: "ผู้ให้บริการ RDAP จำกัดอัตราการเรียก กรุณาลองใหม่ภายหลัง", EN: "The RDAP provider rate-limited the request. Try again later."},
	"RDAP_BOOTSTRAP_NOT_FOUND":         {TH: "ไม่พบผู้ให้บริการ RDAP ที่รับผิดชอบ TLD นี้", EN: "No authoritative RDAP service was found for this TLD."},
	"RDAP_UNAVAILABLE":                 {TH: "บริการ RDAP ไม่พร้อมใช้งานชั่วคราว", EN: "The RDAP service is temporarily unavailable."},
	"FINANCE_VALIDATION_FAILED":        {TH: "ข้อมูลการเงินไม่ผ่านการตรวจสอบ", EN: "The financial data is invalid."},
	"FINANCE_NOT_FOUND":                {TH: "ไม่พบข้อมูลการเงินที่ร้องขอ", EN: "The requested financial record was not found."},
	"MANUAL_OVERRIDE_REVOKED":          {TH: "ยกเลิกค่าที่กำหนดเองแล้ว", EN: "The manual override was revoked."},
	"SHEETS_CREDENTIALS_UNAVAILABLE":   {TH: "ยังไม่ได้ตั้งค่าข้อมูลยืนยันตัวตน Google Sheets หรือข้อมูลไม่ถูกต้อง", EN: "Google Sheets credentials are missing or invalid."},
	"SHEETS_UNAVAILABLE":               {TH: "Google Sheets API ไม่พร้อมใช้งานชั่วคราว", EN: "The Google Sheets API is temporarily unavailable."},
	"SHEETS_VALIDATION_FAILED":         {TH: "ข้อมูลหรือ column mapping ของ Google Sheet ไม่ถูกต้อง", EN: "The Google Sheet data or column mapping is invalid."},
	"SHEETS_NOT_FOUND":                 {TH: "ไม่พบการตั้งค่าหรือประวัติ Google Sheet", EN: "The Google Sheet configuration or import was not found."},
	"SHEETS_CONFLICT":                  {TH: "สถานะ Google Sheet import เปลี่ยนแปลงแล้ว กรุณาโหลดใหม่", EN: "The Google Sheet import state changed. Reload and try again."},
	"EXCEL_IMPORT_INVALID":             {TH: "ไฟล์ Excel หรือข้อมูลนำเข้าไม่ถูกต้อง", EN: "The Excel workbook or import data is invalid."},
	"EXCEL_IMPORT_TOO_LARGE":           {TH: "ไฟล์ Excel มีขนาดใหญ่เกินกำหนด", EN: "The Excel workbook exceeds the allowed size."},
	"DRIVE_NOT_CONFIGURED":             {TH: "ยังไม่ได้ตั้งค่า Google Drive OAuth", EN: "Google Drive OAuth is not configured."},
	"DRIVE_NOT_CONNECTED":              {TH: "ยังไม่ได้เชื่อมต่อ Google Drive", EN: "Google Drive is not connected."},
	"DRIVE_OAUTH_STATE_INVALID":        {TH: "คำขอเชื่อมต่อ Google Drive หมดอายุหรือไม่ถูกต้อง", EN: "The Google Drive connection request is invalid or expired."},
	"DRIVE_VALIDATION_FAILED":          {TH: "ข้อมูลการเชื่อมต่อ Google Drive ไม่ถูกต้อง", EN: "The Google Drive connection data is invalid."},
	"DRIVE_UNAVAILABLE":                {TH: "Google Drive ไม่พร้อมใช้งานชั่วคราว", EN: "Google Drive is temporarily unavailable."},
	"RECOMMENDATION_NOT_FOUND":         {TH: "ยังไม่มีคำแนะนำสำหรับโดเมนนี้", EN: "No recommendation exists for this domain."},
	"RECOMMENDATION_VALIDATION_FAILED": {TH: "ข้อมูลสำหรับสร้างคำแนะนำไม่ถูกต้อง", EN: "The recommendation request is invalid."},
	"REPORT_NOT_FOUND":                 {TH: "ไม่พบรายงาน", EN: "The report was not found."},
	"REPORT_VALIDATION_FAILED":         {TH: "รูปแบบหรือเงื่อนไขรายงานไม่ถูกต้อง", EN: "The report format or filters are invalid."},
}

func Parse(value string, fallback Locale) Locale {
	for _, part := range strings.Split(strings.ToLower(value), ",") {
		code := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		switch {
		case code == "th" || strings.HasPrefix(code, "th-"):
			return Thai
		case code == "en" || strings.HasPrefix(code, "en-"):
			return English
		}
	}
	if fallback == Thai || fallback == English {
		return fallback
	}
	return Thai
}

func WithContext(ctx context.Context, locale Locale) context.Context {
	return context.WithValue(ctx, contextKey{}, locale)
}

func FromContext(ctx context.Context, fallback Locale) Locale {
	if locale, ok := ctx.Value(contextKey{}).(Locale); ok {
		return Parse(string(locale), fallback)
	}
	return fallback
}

func Messages(code string) Translation {
	if translation, ok := catalog[code]; ok {
		return translation
	}
	return catalog["INTERNAL_ERROR"]
}

func Message(code string, locale Locale) string {
	translation := Messages(code)
	if locale == English {
		return translation.EN
	}
	return translation.TH
}

func Supported() []map[string]string {
	return []map[string]string{
		{"code": "th", "name": "ไทย", "english_name": "Thai"},
		{"code": "en", "name": "English", "english_name": "English"},
	}
}
