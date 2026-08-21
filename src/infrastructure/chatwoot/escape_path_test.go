package chatwoot

import "testing"

// Cobre o escaping do segmento de path da API publica do Chatwoot.
// O caso que interessa e o JID do WhatsApp: url.PathEscape deixa "@" e "."
// intactos e o router do Rails nao resolve o contacto, devolvendo 404 em HTML.
func TestEscapeChatwootPathSegment(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "JID do WhatsApp: escapa arroba e todos os pontos",
			in:   "5511999999999@s.whatsapp.net",
			want: "5511999999999%40s%2Ewhatsapp%2Enet",
		},
		{
			name: "identifier alfanumerico fica intocado",
			in:   "swkuTjDfw6W8jDcqTyriMNrd",
			want: "swkuTjDfw6W8jDcqTyriMNrd",
		},
		{
			name: "source_id em UUID mantem os hifenes",
			in:   "03205d06-0b89-4ff3-9e44-3d76a625c1da",
			want: "03205d06-0b89-4ff3-9e44-3d76a625c1da",
		},
		{
			name: "percentagem literal nao e duplamente codificada",
			in:   "a%b.c",
			want: "a%25b%2Ec",
		},
		{
			name: "espaco continua a virar %20 e nao +",
			in:   "a b.c",
			want: "a%20b%2Ec",
		},
		{
			name: "vazio",
			in:   "",
			want: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := escapeChatwootPathSegment(tc.in); got != tc.want {
				t.Errorf("escapeChatwootPathSegment(%q)\n  obtido:  %q\n  esperado: %q", tc.in, got, tc.want)
			}
		})
	}
}
