.PHONY: build test testacc

build:
	CGO_ENABLED=0 go build -trimpath -o terraform-provider-fortressedge .

test:
	go test -race ./...

# The acceptance test runs Terraform itself.
testacc:
	TF_ACC=1 go test -race -v ./...
