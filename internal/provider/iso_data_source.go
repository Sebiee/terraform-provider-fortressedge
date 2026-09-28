package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Sebiee/fortressedge/bake"
)

type isoDataSource struct{ cacheDir string }

func newISO() datasource.DataSource { return &isoDataSource{} }

// isoModel's fortress.yml attributes are bake.Config's fields, each named
// as its fortress.yml key.
type isoModel struct {
	ClientCA          types.String `tfsdk:"client_ca"`
	ACME              types.String `tfsdk:"acme"`
	ACMECA            types.String `tfsdk:"acme_ca"`
	NTP               types.String `tfsdk:"ntp"`
	RenewInterval     types.String `tfsdk:"renew_interval"`
	QUIC              types.Bool   `tfsdk:"quic"`
	AccessLog         types.Bool   `tfsdk:"access_log"`
	AccessLogMaxSize  types.String `tfsdk:"access_log_max_size"`
	AccessLogMaxFiles types.Int64  `tfsdk:"access_log_max_files"`

	ReleaseURL    types.String `tfsdk:"release_url"`
	ReleaseSHA256 types.String `tfsdk:"release_sha256"`
	ReleasePath   types.String `tfsdk:"release_path"`
	ID            types.String `tfsdk:"id"`
	Path          types.String `tfsdk:"path"`
	SHA256        types.String `tfsdk:"sha256"`
	FileName      types.String `tfsdk:"file_name"`
}

// config is m's fortress.yml, and whether Terraform knows all of it yet.
func (m isoModel) config() (bake.Config, bool) {
	for _, v := range []attr.Value{m.ClientCA, m.ACME, m.ACMECA, m.NTP, m.RenewInterval,
		m.QUIC, m.AccessLog, m.AccessLogMaxSize, m.AccessLogMaxFiles} {
		if v.IsUnknown() {
			return bake.Config{}, false
		}
	}
	return bake.Config{
		ClientCA:          m.ClientCA.ValueString(),
		ACME:              m.ACME.ValueString(),
		ACMECA:            m.ACMECA.ValueString(),
		NTP:               m.NTP.ValueString(),
		RenewInterval:     m.RenewInterval.ValueString(),
		QUIC:              m.QUIC.ValueBool(),
		AccessLog:         m.AccessLog.ValueBool(),
		AccessLogMaxSize:  m.AccessLogMaxSize.ValueString(),
		AccessLogMaxFiles: int(m.AccessLogMaxFiles.ValueInt64()),
	}, true
}

func (d *isoDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iso"
}

func (d *isoDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A FortressEdge release ISO with fortress.yml baked in, written from these attributes, " +
			"as `fortressctl bake` makes it. The same release and settings give the same bytes, so `sha256` " +
			"is known at plan time. The settings are checked at plan time, as the edge checks them.",
		Attributes: map[string]schema.Attribute{
			"client_ca": schema.StringAttribute{
				Required: true,
				Description: "The PEM CA certificate that signs the dark-node, operator, and log-reader " +
					"certificates: exactly one certificate.",
			},
			"acme": schema.StringAttribute{
				Optional:    true,
				Description: "The ACME directory URL. Default: Let's Encrypt.",
			},
			"acme_ca": schema.StringAttribute{
				Optional:    true,
				Description: "The PEM CA that signed the ACME directory's HTTPS certificate. Default: the system roots.",
			},
			"ntp": schema.StringAttribute{
				Optional:    true,
				Description: "The time source for the boot clock sync, host or host:port. Default: pool.ntp.org.",
			},
			"renew_interval": schema.StringAttribute{
				Optional:    true,
				Description: "How often ACME certificates are checked and renewed once due, a duration such as 4h. Default: 4h.",
			},
			"quic": schema.BoolAttribute{
				Optional:    true,
				Description: "Dark nodes may also connect over QUIC on UDP 443. Default: false.",
			},
			"access_log": schema.BoolAttribute{
				Optional:    true,
				Description: "One JSON line per site request, served at /~!ops/access. Default: false.",
			},
			"access_log_max_size": schema.StringAttribute{
				Optional:    true,
				Description: "The size at which the access log starts a new file, such as 8MiB (KiB, MiB, GiB). Default: 8MiB.",
			},
			"access_log_max_files": schema.Int64Attribute{
				Optional:    true,
				Description: "Access log files kept, the current one included. Default: 3.",
			},
			"release_url": schema.StringAttribute{
				Optional:    true,
				Description: "Where to download the release ISO, such as its GitHub release asset. Needs release_sha256.",
			},
			"release_sha256": schema.StringAttribute{
				Optional:    true,
				Description: "The release ISO's SHA-256, from the release's SHA256SUMS. A download that does not match is refused.",
			},
			"release_path": schema.StringAttribute{
				Optional:    true,
				Description: "A release ISO on this machine, instead of release_url, such as one make iso built.",
			},
			"id":        schema.StringAttribute{Computed: true, Description: "The baked ISO's SHA-256."},
			"path":      schema.StringAttribute{Computed: true, Description: "The baked ISO, in the cache directory."},
			"sha256":    schema.StringAttribute{Computed: true, Description: "The baked ISO's SHA-256, for the platform's upload."},
			"file_name": schema.StringAttribute{Computed: true, Description: "A name for the upload that changes with its content: fortressedge-<12 hex digits>.iso."},
		},
	}
}

func (d *isoDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if dir, ok := req.ProviderData.(string); ok {
		d.cacheDir = dir
	}
}

func (d *isoDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var m isoModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for _, e := range m.check() {
		resp.Diagnostics.AddAttributeError(e.at, "Invalid "+e.at.String(), e.msg)
	}
}

type attrError struct {
	at  path.Path
	msg string
}

var hexSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// check finds what is wrong with the known values of m. An unknown value
// is checked when Terraform knows it.
func (m isoModel) check() []attrError {
	var out []attrError
	url, local := m.ReleaseURL, m.ReleasePath
	switch {
	case url.IsUnknown() || local.IsUnknown():
	case url.IsNull() == local.IsNull():
		out = append(out, attrError{path.Root("release_url"), "set release_url (with release_sha256) or release_path, one of them"})
	case !url.IsNull() && m.ReleaseSHA256.IsNull():
		out = append(out, attrError{path.Root("release_sha256"), "release_url needs release_sha256, from the release's SHA256SUMS"})
	}
	if s := m.ReleaseSHA256; !s.IsNull() && !s.IsUnknown() && !hexSHA256.MatchString(s.ValueString()) {
		out = append(out, attrError{path.Root("release_sha256"), "want 64 lower-case hex digits"})
	}
	if cfg, known := m.config(); known {
		if err := cfg.Check(); err != nil {
			out = append(out, attrError{attribute(err), err.Error()})
		}
	}
	return out
}

// attribute is the attribute an error from bake.Config.Check is about:
// the fortress.yml key it names ("fortress.yml: acme: ..."), which is also
// the attribute's name. client_ca, the one required, holds the others.
func attribute(err error) path.Path {
	msg, _ := strings.CutPrefix(err.Error(), "fortress.yml: ")
	if i := strings.IndexAny(msg, ": "); i > 0 && slices.Contains(bake.Keys(), msg[:i]) {
		return path.Root(msg[:i])
	}
	return path.Root("client_ca")
}

func (d *isoDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m isoModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if errs := m.check(); len(errs) > 0 {
		for _, e := range errs {
			resp.Diagnostics.AddAttributeError(e.at, "Invalid "+e.at.String(), e.msg)
		}
		return
	}
	cfg, _ := m.config() // Read sees known values only
	src := m.ReleasePath.ValueString()
	if src == "" {
		var err error
		if src, err = fetchRelease(ctx, d.cacheDir, m.ReleaseURL.ValueString(), m.ReleaseSHA256.ValueString()); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("release_url"), "Release not downloaded", err.Error())
			return
		}
	}
	out, sum, err := bakeISO(d.cacheDir, src, cfg.YAML())
	if err != nil {
		resp.Diagnostics.AddError("Bake failed", err.Error())
		return
	}
	m.ID, m.Path, m.SHA256 = types.StringValue(sum), types.StringValue(out), types.StringValue(sum)
	m.FileName = types.StringValue(isoName(sum))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func isoName(sum string) string { return "fortressedge-" + sum[:12] + ".iso" }

// fetchRelease returns the release ISO at url, downloaded into cache once
// and kept under its SHA-256, which must be sum.
func fetchRelease(ctx context.Context, cache, url, sum string) (string, error) {
	dir := filepath.Join(cache, "releases")
	final := filepath.Join(dir, sum+".iso")
	if got, err := fileSHA256(final); err == nil && got == sum {
		return final, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	tmp, err := os.CreateTemp(dir, ".download-")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), resp.Body); err != nil {
		tmp.Close()
		return "", fmt.Errorf("GET %s: %w", url, err)
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != sum {
		return "", fmt.Errorf("%s has SHA-256 %s, not the pinned %s", url, got, sum)
	}
	return final, os.Rename(tmp.Name(), final)
}

// bake bakes edge into the release ISO at src, into cache under the
// result's name, and returns its path and SHA-256.
func bakeISO(cache, src string, edge []byte) (string, string, error) {
	dir := filepath.Join(cache, "iso")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	tmp, err := os.CreateTemp(dir, ".bake-*.iso")
	if err != nil {
		return "", "", err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if err := bake.ISO(tmp.Name(), src, edge); err != nil {
		return "", "", err
	}
	sum, err := fileSHA256(tmp.Name())
	if err != nil {
		return "", "", err
	}
	final := filepath.Join(dir, isoName(sum))
	return final, sum, os.Rename(tmp.Name(), final)
}

func fileSHA256(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
