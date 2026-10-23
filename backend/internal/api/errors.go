package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// apiError is the single error shape every endpoint answers with (except the
// OAuth token endpoint, which follows RFC 6749). The code is stable and
// machine-readable; the message is for people and the AI, in the request's
// locale where a translation exists.
type apiError struct {
	Status       int            `json:"status"`
	Code         string         `json:"code"`
	Message      string         `json:"message"`
	RequiredRole string         `json:"required_role,omitempty"`
	RetryAfter   int            `json:"retry_after,omitempty"`
	Fields       []fieldError   `json:"fields,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
	RequestID    string         `json:"request_id,omitempty"`
}

type fieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *apiError) Error() string { return e.Code + ": " + e.Message }

func errf(status int, code, format string, args ...any) *apiError {
	return &apiError{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

func (e *apiError) with(key string, value any) *apiError {
	if e.Details == nil {
		e.Details = map[string]any{}
	}
	e.Details[key] = value
	return e
}

func notFound(what string) *apiError {
	return errf(http.StatusNotFound, what+"_not_found", "No %s with that id exists, or it is not yours.", strings.ReplaceAll(what, "_", " "))
}

// translations replace the English message for codes whose message is fixed.
var translations = map[string]map[string]string{
	"token_missing": {
		"ru": "Нужна авторизация: передайте токен доступа, API-ключ или cookie сессии.",
		"uz": "Avtorizatsiya kerak: kirish tokeni, API kalit yoki sessiya cookie yuboring.",
	},
	"token_expired": {
		"ru": "Срок действия токена доступа истёк. Обновите его через refresh-токен.",
		"uz": "Kirish tokeni muddati tugagan. Uni refresh token orqali yangilang.",
	},
	"token_invalid": {
		"ru": "Токен доступа недействителен.",
		"uz": "Kirish tokeni yaroqsiz.",
	},
	"session_ended": {
		"ru": "Сессия завершена. Войдите снова.",
		"uz": "Sessiya tugatilgan. Qaytadan kiring.",
	},
	"account_disabled": {
		"ru": "Учётная запись отключена администратором.",
		"uz": "Hisob administrator tomonidan o'chirilgan.",
	},
	"invalid_credentials": {
		"ru": "Неверный email или пароль.",
		"uz": "Email yoki parol noto'g'ri.",
	},
	"too_many_attempts": {
		"ru": "Слишком много попыток. Подождите и попробуйте снова.",
		"uz": "Juda ko'p urinish. Kuting va qaytadan urinib ko'ring.",
	},
	"validation_failed": {
		"ru": "Некоторые поля заполнены неверно. Исправьте все перечисленные поля.",
		"uz": "Ba'zi maydonlar noto'g'ri. Ko'rsatilgan barcha maydonlarni tuzating.",
	},
	"product_not_found": {
		"ru": "Товар с таким id не найден.",
		"uz": "Bunday id li mahsulot topilmadi.",
	},
	"order_not_found": {
		"ru": "Заказ с таким id не найден.",
		"uz": "Bunday id li buyurtma topilmadi.",
	},
	"order_already_paid": {
		"ru": "Заказ уже оплачен. Повторная оплата невозможна.",
		"uz": "Buyurtma allaqachon to'langan. Qayta to'lab bo'lmaydi.",
	},
	"insufficient_stock": {
		"ru": "Недостаточно товара на складе.",
		"uz": "Omborda mahsulot yetarli emas.",
	},
	"card_declined": {
		"ru": "Банк отклонил карту.",
		"uz": "Bank kartani rad etdi.",
	},
	"insufficient_funds": {
		"ru": "На карте недостаточно средств.",
		"uz": "Kartada mablag' yetarli emas.",
	},
}

func writeError(w http.ResponseWriter, r *http.Request, e *apiError) {
	if t, ok := translations[e.Code][locale(r)]; ok {
		e.Message = t
	}
	e.RequestID = w.Header().Get("X-Request-Id")
	if e.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(e.RetryAfter))
	}
	if e.Status == http.StatusUnauthorized && w.Header().Get("WWW-Authenticate") == "" {
		if e.Code == "token_missing" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="pharmacy"`)
		} else {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="pharmacy", error="invalid_token", error_description=%q`, e.Code))
		}
	}
	writeJSON(w, e.Status, map[string]any{"error": e})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

const maxBody = 1 << 20

// decode reads a JSON body strictly: broken JSON and unknown fields are 400,
// a value of the wrong type is a 422 naming the field.
func decode(r *http.Request, dst any) *apiError {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil {
		return errf(http.StatusBadRequest, "invalid_body", "The request body could not be read.")
	}
	if len(body) > maxBody {
		return errf(http.StatusRequestEntityTooLarge, "body_too_large", "The request body is larger than 1 MB.")
	}
	return decodeBytes(body, dst)
}

func decodeBytes(body []byte, dst any) *apiError {
	if len(strings.TrimSpace(string(body))) == 0 {
		return errf(http.StatusBadRequest, "invalid_json", "A JSON request body is required.")
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err == nil && dec.More() {
		err = errors.New("extra data after the JSON value")
	}
	if err == nil {
		return nil
	}
	var syntax *json.SyntaxError
	var typ *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		return errf(http.StatusBadRequest, "invalid_json", "The body is not valid JSON (error at byte %d).", syntax.Offset)
	case errors.As(err, &typ):
		field := typ.Field
		if field == "" {
			field = "(body)"
		}
		return &apiError{Status: http.StatusUnprocessableEntity, Code: "validation_failed",
			Message: "Some fields are invalid. Fix every field listed.",
			Fields:  []fieldError{{Field: field, Code: "invalid_type", Message: fmt.Sprintf("must be a %s, not a %s", typ.Type, typ.Value)}}}
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		name := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
		return errf(http.StatusBadRequest, "unknown_field", "The field %q is not part of this request. Remove it.", name).with("field", name)
	default:
		return errf(http.StatusBadRequest, "invalid_json", "The body is not valid JSON: %s.", err.Error())
	}
}

// validator collects every field problem so a 422 lists them all at once.
type validator struct{ fields []fieldError }

func (v *validator) check(ok bool, field, code, format string, args ...any) {
	if !ok {
		v.fields = append(v.fields, fieldError{Field: field, Code: code, Message: fmt.Sprintf(format, args...)})
	}
}

func (v *validator) add(field, code, format string, args ...any) {
	v.check(false, field, code, format, args...)
}

func (v *validator) err() *apiError {
	if len(v.fields) == 0 {
		return nil
	}
	return &apiError{Status: http.StatusUnprocessableEntity, Code: "validation_failed",
		Message: "Some fields are invalid. Fix every field listed.", Fields: v.fields}
}

// locale picks en, ru or uz from the Locale header, then Accept-Language.
func locale(r *http.Request) string {
	for _, h := range []string{r.Header.Get("Locale"), r.Header.Get("Accept-Language")} {
		for _, part := range strings.Split(h, ",") {
			tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
			if len(tag) >= 2 {
				switch tag[:2] {
				case "en", "ru", "uz":
					return tag[:2]
				}
			}
		}
	}
	return "en"
}
