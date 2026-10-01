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
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
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

// testCA is a fresh client CA, PEM.
func testCA(t *testing.T) string {
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
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// edgeYML is a fortress.yml with a fresh client CA.
func edgeYML(t *testing.T) string {
	t.Helper()
	return string(bake.Config{ClientCA: testCA(t), QUIC: true}.YAML())
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

// nullModel is an isoModel with every attribute null, as Terraform passes
// an empty block.
func nullModel() isoModel {
	m := isoModel{}
	v := reflect.ValueOf(&m).Elem()
	for i := range v.NumField() {
		switch v.Field(i).Interface().(type) {
		case types.String:
			v.Field(i).Set(reflect.ValueOf(types.StringNull()))
		case types.Bool:
			v.Field(i).Set(reflect.ValueOf(types.BoolNull()))
		case types.Int64:
			v.Field(i).Set(reflect.ValueOf(types.Int64Null()))
		case types.List:
			v.Field(i).Set(reflect.ValueOf(types.ListNull(types.StringType)))
		}
	}
	return m
}

// Every fortress.yml key is an attribute of the same name, in the model
// and in the schema: a key fortressedge adds fails here until it is.
func TestAttributesCoverConfig(t *testing.T) {
	var tags []string
	for f := range reflect.TypeFor[isoModel]().Fields() {
		tags = append(tags, f.Tag.Get("tfsdk"))
	}
	var resp datasource.SchemaResponse
	newISO().Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	for _, k := range bake.Keys() {
		if !slices.Contains(tags, k) {
			t.Errorf("isoModel has no %s", k)
		}
		if _, ok := resp.Schema.Attributes[k]; !ok {
			t.Errorf("the schema has no %s", k)
		}
	}
}

// The AppRole's secret ID stays out of plans and logs.
func TestVaultSecretIDIsSensitive(t *testing.T) {
	var resp datasource.SchemaResponse
	newISO().Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	if !resp.Schema.Attributes["vault_secret_id"].IsSensitive() {
		t.Fatal("vault_secret_id is not sensitive")
	}
}

func TestCheck(t *testing.T) {
	good := nullModel()
	good.ClientCA, good.ReleasePath = types.StringValue(testCA(t)), types.StringValue("r.iso")
	if errs := good.check(); len(errs) != 0 {
		t.Fatalf("%v", errs)
	}
	with := func(f func(*isoModel)) isoModel { m := good; f(&m); return m }
	for name, tc := range map[string]struct {
		m  isoModel
		at string
	}{
		"both sources": {with(func(m *isoModel) {
			m.ReleaseURL, m.ReleaseSHA256 = types.StringValue("https://x/r.iso"), types.StringValue(strings.Repeat("a", 64))
		}), "release_url"},
		"no source": {with(func(m *isoModel) { m.ReleasePath = types.StringNull() }), "release_url"},
		"no pin": {with(func(m *isoModel) {
			m.ReleasePath, m.ReleaseURL = types.StringNull(), types.StringValue("https://x/r.iso")
		}), "release_sha256"},
		"bad pin": {with(func(m *isoModel) {
			m.ReleasePath, m.ReleaseURL, m.ReleaseSHA256 = types.StringNull(), types.StringValue("https://x/r.iso"), types.StringValue("ABC")
		}), "release_sha256"},
		"not a CA":       {with(func(m *isoModel) { m.ClientCA = types.StringValue("hello") }), "client_ca"},
		"acme not a URL": {with(func(m *isoModel) { m.ACME = types.StringValue("not a url") }), "acme"},
		"bad interval":   {with(func(m *isoModel) { m.RenewInterval = types.StringValue("often") }), "renew_interval"},
		"vault over http": {with(func(m *isoModel) {
			m.Vault, m.VaultMount, m.VaultPath = types.StringValue("http://v.example:8200"), types.StringValue("m"), types.StringValue("p")
			m.VaultRoleID, m.VaultSecretID = types.StringValue("r"), types.StringValue("s")
		}), "vault"},
		"vault without its approle": {with(func(m *isoModel) {
			m.Vault, m.VaultMount, m.VaultPath = types.StringValue("https://v.example:8200"), types.StringValue("m"), types.StringValue("p")
		}), "vault_role_id"},
		"bad ntp": {with(func(m *isoModel) {
			m.NTP = types.ListValueMust(types.StringType, []attr.Value{types.StringValue("ntp11.metas.ch"), types.StringValue("not a name")})
		}), "ntp"},
	} {
		errs := tc.m.check()
		if len(errs) == 0 {
			t.Errorf("%s: accepted", name)
			continue
		}
		if at := errs[0].at.String(); at != tc.at {
			t.Errorf("%s: on %s, want %s: %s", name, at, tc.at, errs[0].msg)
		}
	}
	// A value Terraform does not know yet is checked once it does.
	unknown := with(func(m *isoModel) { m.ClientCA, m.ACME = types.StringUnknown(), types.StringValue("not a url") })
	if errs := unknown.check(); len(errs) != 0 {
		t.Fatalf("unknown client_ca: %v", errs)
	}
}

// Through Terraform itself (TF_ACC=1): two plans of the same inputs agree,
// and the checksum is the bake's.
func TestAccISO(t *testing.T) {
	src, ca := fakeRelease(t), testCA(t)
	direct := filepath.Join(t.TempDir(), "edge.iso")
	ntp := []string{"ntp11.metas.ch", "ntp12.metas.ch", "ntp13.metas.ch"}
	if err := bake.ISO(direct, src, bake.Config{ClientCA: ca, QUIC: true, NTP: ntp}.YAML()); err != nil {
		t.Fatal(err)
	}
	want, _ := fileSHA256(direct)
	cfg := `provider "fortressedge" {
  cache_dir = "` + t.TempDir() + `"
}
data "fortressedge_iso" "edge" {
  release_path = "` + src + `"
  quic         = true
  ntp          = ["ntp11.metas.ch", "ntp12.metas.ch", "ntp13.metas.ch"]
  client_ca    = <<-EOT
` + ca + `EOT
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
				Config:      strings.Replace(cfg, "quic         = true", `acme = "not a url"`, 1),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid acme`),
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
