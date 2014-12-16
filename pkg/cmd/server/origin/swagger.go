package origin

import (
	"fmt"

	"github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful/swagger"
)

// SwaggerAPI registers the swagger API endpoints.
type SwaggerAPI struct{}

const swaggerAPIPrefix = "/swaggerapi/"

func (s *SwaggerAPI) InstallAPI(container *restful.Container) []string {
	// TODO: keep in sync with upstream
	swaggerConfig := swagger.Config{
		WebServices: container.RegisteredWebServices(),
		ApiPath:     swaggerAPIPrefix,
	}
	swagger.RegisterSwaggerService(swaggerConfig, container)
	return []string{
		fmt.Sprintf("Started Swagger Schema API at %%s%s", swaggerAPIPrefix),
	}
}
