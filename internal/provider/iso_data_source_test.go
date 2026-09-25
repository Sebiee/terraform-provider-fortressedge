package provider

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"

	"github.com/Sebiee/fortressedge/bake"
)

// fakeRelease writes a stand-in release ISO: a kernel, an initramfs, and
// the boot loader, dated as a release build is.
func fakeRelease(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"vmlinuz", "isolinux.bin", "ldlinux.c32"} {
		if err := os.WriteFile(filepath.Join(dir, name), append([]byte(name), make([]byte, 4096)...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var initrd bytes.Buffer
	gz := gzip.NewWriter(&initrd)
	gz.Write([]byte("070701")) // any archive: bake only appends
	gz.Close()
	if err := os.WriteFile(filepath.Join(dir, "initrd"), initrd.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	iso := filepath.Join(dir, "release.iso")
	at := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	if err := bake.WriteISO(iso, filepath.Join(dir, "vmlinuz"), filepath.Join(dir, "initrd"),
		filepath.Join(dir, "isolinux.bin"), filepath.Join(dir, "ldlinux.c32"), at); err != nil {
		t.Fatal(err)
	}
	return iso
}

// edgeYML is a fortress.yml with a fresh client CA.
func edgeYML(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test-ca"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	crt := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return "quic: true\nclient_ca: |\n  " + strings.ReplaceAll(strings.TrimSpace(string(crt)), "\n", "\n  ") + "\n"
}

// The provider's bake is fortressctl's: the same release and fortress.yml
// give the same bytes, whoever bakes them.
func TestBakeMatchesFortressctl(t *testing.T) {
	src, edge := fakeRelease(t), edgeYML(t)
	out, sum, err := bakeISO(t.TempDir(), src, []byte(edge))
	if err != nil {
		t.Fatal(err)
	}
	direct := filepath.Join(t.TempDir(), "edge.iso")
	if err := bake.ISO(direct, src, []byte(edge)); err != nil {
		t.Fatal(err)
	}
	if want, _ := fileSHA256(direct); sum != want {
		t.Fatalf("provider %s, bootimg %s", sum, want)
	}
	if got, _ := fileSHA256(out); got != sum || filepath.Base(out) != isoName(sum) {
		t.Fatalf("%s: %s", out, got)
	}
	if _, _, err := bakeISO(t.TempDir(), src, []byte("quic: true\n")); err == nil {
		t.Fatal("baked a fortress.yml without client_ca")
	}
}

func TestFetchReleasePinsAndCaches(t *testing.T) {
	src := fakeRelease(t)
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	sum, _ := fileSHA256(src)
	var gets atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		gets.Add(1)
		w.Write(body)
	}))
	defer srv.Close()
	cache := t.TempDir()

	bad := strings.Repeat("0", 64)
	if _, err := fetchRelease(context.Background(), cache, srv.URL, bad); err == nil || !strings.Contains(err.Error(), "not the pinned") {
		t.Fatalf("wrong pin: %v", err)
	}
	if ents, _ := os.ReadDir(filepath.Join(cache, "releases")); len(ents) != 0 {
		t.Fatalf("kept a download that did not match: %v", ents)
	}
	for range 2 {
		p, err := fetchRelease(context.Background(), cache, srv.URL, sum)
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := fileSHA256(p); got != sum {
			t.Fatalf("%s: %s", p, got)
		}
	}
	if n := gets.Load(); n != 2 {
		t.Fatalf("%d downloads, want 2: the refused one and the first good one", n)
	}
}

func TestCheck(t *testing.T) {
	good := isoModel{Config: types.StringValue(edgeYML(t)), ReleasePath: types.StringValue("r.iso"),
		ReleaseURL: types.StringNull(), ReleaseSHA256: types.StringNull()}
	if errs := good.check(); len(errs) != 0 {
		t.Fatalf("%v", errs)
	}
	for name, m := range map[string]isoModel{
		"both sources": {Config: good.Config, ReleasePath: good.ReleasePath, ReleaseURL: types.StringValue("https://x/r.iso"), ReleaseSHA256: types.StringValue(strings.Repeat("a", 64))},
		"no source":    {Config: good.Config, ReleasePath: types.StringNull(), ReleaseURL: types.StringNull(), ReleaseSHA256: types.StringNull()},
		"no pin":       {Config: good.Config, ReleasePath: types.StringNull(), ReleaseURL: types.StringValue("https://x/r.iso"), ReleaseSHA256: types.StringNull()},
		"bad pin":      {Config: good.Config, ReleasePath: types.StringNull(), ReleaseURL: types.StringValue("https://x/r.iso"), ReleaseSHA256: types.StringValue("ABC")},
		"policy key":   {Config: types.StringValue("block: [192.0.2.9]\n"), ReleasePath: good.ReleasePath, ReleaseURL: types.StringNull(), ReleaseSHA256: types.StringNull()},
	} {
		if errs := m.check(); len(errs) == 0 {
			t.Errorf("%s: accepted", name)
		}
	}
	// A value Terraform does not know yet is checked once it does.
	unknown := good
	unknown.Config = types.StringUnknown()
	if errs := unknown.check(); len(errs) != 0 {
		t.Fatalf("unknown config: %v", errs)
	}
}

// Through Terraform itself (TF_ACC=1): two plans of the same inputs agree,
// and the checksum is the bake's.
func TestAccISO(t *testing.T) {
	src, edge := fakeRelease(t), edgeYML(t)
	direct := filepath.Join(t.TempDir(), "edge.iso")
	if err := bake.ISO(direct, src, []byte(edge)); err != nil {
		t.Fatal(err)
	}
	want, _ := fileSHA256(direct)
	cfg := `provider "fortressedge" {
  cache_dir = "` + t.TempDir() + `"
}
data "fortressedge_iso" "edge" {
  release_path = "` + src + `"
  config       = <<-EOT
` + edge + `EOT
}
output "sha256" { value = data.fortressedge_iso.edge.sha256 }
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"fortressedge": providerserver.NewProtocol6WithError(New("test")()),
		},
		// The refused config goes first: the test destroys with the last one.
		Steps: []resource.TestStep{
			{
				Config:      strings.Replace(cfg, "quic: true", "quic: true\nblock: [192.0.2.9]", 1),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("fortressctl apply"),
			},
			{
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownOutputValue("sha256", knownvalue.StringExact(want)),
				},
			},
			{Config: cfg, PlanOnly: true}, // the same inputs plan no change
		},
	})
}
