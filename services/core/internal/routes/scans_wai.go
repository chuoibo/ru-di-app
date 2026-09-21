package routes

import (
	"context"
	"errors"
	"io/fs"
	"strconv"

	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/domain/faces"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
)

func scanReceipt() Route {
	return Route{ID: "POST /receipts/scan", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if err := spendActorWindow(call, call.Limits.ReceiptScanLimiter); err != nil {
			return endpoint.Reply{}, err
		}
		file, err := bodyUpload(call, "image")
		if err != nil {
			return endpoint.Reply{}, err
		}
		raw, err := brain.Configured().PostJSON("receipt-scan", brain.ImageBody(file.Content, uploadContentType(file)))
		if err != nil {
			return endpoint.Reply{}, mapReceiptErr(err)
		}
		obj, err := brain.AsObject(raw)
		if err != nil {
			return endpoint.Reply{}, endpoint.Refuse(502, "receipt_reader_unavailable", "Không đọc được bill lúc này, thử lại sau.")
		}
		return endpoint.Reply{Body: wireReceiptScan(obj)}, nil
	}}
}

func scanScreenshot() Route {
	return Route{ID: "POST /screenshots/scan", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if err := spendActorWindow(call, call.Limits.ScreenshotScanLimiter); err != nil {
			return endpoint.Reply{}, err
		}
		file, err := bodyUpload(call, "image")
		if err != nil {
			return endpoint.Reply{}, err
		}
		raw, err := brain.Configured().PostJSON("screenshot-scan", brain.ImageBody(file.Content, uploadContentType(file)))
		if err != nil {
			return endpoint.Reply{}, mapScreenshotErr(err)
		}
		obj, err := brain.AsObject(raw)
		if err != nil {
			return endpoint.Reply{}, endpoint.Refuse(502, "screenshot_reader_unavailable", "Dịch vụ đọc ảnh chụp màn hình đang lỗi phía máy chủ. Vui lòng thử lại sau.")
		}
		return endpoint.Reply{Body: wireScreenshotScan(obj)}, nil
	}}
}

func detectFaces() Route {
	return Route{ID: "POST /contexts/{context_id}/photos/{photo_id}/face-boxes", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if err := spendActorWindow(call, call.Limits.FaceDetectionLimiter); err != nil {
			return endpoint.Reply{}, err
		}
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		photoID, err := pathUUID(call, "photo_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "view_group_memories", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		record, err := store.GetContextImage(ctx, contextID, photoID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if record == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, "photo_not_found", "Photo does not exist")
		}
		content, err := call.Photos.Read(record.StorageKey)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return endpoint.Reply{}, endpoint.Refuse(404, "photo_not_found", "Photo does not exist")
			}
			return endpoint.Reply{}, err
		}
		raw, err := brain.Configured().PostJSON("face-boxes", brain.ImageBody(content, record.ContentType))
		if err != nil {
			return endpoint.Reply{}, mapFaceErr(err)
		}
		obj, err := brain.AsObject(raw)
		if err != nil {
			return endpoint.Reply{}, endpoint.Refuse(502, "face_detection_failed", "Không tìm được khuôn mặt trong ảnh lúc này, thử lại sau.")
		}
		width := intOf(obj, "image_width")
		height := intOf(obj, "image_height")
		rawFaces, _ := obj.Get("faces")
		list, _ := rawFaces.(pyjson.List)
		boxes := make([]faces.Box, 0, len(list))
		for _, item := range list {
			row, ok := item.(*pyjson.OrderedMap)
			if !ok {
				continue
			}
			boxes = append(boxes, faces.Box{
				X: intOf(row, "x"), Y: intOf(row, "y"), Width: intOf(row, "width"), Height: intOf(row, "height"),
			})
		}
		published, err := faces.AnonymousBoxes(boxes, width, height)
		if err != nil {
			var refused *faces.Error
			if errors.As(err, &refused) && refused.Code == "TOO_MANY_FACES" {
				return endpoint.Reply{}, endpoint.Refuse(422, "too_many_faces",
					"Ảnh này có quá nhiều khuôn mặt (tối đa "+strconv.Itoa(faces.MaxFaces)+"). Hãy chọn ảnh chụp gần hơn để mọi người bấm đúng ô của mình.")
			}
			return endpoint.Reply{}, endpoint.Refuse(502, "face_detection_failed", "Không tìm được khuôn mặt trong ảnh lúc này, thử lại sau.")
		}
		out := pyjson.List{}
		for _, box := range published {
			entry := pyjson.NewOrderedMap()
			entry.Set("box_key", pyjson.String(box.BoxKey))
			entry.Set("x", pyjson.Float(box.X))
			entry.Set("y", pyjson.Float(box.Y))
			entry.Set("width", pyjson.Float(box.Width))
			entry.Set("height", pyjson.Float(box.Height))
			out = append(out, entry)
		}
		body := pyjson.NewOrderedMap()
		body.Set("photo_id", pyjson.String(photoID))
		body.Set("boxes", out)
		return endpoint.Reply{Body: body}, nil
	}}
}

func intOf(obj *pyjson.OrderedMap, key string) int {
	value, _ := obj.Get(key)
	switch v := value.(type) {
	case pyjson.Int:
		n, _ := v.Int64()
		return int(n)
	case pyjson.Float:
		return int(v)
	}
	return 0
}

func mapReceiptErr(err error) error {
	if refused := brainErr(err); refused != nil {
		switch refused.Code {
		case "unsupported_image_type":
			return endpoint.Refuse(415, "unsupported_image_type", "Định dạng ảnh không được hỗ trợ.")
		case "image_too_large":
			return endpoint.Refuse(413, "image_too_large", "Ảnh bill vượt quá giới hạn 8 MB.")
		case "receipt_too_blurry":
			return endpoint.Refuse(422, "receipt_too_blurry", "Ảnh bill quá mờ. Vui lòng chụp lại ảnh rõ hơn.")
		case "receipt_reader_not_configured":
			return endpoint.Refuse(503, "receipt_reader_not_configured", "Máy chủ chưa cấu hình khoá đọc bill nên không gọi được AI. Đây là lỗi cấu hình phía máy chủ, không phải ảnh bạn chụp — chụp lại cũng không giúp được. Người dựng hệ cần đặt biến GEMINI_API_KEY rồi khởi động lại API.")
		case "not_a_receipt_price_list":
			return endpoint.Refuse(422, "not_a_receipt", "Đây là thực đơn hoặc bảng giá, không phải hoá đơn. Bảng giá chỉ nói món bao nhiêu tiền, không nói ai đã gọi gì, nên không chia tiền được. Hãy chụp tờ bill có dòng tổng tiền.")
		case "not_a_receipt":
			return endpoint.Refuse(422, "not_a_receipt", "Ảnh này không phải hoá đơn. Hãy chụp tờ bill có danh sách món và dòng tổng tiền.")
		case "receipt_unreadable":
			return endpoint.Refuse(422, "receipt_unreadable", "Không đọc được bill. Vui lòng kiểm tra ảnh và thử lại.")
		case "receipt_reader_unavailable", "brain_unavailable":
			return endpoint.Refuse(502, "receipt_reader_unavailable", "Không đọc được bill lúc này, thử lại sau.")
		}
		if refused.Status != 0 {
			return endpoint.Refuse(refused.Status, refused.Code, "Không đọc được bill lúc này, thử lại sau.")
		}
	}
	return err
}

func mapScreenshotErr(err error) error {
	if refused := brainErr(err); refused != nil {
		switch refused.Code {
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
		case "screenshot_reader_unavailable", "brain_unavailable":
			return endpoint.Refuse(502, "screenshot_reader_unavailable", "Dịch vụ đọc ảnh chụp màn hình đang lỗi phía máy chủ. Vui lòng thử lại sau.")
		}
	}
	return err
}

func mapFaceErr(err error) error {
	if refused := brainErr(err); refused != nil {
		switch refused.Code {
		case "face_detector_not_configured":
			return endpoint.Refuse(503, "face_detector_not_configured", "Máy chủ chưa cài mô hình tìm khuôn mặt nên không quét được ảnh. Đây là lỗi cấu hình phía máy chủ, không phải ảnh bạn chụp — chụp lại cũng không giúp được.")
		case "face_detector_unavailable", "face_detection_failed", "brain_unavailable":
			return endpoint.Refuse(502, "face_detection_failed", "Không tìm được khuôn mặt trong ảnh lúc này, thử lại sau.")
		}
	}
	return err
}

func wireReceiptScan(obj *pyjson.OrderedMap) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	for _, key := range []string{"items", "items_total_vnd", "total_vnd", "totals_agree", "total_difference_vnd", "needs_review", "warnings"} {
		if value, ok := obj.Get(key); ok {
			out.Set(key, value)
		} else if key == "warnings" {
			out.Set(key, pyjson.List{})
		} else {
			out.Set(key, pyjson.Null{})
		}
	}
	return out
}

func wireScreenshotScan(obj *pyjson.OrderedMap) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	for _, key := range []string{"source", "merchant", "total_vnd", "occurred_on", "needs_review"} {
		if value, ok := obj.Get(key); ok {
			out.Set(key, value)
		} else {
			out.Set(key, pyjson.Null{})
		}
	}
	return out
}

func uploadContentType(file *pyval.UploadFile) string {
	for _, header := range file.Headers {
		if header[0] == "content-type" {
			return header[1]
		}
	}
	return ""
}
