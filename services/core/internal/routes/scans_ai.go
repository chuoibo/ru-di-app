package routes

import (
	"context"
	"errors"
	"time"

	"mobile/services/core/internal/aiharness/docanh"
	"mobile/services/core/internal/aiharness/docbill"
	"mobile/services/core/internal/aiharness/motluot"
	"mobile/services/core/internal/domain/receipt"
	"mobile/services/core/internal/domain/screenshot"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/media/sanitize"
	"mobile/services/core/internal/pyjson"
)

// The scan routes' model step runs here (ADR-0052), where the Python brain's
// receipt skill used to: the upload is checked and re-encoded before any
// model sees it, the model transcribes, and domain/receipt decides what the
// transcription is worth.

// maxScanBytes bounds the upload as received, before anything decodes it: a
// limit read off the re-encode would let a 20 MB upload be decoded first.
const maxScanBytes = 8 * 1024 * 1024

// scanTypes are the declared types a scan accepts. HEIC is not among them:
// nothing here decodes it, and an iPhone's default format is the one that
// always carries GPS. The app re-encodes to JPEG before upload.
var scanTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true}

// scanCallTimeout bounds one scan's model step; agy-proxy runs a short turn
// in a few seconds and queues a busy pool in front of that.
const scanCallTimeout = 45 * time.Second

// scanCalls is one scan's budget: the call and two retries on 429/503.
const scanCalls = 3

// errScanImage is an upload refused before the model: its public code.
type errScanImage struct{ code string }

func (e errScanImage) Error() string { return e.code }

// sanitizeScan runs the checks every scan makes before the model: something
// uploaded, a declared type it accepts, the byte ceiling on the upload as
// received, then the sanitiser's re-encode (EXIF and GPS gone). The model is
// handed the rebuilt pixels and the type they really are, never the
// uploader's claim. rd-qa-33: a bill photographed at the table reached the
// model with its GPS intact before this existed.
func sanitizeScan(content []byte, contentType string) (sanitize.Sanitized, error) {
	switch {
	case len(content) == 0:
		return sanitize.Sanitized{}, errScanImage{"empty_image"}
	case !scanTypes[contentType]:
		return sanitize.Sanitized{}, errScanImage{"unsupported_image_type"}
	case len(content) > maxScanBytes:
		return sanitize.Sanitized{}, errScanImage{"image_too_large"}
	}
	out, err := sanitize.Sanitize(content)
	var rejected *sanitize.Rejected
	var unsupported *sanitize.UnsupportedError
	switch {
	case err == nil:
		return out, nil
	case errors.As(err, &rejected) && (rejected.Code == sanitize.CodeImageTooLarge || rejected.Code == sanitize.CodeImageDimensionsTooLarge):
		// A pixel bomb is a size refusal: 413 is the honest status.
		return sanitize.Sanitized{}, errScanImage{"image_too_large"}
	case errors.As(err, &rejected), errors.As(err, &unsupported):
		return sanitize.Sanitized{}, errScanImage{"unsupported_image_type"}
	}
	return sanitize.Sanitized{}, err
}

// docHoaDon is POST /receipts/scan's step after the rate limit: the upload's
// checks, one model reading of the re-encoded photo, then domain/receipt.
// Every failure is a closed refusal with the codes the app has always read.
func docHoaDon(ctx context.Context, may *motluot.May, content []byte, contentType string) (receipt.Reading, error) {
	clean, err := sanitizeScan(content, contentType)
	if err != nil {
		var refused errScanImage
		if !errors.As(err, &refused) {
			return receipt.Reading{}, refuseReceipt("receipt_reader_unavailable")
		}
		if refused.code == "empty_image" {
			return receipt.Reading{}, refuseReceipt("receipt_unreadable")
		}
		return receipt.Reading{}, refuseReceipt(refused.code)
	}
	if !may.CoMay() {
		return receipt.Reading{}, refuseReceipt("receipt_reader_not_configured")
	}
	ctx, cancel := context.WithTimeout(ctx, scanCallTimeout)
	defer cancel()
	raw, err := docbill.Doc(ctx, may.Luot(scanCalls), clean.ContentType, clean.Data)
	if err != nil {
		return receipt.Reading{}, refuseReceipt("receipt_reader_unavailable")
	}
	reading, err := receipt.ReadScannedDocument(raw)
	var refused *receipt.Error
	switch {
	case err == nil:
		return reading, nil
	case !errors.As(err, &refused):
		return receipt.Reading{}, refuseReceipt("receipt_reader_unavailable")
	case refused.Code == "RECEIPT_TOO_BLURRY":
		return receipt.Reading{}, refuseReceipt("receipt_too_blurry")
	case refused.Code == "NOT_A_RECEIPT_PRICE_LIST":
		return receipt.Reading{}, refuseReceipt("not_a_receipt_price_list")
	case refused.Code == "NOT_A_RECEIPT":
		return receipt.Reading{}, refuseReceipt("not_a_receipt")
	}
	return receipt.Reading{}, refuseReceipt("receipt_unreadable")
}

// refuseReceipt is the public problem of a receipt refusal.
func refuseReceipt(code string) error {
	switch code {
	case "unsupported_image_type":
		return endpoint.Refuse(415, "unsupported_image_type", "Định dạng ảnh không được hỗ trợ.")
	case "image_too_large":
		return endpoint.Refuse(413, "image_too_large", "Ảnh bill vượt quá giới hạn 8 MB.")
	case "receipt_too_blurry":
		return endpoint.Refuse(422, "receipt_too_blurry", "Ảnh bill quá mờ. Vui lòng chụp lại ảnh rõ hơn.")
	case "receipt_reader_not_configured":
		return endpoint.Refuse(503, "receipt_reader_not_configured", "Máy chủ chưa cấu hình model đọc bill nên không gọi được AI. Đây là lỗi cấu hình phía máy chủ, không phải ảnh bạn chụp — chụp lại cũng không giúp được. Người dựng hệ cần đặt AGY_PROXY_URL và AGY_PROXY_KEY (hoặc GEMINI_API_KEY) cho core rồi khởi động lại.")
	case "not_a_receipt_price_list":
		return endpoint.Refuse(422, "not_a_receipt", "Đây là thực đơn hoặc bảng giá, không phải hoá đơn. Bảng giá chỉ nói món bao nhiêu tiền, không nói ai đã gọi gì, nên không chia tiền được. Hãy chụp tờ bill có dòng tổng tiền.")
	case "not_a_receipt":
		return endpoint.Refuse(422, "not_a_receipt", "Ảnh này không phải hoá đơn. Hãy chụp tờ bill có danh sách món và dòng tổng tiền.")
	case "receipt_unreadable":
		return endpoint.Refuse(422, "receipt_unreadable", "Không đọc được bill. Vui lòng kiểm tra ảnh và thử lại.")
	}
	return endpoint.Refuse(502, "receipt_reader_unavailable", "Không đọc được bill lúc này, thử lại sau.")
}

// wireReceiptScan is the reading as the app receives it. The confidence
// decided the outcome and is never published (ADR-0009 decision 4): a
// percentage on a screen invites a rule it does not support.
func wireReceiptScan(r receipt.Reading) *pyjson.OrderedMap {
	items := pyjson.List{}
	for _, it := range r.Items {
		row := pyjson.NewOrderedMap()
		row.Set("name", pyjson.String(it.Name))
		row.Set("quantity", pyjson.NewBigInt(it.Quantity))
		row.Set("unit_price_vnd", optInt(it.UnitPriceVND))
		row.Set("line_total_vnd", pyjson.NewInt(it.LineTotalVND))
		items = append(items, row)
	}
	warnings := pyjson.List{}
	for _, w := range r.Warnings {
		warnings = append(warnings, pyjson.String(w))
	}
	out := pyjson.NewOrderedMap()
	out.Set("items", items)
	out.Set("items_total_vnd", pyjson.NewInt(r.ItemsTotalVND))
	out.Set("total_vnd", optInt(r.TotalVND))
	if r.TotalsAgree == nil {
		out.Set("totals_agree", pyjson.Null{})
	} else {
		out.Set("totals_agree", pyjson.Bool(*r.TotalsAgree))
	}
	out.Set("total_difference_vnd", optInt(r.TotalDifferenceVND))
	out.Set("needs_review", pyjson.Bool(r.NeedsReview))
	out.Set("warnings", warnings)
	return out
}

func optInt(v *int64) pyjson.Value {
	if v == nil {
		return pyjson.Null{}
	}
	return pyjson.NewInt(*v)
}

// docManHinh is POST /screenshots/scan's step after the rate limit: the same
// upload checks as a bill, one model reading of the re-encoded screenshot,
// then domain/screenshot, which admits no person.
func docManHinh(ctx context.Context, may *motluot.May, content []byte, contentType string) (screenshot.Reading, error) {
	clean, err := sanitizeScan(content, contentType)
	if err != nil {
		var refused errScanImage
		if !errors.As(err, &refused) {
			return screenshot.Reading{}, refuseScreenshot("screenshot_reader_unavailable")
		}
		if refused.code == "empty_image" {
			return screenshot.Reading{}, refuseScreenshot("screenshot_unreadable")
		}
		return screenshot.Reading{}, refuseScreenshot(refused.code)
	}
	if !may.CoMay() {
		return screenshot.Reading{}, refuseScreenshot("screenshot_reader_not_configured")
	}
	ctx, cancel := context.WithTimeout(ctx, scanCallTimeout)
	defer cancel()
	raw, err := docanh.Doc(ctx, may.Luot(scanCalls), clean.ContentType, clean.Data)
	if err != nil {
		return screenshot.Reading{}, refuseScreenshot("screenshot_reader_unavailable")
	}
	reading, err := screenshot.Read(raw)
	var refused *screenshot.Error
	switch {
	case err == nil:
		return reading, nil
	case !errors.As(err, &refused):
		return screenshot.Reading{}, refuseScreenshot("screenshot_reader_unavailable")
	case refused.Code == "NOT_A_TRANSACTION":
		return screenshot.Reading{}, refuseScreenshot("not_a_transaction")
	case refused.Code == "MODEL_NAMED_A_PERSON":
		return screenshot.Reading{}, refuseScreenshot("screenshot_model_named_a_person")
	}
	return screenshot.Reading{}, refuseScreenshot("screenshot_unreadable")
}

// refuseScreenshot is the public problem of a screenshot refusal.
func refuseScreenshot(code string) error {
	switch code {
	case "unsupported_image_type":
		return endpoint.Refuse(415, "unsupported_image_type", "Định dạng ảnh chụp màn hình không được hỗ trợ.")
	case "image_too_large":
		return endpoint.Refuse(413, "image_too_large", "Ảnh chụp màn hình vượt quá giới hạn 8 MB.")
	case "not_a_transaction":
		return endpoint.Refuse(422, "not_a_transaction", "Ảnh này không thể hiện một giao dịch đã hoàn tất để tạo khoản chi.")
	case "screenshot_model_named_a_person":
		return endpoint.Refuse(422, "screenshot_model_named_a_person", "AI đã cố nêu một người; kết quả bị từ chối để định danh chỉ đến từ phiên đăng nhập.")
	case "screenshot_reader_not_configured":
		return endpoint.Refuse(503, "screenshot_reader_not_configured", "Máy chủ chưa cấu hình khoá đọc ảnh chụp màn hình. Đây là lỗi cấu hình phía máy chủ, không phải ảnh bạn tải lên.")
	case "screenshot_unreadable":
		return endpoint.Refuse(422, "screenshot_unreadable", "Không đọc được giao dịch từ ảnh chụp màn hình. Vui lòng kiểm tra ảnh.")
	}
	return endpoint.Refuse(502, "screenshot_reader_unavailable", "Dịch vụ đọc ảnh chụp màn hình đang lỗi phía máy chủ. Vui lòng thử lại sau.")
}

// wireScreenshotScan is the reading as the app receives it: always a draft.
func wireScreenshotScan(r screenshot.Reading) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("source", pyjson.String(r.Source))
	out.Set("merchant", pyjson.String(r.Merchant))
	out.Set("total_vnd", pyjson.NewInt(r.TotalVND))
	if r.OccurredOn == nil {
		out.Set("occurred_on", pyjson.Null{})
	} else {
		out.Set("occurred_on", pyjson.String(*r.OccurredOn))
	}
	out.Set("needs_review", pyjson.Bool(true))
	return out
}
