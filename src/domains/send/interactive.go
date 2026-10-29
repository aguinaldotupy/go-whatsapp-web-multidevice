package send

import "encoding/json"

// InteractiveRequest is an experimental, immediate-only native-flow message.
// Params uses WhatsApp's button-specific JSON, not XML in the message body.
type InteractiveRequest struct {
	Phone   string             `json:"phone" form:"phone"`
	Body    string             `json:"body" form:"body"`
	Footer  string             `json:"footer,omitempty" form:"footer"`
	Buttons []NativeFlowButton `json:"buttons,omitempty" form:"buttons"`
	Cards   []InteractiveCard  `json:"cards,omitempty" form:"cards"`
}

type NativeFlowButton struct {
	Name   string          `json:"name" form:"name"`
	Params json.RawMessage `json:"params" form:"params"`
}

// InteractiveButton is an alias for NativeFlowButton for backward compatibility.
type InteractiveButton = NativeFlowButton

type InteractiveCard struct {
	ImageBase64 string             `json:"image_base64,omitempty" form:"image_base64"`
	Title       string             `json:"title,omitempty" form:"title"`
	Body        string             `json:"body" form:"body"`
	Footer      string             `json:"footer,omitempty" form:"footer"`
	Buttons     []NativeFlowButton `json:"buttons" form:"buttons"`
}
