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
		reading, err := docHoaDon(ctx, call.AI, file.Content, uploadContentType(file))
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wireReceiptScan(reading)}, nil
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
		reading, err := docManHinh(ctx, call.AI, file.Content, uploadContentType(file))
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wireScreenshotScan(reading)}, nil
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

func uploadContentType(file *pyval.UploadFile) string {
	for _, header := range file.Headers {
		if header[0] == "content-type" {
			return header[1]
		}
	}
	return ""
}
