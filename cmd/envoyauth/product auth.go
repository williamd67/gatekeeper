package main

import (
	"errors"

	"github.com/bmatcuk/doublestar"
	"go.uber.org/zap"

	"github.com/erikbos/gatekeeper/pkg/shared"
	"github.com/erikbos/gatekeeper/pkg/types"
)

// CheckProductEntitlement loads developer, dev app, apiproduct details,
// as input request.apikey must be set
//
func (a *authorizationServer) CheckProductEntitlement(organization string, request *requestInfo) error {

	if err := a.getAPIKeyDevDevAppDetails(request); err != nil {
		return err
	}
	if err := checkDevAndKeyValidity(request); err != nil {
		return err
	}

	var err error
	request.APIProduct, err = a.IsRequestPathAllowed(organization, request.URL.Path, request.appCredential)
	return err
}

// getAPIKeyDevDevAppDetails populates apikey, developer and developerapp details
func (a *authorizationServer) getAPIKeyDevDevAppDetails(request *requestInfo) error {
	var err error

	request.appCredential, err = a.cache.GetDeveloperAppKey(request.apikey)
	// in case we do not have this apikey in cache let's try to retrieve it from database
	if err != nil {
		request.appCredential, err = a.db.Credential.GetByKey(&request.vhost.OrganizationName, request.apikey)
		if err != nil {
			// FIX ME increase unknown apikey counter (not an error state)
			return errors.New("Cannot find apikey")
		}
		// Store retrieved app credential in cache, in case of error we proceed as we can
		// statisfy the request as we did retrieve succesful from database
		if err = a.cache.StoreDeveloperAppKey(request.apikey, request.appCredential); err != nil {
			a.logger.Debug("Cannot store apikey in cache", zap.String("key", *request.apikey))
		}
	}

	request.developerApp, err = a.cache.GetDeveloperApp(&request.appCredential.AppID)
	// in case we do not have developer app in cache let's try to retrieve it from database
	if err != nil {
		request.developerApp, err = a.db.DeveloperApp.GetByID(request.vhost.OrganizationName,
			request.appCredential.AppID)
		if err != nil {
			// FIX ME increase counter as every apikey should link to dev app (error state)
			return errors.New("Cannot find developer app of this apikey")
		}
		// Store retrieved developer app in cache, in case of error we proceed as we can
		// statisfy the request as we did retrieve succesful from database
		if err = a.cache.StoreDeveloperApp(&request.developerApp.AppID, request.developerApp); err != nil {
			a.logger.Debug("Cannot store developer app in cache", zap.String("appid", request.developerApp.AppID))
		}
	}

	request.developer, err = a.cache.GetDeveloper(&request.developerApp.DeveloperID)
	// in case we do not have develop in cache let's try to retrieve it from database
	if err != nil {
		request.developer, err = a.db.Developer.GetByID(request.developerApp.DeveloperID)
		if err != nil {
			// FIX ME increase counter as every devapp should link to developer (error state)
			return errors.New("Cannot find developer of developer app")
		}
		// Store retrieved developer in cache, in case of error we proceed as we can
		// statisfy the request as we did retrieve succesful from database
		if err = a.cache.StoreDeveloper(&request.developer.DeveloperID, request.developer); err != nil {
			a.logger.Debug("Cannot store developer in cache", zap.String("devid", request.developer.DeveloperID))
		}
	}

	return nil
}

// checkDevAndKeyValidity checks devapp approval and expiry status
func checkDevAndKeyValidity(request *requestInfo) error {

	now := shared.GetCurrentTimeMilliseconds()

	if request.developer.SuspendedTill != -1 &&
		now < request.developer.SuspendedTill {

		return errors.New("Developer suspended")
	}

	if request.appCredential.Status != "approved" {
		// FIXME increase unapproved dev app counter (not an error state)
		return errors.New("Unapproved apikey")
	}

	if request.appCredential.ExpiresAt != -1 {
		if now > request.appCredential.ExpiresAt {
			// FIXME increase expired dev app credentials counter (not an error state))
			return errors.New("Expired apikey")
		}
	}
	return nil
}

// IsRequestPathAllowed
// - iterate over products in apikey
// - 	iterate over path(s) of each product:
// - 		if requestor path matches paths(s)
// -			- return 200
// - if not 403

func (a *authorizationServer) IsRequestPathAllowed(organization, requestPath string,
	credential *types.DeveloperAppKey) (*types.APIProduct, error) {

	// Does this apikey have any products assigned?
	if len(credential.APIProducts) == 0 {
		return nil, errors.New("No active products")
	}

	// Iterate over this key's apiproducts
	for _, apiproduct := range credential.APIProducts {
		if apiproduct.Status == "approved" {

			// apiproductDetails, err := a.db.GetAPIProductByName(organization, apiproduct.Apiproduct)
			apiproductDetails, err := a.getAPIProduct(&organization, &apiproduct.Apiproduct)
			if err != nil {
				// apikey has product in it which we cannot find:
				// FIXME increase "unknown product in apikey" counter (not an error state)
			} else {
				// Iterate over all paths of apiproduct and try to match with path of request
				for _, productPath := range apiproductDetails.Paths {
					a.logger.Debug("IsRequestPathAllowed",
						zap.String("productpath", productPath),
						zap.String("requestpath", requestPath))

					if ok, _ := doublestar.Match(productPath, requestPath); ok {
						return apiproductDetails, nil
					}
				}
			}
		}
	}
	return nil, errors.New("Not authorized for requested path")
}

// getAPIPRoduct retrieves an API Product through mem cache
func (a *authorizationServer) getAPIProduct(organization, apiproductname *string) (*types.APIProduct, error) {

	var product *types.APIProduct

	product, err := a.cache.GetAPIProduct(organization, apiproductname)
	// in case we do not have product in cache let's try to retrieve it from database
	if err != nil {
		product, err = a.db.APIProduct.Get(*organization, *apiproductname)
		if err == nil {
			// Store retrieved APIProduct in cache, in case of error we proceed as we can
			// statisfy the request as we did retrieve succesful from database
			if err2 := a.cache.StoreAPIProduct(organization, apiproductname, product); err2 != nil {
				a.logger.Debug("Cannot store api product in cache", zap.String("apiproduct", *apiproductname))
			}
		}
	}
	return product, err
}
