package whatsapp

import (
	"context"
	"testing"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/stretchr/testify/assert"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func TestBuildEventPayloadIncludesIsFromMe(t *testing.T) {
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     types.NewJID("123", types.DefaultUserServer),
				Sender:   types.NewJID("123", types.DefaultUserServer),
				IsFromMe: true,
			},
			ID:        "MSG123",
			Timestamp: time.Date(2026, time.February, 8, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			Conversation: protoString("hello"),
		},
	}

	eventType, payload, err := buildEventPayload(context.Background(), nil, evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if eventType != EventTypeMessage {
		t.Fatalf("expected event type %s, got %s", EventTypeMessage, eventType)
	}
	if value, ok := payload["is_from_me"]; !ok {
		t.Fatalf("expected is_from_me in payload")
	} else if isFromMe, ok := value.(bool); !ok || !isFromMe {
		t.Fatalf("expected is_from_me=true, got %v", value)
	}
}

func TestBuildEventPayloadIncludesPreparedPollAndStableBody(t *testing.T) {
	selected := []string{"Sushi"}
	hashes := []string{pollOptionHash("Sushi")}
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:   types.NewJID("120363000000", types.GroupServer),
				Sender: types.NewJID("628222", types.DefaultUserServer),
			},
			ID:        "VOTE-1",
			Timestamp: time.Date(2026, time.August, 26, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{PollUpdateMessage: &waE2E.PollUpdateMessage{}},
	}
	poll := &webhookPollPayload{
		Type:                 "vote",
		PollID:               "POLL-1",
		Question:             "Lunch?",
		SelectedOptions:      &selected,
		SelectedOptionHashes: &hashes,
		ResolutionStatus:     pollResolutionResolved,
	}

	eventType, payload, err := buildEventPayload(context.Background(), nil, evt, poll)
	if err != nil {
		t.Fatalf("buildEventPayload: %v", err)
	}
	if eventType != EventTypeMessage {
		t.Fatalf("event type = %q, want %q", eventType, EventTypeMessage)
	}
	if payload["poll"] != poll {
		t.Fatalf("poll payload = %#v, want prepared payload", payload["poll"])
	}
	if payload["body"] != "Poll vote: Sushi" {
		t.Fatalf("body = %v, want stable poll summary", payload["body"])
	}
}

func TestBuildEventPayloadRevokedIncludesIsFromMe(t *testing.T) {
	key := &waCommon.MessageKey{
		RemoteJID: protoString("123@s.whatsapp.net"),
		FromMe:    protoBool(true),
		ID:        protoString("REV123"),
	}
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     types.NewJID("123", types.DefaultUserServer),
				Sender:   types.NewJID("123", types.DefaultUserServer),
				IsFromMe: true,
			},
			ID:        "MSG124",
			Timestamp: time.Date(2026, time.February, 8, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			ProtocolMessage: &waE2E.ProtocolMessage{
				Type: protoProtocolMessageType(waE2E.ProtocolMessage_REVOKE),
				Key:  key,
			},
		},
	}

	eventType, payload, err := buildEventPayload(context.Background(), nil, evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if eventType != EventTypeMessageRevoked {
		t.Fatalf("expected event type %s, got %s", EventTypeMessageRevoked, eventType)
	}
	if value, ok := payload["is_from_me"]; !ok {
		t.Fatalf("expected is_from_me in payload")
	} else if isFromMe, ok := value.(bool); !ok || !isFromMe {
		t.Fatalf("expected is_from_me=true, got %v", value)
	}
}

func TestBuildEventPayloadReactionIncludesTargetMessageID(t *testing.T) {
	evt := reactionEventForTest("reaction-event-1", "MSG100", "\U0001f44d")

	eventType, payload, err := buildEventPayload(context.Background(), nil, evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if eventType != EventTypeMessageReaction {
		t.Fatalf("expected event type %s, got %s", EventTypeMessageReaction, eventType)
	}
	if got := payload["reaction"]; got != "\U0001f44d" {
		t.Fatalf("expected reaction payload, got %v", got)
	}
	if got := payload["reacted_message_id"]; got != "MSG100" {
		t.Fatalf("expected reacted message id MSG100, got %v", got)
	}
}

func TestBuildEventPayloadReactionWithoutKeyDoesNotPanic(t *testing.T) {
	evt := reactionEventForTest("reaction-event-2", "MSG101", "\U0001f44d")
	evt.Message.ReactionMessage.Key = nil

	eventType, payload, err := buildEventPayload(context.Background(), nil, evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if eventType != EventTypeMessageReaction {
		t.Fatalf("expected event type %s, got %s", EventTypeMessageReaction, eventType)
	}
	if _, ok := payload["reacted_message_id"]; ok {
		t.Fatalf("expected no reacted_message_id when reaction key is missing")
	}
}

func protoString(value string) *string {
	return &value
}

func protoBool(value bool) *bool {
	return &value
}

func protoProtocolMessageType(value waE2E.ProtocolMessage_Type) *waE2E.ProtocolMessage_Type {
	return &value
}

func TestBuildEventPayloadImageWithCaption(t *testing.T) {
	config.WhatsappAutoDownloadMedia = false
	caption := "Check this out!"
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     types.NewJID("123", types.DefaultUserServer),
				Sender:   types.NewJID("456", types.DefaultUserServer),
				IsFromMe: false,
			},
			ID:        "MSG200",
			Timestamp: time.Date(2026, time.February, 8, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{
				Caption: &caption,
			},
		},
	}

	eventType, payload, err := buildEventPayload(context.Background(), nil, evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if eventType != EventTypeMessage {
		t.Fatalf("expected event type %s, got %s", EventTypeMessage, eventType)
	}
	body, ok := payload["body"]
	if !ok {
		t.Fatal("expected body in payload for image with caption")
	}
	if body != "Check this out!" {
		t.Fatalf("expected body='Check this out!', got %v", body)
	}
}

func TestBuildEventPayloadVideoWithCaption(t *testing.T) {
	config.WhatsappAutoDownloadMedia = false
	caption := "Watch this video"
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     types.NewJID("123", types.DefaultUserServer),
				Sender:   types.NewJID("456", types.DefaultUserServer),
				IsFromMe: false,
			},
			ID:        "MSG201",
			Timestamp: time.Date(2026, time.February, 8, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			VideoMessage: &waE2E.VideoMessage{
				Caption: &caption,
			},
		},
	}

	eventType, payload, err := buildEventPayload(context.Background(), nil, evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if eventType != EventTypeMessage {
		t.Fatalf("expected event type %s, got %s", EventTypeMessage, eventType)
	}
	body, ok := payload["body"]
	if !ok {
		t.Fatal("expected body in payload for video with caption")
	}
	if body != "Watch this video" {
		t.Fatalf("expected body='Watch this video', got %v", body)
	}
}

func TestBuildEventPayloadImageWithoutCaption(t *testing.T) {
	config.WhatsappAutoDownloadMedia = false
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     types.NewJID("123", types.DefaultUserServer),
				Sender:   types.NewJID("456", types.DefaultUserServer),
				IsFromMe: false,
			},
			ID:        "MSG202",
			Timestamp: time.Date(2026, time.February, 8, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{},
		},
	}

	_, payload, err := buildEventPayload(context.Background(), nil, evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, ok := payload["body"]; ok {
		t.Fatal("expected no body in payload for image without caption")
	}
}

func TestBuildEventPayloadDocumentWithCaption(t *testing.T) {
	config.WhatsappAutoDownloadMedia = false
	caption := "Important document"
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     types.NewJID("123", types.DefaultUserServer),
				Sender:   types.NewJID("456", types.DefaultUserServer),
				IsFromMe: false,
			},
			ID:        "MSG203",
			Timestamp: time.Date(2026, time.February, 8, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			DocumentMessage: &waE2E.DocumentMessage{
				Caption: &caption,
			},
		},
	}

	eventType, payload, err := buildEventPayload(context.Background(), nil, evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if eventType != EventTypeMessage {
		t.Fatalf("expected event type %s, got %s", EventTypeMessage, eventType)
	}
	body, ok := payload["body"]
	if !ok {
		t.Fatal("expected body in payload for document with caption")
	}
	if body != "Important document" {
		t.Fatalf("expected body='Important document', got %v", body)
	}
}

func TestBuildEventPayloadQuotedBodyUsesQuotedCaption(t *testing.T) {
	oldAutoDownload := config.WhatsappAutoDownloadMedia
	config.WhatsappAutoDownloadMedia = false
	t.Cleanup(func() {
		config.WhatsappAutoDownloadMedia = oldAutoDownload
	})

	tests := []struct {
		name           string
		quotedMessage  func(*string) *waE2E.Message
		wantQuotedBody string
	}{
		{
			name: "uses quoted image caption",
			quotedMessage: func(caption *string) *waE2E.Message {
				return &waE2E.Message{
					ImageMessage: &waE2E.ImageMessage{
						Caption: caption,
					},
				}
			},
			wantQuotedBody: "Launch checklist",
		},
		{
			name: "uses quoted document caption",
			quotedMessage: func(caption *string) *waE2E.Message {
				return &waE2E.Message{
					DocumentMessage: &waE2E.DocumentMessage{
						Caption: caption,
					},
				}
			},
			wantQuotedBody: "Project brief",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replyText := "Thanks for the update"
			quotedCaption := tt.wantQuotedBody
			evt := &events.Message{
				Info: types.MessageInfo{
					MessageSource: types.MessageSource{
						Chat:     types.NewJID("123", types.DefaultUserServer),
						Sender:   types.NewJID("456", types.DefaultUserServer),
						IsFromMe: false,
					},
					ID:        "MSG206",
					Timestamp: time.Date(2026, time.February, 8, 10, 0, 0, 0, time.UTC),
				},
				Message: &waE2E.Message{
					ExtendedTextMessage: &waE2E.ExtendedTextMessage{
						Text: &replyText,
						ContextInfo: &waE2E.ContextInfo{
							StanzaID:      protoString("QUOTE206"),
							QuotedMessage: tt.quotedMessage(&quotedCaption),
						},
					},
				},
			}

			eventType, payload, err := buildEventPayload(context.Background(), nil, evt)
			assert.NoError(t, err)
			assert.Equal(t, EventTypeMessage, eventType)
			assert.Contains(t, payload, "quoted_body")
			assert.Equal(t, tt.wantQuotedBody, payload["quoted_body"])
		})
	}
}

func TestBuildEventPayloadContactIncludesPhoneNumber(t *testing.T) {
	name := "Alice"
	vcard := "BEGIN:VCARD\nVERSION:3.0\nN:;Alice;;;\nFN:Alice\nTEL;type=Mobile:+62 812 3456 7890\nEND:VCARD"
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     types.NewJID("123", types.DefaultUserServer),
				Sender:   types.NewJID("456", types.DefaultUserServer),
				IsFromMe: false,
			},
			ID:        "MSG204",
			Timestamp: time.Date(2026, time.February, 8, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			ContactMessage: &waE2E.ContactMessage{
				DisplayName: &name,
				Vcard:       &vcard,
			},
		},
	}

	_, payload, err := buildEventPayload(context.Background(), nil, evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	contact, ok := payload["contact"].(webhookContactPayload)
	if !ok {
		t.Fatalf("expected contact payload to be webhookContactPayload, got %T", payload["contact"])
	}
	if contact.DisplayName != "Alice" {
		t.Fatalf("expected display name Alice, got %q", contact.DisplayName)
	}
	if contact.PhoneNumber != "+62 812 3456 7890" {
		t.Fatalf("expected phone number from vCard, got %q", contact.PhoneNumber)
	}
}

func TestBuildEventPayloadContactsArrayIncludesPhoneNumbers(t *testing.T) {
	nameOne := "Alice"
	vcardOne := "BEGIN:VCARD\nVERSION:3.0\nN:;Alice;;;\nFN:Alice\nTEL;type=Mobile:+62 812 3456 7890\nEND:VCARD"
	nameTwo := "Bob"
	vcardTwo := "BEGIN:VCARD\nVERSION:3.0\nN:;Bob;;;\nFN:Bob\nTEL;type=Mobile:+62 813 9876 5432\nEND:VCARD"
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     types.NewJID("123", types.DefaultUserServer),
				Sender:   types.NewJID("456", types.DefaultUserServer),
				IsFromMe: false,
			},
			ID:        "MSG205",
			Timestamp: time.Date(2026, time.February, 8, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			ContactsArrayMessage: &waE2E.ContactsArrayMessage{
				Contacts: []*waE2E.ContactMessage{
					{
						DisplayName: &nameOne,
						Vcard:       &vcardOne,
					},
					{
						DisplayName: &nameTwo,
						Vcard:       &vcardTwo,
					},
				},
			},
		},
	}

	_, payload, err := buildEventPayload(context.Background(), nil, evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	contacts, ok := payload["contacts_array"].([]webhookContactPayload)
	if !ok {
		t.Fatalf("expected contacts_array to be []webhookContactPayload, got %T", payload["contacts_array"])
	}
	if len(contacts) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(contacts))
	}
	if contacts[0].PhoneNumber != "+62 812 3456 7890" {
		t.Fatalf("expected first phone number, got %q", contacts[0].PhoneNumber)
	}
	if contacts[1].PhoneNumber != "+62 813 9876 5432" {
		t.Fatalf("expected second phone number, got %q", contacts[1].PhoneNumber)
	}
}

func TestBuildEventPayloadIncludesSenderDisplayName(t *testing.T) {
	accountJID := types.NewJID("628111111111", types.DefaultUserServer)
	senderJID := types.NewJID("628123456789", types.DefaultUserServer)
	contacts := &senderDisplayNameContactGetter{
		contacts: map[types.JID]types.ContactInfo{
			senderJID: {
				Found:    true,
				FullName: "Saved Contact",
			},
		},
	}
	client := &whatsmeow.Client{
		Store: &store.Device{
			ID:       &accountJID,
			PushName: "Active Account",
			Contacts: contacts,
		},
	}

	tests := []struct {
		name      string
		isFromMe  bool
		sender    types.JID
		message   *waE2E.Message
		wantEvent string
		wantName  string
	}{
		{
			name:      "message",
			sender:    senderJID,
			message:   &waE2E.Message{Conversation: protoString("hello")},
			wantEvent: EventTypeMessage,
			wantName:  "Saved Contact",
		},
		{
			name:   "reaction before early return",
			sender: senderJID,
			message: &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{
				Key: &waCommon.MessageKey{
					RemoteJID: protoString("628100000000@s.whatsapp.net"),
					ID:        protoString("target-message"),
				},
				Text: protoString("👍"),
			}},
			wantEvent: EventTypeMessageReaction,
			wantName:  "Saved Contact",
		},
		{
			name:   "revoke before early return",
			sender: senderJID,
			message: &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
				Type: protoProtocolMessageType(waE2E.ProtocolMessage_REVOKE),
				Key:  &waCommon.MessageKey{ID: protoString("revoked-message")},
			}},
			wantEvent: EventTypeMessageRevoked,
			wantName:  "Saved Contact",
		},
		{
			name:   "edit before early return",
			sender: senderJID,
			message: &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
				Type: protoProtocolMessageType(waE2E.ProtocolMessage_MESSAGE_EDIT),
				Key:  &waCommon.MessageKey{ID: protoString("edited-message")},
				EditedMessage: &waE2E.Message{
					Conversation: protoString("updated"),
				},
			}},
			wantEvent: EventTypeMessageEdited,
			wantName:  "Saved Contact",
		},
		{
			name:      "outgoing active account",
			isFromMe:  true,
			sender:    accountJID,
			message:   &waE2E.Message{Conversation: protoString("hello")},
			wantEvent: EventTypeMessage,
			wantName:  "Active Account",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &events.Message{
				Info: types.MessageInfo{
					MessageSource: types.MessageSource{
						Chat:     types.NewJID("628100000000", types.DefaultUserServer),
						Sender:   tt.sender,
						IsFromMe: tt.isFromMe,
					},
					ID:        "sender-display-name",
					PushName:  "Live Push",
					Timestamp: time.Date(2026, time.July, 28, 10, 0, 0, 0, time.UTC),
				},
				Message: tt.message,
			}

			eventType, payload, err := buildEventPayload(context.Background(), client, evt)

			assert.NoError(t, err)
			assert.Equal(t, tt.wantEvent, eventType)
			assert.Equal(t, tt.sender.String(), payload["from"])
			assert.Equal(t, "Live Push", payload["from_name"])
			assert.Equal(t, tt.wantName, payload["sender_display_name"])
		})
	}
}

func TestBuildEventPayloadSenderContactName(t *testing.T) {
	accountJID := types.NewJID("628111111111", types.DefaultUserServer)
	savedJID := types.NewJID("628123456789", types.DefaultUserServer)
	unsavedJID := types.NewJID("628987654321", types.DefaultUserServer)
	client := &whatsmeow.Client{
		Store: &store.Device{
			ID:       &accountJID,
			PushName: "Active Account",
			Contacts: &senderDisplayNameContactGetter{
				contacts: map[types.JID]types.ContactInfo{
					savedJID:   {Found: true, FullName: "Saved Contact", PushName: "Stored Push"},
					unsavedJID: {Found: true, PushName: "Stored Push"},
				},
			},
		},
	}

	tests := []struct {
		name     string
		sender   types.JID
		isFromMe bool
		want     any
		present  bool
	}{
		{name: "saved sender", sender: savedJID, want: "Saved Contact", present: true},
		{name: "unsaved sender is explicit empty", sender: unsavedJID, want: "", present: true},
		{name: "outgoing message omits the field", sender: accountJID, isFromMe: true, present: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &events.Message{
				Info: types.MessageInfo{
					MessageSource: types.MessageSource{
						Chat:     types.NewJID("628100000000", types.DefaultUserServer),
						Sender:   tt.sender,
						IsFromMe: tt.isFromMe,
					},
					ID:        "sender-contact-name",
					PushName:  "Live Push",
					Timestamp: time.Date(2026, time.September, 28, 10, 0, 0, 0, time.UTC),
				},
				Message: &waE2E.Message{Conversation: protoString("hello")},
			}

			_, payload, err := buildEventPayload(context.Background(), client, evt)

			assert.NoError(t, err)
			value, ok := payload["sender_contact_name"]
			assert.Equal(t, tt.present, ok)
			if tt.present {
				assert.Equal(t, tt.want, value)
			}
		})
	}
}

func TestBuildEventPayloadInteractiveCopyCode(t *testing.T) {
	code := "PAY-7F3K9Q"
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     types.NewJID("551199999999", types.DefaultUserServer),
				Sender:   types.NewJID("551188888888", types.DefaultUserServer),
				IsFromMe: false,
			},
			ID:        "COPY-MSG-1",
			Timestamp: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			InteractiveMessage: &waE2E.InteractiveMessage{
				Header: &waE2E.InteractiveMessage_Header{Title: protoString("Your order")},
				Body:   &waE2E.InteractiveMessage_Body{Text: protoString("Copy the code below to pay")},
				InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
					NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
						Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
							{
								Name:             protoString("cta_copy"),
								ButtonParamsJSON: protoString(`{"display_text":"Copy payment code","copy_code":"` + code + `"}`),
							},
						},
					},
				},
			},
		},
	}

	eventType, payload, err := buildEventPayload(context.Background(), nil, evt)
	assert.NoError(t, err)
	assert.Equal(t, EventTypeMessage, eventType)

	// Body fallback should contain formatted text
	assert.Contains(t, payload["body"], "Your order")
	assert.Contains(t, payload["body"], code)

	// Interactive summary preserved
	assert.NotEmpty(t, payload["interactive"])

	// Buttons populated
	buttons, ok := payload["buttons"].([]utils.InteractiveButtonPayload)
	assert.True(t, ok, "buttons should be []utils.InteractiveButtonPayload")
	assert.Len(t, buttons, 1)
	assert.Equal(t, "cta_copy", buttons[0].Type)
	// The copy code is passed through raw; interpreting it (coupon, payment
	// key, ...) is left to the webhook consumer.
	assert.Equal(t, code, buttons[0].Value)
}

func TestBuildEventPayloadInteractiveMultipleButtons(t *testing.T) {
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     types.NewJID("551199999999", types.DefaultUserServer),
				Sender:   types.NewJID("551188888888", types.DefaultUserServer),
				IsFromMe: false,
			},
			ID:        "COUPON-MSG-1",
			Timestamp: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			InteractiveMessage: &waE2E.InteractiveMessage{
				Header: &waE2E.InteractiveMessage_Header{Title: protoString("Special discount")},
				Body:   &waE2E.InteractiveMessage_Body{Text: protoString("Use the coupon below")},
				InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
					NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
						Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
							{
								Name:             protoString("cta_copy"),
								ButtonParamsJSON: protoString(`{"display_text":"Copy coupon","copy_code":"PROMO50"}`),
							},
							{
								Name:             protoString("cta_url"),
								ButtonParamsJSON: protoString(`{"display_text":"Visit store","url":"https://example.com"}`),
							},
						},
					},
				},
			},
		},
	}

	eventType, payload, err := buildEventPayload(context.Background(), nil, evt)
	assert.NoError(t, err)
	assert.Equal(t, EventTypeMessage, eventType)

	// Buttons populated
	buttons, ok := payload["buttons"].([]utils.InteractiveButtonPayload)
	assert.True(t, ok)
	assert.Len(t, buttons, 2)
	assert.Equal(t, "PROMO50", buttons[0].Value)
	assert.Equal(t, "https://example.com", buttons[1].URL)
}

// Runs an interactive message through the whole webhook-to-Chatwoot path. Its
// body is the same summary as payload["interactive"]; Chatwoot must receive it
// verbatim, because the markdown translation applied to plain bodies would turn
// the underscores and asterisks in URLs and copy codes into emphasis.
func TestChatwootContentKeepsInteractiveCodesVerbatim(t *testing.T) {
	code := "ORDER_2026_0042***X"
	link := "https://store.example/p?utm_source=wa&utm_medium=cta"
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     types.NewJID("120363000000", types.GroupServer),
				Sender:   types.NewJID("628111222333", types.DefaultUserServer),
				IsFromMe: false,
			},
			ID:        "CW-INTERACTIVE-1",
			Timestamp: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			InteractiveMessage: &waE2E.InteractiveMessage{
				Body: &waE2E.InteractiveMessage_Body{Text: protoString("Your order")},
				InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
					NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
						Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
							{
								Name:             protoString("cta_copy"),
								ButtonParamsJSON: protoString(`{"display_text":"Copy code","copy_code":"` + code + `"}`),
							},
							{
								Name:             protoString("cta_url"),
								ButtonParamsJSON: protoString(`{"display_text":"View order","url":"` + link + `"}`),
							},
						},
					},
				},
			},
		},
	}

	_, payload, err := buildEventPayload(context.Background(), nil, evt)
	assert.NoError(t, err)

	content, _ := buildChatwootMessageContent(payload, true, "Alice")
	assert.Equal(t, "Alice: Your order\n📋 Copy code: "+code+"\n🔗 View order: "+link, content)
}

// A button reply without text falls back to its id as the body. Ids like
// opt_yes_1 must reach Chatwoot untouched, not as opt*yes*1.
func TestChatwootContentKeepsButtonResponseIDVerbatim(t *testing.T) {
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:   types.NewJID("551199999999", types.DefaultUserServer),
				Sender: types.NewJID("551188888888", types.DefaultUserServer),
			},
			ID:        "CW-RESP-1",
			Timestamp: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{
				InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{
					NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
						Name:       protoString("quick_reply"),
						ParamsJSON: protoString(`{"id":"opt_yes_1"}`),
					},
				},
			},
		},
	}

	_, payload, err := buildEventPayload(context.Background(), nil, evt)
	assert.NoError(t, err)
	assert.Equal(t, "Selected option opt_yes_1", payload["body"])

	content, _ := buildChatwootMessageContent(payload, false, "")
	assert.Equal(t, "Selected option opt_yes_1", content)
}

func TestBuildEventPayloadVideoNoteCaption(t *testing.T) {
	originalAutoDownload := config.WhatsappAutoDownloadMedia
	config.WhatsappAutoDownloadMedia = false
	defer func() { config.WhatsappAutoDownloadMedia = originalAutoDownload }()
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:   types.NewJID("123", types.DefaultUserServer),
				Sender: types.NewJID("456", types.DefaultUserServer),
			},
			ID:        "PTV-MSG-1",
			Timestamp: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			PtvMessage: &waE2E.VideoMessage{Caption: protoString("see you soon")},
		},
	}

	_, payload, err := buildEventPayload(context.Background(), nil, evt)
	assert.NoError(t, err)
	assert.Equal(t, "see you soon", payload["body"])
}

// request_from is read straight from the proto; when it is a LID it must be
// resolved to the phone JID like from/chat_id are.
func TestBuildEventPayloadRequestPaymentNormalizesRequestFromLID(t *testing.T) {
	originalLog := log
	log = waLog.Noop
	defer func() { log = originalLog }()

	lid := types.NewJID("123456789012345", types.HiddenUserServer)
	pn := types.NewJID("628123456789", types.DefaultUserServer)
	client := &whatsmeow.Client{Store: &store.Device{
		LIDs: &chatDisplayNameLIDStore{lid: lid, pn: pn},
	}}
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:   types.NewJID("551199999999", types.DefaultUserServer),
				Sender: types.NewJID("551188888888", types.DefaultUserServer),
			},
			ID:        "PAY-LID-1",
			Timestamp: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			RequestPaymentMessage: &waE2E.RequestPaymentMessage{
				CurrencyCodeIso4217: protoString("BRL"),
				Amount1000:          protoUint64(5000),
				RequestFrom:         protoString(lid.String()),
			},
		},
	}

	_, payload, err := buildEventPayload(context.Background(), client, evt)
	assert.NoError(t, err)
	payReq, ok := payload["payment_request"].(*utils.PaymentRequestPayload)
	if assert.True(t, ok) {
		assert.Equal(t, pn.String(), payReq.RequestFrom)
	}

	// A device suffix on the LID must not survive, so the value matches the
	// non-AD form used by from/chat_id.
	evt.Message.RequestPaymentMessage.RequestFrom = protoString(lid.User + ":7@" + types.HiddenUserServer)
	_, payload, err = buildEventPayload(context.Background(), client, evt)
	assert.NoError(t, err)
	payReq, ok = payload["payment_request"].(*utils.PaymentRequestPayload)
	if assert.True(t, ok) {
		assert.Equal(t, pn.String(), payReq.RequestFrom)
	}
	evt.Message.RequestPaymentMessage.RequestFrom = protoString(lid.String())

	// Without a resolvable mapping (nil client) the value is left untouched.
	_, payload, err = buildEventPayload(context.Background(), nil, evt)
	assert.NoError(t, err)
	payReq, ok = payload["payment_request"].(*utils.PaymentRequestPayload)
	if assert.True(t, ok) {
		assert.Equal(t, lid.String(), payReq.RequestFrom)
	}
}

func TestBuildEventPayloadInteractiveResponse(t *testing.T) {
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     types.NewJID("551199999999", types.DefaultUserServer),
				Sender:   types.NewJID("551188888888", types.DefaultUserServer),
				IsFromMe: false,
			},
			ID:        "RESP-MSG-1",
			Timestamp: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{
				InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{
					NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
						Name:       protoString("quick_reply"),
						ParamsJSON: protoString(`{"id":"confirm_btn","title":"Confirm"}`),
					},
				},
			},
		},
	}

	eventType, payload, err := buildEventPayload(context.Background(), nil, evt)
	assert.NoError(t, err)
	assert.Equal(t, EventTypeMessage, eventType)

	assert.Equal(t, "Confirm", payload["body"])

	btnResp, ok := payload["button_response"].(*utils.ButtonResponsePayload)
	assert.True(t, ok, "button_response should be *utils.ButtonResponsePayload")
	assert.NotNil(t, btnResp)
	assert.Equal(t, "confirm_btn", btnResp.ID)
	assert.Equal(t, "Confirm", btnResp.Title)
}

func TestBuildEventPayloadRequestPayment(t *testing.T) {
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     types.NewJID("551199999999", types.DefaultUserServer),
				Sender:   types.NewJID("551188888888", types.DefaultUserServer),
				IsFromMe: false,
			},
			ID:        "PAY-MSG-1",
			Timestamp: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{
			RequestPaymentMessage: &waE2E.RequestPaymentMessage{
				CurrencyCodeIso4217: protoString("BRL"),
				Amount1000:          protoUint64(150000),
				NoteMessage: &waE2E.Message{
					Conversation: protoString("Service payment"),
				},
			},
		},
	}

	eventType, payload, err := buildEventPayload(context.Background(), nil, evt)
	assert.NoError(t, err)
	assert.Equal(t, EventTypeMessage, eventType)

	assert.Contains(t, payload["body"], "Payment request: BRL 150.00 - Service payment")

	payReq, ok := payload["payment_request"].(*utils.PaymentRequestPayload)
	assert.True(t, ok, "payment_request should be *utils.PaymentRequestPayload")
	assert.NotNil(t, payReq)
	assert.Equal(t, "BRL", payReq.Currency)
	assert.Equal(t, float64(150), payReq.Amount)
	assert.Equal(t, "Service payment", payReq.Note)
}

func protoUint64(v uint64) *uint64 {
	return &v
}

func albumEventForTest(id string, message *waE2E.Message) *events.Message {
	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:   types.NewJID("123", types.DefaultUserServer),
				Sender: types.NewJID("456", types.DefaultUserServer),
			},
			ID:        types.MessageID(id),
			Timestamp: time.Date(2026, time.October, 7, 10, 0, 0, 0, time.UTC),
		},
		Message: message,
	}
}

func TestBuildEventPayloadAlbumHeaderIncludesExpectedCounts(t *testing.T) {
	images, videos := uint32(3), uint32(1)
	evt := albumEventForTest("ALBUM1", &waE2E.Message{
		AlbumMessage: &waE2E.AlbumMessage{ExpectedImageCount: &images, ExpectedVideoCount: &videos},
	})

	eventType, payload, err := buildEventPayload(context.Background(), nil, evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	assert.Equal(t, EventTypeMessage, eventType)
	assert.Equal(t, map[string]any{"expected_image_count": images, "expected_video_count": videos}, payload["album"])
	assert.False(t, payloadHasNoRenderableContent(payload))
	assert.True(t, hasRecognizedMessageType(evt.Message))
	_, hasAlbumID := payload["album_id"]
	assert.False(t, hasAlbumID)
}

func TestBuildEventPayloadAlbumItemIncludesAlbumID(t *testing.T) {
	previous := config.WhatsappAutoDownloadMedia
	config.WhatsappAutoDownloadMedia = false
	t.Cleanup(func() { config.WhatsappAutoDownloadMedia = previous })

	albumType := waE2E.MessageAssociation_MEDIA_ALBUM
	evt := albumEventForTest("ITEM1", &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{},
		MessageContextInfo: &waE2E.MessageContextInfo{
			MessageAssociation: &waE2E.MessageAssociation{
				AssociationType:  &albumType,
				ParentMessageKey: &waCommon.MessageKey{ID: protoString("ALBUM1")},
			},
		},
	})

	eventType, payload, err := buildEventPayload(context.Background(), nil, evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	assert.Equal(t, EventTypeMessage, eventType)
	assert.Equal(t, "ALBUM1", payload["album_id"])
	assert.Contains(t, payload, "image")
}

func TestBuildEventPayloadIgnoresNonAlbumAssociations(t *testing.T) {
	previous := config.WhatsappAutoDownloadMedia
	config.WhatsappAutoDownloadMedia = false
	t.Cleanup(func() { config.WhatsappAutoDownloadMedia = previous })

	otherType := waE2E.MessageAssociation_MOTION_PHOTO
	evt := albumEventForTest("ITEM2", &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{},
		MessageContextInfo: &waE2E.MessageContextInfo{
			MessageAssociation: &waE2E.MessageAssociation{
				AssociationType:  &otherType,
				ParentMessageKey: &waCommon.MessageKey{ID: protoString("PARENT")},
			},
		},
	})

	_, payload, err := buildEventPayload(context.Background(), nil, evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	_, hasAlbumID := payload["album_id"]
	assert.False(t, hasAlbumID)
}

func TestAlbumHeaderIsNotForwardedToChatwootAsUnsupported(t *testing.T) {
	images := uint32(2)
	evt := albumEventForTest("ALBUM2", &waE2E.Message{AlbumMessage: &waE2E.AlbumMessage{ExpectedImageCount: &images}})

	eventType, payload, err := buildEventPayload(context.Background(), nil, evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	assert.True(t, isAlbumHeaderPayload(eventType, payload))
	assert.False(t, isAlbumHeaderPayload(EventTypeMessage, map[string]any{"album_id": "ALBUM2", "image": "x.jpg"}))
	assert.False(t, isAlbumHeaderPayload(EventTypeMessageReaction, payload))
}
