package send

import "encoding/json"

// InteractiveButton is one native flow button of an interactive message.
// Name is the WhatsApp button type (cta_url, cta_copy, cta_call, quick_reply,
// galaxy_message, ...) and Params its buttonParamsJSON object, passed through
// verbatim: the accepted keys depend on the button type (display_text plus
// url / copy_code / phone_number / id, or the flow_* keys of a form).
type InteractiveButton struct {
	Name   string          `json:"name" form:"name"`
	Params json.RawMessage `json:"params" form:"params"`
}

// InteractiveRequest sends an interactive message: optional header title,
// body and footer text plus one or more native flow buttons.
type InteractiveRequest struct {
	BaseRequest
	Header  string              `json:"header" form:"header"`
	Body    string              `json:"body" form:"body"`
	Footer  string              `json:"footer" form:"footer"`
	Buttons []InteractiveButton `json:"buttons" form:"buttons"`
}
