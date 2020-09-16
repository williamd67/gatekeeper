package types

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Attribute is an array with attributes of developer or developer app
type Attribute struct {
	Name  string `json:"name"`  // Attribute name
	Value string `json:"value"` // Attribute value
}

// Attributes holds one or more attributes
type Attributes []Attribute

// Attributes which are shared amongst listener, route and cluster
const (

	// AttributeTLSCertificate holds pem encoded certicate
	AttributeTLSCertificate = "TLSCertificate"

	// AttributeTLSCertificateKey holds certicate key
	AttributeTLSCertificateKey = "TLSCertificateKey"

	// AttributeTLSMinimumVersion determines minimum TLS version accepted
	AttributeTLSMinimumVersion = "TLSMinimumVersion"

	// AttributeTLSMaximumVersion determines maximum TLS version accepted
	AttributeTLSMaximumVersion = "TLSMaximumVersion"

	// AttributeTLSCipherSuites determines set of allowed TLS ciphers
	AttributeTLSCipherSuites = "TLSCipherSuites"

	// AttributeTLSCipherSuites sets HTTP protocol to accept
	AttributeHTTPProtocol = "HTTPProtocol"

	AttributeValueTrue                    = "true"
	AttributeValueTLSVersion10            = "TLSv10"
	AttributeValueTLSVersion11            = "TLSv11"
	AttributeValueTLSVersion12            = "TLSv12"
	AttributeValueTLSVersion13            = "TLSv13"
	AttributeValueHTTPProtocol11          = "HTTP/1.1"
	AttributeValueHTTPProtocol2           = "HTTP/2"
	AttributeValueHTTPProtocol3           = "HTTP/3"
	AttributeValueHealthCheckProtocolHTTP = "HTTP"
)

// Get return one named attribute from attributes
func (attributes *Attributes) Get(name string) (string, error) {

	for _, element := range *attributes {
		if element.Name == name {
			return element.Value, nil
		}
	}
	return "", errors.New("Attribute not found")
}

// GetAsString returns attribute value (or provided default if not found) as type string
func (attributes *Attributes) GetAsString(name, defaultValue string) string {

	value, err := attributes.Get(name)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetAsUInt32 returns attribute value (or provided default) as type integer
func (attributes *Attributes) GetAsUInt32(name string, defaultValue uint32) uint32 {

	value, err := attributes.Get(name)
	if err == nil {
		integer, err := strconv.ParseUint(value, 10, 32)
		if err == nil {
			return uint32(integer)
		}
	}
	return defaultValue
}

// GetAsDuration returns attribute value (or provided default) as type time.Duration
func (attributes *Attributes) GetAsDuration(name string, defaultDuration time.Duration) time.Duration {

	value, err := attributes.Get(name)
	if err == nil {
		duration, err := time.ParseDuration(value)
		if err == nil {
			return duration
		}
	}
	return defaultDuration
}

// Set updates or adds attribute in slice.
func (attributes *Attributes) Set(name, value string) {

	updatedAttributes := Attributes{}

	var existingAttribute bool
	for _, oldAttribute := range *attributes {
		// In case attribute exists overwrite it
		if oldAttribute.Name == name {
			existingAttribute = true
			updatedAttributes = append(updatedAttributes, Attribute{
				Name:  name,
				Value: value,
			})
		} else {
			updatedAttributes = append(updatedAttributes, oldAttribute)
		}
	}
	// In case it is an new attribute append it
	if !existingAttribute {
		updatedAttributes = append(updatedAttributes, Attribute{
			Name:  name,
			Value: value,
		})
	}

	// Overwrite existing slice with new slice
	*attributes = updatedAttributes
}

// Tidy removes duplicate, trims all names & values and sorts attribute by name
func (attributes *Attributes) Tidy() {
	// Use map to record duplicates as we find them.
	encountered := map[string]bool{}
	updatedAttributes := Attributes{}

	// Remove dups & trim
	for _, attribute := range *attributes {
		if encountered[strings.TrimSpace(attribute.Name)] {
			// Do not add duplicate.
		} else {
			// Trim whitespace we like tidy
			attribute.Name = strings.TrimSpace(attribute.Name)
			attribute.Value = strings.TrimSpace(attribute.Value)
			// Record this element as an encountered element.
			encountered[attribute.Name] = true
			// Append to result slice.
			updatedAttributes = append(updatedAttributes, attribute)
		}
	}

	// Sort slice by attribute name
	sort.SliceStable(updatedAttributes, func(i, j int) bool {
		return updatedAttributes[i].Name < updatedAttributes[j].Name
	})

	*attributes = updatedAttributes
}

// Delete removes attribute from slice. Returns delete status and deleted attribute's value
func (attributes *Attributes) Delete(name string) (deleted bool, oldValue string) {

	updatedAttributes := Attributes{}

	var attributeDeleted bool
	var valueOfDeletedAttribute string

	for _, attribute := range *attributes {
		if attribute.Name == name {
			valueOfDeletedAttribute = attribute.Value
			attributeDeleted = true
		} else {
			// No match, keep attribute
			updatedAttributes = append(updatedAttributes, attribute)
		}
	}
	*attributes = updatedAttributes

	if attributeDeleted {
		return true, valueOfDeletedAttribute
	}
	return false, ""
}

// Unmarshal unpacks JSON array of attributes
// Example input: [{"name":"Shoesize","value":"42"}, {"name":"Destination","value":"Mars"}]
//
func (attributes Attributes) Unmarshal(jsonArrayOfAttributes string) Attributes {

	if jsonArrayOfAttributes != "" {
		var attributes = make(Attributes, 0)
		if err := json.Unmarshal([]byte(jsonArrayOfAttributes), &attributes); err == nil {
			return attributes
		}
	}
	return nil
}

// Marshal packs slice of attributes into JSON
// Example output: [{"name":"Shoesize","value":"42"}, {"name":"Destination","value":"Mars"}]
//
func (attributes Attributes) Marshal() string {

	if len(attributes) > 0 {
		json, err := json.Marshal(attributes)
		if err == nil {
			return string(json)
		}
	}
	return "[]"
}