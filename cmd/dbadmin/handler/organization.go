package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/erikbos/gatekeeper/pkg/types"
)

// registerOrganizationRoutes registers all routes we handle
func (h *Handler) registerOrganizationRoutes(r *gin.RouterGroup) {
	r.GET("/organizations", h.handler(h.getAllOrganizations))
	r.POST("/organizations", h.handler(h.createOrganization))

	r.GET("/organizations/:organization", h.handler(h.getOrganization))
	r.POST("/organizations/:organization", h.handler(h.updateOrganization))
	r.DELETE("/organizations/:organization", h.handler(h.deleteOrganization))

	r.GET("/organizations/:organization/attributes", h.handler(h.getOrganizationAttributes))
	r.POST("/organizations/:organization/attributes", h.handler(h.updateOrganizationAttributes))

	r.GET("/organizations/:organization/attributes/:attribute", h.handler(h.getOrganizationAttribute))
	r.POST("/organizations/:organization/attributes/:attribute", h.handler(h.updateOrganizationAttribute))
	r.DELETE("/organizations/:organization/attributes/:attribute", h.handler(h.deleteOrganizationAttribute))
}

const (
	// Name of organization parameter in the route definition
	organizationParameter = "organization"

	// Name of organization parameter in the route definition
	attributeParameter = "attribute"
)

// getAllOrganizations returns all organizations
func (h *Handler) getAllOrganizations(c *gin.Context) handlerResponse {

	organizations, err := h.service.Organization.GetAll()
	if err != nil {
		return handleError(err)
	}
	return handleOK(StringMap{"organizations": organizations})
}

// getOrganization returns details of an organization
func (h *Handler) getOrganization(c *gin.Context) handlerResponse {

	organization, err := h.service.Organization.Get(c.Param(organizationParameter))
	if err != nil {
		return handleError(err)
	}
	return handleOK(organization)
}

// getOrganizationAttributes returns attributes of an organization
func (h *Handler) getOrganizationAttributes(c *gin.Context) handlerResponse {

	organization, err := h.service.Organization.Get(c.Param(organizationParameter))
	if err != nil {
		return handleError(err)
	}
	return handleOKAttributes(organization.Attributes)
}

// getOrganizationAttribute returns one particular attribute of an organization
func (h *Handler) getOrganizationAttribute(c *gin.Context) handlerResponse {

	organization, err := h.service.Organization.Get(c.Param(organizationParameter))
	if err != nil {
		return handleError(err)
	}
	value, err := organization.Attributes.Get(c.Param(attributeParameter))
	if err != nil {
		return handleError(err)
	}
	return handleOK(value)
}

// createOrganization creates an organization
func (h *Handler) createOrganization(c *gin.Context) handlerResponse {

	var newOrganization types.Organization
	if err := c.ShouldBindJSON(&newOrganization); err != nil {
		return handleBadRequest(err)
	}
	storedOrganization, err := h.service.Organization.Create(newOrganization, h.who(c))
	if err != nil {
		return handleError(err)
	}
	return handleCreated(storedOrganization)
}

// updateOrganization updates an existing organization
func (h *Handler) updateOrganization(c *gin.Context) handlerResponse {

	var updatedOrganization types.Organization
	if err := c.ShouldBindJSON(&updatedOrganization); err != nil {
		return handleBadRequest(err)
	}
	// organization name in path must match organization name in posted body
	if updatedOrganization.Name != c.Param(organizationParameter) {
		return handleNameMismatch()
	}
	storedOrganization, err := h.service.Organization.Update(updatedOrganization, h.who(c))
	if err != nil {
		return handleError(err)
	}
	return handleOK(storedOrganization)
}

// updateOrganizationAttributes updates attributes of an organization
func (h *Handler) updateOrganizationAttributes(c *gin.Context) handlerResponse {

	var receivedAttributes struct {
		Attributes types.Attributes `json:"attribute"`
	}
	if err := c.ShouldBindJSON(&receivedAttributes); err != nil {
		return handleBadRequest(err)
	}
	if err := h.service.Organization.UpdateAttributes(c.Param(organizationParameter),
		receivedAttributes.Attributes, h.who(c)); err != nil {
		return handleError(err)
	}
	return handleOKAttributes(receivedAttributes.Attributes)
}

// updateOrganizationAttribute update an attribute of developer
func (h *Handler) updateOrganizationAttribute(c *gin.Context) handlerResponse {

	var receivedValue types.AttributeValue
	if err := c.ShouldBindJSON(&receivedValue); err != nil {
		return handleBadRequest(err)
	}
	newAttribute := types.Attribute{
		Name:  c.Param(attributeParameter),
		Value: receivedValue.Value,
	}
	if err := h.service.Organization.UpdateAttribute(c.Param(organizationParameter),
		newAttribute, h.who(c)); err != nil {
		return handleError(err)
	}
	return handleOKAttribute(newAttribute)
}

// deleteOrganizationAttribute removes an attribute of an organization
func (h *Handler) deleteOrganizationAttribute(c *gin.Context) handlerResponse {

	attributeToDelete := c.Param(attributeParameter)
	oldValue, err := h.service.Organization.DeleteAttribute(c.Param(organizationParameter),
		attributeToDelete, h.who(c))
	if err != nil {
		return handleBadRequest(err)
	}
	return handleOKAttribute(types.Attribute{
		Name:  attributeToDelete,
		Value: oldValue,
	})
}

// deleteOrganization deletes an organization
func (h *Handler) deleteOrganization(c *gin.Context) handlerResponse {

	deletedOrganization, err := h.service.Organization.Delete(c.Param(organizationParameter), h.who(c))
	if err != nil {
		return handleError(err)
	}
	return handleOK(deletedOrganization)
}