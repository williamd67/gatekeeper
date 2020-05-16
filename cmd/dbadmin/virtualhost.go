package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/erikbos/apiauth/pkg/shared"
)

// registerVirtualHostRoutes registers all virtualhosts we handle
func (s *server) registerVirtualHostRoutes(r *gin.Engine) {
	r.GET("/v1/virtualhosts", s.GetVirtualHosts)
	r.POST("/v1/virtualhosts", shared.AbortIfContentTypeNotJSON, s.PostCreateVirtualHost)

	r.GET("/v1/virtualhosts/:virtualhost", s.GetVirtualHostByName)
	r.POST("/v1/virtualhosts/:virtualhost", shared.AbortIfContentTypeNotJSON, s.PostVirtualHost)
	r.DELETE("/v1/virtualhosts/:virtualhost", s.DeleteVirtualHostByName)

	r.GET("/v1/virtualhosts/:virtualhost/attributes", s.GetVirtualHostAttributes)
	r.POST("/v1/virtualhosts/:virtualhost/attributes", shared.AbortIfContentTypeNotJSON, s.PostVirtualHostAttributes)

	r.GET("/v1/virtualhosts/:virtualhost/attributes/:attribute", s.GetVirtualHostAttributeByName)
	r.POST("/v1/virtualhosts/:virtualhost/attributes/:attribute", shared.AbortIfContentTypeNotJSON, s.PostVirtualHostAttributeByName)
	r.DELETE("/v1/virtualhosts/:virtualhost/attributes/:attribute", s.DeleteVirtualHostAttributeByName)
}

// GetVirtualHosts returns all virtualhosts
func (s *server) GetVirtualHosts(c *gin.Context) {

	virtualhosts, err := s.db.GetVirtualHosts()
	if err != nil {
		returnJSONMessage(c, http.StatusNotFound, err)
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"virtualhosts": virtualhosts})
}

// GetVirtualHostByName returns details of an route
func (s *server) GetVirtualHostByName(c *gin.Context) {

	route, err := s.db.GetVirtualHostByName(c.Param("virtualhost"))
	if err != nil {
		returnJSONMessage(c, http.StatusNotFound, err)
		return
	}

	setLastModifiedHeader(c, route.LastmodifiedAt)
	c.IndentedJSON(http.StatusOK, route)
}

// GetVirtualHostAttributes returns attributes of a virtual host
func (s *server) GetVirtualHostAttributes(c *gin.Context) {

	route, err := s.db.GetVirtualHostByName(c.Param("virtualhost"))
	if err != nil {
		returnJSONMessage(c, http.StatusNotFound, err)
		return
	}

	setLastModifiedHeader(c, route.LastmodifiedAt)
	c.IndentedJSON(http.StatusOK, gin.H{"attribute": route.Attributes})
}

// GetVirtualHostAttributeByName returns one particular attribute of a virtual host
func (s *server) GetVirtualHostAttributeByName(c *gin.Context) {

	virtualhost, err := s.db.GetVirtualHostByName(c.Param("virtualhost"))
	if err != nil {
		returnJSONMessage(c, http.StatusNotFound, err)
		return
	}

	value, err := shared.GetAttribute(virtualhost.Attributes, c.Param("attribute"))
	if err != nil {
		returnCanNotFindAttribute(c, c.Param("attribute"))
		return
	}

	setLastModifiedHeader(c, virtualhost.LastmodifiedAt)
	c.IndentedJSON(http.StatusOK, value)
}

// PostCreateVirtualHost creates a virtual host
func (s *server) PostCreateVirtualHost(c *gin.Context) {

	var newVirtualHost shared.VirtualHost
	if err := c.ShouldBindJSON(&newVirtualHost); err != nil {
		returnJSONMessage(c, http.StatusBadRequest, err)
		return
	}

	existingVirtualHost, err := s.db.GetVirtualHostByName(newVirtualHost.Name)
	if err == nil {
		returnJSONMessage(c, http.StatusBadRequest,
			fmt.Errorf("VirtualHost '%s' already exists", existingVirtualHost.Name))
		return
	}

	// Automatically set default fields
	newVirtualHost.CreatedAt = shared.GetCurrentTimeMilliseconds()
	newVirtualHost.CreatedBy = s.whoAmI()
	newVirtualHost.LastmodifiedBy = s.whoAmI()

	if err := s.db.UpdateVirtualHostByName(&newVirtualHost); err != nil {
		returnJSONMessage(c, http.StatusBadRequest, err)
		return
	}

	c.IndentedJSON(http.StatusCreated, newVirtualHost)
}

// PostVirtualHost updates an existing virtual host
func (s *server) PostVirtualHost(c *gin.Context) {

	virtualHostToUpdate, err := s.db.GetVirtualHostByName(c.Param("virtualhost"))
	if err != nil {
		returnJSONMessage(c, http.StatusBadRequest, err)
		return
	}

	var updateRequest shared.VirtualHost
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		returnJSONMessage(c, http.StatusBadRequest, err)
		return
	}

	// Copy over the fields we allow to be updated
	virtualHostToUpdate.VirtualHosts = updateRequest.VirtualHosts
	virtualHostToUpdate.Port = updateRequest.Port
	virtualHostToUpdate.DisplayName = updateRequest.DisplayName
	virtualHostToUpdate.Attributes = updateRequest.Attributes
	virtualHostToUpdate.RouteSet = updateRequest.RouteSet

	if err := s.db.UpdateVirtualHostByName(&virtualHostToUpdate); err != nil {
		returnJSONMessage(c, http.StatusBadRequest, err)
		return
	}

	c.IndentedJSON(http.StatusOK, virtualHostToUpdate)
}

// PostVirtualHostAttributes updates attributes of a virtual host
func (s *server) PostVirtualHostAttributes(c *gin.Context) {

	virtualHostToUpdate, err := s.db.GetVirtualHostByName(c.Param("virtualhost"))
	if err != nil {
		returnJSONMessage(c, http.StatusNotFound, err)
		return
	}

	var body struct {
		Attributes []shared.AttributeKeyValues `json:"attribute"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		returnJSONMessage(c, http.StatusBadRequest, err)
		return
	}
	if len(body.Attributes) == 0 {
		returnJSONMessage(c, http.StatusBadRequest, errors.New("No attributes posted"))
		return
	}

	virtualHostToUpdate.Attributes = body.Attributes

	virtualHostToUpdate.LastmodifiedBy = s.whoAmI()

	if err := s.db.UpdateVirtualHostByName(&virtualHostToUpdate); err != nil {
		returnJSONMessage(c, http.StatusBadRequest, err)
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"attribute": virtualHostToUpdate.Attributes})
}

// PostVirtualHostAttributeByName update an attribute of virtual host
func (s *server) PostVirtualHostAttributeByName(c *gin.Context) {

	virtualHostToUpdate, err := s.db.GetVirtualHostByName(c.Param("virtualhost"))
	if err != nil {
		returnJSONMessage(c, http.StatusNotFound, err)
		return
	}

	var body struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		returnJSONMessage(c, http.StatusBadRequest, err)
		return
	}

	attributeToUpdate := c.Param("attribute")
	virtualHostToUpdate.Attributes = shared.UpdateAttribute(virtualHostToUpdate.Attributes,
		attributeToUpdate, body.Value)

	virtualHostToUpdate.LastmodifiedBy = s.whoAmI()

	if err := s.db.UpdateVirtualHostByName(&virtualHostToUpdate); err != nil {
		returnJSONMessage(c, http.StatusBadRequest, err)
		return
	}

	c.IndentedJSON(http.StatusOK,
		gin.H{"name": attributeToUpdate, "value": body.Value})
}

// DeleteVirtualHostAttributeByName removes an attribute of virtual host
func (s *server) DeleteVirtualHostAttributeByName(c *gin.Context) {

	updatedVirtualHost, err := s.db.GetVirtualHostByName(c.Param("virtualhost"))
	if err != nil {
		returnJSONMessage(c, http.StatusNotFound, err)
		return
	}

	attributeToDelete := c.Param("attribute")
	updatedAttributes, index, oldValue :=
		shared.DeleteAttribute(updatedVirtualHost.Attributes, attributeToDelete)
	if index == -1 {
		returnJSONMessage(c, http.StatusNotFound,
			fmt.Errorf("Could not find attribute '%s'", attributeToDelete))
		return
	}
	updatedVirtualHost.Attributes = updatedAttributes

	updatedVirtualHost.LastmodifiedBy = s.whoAmI()

	if err := s.db.UpdateVirtualHostByName(&updatedVirtualHost); err != nil {
		returnJSONMessage(c, http.StatusBadRequest, err)
		return
	}

	c.IndentedJSON(http.StatusOK,
		gin.H{"name": attributeToDelete, "value": oldValue})
}

// DeleteVirtualHostByName deletes a virtual host
func (s *server) DeleteVirtualHostByName(c *gin.Context) {

	virtualhost, err := s.db.GetVirtualHostByName(c.Param("virtualhost"))
	if err != nil {
		returnJSONMessage(c, http.StatusNotFound, err)
		return
	}

	if err := s.db.DeleteVirtualHostByName(virtualhost.Name); err != nil {
		returnJSONMessage(c, http.StatusServiceUnavailable, err)
	}
	c.IndentedJSON(http.StatusOK, virtualhost)
}
