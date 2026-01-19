package swagger

import "embed"

// SwaggerUI contains the Swagger UI static files
//
//go:embed swagger-ui/*
var SwaggerUI embed.FS
