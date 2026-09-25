// Command terraform-provider-fortressedge is the Terraform provider for
// FortressEdge. Its data source, fortressedge_iso, bakes a release ISO
// with fortress.yml on the machine that runs Terraform, as fortressctl
// bake does, byte for byte.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/Sebiee/terraform-provider-fortressedge/internal/provider"
)

// version is set by goreleaser: -X main.version={{.Version}}.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "serve for a debugger (TF_REATTACH_PROVIDERS)")
	flag.Parse()
	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/sebiee/fortressedge",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
