package boot

import (
	"fmt"

	"github.com/Azure/open-service-broker-azure/pkg/azure"
	"github.com/Azure/open-service-broker-azure/pkg/service"
)

// GetCatalog returns a fully initialized catalog
func GetCatalog(
	catalogConfig service.CatalogConfig,
	azureConfig azure.Config,
) (service.Catalog, error) {
	modules, err := getModules(azureConfig)
	if err != nil {
		return nil, fmt.Errorf("error getting modules: %s", err)
	}

	// Consolidate the catalogs from all the individual modules into a single
	// catalog. Check as we go along to make sure that no two modules provide
	// services having the same ID.
	services := []service.Service{}
	usedServiceIDs := map[string]string{}
	for _, module := range modules {
		moduleName := module.GetName()
		catalog, err := module.GetCatalog()
		if err != nil {
			return nil, fmt.Errorf(
				`error retrieving catalog from module "%s": %s`,
				moduleName,
				err,
			)
		}

		enableMigrationServices := catalogConfig.EnableMigrationServices
		for _, svc := range catalog.GetServices() {
			serviceID := svc.GetID()
			if moduleNameForUsedServiceID, ok := usedServiceIDs[serviceID]; ok {
				return nil, fmt.Errorf(
					`modules "%s" and "%s" both provide a service with the id "%s"`,
					moduleNameForUsedServiceID,
					moduleName,
					serviceID,
				)
			}

			serviceTags := svc.GetTags()
			tagsMap := map[string]bool{}
			for _, t := range serviceTags {
				tagsMap[t] = true
			}
			// Skip migration services if disabled
			if !enableMigrationServices && tagsMap[service.MigrationTag] {
				continue
			}

			filteredPlans := []service.Plan{}
			for _, plan := range svc.GetPlans() {
				if plan.GetStability() >= catalogConfig.MinStability {
					pProp := plan.GetProperties()
					pProp.Schemas.AddCommonSchema(svc.GetProperties())
					filteredPlans = append(filteredPlans, service.NewPlan(pProp))
				}
			}
			if len(filteredPlans) > 0 {
				services = append(services, service.NewService(
					svc.GetProperties(),
					svc.GetServiceManager(),
					filteredPlans...,
				))
				usedServiceIDs[serviceID] = moduleName
			}
		}
	}
	catalog := service.NewCatalog(services)

	return catalog, nil
}
