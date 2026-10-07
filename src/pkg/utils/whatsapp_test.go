package utils

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestDetermineMediaExtension(t *testing.T) {
	tests := []struct {
		name       string
		filename   string
		mimeType   string
		wantSuffix string
	}{
		{
			name:       "DocxFromFilename",
			filename:   "report.docx",
			mimeType:   "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			wantSuffix: ".docx",
		},
		{
			name:       "XlsxFromMime",
			filename:   "",
			mimeType:   "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			wantSuffix: ".xlsx",
		},
		{
			name:       "PptxFromMime",
			filename:   "",
			mimeType:   "application/vnd.openxmlformats-officedocument.presentationml.presentation",
			wantSuffix: ".pptx",
		},
		{
			name:       "ZipFallback",
			filename:   "",
			mimeType:   "application/zip",
			wantSuffix: ".zip",
		},
		{
			name:       "AudioOgaWithCodecsParam",
			filename:   "",
			mimeType:   "audio/ogg; codecs=opus",
			wantSuffix: ".oga",
		},
		{
			name:       "ExeFromFilename",
			filename:   "installer.exe",
			mimeType:   "application/octet-stream",
			wantSuffix: ".exe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineMediaExtension(tt.filename, tt.mimeType)
			if got != tt.wantSuffix {
				t.Fatalf("determineMediaExtension() = %q, want %q", got, tt.wantSuffix)
			}
		})
	}
}

func TestExtractPhoneFromVCard(t *testing.T) {
	tests := []struct {
		name  string
		vcard string
		want  string
	}{
		{
			name:  "LFEndings",
			vcard: "BEGIN:VCARD\nVERSION:3.0\nFN:Alice\nTEL;type=Mobile:+62 812 3456 7890\nEND:VCARD",
			want:  "+62 812 3456 7890",
		},
		{
			name:  "CRLFEndings",
			vcard: "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Bob\r\nTEL:+1 555 0100\r\nEND:VCARD",
			want:  "+1 555 0100",
		},
		{
			name:  "FoldedLine",
			vcard: "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Julio\r\nTEL;type=CELL;waid=5511998913283:\r\n +5511998913283\r\nEND:VCARD",
			want:  "+5511998913283",
		},
		{
			// iOS exports every phone with a group prefix and an Apple label line.
			name:  "GroupedTelFromIOS",
			vcard: "BEGIN:VCARD\nVERSION:3.0\nN:;Dave;;;\nFN:Dave\nitem1.TEL;type=CELL;type=VOICE;type=pref;waid=5511999990006:+55 11 99999-0006\nitem1.X-ABLabel:Mobile\nEND:VCARD",
			want:  "+55 11 99999-0006",
		},
		{
			name:  "GroupedTelLowercase",
			vcard: "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Erin\r\nitem2.tel;type=CELL:+1 555 0101\r\nEND:VCARD",
			want:  "+1 555 0101",
		},
		{
			name:  "GroupedTelFolded",
			vcard: "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Frank\r\nitem1.TEL;type=CELL;waid=5511998913283:\r\n +5511998913283\r\nEND:VCARD",
			want:  "+5511998913283",
		},
		{
			// Only the property name counts: a group or a value that merely
			// contains "TEL" is not a phone.
			name:  "OnlyTelPropertyName",
			vcard: "BEGIN:VCARD\nVERSION:3.0\nFN:Grace\nTELEX:12345\nitem3.URL:https://example.com/TEL\nitem4.TEL:+1 555 0102\nEND:VCARD",
			want:  "+1 555 0102",
		},
		{
			name:  "NoTelLine",
			vcard: "BEGIN:VCARD\nVERSION:3.0\nFN:Carol\nEND:VCARD",
			want:  "",
		},
		{
			name:  "Empty",
			vcard: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPhoneFromVCard(tt.vcard)
			if got != tt.want {
				t.Fatalf("ExtractPhoneFromVCard() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatContactSummary(t *testing.T) {
	tests := []struct {
		name   string
		dName  string
		phone  string
		plural bool
		want   string
	}{
		{name: "SingleNameAndPhone", dName: "Alice", phone: "+62 812", plural: false, want: "Contact: Alice (+62 812)"},
		{name: "SingleNameOnly", dName: "Alice", phone: "", plural: false, want: "Contact: Alice"},
		{name: "SinglePhoneOnly", dName: "", phone: "+62 812", plural: false, want: "Contact: +62 812"},
		{name: "SingleEmpty", dName: "", phone: "", plural: false, want: "Contact shared"},
		{name: "SingleWhitespaceOnly", dName: " ", phone: " ", plural: false, want: "Contact shared"},
		{name: "PluralNameAndPhone", dName: "Alice", phone: "+62 812", plural: true, want: "Contacts: Alice (+62 812)"},
		{name: "PluralEmpty", dName: "", phone: "", plural: true, want: "Contacts shared"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatContactSummary(tt.dName, tt.phone, tt.plural)
			if got != tt.want {
				t.Fatalf("FormatContactSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractMessageTextFromProtoContactMessage(t *testing.T) {
	phoneVCard := "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Alice\r\nTEL;type=Mobile:\r\n +62 812 3456 7890\r\nEND:VCARD"

	tests := []struct {
		name string
		msg  *waE2E.Message
		want string
	}{
		{
			name: "NameAndPhone",
			msg: &waE2E.Message{
				ContactMessage: &waE2E.ContactMessage{
					DisplayName: strPtr("Alice"),
					Vcard:       strPtr(phoneVCard),
				},
			},
			want: "Contact: Alice (+62 812 3456 7890)",
		},
		{
			name: "NameOnly",
			msg: &waE2E.Message{
				ContactMessage: &waE2E.ContactMessage{
					DisplayName: strPtr("Alice"),
					Vcard:       strPtr(""),
				},
			},
			want: "Contact: Alice",
		},
		{
			name: "PhoneOnly",
			msg: &waE2E.Message{
				ContactMessage: &waE2E.ContactMessage{
					DisplayName: strPtr(""),
					Vcard:       strPtr(phoneVCard),
				},
			},
			want: "Contact: +62 812 3456 7890",
		},
		{
			name: "Neither",
			msg: &waE2E.Message{
				ContactMessage: &waE2E.ContactMessage{
					DisplayName: strPtr(""),
					Vcard:       strPtr(""),
				},
			},
			want: "Contact shared",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractMessageTextFromProto(tt.msg)
			if got != tt.want {
				t.Fatalf("ExtractMessageTextFromProto() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractMessageTextFromProtoContactsArrayMessage(t *testing.T) {
	bob := "Bob"
	bobVcard := "BEGIN:VCARD\nVERSION:3.0\nFN:Bob\nTEL:+1 555 0100\nEND:VCARD"
	carol := "Carol"
	carolVcard := "BEGIN:VCARD\nVERSION:3.0\nFN:Carol\nTEL:+1 555 0200\nEND:VCARD"

	tests := []struct {
		name string
		msg  *waE2E.Message
		want string
	}{
		{
			name: "FirstContactWithNameAndPhone",
			msg: &waE2E.Message{
				ContactsArrayMessage: &waE2E.ContactsArrayMessage{
					Contacts: []*waE2E.ContactMessage{
						{DisplayName: &bob, Vcard: &bobVcard},
						{DisplayName: &carol, Vcard: &carolVcard},
					},
				},
			},
			want: "Contacts: Bob (+1 555 0100)",
		},
		{
			name: "EmptyContactsArray",
			msg: &waE2E.Message{
				ContactsArrayMessage: &waE2E.ContactsArrayMessage{Contacts: nil},
			},
			want: "Contacts shared",
		},
		{
			name: "FirstContactEmpty",
			msg: &waE2E.Message{
				ContactsArrayMessage: &waE2E.ContactsArrayMessage{
					Contacts: []*waE2E.ContactMessage{{}},
				},
			},
			want: "Contacts shared",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractMessageTextFromProto(tt.msg)
			if got != tt.want {
				t.Fatalf("ExtractMessageTextFromProto() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractMediaInfoIncludesDirectPath(t *testing.T) {
	mediaKey := []byte("media-key")
	fileSHA256 := []byte("file-sha")
	fileEncSHA256 := []byte("file-enc-sha")
	fileLength := uint64(1234)
	mediaURL := "https://mmg.whatsapp.net/v/t62.7118-24/media.enc?ccb=11-4"
	directPath := "/v/t62.7118-24/media.enc?ccb=11-4"

	tests := []struct {
		name     string
		msg      *waE2E.Message
		wantType string
	}{
		{
			name: "Image",
			msg: &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
				URL:           proto.String(mediaURL),
				DirectPath:    proto.String(directPath),
				MediaKey:      mediaKey,
				FileSHA256:    fileSHA256,
				FileEncSHA256: fileEncSHA256,
				FileLength:    proto.Uint64(fileLength),
			}},
			wantType: "image",
		},
		{
			name: "Video",
			msg: &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
				URL:           proto.String(mediaURL),
				DirectPath:    proto.String(directPath),
				MediaKey:      mediaKey,
				FileSHA256:    fileSHA256,
				FileEncSHA256: fileEncSHA256,
				FileLength:    proto.Uint64(fileLength),
			}},
			wantType: "video",
		},
		{
			name: "VideoNote",
			msg: &waE2E.Message{PtvMessage: &waE2E.VideoMessage{
				URL:           proto.String(mediaURL),
				DirectPath:    proto.String(directPath),
				MediaKey:      mediaKey,
				FileSHA256:    fileSHA256,
				FileEncSHA256: fileEncSHA256,
				FileLength:    proto.Uint64(fileLength),
			}},
			wantType: "video_note",
		},
		{
			name: "Audio",
			msg: &waE2E.Message{AudioMessage: &waE2E.AudioMessage{
				URL:           proto.String(mediaURL),
				DirectPath:    proto.String(directPath),
				MediaKey:      mediaKey,
				FileSHA256:    fileSHA256,
				FileEncSHA256: fileEncSHA256,
				FileLength:    proto.Uint64(fileLength),
			}},
			wantType: "audio",
		},
		{
			name: "Document",
			msg: &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
				URL:           proto.String(mediaURL),
				DirectPath:    proto.String(directPath),
				MediaKey:      mediaKey,
				FileSHA256:    fileSHA256,
				FileEncSHA256: fileEncSHA256,
				FileLength:    proto.Uint64(fileLength),
				FileName:      proto.String("report.pdf"),
			}},
			wantType: "document",
		},
		{
			name: "Sticker",
			msg: &waE2E.Message{StickerMessage: &waE2E.StickerMessage{
				URL:           proto.String(mediaURL),
				DirectPath:    proto.String(directPath),
				MediaKey:      mediaKey,
				FileSHA256:    fileSHA256,
				FileEncSHA256: fileEncSHA256,
				FileLength:    proto.Uint64(fileLength),
			}},
			wantType: "sticker",
		},
		{
			name: "TemplateDocument",
			msg: &waE2E.Message{TemplateMessage: &waE2E.TemplateMessage{
				HydratedTemplate: &waE2E.TemplateMessage_HydratedFourRowTemplate{
					Title: &waE2E.TemplateMessage_HydratedFourRowTemplate_DocumentMessage{
						DocumentMessage: &waE2E.DocumentMessage{
							URL:           proto.String(mediaURL),
							DirectPath:    proto.String(directPath),
							MediaKey:      mediaKey,
							FileSHA256:    fileSHA256,
							FileEncSHA256: fileEncSHA256,
							FileLength:    proto.Uint64(fileLength),
							FileName:      proto.String("boleto.pdf"),
						},
					},
				},
			}},
			wantType: "document",
		},
		{
			name: "InteractiveImage",
			msg: &waE2E.Message{InteractiveMessage: &waE2E.InteractiveMessage{
				Header: &waE2E.InteractiveMessage_Header{
					Media: &waE2E.InteractiveMessage_Header_ImageMessage{
						ImageMessage: &waE2E.ImageMessage{
							URL:           proto.String(mediaURL),
							DirectPath:    proto.String(directPath),
							MediaKey:      mediaKey,
							FileSHA256:    fileSHA256,
							FileEncSHA256: fileEncSHA256,
							FileLength:    proto.Uint64(fileLength),
						},
					},
				},
			}},
			wantType: "image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, _, gotURL, gotDirectPath, gotMediaKey, gotFileSHA256, gotFileEncSHA256, gotFileLength := ExtractMediaInfo(tt.msg)
			if gotType != tt.wantType {
				t.Fatalf("mediaType = %q, want %q", gotType, tt.wantType)
			}
			if gotURL != mediaURL {
				t.Fatalf("url = %q, want %q", gotURL, mediaURL)
			}
			if gotDirectPath != directPath {
				t.Fatalf("directPath = %q, want %q", gotDirectPath, directPath)
			}
			if !bytes.Equal(gotMediaKey, mediaKey) {
				t.Fatalf("mediaKey = %q, want %q", gotMediaKey, mediaKey)
			}
			if !bytes.Equal(gotFileSHA256, fileSHA256) {
				t.Fatalf("fileSHA256 = %q, want %q", gotFileSHA256, fileSHA256)
			}
			if !bytes.Equal(gotFileEncSHA256, fileEncSHA256) {
				t.Fatalf("fileEncSHA256 = %q, want %q", gotFileEncSHA256, fileEncSHA256)
			}
			if gotFileLength != fileLength {
				t.Fatalf("fileLength = %d, want %d", gotFileLength, fileLength)
			}
		})
	}
}

func TestBuildDownloadableMessageSetsDirectPath(t *testing.T) {
	mediaKey := []byte("media-key")
	fileSHA256 := []byte("file-sha")
	fileEncSHA256 := []byte("file-enc-sha")
	fileLength := uint64(1234)
	directPath := "/v/t62.7118-24/media.enc?ccb=11-4"

	tests := []struct {
		name      string
		mediaType string
		wantType  any
	}{
		{name: "Image", mediaType: "image", wantType: &waE2E.ImageMessage{}},
		{name: "Video", mediaType: "video", wantType: &waE2E.VideoMessage{}},
		{name: "VideoNote", mediaType: "video_note", wantType: &waE2E.VideoMessage{}},
		{name: "Audio", mediaType: "audio", wantType: &waE2E.AudioMessage{}},
		{name: "PTT", mediaType: "ptt", wantType: &waE2E.AudioMessage{}},
		{name: "Document", mediaType: "document", wantType: &waE2E.DocumentMessage{}},
		{name: "Sticker", mediaType: "sticker", wantType: &waE2E.StickerMessage{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildDownloadableMessage(tt.mediaType, "https://mmg.whatsapp.net/ignored", directPath, "report.pdf", mediaKey, fileSHA256, fileEncSHA256, fileLength)
			if err != nil {
				t.Fatalf("BuildDownloadableMessage() error = %v", err)
			}
			if got.GetDirectPath() != directPath {
				t.Fatalf("directPath = %q, want %q", got.GetDirectPath(), directPath)
			}
			switch tt.wantType.(type) {
			case *waE2E.ImageMessage:
				if _, ok := got.(*waE2E.ImageMessage); !ok {
					t.Fatalf("message type = %T, want *waE2E.ImageMessage", got)
				}
			case *waE2E.VideoMessage:
				if _, ok := got.(*waE2E.VideoMessage); !ok {
					t.Fatalf("message type = %T, want *waE2E.VideoMessage", got)
				}
			case *waE2E.AudioMessage:
				if _, ok := got.(*waE2E.AudioMessage); !ok {
					t.Fatalf("message type = %T, want *waE2E.AudioMessage", got)
				}
			case *waE2E.DocumentMessage:
				if _, ok := got.(*waE2E.DocumentMessage); !ok {
					t.Fatalf("message type = %T, want *waE2E.DocumentMessage", got)
				}
			case *waE2E.StickerMessage:
				if _, ok := got.(*waE2E.StickerMessage); !ok {
					t.Fatalf("message type = %T, want *waE2E.StickerMessage", got)
				}
			}
		})
	}
}

func TestResolveMediaDirectPathFallsBackToURLRequestURI(t *testing.T) {
	storedDirectPath := "/stored/path.enc?auth=stored"
	mediaURL := "https://mmg.whatsapp.net/v/t62.7118-24/media.enc?ccb=11-4&oh=token"
	wantFallback := "/v/t62.7118-24/media.enc?ccb=11-4&oh=token"

	if got := ResolveMediaDirectPath(storedDirectPath, mediaURL); got != storedDirectPath {
		t.Fatalf("ResolveMediaDirectPath() = %q, want stored direct path %q", got, storedDirectPath)
	}
	if got := ResolveMediaDirectPath("", mediaURL); got != wantFallback {
		t.Fatalf("ResolveMediaDirectPath() = %q, want URL request URI %q", got, wantFallback)
	}
	if got := ResolveMediaDirectPath("", "://not-a-url"); got != "" {
		t.Fatalf("ResolveMediaDirectPath() = %q, want empty for invalid URL", got)
	}
}

func TestFormatLocationSummary(t *testing.T) {
	tests := []struct {
		name    string
		locName string
		address string
		lat     float64
		long    float64
		want    string
	}{
		{
			name:    "NameAndAddress",
			locName: "Monas",
			address: "Gambir, Jakarta",
			lat:     -6.175392,
			long:    106.827153,
			want:    "Monas — Gambir, Jakarta — https://maps.google.com/?q=-6.175392,106.827153",
		},
		{
			name:    "NameOnly",
			locName: "Monas",
			address: "",
			lat:     -6.2,
			long:    106.8,
			want:    "Monas — https://maps.google.com/?q=-6.2,106.8",
		},
		{
			name:    "CoordinatesOnly",
			locName: "",
			address: "",
			lat:     -6.2,
			long:    106.8,
			want:    "https://maps.google.com/?q=-6.2,106.8",
		},
		{
			name:    "WhitespaceOnlyNameAndAddress",
			locName: " ",
			address: " ",
			lat:     0,
			long:    0,
			want:    "https://maps.google.com/?q=0,0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatLocationSummary(tt.locName, tt.address, tt.lat, tt.long)
			if got != tt.want {
				t.Fatalf("FormatLocationSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractMessageTextFromProtoLocationMessages(t *testing.T) {
	tests := []struct {
		name string
		msg  *waE2E.Message
		want string
	}{
		{
			name: "LocationWithNameAndAddress",
			msg: &waE2E.Message{
				LocationMessage: &waE2E.LocationMessage{
					Name:             strPtr("Monas"),
					Address:          strPtr("Gambir, Jakarta"),
					DegreesLatitude:  proto.Float64(-6.2),
					DegreesLongitude: proto.Float64(106.8),
				},
			},
			want: "Monas — Gambir, Jakarta — https://maps.google.com/?q=-6.2,106.8",
		},
		{
			name: "LocationCoordinatesOnly",
			msg: &waE2E.Message{
				LocationMessage: &waE2E.LocationMessage{
					DegreesLatitude:  proto.Float64(-6.2),
					DegreesLongitude: proto.Float64(106.8),
				},
			},
			want: "https://maps.google.com/?q=-6.2,106.8",
		},
		{
			name: "LiveLocationWithCaption",
			msg: &waE2E.Message{
				LiveLocationMessage: &waE2E.LiveLocationMessage{
					Caption:          strPtr("On my way"),
					DegreesLatitude:  proto.Float64(-6.2),
					DegreesLongitude: proto.Float64(106.8),
				},
			},
			want: "On my way — https://maps.google.com/?q=-6.2,106.8",
		},
		{
			name: "LiveLocationWithoutCaption",
			msg: &waE2E.Message{
				LiveLocationMessage: &waE2E.LiveLocationMessage{
					DegreesLatitude:  proto.Float64(-6.2),
					DegreesLongitude: proto.Float64(106.8),
				},
			},
			want: "https://maps.google.com/?q=-6.2,106.8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractMessageTextFromProto(tt.msg)
			if got != tt.want {
				t.Fatalf("ExtractMessageTextFromProto() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractMessageTextFromProtoExtendedTextMessage(t *testing.T) {
	tests := []struct {
		name string
		msg  *waE2E.Message
		want string
	}{
		{
			name: "TextTakesPriorityOverMatchedText",
			msg: &waE2E.Message{
				ExtendedTextMessage: &waE2E.ExtendedTextMessage{
					Text:        strPtr("check this out https://example.com"),
					MatchedText: strPtr("https://example.com"),
				},
			},
			want: "check this out https://example.com",
		},
		{
			name: "FallsBackToMatchedTextWhenTextEmpty",
			msg: &waE2E.Message{
				ExtendedTextMessage: &waE2E.ExtendedTextMessage{
					Text:        strPtr(""),
					MatchedText: strPtr("https://example.com"),
				},
			},
			want: "https://example.com",
		},
		{
			name: "EmptyTextAndMatchedTextFallsThrough",
			msg: &waE2E.Message{
				ExtendedTextMessage: &waE2E.ExtendedTextMessage{
					Text:        strPtr(""),
					MatchedText: strPtr(""),
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractMessageTextFromProto(tt.msg)
			if got != tt.want {
				t.Fatalf("ExtractMessageTextFromProto() = %q, want %q", got, tt.want)
			}
		})
	}
}

func strPtr(value string) *string {
	return &value
}

func TestBuildForwardMessageFromStorageText(t *testing.T) {
	msg, err := BuildForwardMessageFromStorage(&domainChatStorage.Message{
		Content: "hello forward",
	}, ForwardBuildOptions{})
	if err != nil {
		t.Fatalf("BuildForwardMessageFromStorage() error = %v", err)
	}
	ext := msg.GetExtendedTextMessage()
	if ext == nil {
		t.Fatal("expected ExtendedTextMessage")
	}
	if ext.GetText() != "hello forward" {
		t.Fatalf("text = %q, want %q", ext.GetText(), "hello forward")
	}
	if !ext.GetContextInfo().GetIsForwarded() {
		t.Fatal("expected IsForwarded=true")
	}
	if ext.GetContextInfo().GetForwardingScore() != 100 {
		t.Fatalf("forwarding score = %d, want 100", ext.GetContextInfo().GetForwardingScore())
	}
}

func TestBuildForwardMessageFromStorageImage(t *testing.T) {
	directPath := "/v/t62.media.enc"
	msg, err := BuildForwardMessageFromStorage(&domainChatStorage.Message{
		MediaType:     "image",
		Content:       "caption text",
		URL:           "https://mmg.whatsapp.net/ignored",
		DirectPath:    directPath,
		MediaKey:      []byte("key"),
		FileSHA256:    []byte("sha"),
		FileEncSHA256: []byte("enc"),
		FileLength:    42,
	}, ForwardBuildOptions{})
	if err != nil {
		t.Fatalf("BuildForwardMessageFromStorage() error = %v", err)
	}
	img := msg.GetImageMessage()
	if img == nil {
		t.Fatal("expected ImageMessage")
	}
	if img.GetCaption() != "caption text" {
		t.Fatalf("caption = %q, want %q", img.GetCaption(), "caption text")
	}
	if img.GetDirectPath() != directPath {
		t.Fatalf("directPath = %q, want %q", img.GetDirectPath(), directPath)
	}
	if !img.GetContextInfo().GetIsForwarded() {
		t.Fatal("expected IsForwarded=true")
	}
	if img.GetMimetype() != "image/jpeg" {
		t.Fatalf("mimetype = %q, want image/jpeg", img.GetMimetype())
	}
}

func TestBuildForwardMessageFromStorageDocument(t *testing.T) {
	msg, err := BuildForwardMessageFromStorage(&domainChatStorage.Message{
		MediaType:     "document",
		Filename:      "report.pdf",
		Content:       "see attached",
		URL:           "https://mmg.whatsapp.net/doc",
		DirectPath:    "/v/doc.enc",
		MediaKey:      []byte("key"),
		FileSHA256:    []byte("sha"),
		FileEncSHA256: []byte("enc"),
		FileLength:    100,
	}, ForwardBuildOptions{})
	if err != nil {
		t.Fatalf("BuildForwardMessageFromStorage() error = %v", err)
	}
	doc := msg.GetDocumentMessage()
	if doc == nil {
		t.Fatal("expected DocumentMessage")
	}
	if doc.GetFileName() != "report.pdf" {
		t.Fatalf("filename = %q, want report.pdf", doc.GetFileName())
	}
	if doc.GetCaption() != "see attached" {
		t.Fatalf("caption = %q, want see attached", doc.GetCaption())
	}
	if doc.GetMimetype() != "application/pdf" {
		t.Fatalf("mimetype = %q, want application/pdf", doc.GetMimetype())
	}
}

func TestBuildForwardMessageFromStorageUnsupportedType(t *testing.T) {
	_, err := BuildForwardMessageFromStorage(&domainChatStorage.Message{
		MediaType: "call",
		Content:   "incoming call",
	}, ForwardBuildOptions{})
	if err == nil {
		t.Fatal("expected error for call type")
	}
	if err.Error() != ErrUnsupportedForwardType {
		t.Fatalf("error = %q, want %q", err.Error(), ErrUnsupportedForwardType)
	}

	_, err = BuildForwardMessageFromStorage(&domainChatStorage.Message{
		Content: "Contact: Alice (+62 812)",
	}, ForwardBuildOptions{})
	if err == nil {
		t.Fatal("expected error for contact summary")
	}
}

func TestIsForwardableStorageMessage(t *testing.T) {
	tests := []struct {
		name    string
		message *domainChatStorage.Message
		want    bool
	}{
		{name: "text", message: &domainChatStorage.Message{Content: "hi"}, want: true},
		{name: "image", message: &domainChatStorage.Message{MediaType: "image", URL: "u", DirectPath: "/p"}, want: true},
		{name: "call", message: &domainChatStorage.Message{MediaType: "call"}, want: false},
		{name: "contact", message: &domainChatStorage.Message{Content: "Contact: Bob"}, want: false},
		{name: "location", message: &domainChatStorage.Message{Content: "Pin — https://maps.google.com/?q=1,2"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsForwardableStorageMessage(tt.message); got != tt.want {
				t.Fatalf("IsForwardableStorageMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatJIDStripsDeviceSuffix(t *testing.T) {
	tests := []struct {
		name string
		jid  string
		want string
	}{
		{
			name: "PhoneNumberJIDWithoutDevice",
			jid:  "6281234567890@s.whatsapp.net",
			want: "6281234567890@s.whatsapp.net",
		},
		{
			name: "PhoneNumberJIDWithDevice",
			jid:  "6281234567890:12@s.whatsapp.net",
			want: "6281234567890@s.whatsapp.net",
		},
		{
			name: "LIDWithoutDevice",
			jid:  "123456789012345@lid",
			want: "123456789012345@lid",
		},
		{
			// Messages sent from WhatsApp Web/Desktop carry a device suffix.
			// whatsmeow rejects AD JIDs as a send recipient, so it must be stripped
			// for @lid senders too, not just @s.whatsapp.net ones.
			name: "LIDWithDevice",
			jid:  "123456789012345:12@lid",
			want: "123456789012345@lid",
		},
		{
			name: "BareNumberGetsDefaultServer",
			jid:  "6281234567890",
			want: "6281234567890@s.whatsapp.net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatJID(tt.jid)
			if got.String() != tt.want {
				t.Errorf("FormatJID(%q) = %q, want %q", tt.jid, got.String(), tt.want)
			}
			if got.Device != 0 {
				t.Errorf("FormatJID(%q) kept device %d, want 0", tt.jid, got.Device)
			}
		})
	}
}

// TestExtractContextInfoInteractiveMessage pins that quoted/forwarded context
// on an InteractiveMessage (business/Cloud API CTA buttons) is preserved.
// Before this case existed, BuildEventMessage/BuildForwarded silently lost
// replied_to_id/quoted_body and the forwarded flag for these messages, even
// though every other message type carries them through ExtractContextInfo.
func TestExtractContextInfoInteractiveMessage(t *testing.T) {
	t.Run("interactive message with context info", func(t *testing.T) {
		msg := &waE2E.Message{
			InteractiveMessage: &waE2E.InteractiveMessage{
				Body: &waE2E.InteractiveMessage_Body{Text: proto.String("Check this out")},
				ContextInfo: &waE2E.ContextInfo{
					StanzaID: proto.String("ABC123"),
				},
			},
		}
		ci := ExtractContextInfo(msg)
		if ci == nil {
			t.Fatal("expected non-nil ContextInfo")
		}
		if ci.GetStanzaID() != "ABC123" {
			t.Fatalf("got StanzaID %q, want %q", ci.GetStanzaID(), "ABC123")
		}
	})

	t.Run("interactive message with no context info yields nil", func(t *testing.T) {
		msg := &waE2E.Message{
			InteractiveMessage: &waE2E.InteractiveMessage{
				Body: &waE2E.InteractiveMessage_Body{Text: proto.String("Check this out")},
			},
		}
		if ci := ExtractContextInfo(msg); ci != nil {
			t.Fatalf("expected nil ContextInfo, got %+v", ci)
		}
	})
}

// Brazil payment requests arrive as an interactive message whose only content
// is a review_and_pay button; everything (amount, items, Pix code) is in the
// button params, in the order_details format of Meta's Cloud API docs.
func TestOrderDetailsButtons(t *testing.T) {
	// Verbatim parameters example from Meta's order_details documentation.
	metaExample := `{"reference_id":"unique-reference-id","type":"digital-goods","payment_type":"br","payment_settings":[{"type":"payment_link","payment_link":{"uri":"https://my-payment-link-url"}}],"currency":"BRL","total_amount":{"value":50000,"offset":100},"order":{"status":"pending","tax":{"value":0,"offset":100,"description":"optional text"},"items":[{"retailer_id":"1234567","name":"Cake","amount":{"value":50000,"offset":100},"quantity":1}],"subtotal":{"value":50000,"offset":100}}}`
	pixExample := `{"reference_id":"order-42","type":"physical-goods","payment_type":"br","payment_settings":[{"type":"pix_dynamic_code","pix_dynamic_code":{"code":"00020126580014br.gov.bcb.pix0136abc","merchant_name":"Loja","key":"11144477735","key_type":"CPF"}}],"currency":"BRL","total_amount":{"value":1990,"offset":100},"order":{"status":"pending","items":[{"name":"Camiseta","amount":{"value":995,"offset":100},"quantity":2}]}}`
	button := func(name, params string) *waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton {
		return &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{Name: proto.String(name), ButtonParamsJSON: proto.String(params)}
	}

	t.Run("payment link order renders total, items, link and reference", func(t *testing.T) {
		got := FormatNativeFlowButton(button("review_and_pay", metaExample))
		want := "💳 Pay order: BRL 500.00\n  • Cake — BRL 500.00\n🔗 Pay: https://my-payment-link-url\nRef: unique-reference-id"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("pix order renders code and quantities", func(t *testing.T) {
		got := FormatNativeFlowButton(button("review_and_pay", pixExample))
		want := "💳 Pay order: BRL 19.90\n  • 2× Camiseta — BRL 9.95\n📋 Pix: 00020126580014br.gov.bcb.pix0136abc\nRef: order-42"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("order status button", func(t *testing.T) {
		got := FormatNativeFlowButton(button("review_order", `{"reference_id":"order-42","order":{"status":"processing"},"payment":{"status":"captured","timestamp":1722445231}}`))
		want := "🧾 Order status: order processing, payment captured\nRef: order-42"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("empty params still render a label, never the raw name", func(t *testing.T) {
		if got := FormatNativeFlowButton(button("review_and_pay", "")); got != "💳 Pay order" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("structured button payload carries amount, reference and code", func(t *testing.T) {
		im := &waE2E.InteractiveMessage{InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
			NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
				Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{button("review_and_pay", pixExample), button("review_and_pay", metaExample)},
			},
		}}
		buttons := ExtractButtonsFromInteractive(im)
		if len(buttons) != 2 {
			t.Fatalf("expected 2 buttons, got %d", len(buttons))
		}
		pix, link := buttons[0], buttons[1]
		if pix.Type != "payment" || pix.Label != "Pay order" || pix.Currency != "BRL" || pix.Amount != 19.9 || pix.ReferenceID != "order-42" || pix.Status != "pending" || pix.Value != "00020126580014br.gov.bcb.pix0136abc" {
			t.Errorf("pix button mismatch: %+v", pix)
		}
		if link.Amount != 500 || link.URL != "https://my-payment-link-url" || link.Value != "" {
			t.Errorf("link button mismatch: %+v", link)
		}
		if got := ExtractMessageTextFromProto(&waE2E.Message{InteractiveMessage: im}); !strings.Contains(got, "BRL 19.90") || strings.Contains(got, "[review_and_pay]") {
			t.Errorf("message text = %q", got)
		}
	})
}

// WhatsApp Flows (forms) arrive as a native flow button named galaxy_message
// (the apps) or flow (Cloud API); the button label is flow_cta. A submission
// comes back as a native flow response whose params are the form values plus
// flow bookkeeping keys.
func TestFlowButtonsAndResponses(t *testing.T) {
	button := func(name, params string) *waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton {
		return &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{Name: proto.String(name), ButtonParamsJSON: proto.String(params)}
	}
	flowParams := `{"flow_message_version":"3","flow_token":"tok-123","flow_id":"1234567890","flow_cta":"Responder","flow_action":"navigate","flow_action_payload":{"screen":"QUESTION_ONE"}}`

	t.Run("flow button renders its cta", func(t *testing.T) {
		if got := FormatNativeFlowButton(button("galaxy_message", flowParams)); got != "📝 [Responder]" {
			t.Errorf("got %q", got)
		}
		if got := FormatNativeFlowButton(button("flow", `{}`)); got != "📝 [Open form]" {
			t.Errorf("empty params: got %q", got)
		}
	})

	t.Run("interactive message with flow button", func(t *testing.T) {
		im := &waE2E.InteractiveMessage{
			Body: &waE2E.InteractiveMessage_Body{Text: proto.String("Colher a experiência do evento")},
			InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
				Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{button("galaxy_message", flowParams)},
			}},
		}
		if got := FormatInteractiveMessageSummary(im); got != "Colher a experiência do evento\n📝 [Responder]" {
			t.Errorf("summary = %q", got)
		}
		buttons := ExtractButtonsFromInteractive(im)
		if len(buttons) != 1 || buttons[0].Type != "flow" || buttons[0].Label != "Responder" || buttons[0].ID != "1234567890" {
			t.Errorf("buttons = %+v", buttons)
		}
		if string(buttons[0].Params) != flowParams {
			t.Errorf("params = %s, want the raw button params", buttons[0].Params)
		}
		encoded, _ := json.Marshal(buttons[0])
		if !strings.Contains(string(encoded), `"params":{"flow_message_version"`) {
			t.Errorf("params must serialize as a JSON object, got %s", encoded)
		}
	})

	t.Run("form submission renders and exposes the fields", func(t *testing.T) {
		ir := &waE2E.InteractiveResponseMessage{
			Body: &waE2E.InteractiveResponseMessage_Body{Text: proto.String("Sent")},
			InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{
				NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
					Name:       proto.String("galaxy_message"),
					ParamsJSON: proto.String(`{"flow_token":"tok-123","nome":"Ana","avaliacao":"5","interesses":["podcast","evento"]}`),
				},
			},
		}
		want := "📝 Form response\n  avaliacao: 5\n  interesses: [\"podcast\",\"evento\"]\n  nome: Ana"
		if got := ExtractInteractiveResponseText(ir); got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
		resp := ExtractButtonResponse(ir)
		if resp.Type != "flow" || resp.Title != "Sent" || resp.ID != "" {
			t.Errorf("response = %+v", resp)
		}
		if len(resp.Fields) != 3 || resp.Fields["nome"] != "Ana" || resp.Fields["avaliacao"] != "5" {
			t.Errorf("fields = %+v", resp.Fields)
		}
		if _, leaked := resp.Fields["flow_token"]; leaked {
			t.Error("flow_token must not be exposed as a field")
		}
	})

	t.Run("directionality marks are stripped from labels", func(t *testing.T) {
		b := button("galaxy_message", `{"flow_id":"1","flow_cta":"\u200eComplete form"}`)
		if got := FormatNativeFlowButton(b); got != "📝 [Complete form]" {
			t.Errorf("text = %q", got)
		}
		im := &waE2E.InteractiveMessage{InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
			Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{b},
		}}}
		if got := ExtractButtonsFromInteractive(im)[0].Label; got != "Complete form" {
			t.Errorf("label = %q", got)
		}
	})

	t.Run("flow_cta wins over title for the structured label", func(t *testing.T) {
		im := &waE2E.InteractiveMessage{InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
			Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{button("galaxy_message", `{"flow_id":"1","flow_cta":"Responder","title":"Formulário"}`)},
		}}}
		if got := ExtractButtonsFromInteractive(im)[0].Label; got != "Responder" {
			t.Errorf("label = %q", got)
		}
	})

	t.Run("extra keys on a plain reply are not form fields", func(t *testing.T) {
		ir := &waE2E.InteractiveResponseMessage{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{
			NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
				Name:       proto.String("single_select"),
				ParamsJSON: proto.String(`{"id":"opt_1","title":"Option 1","description":"First option"}`),
			},
		}}
		if got := ExtractInteractiveResponseText(ir); got != "Option 1" {
			t.Errorf("text = %q", got)
		}
		if resp := ExtractButtonResponse(ir); resp.Fields != nil {
			t.Errorf("fields = %+v, want none", resp.Fields)
		}
	})

	t.Run("business app form submission (real payload)", func(t *testing.T) {
		// Verbatim paramsJSON of a form submitted from the WhatsApp Business app
		// (form built in the app, sent through the API, filled on the phone).
		real := `{"wa_flow_response_params":{"flow_creation_source":"FLEXIBLE_CHECKOUT","flow_id":"1332768858934028","flow_name":"custom_form_be6b3e","response_message":"{\"version\":2,\"screens\":[{\"id\":\"contact_details\",\"title\":\"\",\"components\":[{\"name\":\"experincia_do_evento\",\"type\":\"TextArea\",\"isSensitive\":false,\"label\":\"Experiência do Evento\"},{\"isSensitive\":false,\"label\":\"Atendimento\",\"value\":\"Top\",\"name\":\"atendimento\",\"type\":\"TextInput\"},{\"name\":\"autgrafo\",\"value\":\"Top\",\"label\":\"Autógrafo\",\"isSensitive\":false,\"type\":\"TextInput\"},{\"value\":\"Top\",\"label\":\"Postou nas Redes Sociais\",\"type\":\"TextInput\",\"isSensitive\":false,\"name\":\"postou_nas_redes_sociais\"}]}]}","title":"\u200eComplete form"}}`
		ir := &waE2E.InteractiveResponseMessage{
			Body: &waE2E.InteractiveResponseMessage_Body{Text: proto.String("Sent")},
			InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{
				NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
					Name:       proto.String("galaxy_message"),
					ParamsJSON: proto.String(real),
				},
			},
		}
		want := "📝 Form response: custom_form_be6b3e\n  Experiência do Evento: —\n  Atendimento: Top\n  Autógrafo: Top\n  Postou nas Redes Sociais: Top"
		if got := ExtractInteractiveResponseText(ir); got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
		resp := ExtractButtonResponse(ir)
		if resp.Type != "flow" || resp.ID != "1332768858934028" || resp.FlowName != "custom_form_be6b3e" || resp.Title != "Sent" {
			t.Errorf("response = %+v", resp)
		}
		wantFields := map[string]any{"experincia_do_evento": "", "atendimento": "Top", "autgrafo": "Top", "postou_nas_redes_sociais": "Top"}
		if len(resp.Fields) != 4 {
			t.Errorf("fields = %+v", resp.Fields)
		}
		for k, v := range wantFields {
			if resp.Fields[k] != v {
				t.Errorf("fields[%s] = %v, want %v", k, resp.Fields[k], v)
			}
		}
		if resp.Labels["autgrafo"] != "Autógrafo" || len(resp.Labels) != 4 {
			t.Errorf("labels = %+v", resp.Labels)
		}
		if _, leaked := resp.Fields["wa_flow_response_params"]; leaked {
			t.Error("raw wrapper must not be exposed as a field")
		}
	})

	t.Run("plain button reply is unchanged", func(t *testing.T) {
		ir := &waE2E.InteractiveResponseMessage{
			InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{
				NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
					Name:       proto.String("quick_reply"),
					ParamsJSON: proto.String(`{"id":"btn_yes","title":"Yes"}`),
				},
			},
		}
		if got := ExtractInteractiveResponseText(ir); got != "Yes" {
			t.Errorf("text = %q", got)
		}
		resp := ExtractButtonResponse(ir)
		if resp.Type != "quick_reply" || resp.ID != "btn_yes" || resp.Title != "Yes" || resp.Fields != nil {
			t.Errorf("response = %+v", resp)
		}
	})
}

func TestExtractButtonsFromInteractive(t *testing.T) {
	im := &waE2E.InteractiveMessage{
		InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
			NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
				Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
					{
						Name:             proto.String("cta_url"),
						ButtonParamsJSON: proto.String(`{"display_text":"Site","url":"https://example.com"}`),
					},
					{
						Name:             proto.String("cta_call"),
						ButtonParamsJSON: proto.String(`{"display_text":"Call us","phone_number":"+5511999999999"}`),
					},
					{
						Name:             proto.String("cta_copy"),
						ButtonParamsJSON: proto.String(`{"display_text":"Copy","copy_code":"SAVE10"}`),
					},
					{
						Name:             proto.String("quick_reply"),
						ButtonParamsJSON: proto.String(`{"display_text":"Confirm","id":"btn_ok"}`),
					},
					{
						Name:             proto.String("single_select"),
						ButtonParamsJSON: proto.String(`{"title":"Menu"}`),
					},
				},
			},
		},
	}

	// A button with neither a name nor any label carries nothing a consumer
	// could act on; it must not show up as {"type":""} or as "[]" in text.
	im.GetNativeFlowMessage().Buttons = append(im.GetNativeFlowMessage().Buttons,
		&waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{ButtonParamsJSON: proto.String(`{}`)})

	buttons := ExtractButtonsFromInteractive(im)
	if len(buttons) != 5 {
		t.Fatalf("expected 5 buttons, got %d", len(buttons))
	}
	if got := FormatNativeFlowButton(im.GetNativeFlowMessage().Buttons[5]); got != "" {
		t.Errorf("nameless button rendered as %q, want empty", got)
	}

	if buttons[0].Type != "cta_url" || buttons[0].URL != "https://example.com" {
		t.Errorf("button 0 mismatch: %+v", buttons[0])
	}
	if buttons[1].Type != "cta_call" || buttons[1].Phone != "+5511999999999" {
		t.Errorf("button 1 mismatch: %+v", buttons[1])
	}
	if buttons[2].Type != "cta_copy" || buttons[2].Value != "SAVE10" {
		t.Errorf("button 2 mismatch: %+v", buttons[2])
	}
	if buttons[3].Type != "quick_reply" || buttons[3].ID != "btn_ok" {
		t.Errorf("button 3 mismatch: %+v", buttons[3])
	}
	if buttons[4].Type != "single_select" || buttons[4].Label != "Menu" {
		t.Errorf("button 4 mismatch: %+v", buttons[4])
	}
}

func TestExtractMessageTextFromProtoInteractiveAndPayment(t *testing.T) {
	t.Run("extracts interactive message text", func(t *testing.T) {
		msg := &waE2E.Message{
			InteractiveMessage: &waE2E.InteractiveMessage{
				Header: &waE2E.InteractiveMessage_Header{Title: proto.String("Your coupon")},
				Body:   &waE2E.InteractiveMessage_Body{Text: proto.String("Use it at checkout")},
				InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
					NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
						Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
							{
								Name:             proto.String("cta_copy"),
								ButtonParamsJSON: proto.String(`{"display_text":"Copy coupon","copy_code":"SAVE10"}`),
							},
						},
					},
				},
			},
		}

		got := ExtractMessageTextFromProto(msg)
		want := "Your coupon\nUse it at checkout\n📋 Copy coupon: SAVE10"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("extracts interactive response message text", func(t *testing.T) {
		msg := &waE2E.Message{
			InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{
				InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{
					NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
						Name:       proto.String("quick_reply"),
						ParamsJSON: proto.String(`{"id":"btn_yes","title":"Yes, I confirm"}`),
					},
				},
			},
		}

		got := ExtractMessageTextFromProto(msg)
		want := "Yes, I confirm"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("extracts request payment message", func(t *testing.T) {
		msg := &waE2E.Message{
			RequestPaymentMessage: &waE2E.RequestPaymentMessage{
				CurrencyCodeIso4217: proto.String("BRL"),
				Amount1000:          proto.Uint64(50000),
				NoteMessage: &waE2E.Message{
					Conversation: proto.String("Lunch"),
				},
			},
		}

		got := ExtractMessageTextFromProto(msg)
		want := "💰 Payment request: BRL 50.00 - Lunch"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestUnwrapMessage(t *testing.T) {
	text := &waE2E.Message{Conversation: proto.String("inner")}
	wrap := func(m *waE2E.Message) *waE2E.FutureProofMessage { return &waE2E.FutureProofMessage{Message: m} }

	t.Run("peels four nested layers", func(t *testing.T) {
		msg := &waE2E.Message{DeviceSentMessage: &waE2E.DeviceSentMessage{Message: &waE2E.Message{
			EphemeralMessage: wrap(&waE2E.Message{
				ViewOnceMessageV2: wrap(&waE2E.Message{
					ViewOnceMessageV2Extension: wrap(text),
				}),
			}),
		}}}
		if got := UnwrapMessage(msg); got != text {
			t.Fatalf("UnwrapMessage returned %v, want innermost text message", got)
		}
	})

	t.Run("document with caption", func(t *testing.T) {
		doc := &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{Caption: proto.String("Q3 report")}}
		msg := &waE2E.Message{DocumentWithCaptionMessage: wrap(doc)}
		if got := UnwrapMessage(msg); got != doc {
			t.Fatalf("UnwrapMessage returned %v, want inner document", got)
		}
		if got := ExtractMessageTextFromProto(msg); got != "Q3 report" {
			t.Fatalf("text = %q, want the document caption", got)
		}
	})

	t.Run("plain message and nil are returned as-is", func(t *testing.T) {
		if got := UnwrapMessage(text); got != text {
			t.Fatalf("plain message changed: %v", got)
		}
		if got := UnwrapMessage(nil); got != nil {
			t.Fatalf("nil returned %v", got)
		}
	})

	t.Run("empty wrapper is not descended into", func(t *testing.T) {
		msg := &waE2E.Message{EphemeralMessage: &waE2E.FutureProofMessage{}}
		if got := UnwrapMessage(msg); got != msg {
			t.Fatalf("empty wrapper replaced by %v", got)
		}
	})
}

func TestPaymentRequestAmountSources(t *testing.T) {
	money := func(value int64, offset uint32, currency string) *waE2E.Money {
		return &waE2E.Money{Value: proto.Int64(value), Offset: proto.Uint32(offset), CurrencyCode: proto.String(currency)}
	}
	tests := []struct {
		name         string
		req          *waE2E.RequestPaymentMessage
		wantSummary  string
		wantCurrency string
		wantAmount   float64
	}{
		{
			name:         "money only, cloud api offset 100",
			req:          &waE2E.RequestPaymentMessage{Amount: money(2550, 100, "BRL")},
			wantSummary:  "💰 Payment request: BRL 25.50",
			wantCurrency: "BRL",
			wantAmount:   25.5,
		},
		{
			name:         "money only, offset 1000",
			req:          &waE2E.RequestPaymentMessage{Amount: money(12345, 1000, "USD")},
			wantSummary:  "💰 Payment request: USD 12.35",
			wantCurrency: "USD",
			wantAmount:   12.345,
		},
		{
			name: "amount1000 wins over money",
			req: &waE2E.RequestPaymentMessage{
				CurrencyCodeIso4217: proto.String("EUR"),
				Amount1000:          proto.Uint64(10000),
				Amount:              money(99, 1, "BRL"),
			},
			wantSummary:  "💰 Payment request: EUR 10.00",
			wantCurrency: "EUR",
			wantAmount:   10,
		},
		{
			name: "iso currency kept when money carries a different one",
			req: &waE2E.RequestPaymentMessage{
				CurrencyCodeIso4217: proto.String("BRL"),
				Amount:              money(500, 100, "USD"),
			},
			wantSummary:  "💰 Payment request: BRL 5.00",
			wantCurrency: "BRL",
			wantAmount:   5,
		},
		{
			name: "money currency fills in for legacy amount when iso code is empty",
			req: &waE2E.RequestPaymentMessage{
				Amount1000: proto.Uint64(2550),
				Amount:     money(2550, 1000, "BRL"),
			},
			wantSummary:  "💰 Payment request: BRL 2.55",
			wantCurrency: "BRL",
			wantAmount:   2.55,
		},
		{
			name:         "money with zero offset yields no amount",
			req:          &waE2E.RequestPaymentMessage{Amount: money(2550, 0, "BRL")},
			wantSummary:  "💰 Payment request",
			wantCurrency: "BRL",
			wantAmount:   0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatPaymentRequestSummary(tt.req); got != tt.wantSummary {
				t.Errorf("summary = %q, want %q", got, tt.wantSummary)
			}
			p := ExtractPaymentRequest(tt.req)
			if p.Currency != tt.wantCurrency || p.Amount != tt.wantAmount {
				t.Errorf("payload currency/amount = %q/%v, want %q/%v", p.Currency, p.Amount, tt.wantCurrency, tt.wantAmount)
			}
			if p.Amount1000 != tt.req.GetAmount1000() {
				t.Errorf("amount_1000 = %d, want the raw proto value %d", p.Amount1000, tt.req.GetAmount1000())
			}
		})
	}
}

// A note is plain text. A crafted note that nests another payment request must
// not be followed: only the note's own text fields are read.
func TestPaymentNoteIsNotExtractedRecursively(t *testing.T) {
	nested := &waE2E.Message{RequestPaymentMessage: &waE2E.RequestPaymentMessage{
		CurrencyCodeIso4217: proto.String("BRL"),
		Amount1000:          proto.Uint64(1000),
		NoteMessage:         &waE2E.Message{Conversation: proto.String("inner")},
	}}
	req := &waE2E.RequestPaymentMessage{
		CurrencyCodeIso4217: proto.String("BRL"),
		Amount1000:          proto.Uint64(2000),
		NoteMessage:         nested,
	}
	if got := FormatPaymentRequestSummary(req); got != "💰 Payment request: BRL 2.00" {
		t.Errorf("summary = %q, want no note", got)
	}
	if got := ExtractPaymentRequest(req).Note; got != "" {
		t.Errorf("note = %q, want empty", got)
	}

	extended := &waE2E.RequestPaymentMessage{
		NoteMessage: &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("for the tickets")}},
	}
	if got := FormatPaymentRequestSummary(extended); got != "💰 Payment request - for the tickets" {
		t.Errorf("summary = %q, want extended text note", got)
	}
	sent := &waE2E.SendPaymentMessage{NoteMessage: nested}
	if got := FormatSendPaymentSummary(sent); got != "💰 Payment sent" {
		t.Errorf("send summary = %q, want no note", got)
	}
}
