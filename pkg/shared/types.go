package shared

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// Organization contains everything about a Organization
type Organization struct {
	Attributes     []AttributeKeyValues `json:"attributes"`
	CreatedAt      int64                `json:"createdAt"`
	CreatedBy      string               `json:"createdBy"`
	DisplayName    string               `json:"displayName"`
	Key            string               `json:"key"`
	LastmodifiedAt int64                `json:"lastmodifiedAt"`
	LastmodifiedBy string               `json:"lastmodifiedBy"`
	Name           string               `json:"name" binding:"required"`
}

// Developer contains everything about a Developer
type Developer struct {
	DeveloperID      string               `json:"developerId"`
	Apps             []string             `json:"apps"`
	Attributes       []AttributeKeyValues `json:"attributes"`
	CreatedAt        int64                `json:"createdAt"`
	CreatedBy        string               `json:"createdBy"`
	Email            string               `json:"email" binding:"required"`
	FirstName        string               `json:"firstName" binding:"required"`
	LastName         string               `json:"lastName" binding:"required"`
	LastmodifiedAt   int64                `json:"lastmodifiedAt"`
	LastmodifiedBy   string               `json:"lastmodifiedBy"`
	OrganizationName string               `json:"organizationName"`
	Salt             string               `json:"salt"`
	Status           string               `json:"status"`
	UserName         string               `json:"userName" binding:"required"`
}

// DeveloperApp contains everything about a Developer Application
type DeveloperApp struct {
	DeveloperAppID   string               `json:"key"`
	AccessType       string               `json:"accessType"`
	AppFamily        string               `json:"appFamily"`
	AppID            string               `json:"appId"`
	AppType          string               `json:"appType"`
	Attributes       []AttributeKeyValues `json:"attributes"`
	CallbackURL      string               `json:"callbackUrl"`
	CreatedAt        int64                `json:"createdAt"`
	CreatedBy        string               `json:"createdBy"`
	Credentials      []AppCredential      `json:"credentials"`
	DisplayName      string               `json:"displayName"`
	LastmodifiedAt   int64                `json:"lastmodifiedAt"`
	LastmodifiedBy   string               `json:"lastmodifiedBy"`
	Name             string               `json:"name" binding:"required"`
	OrganizationName string               `json:"organizationName"`
	ParentID         string               `json:"parentId"`
	ParentStatus     string               `json:"parentStatus"`
	Status           string               `json:"status"`
	// Key              string               `json:"DeveloperAppID"`
}

// AppCredential contains an apikey entitlement
type AppCredential struct {
	ConsumerKey       string               `json:"key"`
	APIProducts       []APIProductStatus   `json:"apiProducts"`
	AppStatus         string               `json:"appStatus"`
	Attributes        []AttributeKeyValues `json:"attributes"`
	CompanyStatus     string               `json:"companyStatus"`
	ConsumerSecret    string               `json:"consumerSecret"`
	CredentialMethod  string               `json:"credentialMethod"`
	DeveloperStatus   string               `json:"developerStatus"`
	ExpiresAt         int64                `json:"expiresAt"`
	IssuedAt          int64                `json:"issuesAt"`
	OrganizationAppID string               `json:"organizationAppId"`
	OrganizationName  string               `json:"organizationName"`
	Scopes            string               `json:"scopes"`
	Status            string               `json:"status"`
}

// APIProductStatus contains whether an apikey's assigned apiproduct has been approved
type APIProductStatus struct {
	Status     string `json:"status"`
	Apiproduct string `json:"apiProduct"`
}

// APIProduct type contains everything about an API product
type APIProduct struct {
	Key              string               `json:"key"`
	Name             string               `json:"name"`
	DisplayName      string               `json:"displayName"`
	Description      string               `json:"description"`
	RouteSet         string               `json:"routeSet"`
	APIResources     []string             `json:"apiResources"`
	Attributes       []AttributeKeyValues `json:"attributes"`
	OrganizationName string               `json:"organizationName"`
	Scopes           string               `json:"scopes"`
	CreatedAt        int64                `json:"createdAt"`
	CreatedBy        string               `json:"createdBy"`
	LastmodifiedAt   int64                `json:"lastmodifiedAt"`
	LastmodifiedBy   string               `json:"lastmodifiedBy"`
}

// AttributeKeyValues is an array with attributes of developer or developer app
type AttributeKeyValues struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// VirtualHost contains everything about downstream configuration of virtual hosts
type VirtualHost struct {
	Name           string               `json:"name"`
	DisplayName    string               `json:"displayName"`
	VirtualHosts   []string             `json:"virtualHosts"`
	Port           int                  `json:"port"`
	RouteSet       string               `json:"routeSet"`
	Attributes     []AttributeKeyValues `json:"attributes"`
	CreatedAt      int64                `json:"createdAt"`
	CreatedBy      string               `json:"createdBy"`
	LastmodifiedAt int64                `json:"lastmodifiedAt"`
	LastmodifiedBy string               `json:"lastmodifiedBy"`
}

// Route holds configuration of one or more routes
type Route struct {
	Name           string               `json:"name"`
	DisplayName    string               `json:"displayName"`
	RouteSet       string               `json:"routeSet"`
	Path           string               `json:"path"`
	PathType       string               `json:"pathType"`
	Cluster        string               `json:"cluster"`
	Attributes     []AttributeKeyValues `json:"attributes"`
	CreatedAt      int64                `json:"createdAt"`
	CreatedBy      string               `json:"createdBy"`
	LastmodifiedAt int64                `json:"lastmodifiedAt"`
	LastmodifiedBy string               `json:"lastmodifiedBy"`
}

// Cluster holds configuration of an upstream cluster
type Cluster struct {
	Name           string               `json:"name"`
	DisplayName    string               `json:"displayName"`
	HostName       string               `json:"hostName"`
	Port           int                  `json:"port"`
	Attributes     []AttributeKeyValues `json:"attributes"`
	CreatedAt      int64                `json:"createdAt"`
	CreatedBy      string               `json:"createdBy"`
	LastmodifiedAt int64                `json:"lastmodifiedAt"`
	LastmodifiedBy string               `json:"lastmodifiedBy"`
}

// GetAttribute find one named attribute in array of attributes (developer or developerapp)
func GetAttribute(attributes []AttributeKeyValues, name string) (string, error) {
	index := FindIndexOfAttribute(attributes, name)
	if index == -1 {
		return "", errors.New("Attribute not found")
	}
	return attributes[index].Value, nil
}

// GetAttributeAsString returns attribute value (or provided default) as string
func GetAttributeAsString(attributes []AttributeKeyValues, name, defaultValue string) string {
	value, err := GetAttribute(attributes, name)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetAttributeAsInt returns attribute value (or provided default) as integer
func GetAttributeAsInt(attributes []AttributeKeyValues,
	attributeName, defaultValue string) int {

	value, err := GetAttribute(attributes, attributeName)
	if err != nil {
		if defaultValue == "" {
			return 0
		}
		value = defaultValue
	}
	integer, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return integer
}

// GetAttributeAsDuration returns attribute value (or provided default) as type time.Duration
func GetAttributeAsDuration(attributes []AttributeKeyValues,
	attributeName, defaultValue string) time.Duration {

	value, err := GetAttribute(attributes, attributeName)
	if err != nil {
		if defaultValue == "" {
			return 0
		}
		value = defaultValue
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0
	}
	return duration
}

// FindIndexOfAttribute find index of named attribute in slice
func FindIndexOfAttribute(attributes []AttributeKeyValues, name string) int {
	for index, element := range attributes {
		if element.Name == name {
			return index
		}
	}
	return -1
}

// TidyAttributes removes duplicate attributes and trims all names & values
func TidyAttributes(attributes []AttributeKeyValues) []AttributeKeyValues {
	// Use map to record duplicates as we find them.
	encountered := map[string]bool{}
	result := []AttributeKeyValues{}

	for v := range attributes {
		if encountered[strings.TrimSpace(attributes[v].Name)] {
			// Do not add duplicate.
		} else {
			// Trim whitespace we like tidy
			attributes[v].Name = strings.TrimSpace(attributes[v].Name)
			attributes[v].Value = strings.TrimSpace(attributes[v].Value)
			// Record this element as an encountered element.
			encountered[attributes[v].Name] = true
			// Append to result slice.
			result = append(result, attributes[v])
		}
	}
	return result
}

// DeleteAttribute removes attribute from slice. returns slice, index of deleted value, deleted value
func DeleteAttribute(attributes []AttributeKeyValues, attributeName string) ([]AttributeKeyValues, int, string) {
	// Find attribute in array
	index := FindIndexOfAttribute(attributes, attributeName)
	if index == -1 {
		return attributes, -1, ""
	}
	valueOfDeletedAttribute := attributes[index].Value
	attributes = append(attributes[:index], attributes[index+1:]...)
	return attributes, 0, valueOfDeletedAttribute
}
