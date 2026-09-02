package localization

import "testing"

func TestFromAcceptLanguage(t *testing.T) {
	tests := []struct {
		header string
		want   Locale
	}{
		{header: "en-US,en;q=0.9", want: EnglishUS},
		{header: "pt-PT,pt;q=0.9", want: PortugueseBrazil},
		{header: "fr-FR,fr;q=0.9", want: PortugueseBrazil},
		{header: "", want: PortugueseBrazil},
	}

	for _, test := range tests {
		if got := FromAcceptLanguage(test.header); got != test.want {
			t.Errorf("FromAcceptLanguage(%q) = %q, want %q", test.header, got, test.want)
		}
	}
}

func TestMessageUsesSelectedLocale(t *testing.T) {
	if got := Message(EnglishUS, AccountCreated, nil); got != "Administrator account created." {
		t.Errorf("English message = %q", got)
	}
	if got := Message(PortugueseBrazil, AccountCreated, nil); got != "Conta de administrador criada." {
		t.Errorf("Portuguese message = %q", got)
	}
}
