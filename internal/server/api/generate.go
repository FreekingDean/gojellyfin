package api

//go:generate go run ./specpatch -spec ../../../spec/jellyfin-openapi-stable.json -overrides ../../../spec/overrides.json -out ../../../spec/jellyfin-openapi-stable.patched.json
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config=config.yaml ../../../spec/jellyfin-openapi-stable.patched.json
//go:generate go run ./gen
