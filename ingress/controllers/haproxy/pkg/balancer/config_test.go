package balancer

import (
	"reflect"
	"testing"
)

var certs = []struct {
	priv    []byte
	pub     []byte
	name    string
	Domains []string
}{
	{
		[]byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAvQrQcCnN6WoaliacMLmWDjsX74mPGJJ/4OHO7vwelH/ffu0z
5QZPDsMewsLsg+2gNPNK77N4caUlSiwnHr9mlqa1IdBhLmGNxqfLrlgKJQiw8PIh
UZK6yOzZO5OrRfuhOxud+oBOYOfa2O0MzygWZmek2QUPQBS84UuWrHCM1fNFQ114
4xPXNQrTBdugFYK38njtmyotgWBcy1Ay4lUHEIEWfM/S7KQUQBWtPLOyIvrPeCei
vbiTuFtZaZEnuFh62qGqU6BUesOpfcDu0eyHEws+07p5uXsjy+ZOC8f2hOiqN5lg
6lbL9Jjaum9GAdhKUB30WFjxQyYGcD018oSZFwIDAQABAoIBAF+t44WBgxiKfV5V
uGPo6ovVWO4B/4z+SP73CxdmP8wFVIcXY1addNIR67XDlpXHZXinUtwzR9itL0x1
QG+NjEzfn3m30Bf7hBhxwONC6A+KcJPi2P5Cd4tOZTyEJwHKs/YIqlKpMgJWFywH
/p2yunOmLYcxymAyns6gxWgNpxp5n9ScLQkuTAeQ+X4yL44JLbpaebG3ExYWpBlB
f6eKHkcrci8ijyQJ5WVTRsByqMsvmHh/OyqKz+DaRGkAMjix9somxrEYlOzIpCwc
62bxqgP/h/H81sEmwB7jqwGznbo4pZFtwg+g4tpK2acInvxMCvbX/H/Sxml+jol2
/81332ECgYEA+HH2PL5dvp5PSHi5HuCXwxA9s/G7wD4bnMa5kAVNzEVHqHop8ymz
IhSVJmbVW1bc7V90p2qE7KJ1lmUMwTowTvPz1PYCExTZUImteQX1RmxjsjEMFFQY
6m6xbEn2iEq90tpp96aSi54Ubxjiaw1vy9gVNkzV5ethkFV2XkyZcukCgYEAwspt
IlMVyaL3HVXM7nVbHRzSzYSwSNNgs9r+FCsaerPvIKhrwbwMhLTX2HIc0Btv/OxD
I0I8+PljAyCi4FW20PAhBelorZ9LA3MaVS6rIcw6c2LxArNeSyoGDQsh7ot+rH+o
fgPTLIJvFrxn9scwc/NLuP+9d2PkcZjjsn2uK/8CgYBswPYZAPvoRURPZQkkCwxj
xug7rMWTEZzks9jmwmubz3feuBtE5iwT7w6bEMi0gwGSpwZZgrdNHpB6lSFQNDiR
VxiVUFr4H1hBeQMGxyTm/utlRTMUcvu1I19nF7ljT9RoSFO6pJ/hngEz4KC8W9Vk
VeJzMo8vZin/FGoMPVuugQKBgQDB7od5BP9MINOmgSXmwzBTa770noZj+w7sAbu0
mLVkNIB/Iy8lUvOjq+i5teK5zpdQWGj/UZMiziellXiToMLCglBecmOleFJWvOIa
rLv0ikAnYPpSlgHrE4uysMK3nGohk3dM/sHgLnwrRqi7KNU0m6VoKjWYB/wInQ8V
RcuCQQKBgQDc1TZM3spR4ZuZzuaZrwibOUDygquh0irW/NNR69ieX7Vdy8cPsq3J
kRhK2DDuS5YaOxNB2cV8taRuVZ7CnNnqOnhUqjzJ41qsTjIcO5mQ3iwCyBmjIMyF
Geor8l7dvWfg925oJ+ULRrkMAw0a0ZHDVjgNlLCIWy69pNqXEGJVKQ==
-----END RSA PRIVATE KEY-----`),
		[]byte(`-----BEGIN CERTIFICATE-----
MIID1zCCAr+gAwIBAgIJAOan/2K8iXCSMA0GCSqGSIb3DQEBBQUAMFAxCzAJBgNV
BAYTAlVTMRAwDgYDVQQIEwdTZWF0dGxlMRIwEAYDVQQKEwlTdGFja3BvaW4xGzAZ
BgNVBAMTEmdhbWUuc3RhY2twb2ludC5pbzAeFw0xNjA5MTQxMzU1NDZaFw0xNjEw
MTQxMzU1NDZaMFAxCzAJBgNVBAYTAlVTMRAwDgYDVQQIEwdTZWF0dGxlMRIwEAYD
VQQKEwlTdGFja3BvaW4xGzAZBgNVBAMTEmdhbWUuc3RhY2twb2ludC5pbzCCASIw
DQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAL0K0HApzelqGpYmnDC5lg47F++J
jxiSf+Dhzu78HpR/337tM+UGTw7DHsLC7IPtoDTzSu+zeHGlJUosJx6/ZpamtSHQ
YS5hjcany65YCiUIsPDyIVGSusjs2TuTq0X7oTsbnfqATmDn2tjtDM8oFmZnpNkF
D0AUvOFLlqxwjNXzRUNdeOMT1zUK0wXboBWCt/J47ZsqLYFgXMtQMuJVBxCBFnzP
0uykFEAVrTyzsiL6z3gnor24k7hbWWmRJ7hYetqhqlOgVHrDqX3A7tHshxMLPtO6
ebl7I8vmTgvH9oToqjeZYOpWy/SY2rpvRgHYSlAd9FhY8UMmBnA9NfKEmRcCAwEA
AaOBszCBsDAdBgNVHQ4EFgQUcmTQ+QsLcwsC/xBfIsV9ZR98o2AwgYAGA1UdIwR5
MHeAFHJk0PkLC3MLAv8QXyLFfWUffKNgoVSkUjBQMQswCQYDVQQGEwJVUzEQMA4G
A1UECBMHU2VhdHRsZTESMBAGA1UEChMJU3RhY2twb2luMRswGQYDVQQDExJnYW1l
LnN0YWNrcG9pbnQuaW+CCQDmp/9ivIlwkjAMBgNVHRMEBTADAQH/MA0GCSqGSIb3
DQEBBQUAA4IBAQAsL6mYoqZrMsesP44+ZEe42CWTkhSjpp8cSxgG2vqj6BUjgCYs
kz3Z4wtH2amB7rPq5koz715LcSVzgcAOxFzMnsLg//bG4jfiC/1noWFLLSO2BF+8
0i5mku3s6roi7AUx1ZIVgnHJsiVgz1sQ9V3+XIwQEHlPMzxCvaW+Uof+Ks/61tBh
k3hPqK5pSJl09NXOJxKQvlPaNwbJDM2a5h1R0/r03twl9QhIfMeQ2kel+ywcBPZr
GK+KHsjGrpG7wnu9m6A5W3H9xMaporoOrhctuLcX1ongKW8h+uilJv0Ejs1wcOVL
f37y1oNpDFKfx6FXixrCYkSb6JgZBUSNl7Vb
-----END CERTIFICATE-----`),
		"test1",
		[]string{"game.stackpoint.io"},
	},
}

func TestNewcertificate(t *testing.T) {

	for _, tc := range certs {

		c, _ := NewCertificate(tc.priv, tc.pub, tc.name)

		if c.Name != tc.name {
			t.Errorf("expected name '%s', got '%s'", tc.name, c.Name)
		}

		if !reflect.DeepEqual(c.Domains, tc.Domains) {
			t.Errorf("expected domains %+v, got %+v", tc.Domains, c.Domains)
		}
	}
}
