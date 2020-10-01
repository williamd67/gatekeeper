package service

import (
	"fmt"

	"github.com/erikbos/gatekeeper/pkg/db"
	"github.com/erikbos/gatekeeper/pkg/shared"
	"github.com/erikbos/gatekeeper/pkg/types"
)

// OrganizationService has the CRUD methods to manipulate the organization entity
type OrganizationService struct {
	db        *db.Database
	changelog *Changelog
}

// NewOrganizationService returns a new organization instance
func NewOrganizationService(database *db.Database, c *Changelog) *OrganizationService {

	return &OrganizationService{db: database, changelog: c}
}

// GetAll returns all organizations
func (os *OrganizationService) GetAll() (organizations types.Organizations, err types.Error) {

	return os.db.Organization.GetAll()
}

// Get returns details of an organization
func (os *OrganizationService) Get(name string) (organization *types.Organization, err types.Error) {

	return os.db.Organization.Get(name)
}

// GetAttributes returns attributes of an organization
func (os *OrganizationService) GetAttributes(OrganizatioName string) (attributes *types.Attributes, err types.Error) {

	organization, err := os.db.Organization.Get(OrganizatioName)
	if err != nil {
		return nil, err
	}
	return &organization.Attributes, nil
}

// GetAttribute returns one particular attribute of an organization
func (os *OrganizationService) GetAttribute(OrganizationName, attributeName string) (value string, err types.Error) {

	organization, err := os.db.Organization.Get(OrganizationName)
	if err != nil {
		return "", err
	}
	return organization.Attributes.Get(attributeName)
}

// Create creates an organization
func (os *OrganizationService) Create(newOrganization types.Organization,
	who Requester) (types.Organization, types.Error) {

	existingOrganization, err := os.db.Organization.Get(newOrganization.Name)
	if err == nil {
		return types.NullOrganization, types.NewBadRequestError(
			fmt.Errorf("Organization '%s' already exists", existingOrganization.Name))
	}
	// Set default fields
	newOrganization.CreatedAt = shared.GetCurrentTimeMilliseconds()
	newOrganization.CreatedBy = who.Username

	err = os.updateOrganization(&newOrganization, who)
	os.changelog.Create(newOrganization, who)
	return newOrganization, err
}

// Update updates an existing organization
func (os *OrganizationService) Update(updatedOrganization types.Organization,
	who Requester) (types.Organization, types.Error) {

	currentOrganization, err := os.db.Organization.Get(updatedOrganization.Name)
	if err != nil {
		return types.NullOrganization, types.NewItemNotFoundError(err)
	}
	// Copy over fields we do not allow to be updated
	updatedOrganization.Name = currentOrganization.Name
	updatedOrganization.CreatedAt = currentOrganization.CreatedAt
	updatedOrganization.CreatedBy = currentOrganization.CreatedBy

	err = os.updateOrganization(&updatedOrganization, who)
	os.changelog.Update(currentOrganization, updatedOrganization, who)
	return updatedOrganization, err
}

// UpdateAttributes updates attributes of an organization
func (os *OrganizationService) UpdateAttributes(org string,
	receivedAttributes types.Attributes, who Requester) types.Error {

	currentOrganization, err := os.db.Organization.Get(org)
	if err != nil {
		return err
	}
	updatedOrganization := currentOrganization
	updatedOrganization.Attributes.SetMultiple(receivedAttributes)

	err = os.updateOrganization(updatedOrganization, who)
	os.changelog.Update(currentOrganization, updatedOrganization, who)
	return err
}

// UpdateAttribute update an attribute of organization
func (os *OrganizationService) UpdateAttribute(org string,
	attributeValue types.Attribute, who Requester) types.Error {

	currentOrganization, err := os.db.Organization.Get(org)
	if err != nil {
		return err
	}
	updatedOrganization := currentOrganization
	updatedOrganization.Attributes.Set(attributeValue)

	err = os.updateOrganization(updatedOrganization, who)
	os.changelog.Update(currentOrganization, updatedOrganization, who)
	return err
}

// DeleteAttribute removes an attribute of an organization
func (os *OrganizationService) DeleteAttribute(organizationName,
	attributeToDelete string, who Requester) (string, types.Error) {

	currentOrganization, err := os.db.Organization.Get(organizationName)
	if err != nil {
		return "", err
	}
	updatedOrganization := currentOrganization
	oldValue, err := updatedOrganization.Attributes.Delete(attributeToDelete)
	if err != nil {
		return "", err
	}

	err = os.updateOrganization(updatedOrganization, who)
	os.changelog.Update(currentOrganization, updatedOrganization, who)
	return oldValue, err
}

// updateOrganization updates last-modified field(s) and updates organization in database
func (os *OrganizationService) updateOrganization(updatedOrganization *types.Organization, who Requester) types.Error {

	updatedOrganization.Attributes.Tidy()
	updatedOrganization.LastmodifiedAt = shared.GetCurrentTimeMilliseconds()
	updatedOrganization.LastmodifiedBy = who.Username
	return os.db.Organization.Update(updatedOrganization)
}

// Delete deletes an organization
func (os *OrganizationService) Delete(organizationName string, who Requester) (deletedOrganization types.Organization, e types.Error) {

	organization, err := os.db.Organization.Get(organizationName)
	if err != nil {
		return types.NullOrganization, err
	}
	developersInOrganizationCount, err := os.db.Developer.GetCountByOrganization(organization.Name)
	if err != nil {
		return types.NullOrganization, err
	}
	if developersInOrganizationCount > 0 {
		return types.NullOrganization, types.NewUnauthorizedError(
			fmt.Errorf("Cannot delete organization '%s' with %d active developers",
				organization.Name, developersInOrganizationCount))
	}
	if err := os.db.Organization.Delete(organization.Name); err != nil {
		return types.NullOrganization, err
	}
	os.changelog.Delete(organization, who)
	return *organization, nil
}