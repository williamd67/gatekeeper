package db

import (
	"fmt"

	"github.com/gocql/gocql"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/erikbos/gatekeeper/pkg/shared"
)

// Prometheus label for metrics of db interactions
const organizationMetricLabel = "organizations"

// GetOrganizations retrieves all organizations
func (d *Database) GetOrganizations() ([]shared.Organization, error) {
	// FIXME this ugly workaround to have to pass an argument
	query := "SELECT * FROM organizations ALLOW FILTERING"
	organizations, err := d.runGetOrganizationQuery(query, "")
	if err != nil {
		return []shared.Organization{}, fmt.Errorf("Cannot retrieve list of organizations (%s)", err)
	}
	if len(organizations) == 0 {
		d.metricsQueryMiss(organizationMetricLabel)
	} else {
		d.metricsQueryHit(organizationMetricLabel)
	}
	return organizations, nil
}

// GetOrganizationByName retrieves an organization from database
func (d *Database) GetOrganizationByName(organizationName string) (shared.Organization, error) {
	query := "SELECT * FROM organizations WHERE name = ? LIMIT 1"
	organizations, err := d.runGetOrganizationQuery(query, organizationName)
	if err != nil {
		return shared.Organization{}, err
	}
	if len(organizations) == 0 {
		d.metricsQueryMiss(organizationMetricLabel)
		return shared.Organization{},
			fmt.Errorf("Can not find organization (%s)", organizationName)
	}
	d.metricsQueryHit(organizationMetricLabel)
	return organizations[0], nil
}

// runGetOrganizationQuery executes CQL query and returns resultset
func (d *Database) runGetOrganizationQuery(query, queryParameter string) ([]shared.Organization, error) {
	var organizations []shared.Organization

	timer := prometheus.NewTimer(d.dbLookupHistogram)
	defer timer.ObserveDuration()

	var iterable *gocql.Iter
	if queryParameter == "" {
		iterable = d.cassandraSession.Query(query).Iter()
	} else {
		iterable = d.cassandraSession.Query(query, queryParameter).Iter()
	}
	m := make(map[string]interface{})
	for iterable.MapScan(m) {
		organizations = append(organizations, shared.Organization{
			Attributes:     d.unmarshallJSONArrayOfAttributes(m["attributes"].(string)),
			CreatedAt:      m["created_at"].(int64),
			CreatedBy:      m["created_by"].(string),
			DisplayName:    m["display_name"].(string),
			LastmodifiedAt: m["lastmodified_at"].(int64),
			LastmodifiedBy: m["lastmodified_by"].(string),
			Name:           m["name"].(string),
		})
		m = map[string]interface{}{}
	}
	if err := iterable.Close(); err != nil {
		log.Error(err)
		return []shared.Organization{}, err
	}
	return organizations, nil
}

// UpdateOrganizationByName UPSERTs an organization in database
func (d *Database) UpdateOrganizationByName(updatedOrganization *shared.Organization) error {

	updatedOrganization.Attributes = shared.TidyAttributes(updatedOrganization.Attributes)
	Attributes := d.marshallArrayOfAttributesToJSON(updatedOrganization.Attributes)

	updatedOrganization.LastmodifiedAt = shared.GetCurrentTimeMilliseconds()

	if err := d.cassandraSession.Query(
		"INSERT INTO organizations (name, display_name, attributes, "+
			"created_at, created_by, lastmodified_at, lastmodified_by) "+
			"VALUES(?,?,?,?,?,?,?)",
		updatedOrganization.Name,
		updatedOrganization.DisplayName, Attributes, updatedOrganization.CreatedAt,
		updatedOrganization.CreatedBy, updatedOrganization.LastmodifiedAt,
		updatedOrganization.LastmodifiedBy).Exec(); err != nil {
		return fmt.Errorf("Can not update organization (%v)", err)
	}
	return nil
}

// DeleteOrganizationByName deletes an organization from database
func (d *Database) DeleteOrganizationByName(organizationToDelete string) error {
	_, err := d.GetOrganizationByName(organizationToDelete)
	if err != nil {
		return err
	}
	query := "DELETE FROM organizations WHERE name = ?"
	return d.cassandraSession.Query(query, organizationToDelete).Exec()
}
