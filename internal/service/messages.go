package service

import (
	"errors"
	"fmt"
)

type MessageRef struct {
	Code   string            `json:"code,omitempty"`
	Params map[string]string `json:"params,omitempty"`
}

type localizedError struct {
	ref   MessageRef
	cause error
}

func (e *localizedError) Error() string {
	if text := fallbackMessageText(e.ref); text != "" {
		return text
	}
	if e.cause != nil {
		return e.cause.Error()
	}
	return e.ref.Code
}

func (e *localizedError) Unwrap() error {
	return e.cause
}

func newMessageRef(code string, params map[string]string) MessageRef {
	ref := MessageRef{Code: code}
	if len(params) == 0 {
		return ref
	}
	ref.Params = cloneMessageParams(params)
	return ref
}

func cloneMessageParams(params map[string]string) map[string]string {
	if len(params) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(params))
	for key, value := range params {
		cloned[key] = value
	}
	return cloned
}

func cloneMessageRef(ref MessageRef) MessageRef {
	if ref.Code == "" {
		return MessageRef{}
	}
	return MessageRef{
		Code:   ref.Code,
		Params: cloneMessageParams(ref.Params),
	}
}

func messageRefFromError(err error) MessageRef {
	if err == nil {
		return MessageRef{}
	}
	var target *localizedError
	if errors.As(err, &target) && target != nil {
		return cloneMessageRef(target.ref)
	}
	return MessageRef{}
}

func localizedErrorf(code string, params map[string]string, format string, args ...any) error {
	return &localizedError{
		ref:   newMessageRef(code, params),
		cause: fmt.Errorf(format, args...),
	}
}

func fallbackMessageText(ref MessageRef) string {
	switch ref.Code {
	case "backend.hint.game_path_missing":
		return "No game directory is configured yet. Launch Game stays disabled until you set one."
	case "backend.hint.game_path_invalid":
		return "The current game directory is invalid. Select a valid path before launching the game."
	case "backend.hint.qr_expand_manual":
		return "QR login is not open in the game window. Please switch to QR login manually."
	case "backend.hint.qr_refresh_manual":
		return "The QR code has expired. Please click Refresh in the game window."
	case "backend.hint.qr_panel_unrecognized":
		return "Login panel was not recognized in the captured game window. Open the login window and try again."
	case "backend.hint.qr_visible_but_unreadable":
		return "A QR area is visible, but no usable QR code was decoded. Make sure the QR is clear and retry."
	case "backend.error.credentials_required":
		return "Account and password are required."
	case "backend.error.captcha_url_empty":
		return "Captcha URL is empty."
	case "backend.error.ticket_required":
		return "Ticket is required."
	case "backend.error.session_not_ready":
		return "Game session is not ready; login first."
	case "backend.error.scan_blocked":
		if reason := ref.Params["reason"]; reason != "" {
			return fmt.Sprintf("Scan blocked: %s", reason)
		}
		return "Scan blocked."
	case "backend.error.verify_retcode":
		source := ref.Params["source"]
		retcode := ref.Params["retcode"]
		switch {
		case source != "" && retcode != "":
			return fmt.Sprintf("%s verify retcode=%s", source, retcode)
		case source != "":
			return fmt.Sprintf("%s verify failed", source)
		case retcode != "":
			return fmt.Sprintf("verify retcode=%s", retcode)
		default:
			return "Verify failed."
		}
	case "backend.error.dispatch_retcode":
		if retcode := ref.Params["retcode"]; retcode != "" {
			return fmt.Sprintf("dispatch retcode=%s", retcode)
		}
		return "Dispatch request failed."
	case "backend.error.dispatch_missing_data":
		return "Dispatch response is missing data."
	case "backend.error.dispatch_invalid_blob":
		return "Dispatch response is not a usable final blob."
	case "backend.error.empty_bilihitoken":
		return "Fetched empty BILIHITOKEN."
	case "backend.error.auto_fetch_bilihitoken_failed":
		return "Automatic BILIHITOKEN refresh failed. Please fetch it manually."
	default:
		return ""
	}
}
