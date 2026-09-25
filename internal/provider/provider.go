package provider

import (
	"context"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type fortressProvider struct{ version string }

// New returns the provider for providerserver.Serve. version is the
// release's, which the registry shows.
func New(version string) func() provider.Provider {
	return func() provider.Provider { return &fortressProvider{version: version} }
}

func (p *fortressProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "fortressedge"
	resp.Version = p.version
}

func (p *fortressProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Bakes FortressEdge ISOs on the machine that runs Terraform.",
		Attributes: map[string]schema.Attribute{
			"cache_dir": schema.StringAttribute{
				Optional: true,
				Description: "Where downloaded releases and baked ISOs are kept. " +
					"Default: fortressedge in the user's cache directory ($XDG_CACHE_HOME or ~/.cache).",
			},
		},
	}
}

func (p *fortressProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg struct {
		CacheDir types.String `tfsdk:"cache_dir"`
	}
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	dir := cfg.CacheDir.ValueString()
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			resp.Diagnostics.AddError("No cache directory", err.Error()+"; set cache_dir")
			return
		}
		dir = filepath.Join(base, "fortressedge")
	}
	resp.DataSourceData = dir
}

func (p *fortressProvider) Resources(context.Context) []func() resource.Resource { return nil }

func (p *fortressProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{newISO}
}
