package limit

import "fmt"

// The ceilings the Python API ships with. None is configurable: no
// environment variable reaches them, so there is nothing to parse.
const (
	// search_rate_limit.py
	SearchWindowSeconds                        = 60
	SearchLimitPerWindow                       = 12
	ReceiptScanWindowSeconds                   = 60
	ReceiptScanLimitPerWindow                  = 30
	ChatExpenseWindowSeconds                   = ReceiptScanWindowSeconds
	ChatExpenseLimitPerWindow                  = ReceiptScanLimitPerWindow
	ScreenshotScanWindowSeconds                = ReceiptScanWindowSeconds
	ScreenshotScanLimitPerWindow               = ReceiptScanLimitPerWindow
	SuggestionWindowSeconds                    = ReceiptScanWindowSeconds
	SuggestionLimitPerWindow                   = ReceiptScanLimitPerWindow
	ContextualSuggestionWindowSeconds          = ReceiptScanWindowSeconds
	ContextualSuggestionLimitPerWindow         = ReceiptScanLimitPerWindow
	ReelWindowSeconds                          = ReceiptScanWindowSeconds
	ReelLimitPerWindow                         = ReceiptScanLimitPerWindow
	FaceDetectionWindowSeconds                 = ReceiptScanWindowSeconds
	FaceDetectionLimitPerWindow                = ReceiptScanLimitPerWindow
	ItineraryWindowSeconds                     = 60 // literal in main.py create_app
	ItineraryLimitPerWindow                    = 30 // literal in main.py create_app
	PersonIDRateLimit                          = 20 // routes/identity.py RATE_LIMIT
	PersonIDWindowSeconds              float64 = 60.0
	FriendLookupRateLimit                      = 30 // routes/friends.py LOOKUP_RATE_LIMIT
	FriendLookupWindowSeconds          float64 = 60.0
	OTPRequestLimit                            = 10 // routes/auth.py REQUEST_LIMIT
	OTPVerifyLimit                             = 30 // routes/auth.py VERIFY_LIMIT
	GoogleLoginLimit                           = 10 // routes/auth.py GOOGLE_LIMIT
	AuthWindowSeconds                  float64 = 60.0
)

// vietnameseCeiling is the sentence shape the build_* functions share.
func vietnameseCeiling(what string, limit, window int) string {
	return fmt.Sprintf("Quá nhiều lượt %s; tối đa %d lượt mỗi %d giây. Thử lại sau ít phút.", what, limit, window)
}

// Actor-keyed configurations, one per build_* function (and main.py's
// inline itinerary limiter).
var (
	SearchConfig = ActorConfig{
		Limit:         SearchLimitPerWindow,
		WindowSeconds: SearchWindowSeconds,
		Code:          "search_rate_limited",
		Message:       fmt.Sprintf("Too many searches; at most %d per %d seconds.", SearchLimitPerWindow, SearchWindowSeconds),
	}
	ItineraryConfig = ActorConfig{
		Limit:         ItineraryLimitPerWindow,
		WindowSeconds: ItineraryWindowSeconds,
		Code:          "itinerary_rate_limited",
		Message:       "Đã tính nhiều tuyến liên tiếp. Chờ một phút rồi thử lại.",
	}
	ReceiptScanConfig = ActorConfig{
		Limit:         ReceiptScanLimitPerWindow,
		WindowSeconds: ReceiptScanWindowSeconds,
		Code:          "scan_rate_limited",
		Message:       vietnameseCeiling("đọc bill", ReceiptScanLimitPerWindow, ReceiptScanWindowSeconds),
	}
	ChatExpenseConfig = ActorConfig{
		Limit:         ChatExpenseLimitPerWindow,
		WindowSeconds: ChatExpenseWindowSeconds,
		Code:          "chat_expense_rate_limited",
		Message:       vietnameseCeiling("đọc khoản chi từ tin nhắn", ChatExpenseLimitPerWindow, ChatExpenseWindowSeconds),
	}
	ScreenshotScanConfig = ActorConfig{
		Limit:         ScreenshotScanLimitPerWindow,
		WindowSeconds: ScreenshotScanWindowSeconds,
		Code:          "screenshot_scan_rate_limited",
		Message:       vietnameseCeiling("đọc ảnh chụp màn hình", ScreenshotScanLimitPerWindow, ScreenshotScanWindowSeconds),
	}
	SuggestionConfig = ActorConfig{
		Limit:         SuggestionLimitPerWindow,
		WindowSeconds: SuggestionWindowSeconds,
		Code:          "suggestion_rate_limited",
		Message:       vietnameseCeiling("xin gợi ý", SuggestionLimitPerWindow, SuggestionWindowSeconds),
	}
	ContextualSuggestionConfig = ActorConfig{
		Limit:         ContextualSuggestionLimitPerWindow,
		WindowSeconds: ContextualSuggestionWindowSeconds,
		Code:          "contextual_suggestion_rate_limited",
		Message:       vietnameseCeiling("xin gợi ý theo cuộc trò chuyện", ContextualSuggestionLimitPerWindow, ContextualSuggestionWindowSeconds),
	}
	ReelConfig = ActorConfig{
		Limit:         ReelLimitPerWindow,
		WindowSeconds: ReelWindowSeconds,
		Code:          "reel_rate_limited",
		Message:       vietnameseCeiling("dựng thước phim kỷ niệm", ReelLimitPerWindow, ReelWindowSeconds),
	}
	FaceDetectionConfig = ActorConfig{
		Limit:         FaceDetectionLimitPerWindow,
		WindowSeconds: FaceDetectionWindowSeconds,
		Code:          "face_detection_rate_limited",
		Message:       vietnameseCeiling("tìm khuôn mặt trong ảnh", FaceDetectionLimitPerWindow, FaceDetectionWindowSeconds),
	}
)

// Address-keyed configurations. The class takes only limit and window; the
// code and sentence are the ones each route raises when allow() is false.
var (
	PersonIDConfig = AddressConfig{
		Limit:         PersonIDRateLimit,
		WindowSeconds: PersonIDWindowSeconds,
		Code:          "rate_limited",
		Detail:        "Thử lại sau một phút. Máy chủ đang giới hạn số lần tra danh tính.",
	}
	FriendLookupConfig = AddressConfig{
		Limit:         FriendLookupRateLimit,
		WindowSeconds: FriendLookupWindowSeconds,
		Code:          "rate_limited",
		Detail:        "Thử lại sau một phút. Máy chủ đang giới hạn số lần tìm bạn.",
	}
	OTPRequestConfig = AddressConfig{
		Limit:         OTPRequestLimit,
		WindowSeconds: AuthWindowSeconds,
		Code:          "rate_limited",
		Detail:        "Thử lại sau một phút.",
	}
	OTPVerifyConfig = AddressConfig{
		Limit:         OTPVerifyLimit,
		WindowSeconds: AuthWindowSeconds,
		Code:          "rate_limited",
		Detail:        "Thử lại sau một phút.",
	}
	GoogleLoginConfig = AddressConfig{
		Limit:         GoogleLoginLimit,
		WindowSeconds: AuthWindowSeconds,
		Code:          "rate_limited",
		Detail:        "Thử lại sau một phút.",
	}
)

// Set is every window one Python application instance owns, under the
// app.state attribute names routes.json lists as `in_memory`. Build one per
// server, never per request: a limiter built per request counts to one and
// forgets. Actor windows are keyed by auth.Actor.ID (canonical UUID text,
// which is equal exactly when Python's UUID keys are); address windows by
// Caller.
type Set struct {
	SearchLimiter               *ActorWindow[string] // POST /places/search
	ItineraryLimiter            *ActorWindow[string] // POST /outings/{outing_id}/itinerary/preview
	ReceiptScanLimiter          *ActorWindow[string] // POST /receipts/scan
	ChatExpenseLimiter          *ActorWindow[string] // POST /contexts/{context_id}/messages/{message_id}/expense-draft
	ScreenshotScanLimiter       *ActorWindow[string] // POST /screenshots/scan
	SuggestionLimiter           *ActorWindow[string] // GET /contexts/{context_id}/suggestion
	ContextualSuggestionLimiter *ActorWindow[string] // GET /contexts/{context_id}/contextual-suggestion
	FaceDetectionLimiter        *ActorWindow[string] // POST /contexts/{context_id}/photos/{photo_id}/face-boxes
	ReelLimiter                 *ActorWindow[string] // GET /contexts/{context_id}/albums/{outing_id}/reel

	PersonIDLimit     *AddressWindow // POST /identity/person-id
	FriendLookupLimit *AddressWindow // POST /friends/lookup
	OTPRequestLimit   *AddressWindow // POST /auth/otp/request
	OTPVerifyLimit    *AddressWindow // POST /auth/otp/verify
	GoogleLoginLimit  *AddressWindow // POST /auth/google
}

// NewSet builds every window in create_app's order, each actor window
// reading the clock once as Python's constructors do. Python builds the
// address windows lazily on first request; they never read the clock at
// construction, so building them here is indistinguishable.
func NewSet(clock Clock) *Set {
	return &Set{
		SearchLimiter:               NewActorWindow[string](SearchConfig, clock),
		ItineraryLimiter:            NewActorWindow[string](ItineraryConfig, clock),
		ReceiptScanLimiter:          NewActorWindow[string](ReceiptScanConfig, clock),
		ChatExpenseLimiter:          NewActorWindow[string](ChatExpenseConfig, clock),
		ScreenshotScanLimiter:       NewActorWindow[string](ScreenshotScanConfig, clock),
		SuggestionLimiter:           NewActorWindow[string](SuggestionConfig, clock),
		ContextualSuggestionLimiter: NewActorWindow[string](ContextualSuggestionConfig, clock),
		FaceDetectionLimiter:        NewActorWindow[string](FaceDetectionConfig, clock),
		ReelLimiter:                 NewActorWindow[string](ReelConfig, clock),

		PersonIDLimit:     NewAddressWindow(PersonIDConfig, clock),
		FriendLookupLimit: NewAddressWindow(FriendLookupConfig, clock),
		OTPRequestLimit:   NewAddressWindow(OTPRequestConfig, clock),
		OTPVerifyLimit:    NewAddressWindow(OTPVerifyConfig, clock),
		GoogleLoginLimit:  NewAddressWindow(GoogleLoginConfig, clock),
	}
}
