package auth

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("contraseña-segura")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "contraseña-segura" {
		t.Fatal("el hash no puede ser la contraseña en claro")
	}
	if !VerifyPassword("contraseña-segura", hash) {
		t.Fatal("la contraseña correcta debe verificar")
	}
	if VerifyPassword("otra-contraseña", hash) {
		t.Fatal("una contraseña incorrecta no debe verificar")
	}
}

func TestHashIsSalted(t *testing.T) {
	h1, err := HashPassword("misma")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := HashPassword("misma")
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Fatal("dos hashes de la misma contraseña deben diferir (salt aleatorio)")
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	for _, hash := range []string{"", "plain", "$argon2id$v=19$m=1,t=1,p=1$xx", "$otro$v=19$m=1,t=1,p=1$xx$yy"} {
		if VerifyPassword("x", hash) {
			t.Fatalf("hash malformado aceptado: %q", hash)
		}
	}
}
