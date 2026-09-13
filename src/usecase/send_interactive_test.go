package usecase

import (
	"encoding/json"
	"testing"

	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestBuildInteractiveMessage(t *testing.T) {
	// The form params observed from the WhatsApp Business app (galaxy_message).
	formParams := `{"flow_id":"1332768858934028","flow_cta":"Complete form","flow_action":"navigate","flow_token":"0ACEA40F","flow_message_version":"4","form_type":"ai_form_attachment_tray","flow_action_payload":{"screen":"contact_details","data":{"custom_fields":[{"label":"Atendimento","type":"TEXT_INPUT"}]}}}`
	duration := 86400
	msg := buildInteractiveMessage(domainSend.InteractiveRequest{
		BaseRequest: domainSend.BaseRequest{Phone: "6289685024051@s.whatsapp.net", Duration: &duration},
		Header:      " Campanha ",
		Body:        "Colher a experiência do evento",
		Footer:      "",
		Buttons: []domainSend.InteractiveButton{
			{Name: "galaxy_message", Params: json.RawMessage(formParams)},
			{Name: "cta_url", Params: json.RawMessage(`{"display_text":"Site","url":"https://example.com"}`)},
		},
	})

	im := msg.GetInteractiveMessage()
	if assert.NotNil(t, im) {
		assert.Equal(t, "Campanha", im.GetHeader().GetTitle())
		assert.False(t, im.GetHeader().GetHasMediaAttachment())
		assert.Equal(t, "Colher a experiência do evento", im.GetBody().GetText())
		assert.Nil(t, im.GetFooter(), "blank footer must not produce an empty footer")
		buttons := im.GetNativeFlowMessage().GetButtons()
		if assert.Len(t, buttons, 2) {
			assert.Equal(t, "galaxy_message", buttons[0].GetName())
			assert.JSONEq(t, formParams, buttons[0].GetButtonParamsJSON(), "params must pass through untouched")
			assert.Equal(t, "cta_url", buttons[1].GetName())
		}
		assert.EqualValues(t, 1, im.GetNativeFlowMessage().GetMessageVersion())
		assert.EqualValues(t, 86400, im.GetContextInfo().GetExpiration())
	}
	assert.NotNil(t, msg.GetMessageContextInfo().GetDeviceListMetadata())
	assert.EqualValues(t, 2, msg.GetMessageContextInfo().GetDeviceListMetadataVersion())

	// What chat storage / Chatwoot will show for the sent message.
	assert.Equal(t, "Campanha\nColher a experiência do evento\n📝 [Complete form]\n🔗 Site: https://example.com", utils.FormatInteractiveMessageSummary(im))
}

func TestBuildInteractiveMessageForwarded(t *testing.T) {
	msg := buildInteractiveMessage(domainSend.InteractiveRequest{
		BaseRequest: domainSend.BaseRequest{Phone: "6289685024051@s.whatsapp.net", IsForwarded: true},
		Body:        "Hi",
		Buttons:     []domainSend.InteractiveButton{{Name: "quick_reply", Params: json.RawMessage(`{"display_text":"Yes","id":"yes"}`)}},
	})
	ci := msg.GetInteractiveMessage().GetContextInfo()
	assert.True(t, ci.GetIsForwarded())
	assert.EqualValues(t, 100, ci.GetForwardingScore())
	assert.Zero(t, ci.GetExpiration())
}

func TestBuildInteractiveMessageWithoutDuration(t *testing.T) {
	msg := buildInteractiveMessage(domainSend.InteractiveRequest{
		BaseRequest: domainSend.BaseRequest{Phone: "6289685024051@s.whatsapp.net"},
		Body:        "Hi",
		Buttons:     []domainSend.InteractiveButton{{Name: "quick_reply", Params: json.RawMessage(`{"display_text":"Yes","id":"yes"}`)}},
	})
	assert.Nil(t, msg.GetInteractiveMessage().GetContextInfo())
	assert.Nil(t, msg.GetInteractiveMessage().GetHeader())
}
