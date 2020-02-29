package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/erikbos/apiauth/pkg/types"
	"github.com/gocql/gocql"
	"github.com/prometheus/client_golang/prometheus"
)

//Database holds all our database connection information and performance counters
//
type Database struct {
	Hostname              string
	cassandraSession      *gocql.Session
	dbLookupHitsCounter   *prometheus.CounterVec
	dbLookupMissesCounter *prometheus.CounterVec
	dbLookupHistogram     prometheus.Summary
}

//Connect setups up connectivity to Cassandra
//
func Connect(hostname string, port int, username, password, keyspace string) (*Database, error) {
	var err error
	d := Database{}

	d.Hostname = hostname
	cluster := gocql.NewCluster(hostname)
	cluster.Port = port
	cluster.SslOpts = &gocql.SslOptions{
		CertPath:               "selfsigned.crt",
		KeyPath:                "selfsigned.key",
		EnableHostVerification: false,
	}
	cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: username,
		Password: password,
	}
	cluster.Keyspace = keyspace

	d.cassandraSession, err = cluster.CreateSession()
	if err != nil {
		return nil, errors.New("Could not connect to database")
	}

	d.dbLookupHitsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "apiauth_database_lookup_hits_total",
			Help: "Number of successful database lookups.",
		}, []string{"hostname", "table"})
	prometheus.MustRegister(d.dbLookupHitsCounter)

	d.dbLookupMissesCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "apiauth_database_lookup_misses_total",
			Help: "Number of unsuccesful database lookups.",
		}, []string{"hostname", "table"})
	prometheus.MustRegister(d.dbLookupMissesCounter)

	d.dbLookupHistogram = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Name:       "apiauth_database_lookup_latency",
			Help:       "Database lookup latency in seconds.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		})
	prometheus.MustRegister(d.dbLookupHistogram)

	return &d, nil
}

// unmarshallJSONArrayOfStrings unpacks JSON array of strings
// e.g. [\"PetStore5\",\"PizzaShop1\"] to []string
//
func (d *Database) unmarshallJSONArrayOfStrings(jsonArrayOfStrings string) []string {
	if jsonArrayOfStrings != "" {
		var StringValues []string
		err := json.Unmarshal([]byte(jsonArrayOfStrings), &StringValues)
		if err == nil {
			return StringValues
		}
	}
	return nil
}

// MarshallArrayOfStringsToJSON packs array of string into JSON
// e.g. []string to [\"PetStore5\",\"PizzaShop1\"]
//
func (d *Database) MarshallArrayOfStringsToJSON(ArrayOfStrings []string) string {
	if len(ArrayOfStrings) > 0 {
		ArrayOfStringsInJSON, err := json.Marshal(ArrayOfStrings)
		if err == nil {
			return string(ArrayOfStringsInJSON)
		}
	}
	return "[]"
}

// unmarshallJSONArrayOfAttributes unpacks JSON array of attribute bags
// Example input: [{"name":"DisplayName","value":"erikbos teleporter"},{"name":"ErikbosTeleporterExtraAttribute","value":"42"}]
//
func (d *Database) unmarshallJSONArrayOfAttributes(jsonArrayOfAttributes string, ignoreMintAttributes bool) []types.AttributeKeyValues {
	if jsonArrayOfAttributes != "" {
		var ResponseAttributes = make([]types.AttributeKeyValues, 0)
		if err := json.Unmarshal([]byte(jsonArrayOfAttributes), &ResponseAttributes); err == nil {
			return ResponseAttributes
		}
	}
	return nil
}

// marshallArrayOfAttributesToJSON packs array of attributes into JSON
// Example input: [{"name":"DisplayName","value":"erikbos teleporter"},{"name":"ErikbosTeleporterExtraAttribute","value":"42"}]
//
func (d *Database) marshallArrayOfAttributesToJSON(ArrayOfAttributes []types.AttributeKeyValues, ignoreMintAttributes bool) string {

	if len(ArrayOfAttributes) > 0 {
		ArrayOfAttributesInJSON, err := json.Marshal(ArrayOfAttributes)
		if err == nil {
			return string(ArrayOfAttributesInJSON)
		}
	}
	return "[]"
}

///////////////////////////////////////////////////////////////////////////////////////////////////////

//GetOrganizationByName retrieves an organization from database
//
func (d *Database) GetOrganizationByName(organization string) (types.Organization, error) {
	query := "SELECT * FROM organization WHERE name = ? LIMIT 1"
	developers := d.runGetDeveloperQuery(query, organization)
	if len(developers) > 0 {
		d.dbLookupHitsCounter.WithLabelValues(d.Hostname, "developers").Inc()
		return developers[0], nil
	}
	d.dbLookupMissesCounter.WithLabelValues(d.Hostname, "developers").Inc()
	return types.Developer{}, fmt.Errorf("Could not find organization (%s)", organization)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////

//GetDeveloperByEmail retrieves a developer from database
//
func (d *Database) GetDeveloperByEmail(developerEmail string) (types.Developer, error) {
	query := "SELECT * FROM developers WHERE email = ? LIMIT 1"
	developers := d.runGetDeveloperQuery(query, developerEmail)
	if len(developers) > 0 {
		d.dbLookupHitsCounter.WithLabelValues(d.Hostname, "developers").Inc()
		return developers[0], nil
	}
	d.dbLookupMissesCounter.WithLabelValues(d.Hostname, "developers").Inc()
	return types.Developer{}, fmt.Errorf("Could not find developer (%s)", developerEmail)
}

//GetDeveloperByID retrieves a developer from database
//
func (d *Database) GetDeveloperByID(developerID string) (types.Developer, error) {
	query := "SELECT * FROM developers WHERE key = ? LIMIT 1"
	developers := d.runGetDeveloperQuery(query, developerID)
	if len(developers) > 0 {
		d.dbLookupHitsCounter.WithLabelValues(d.Hostname, "developers").Inc()
		return developers[0], nil
	}
	d.dbLookupMissesCounter.WithLabelValues(d.Hostname, "developers").Inc()
	return types.Developer{}, fmt.Errorf("Could not find developerId (%s)", developerID)
}

//GetDevelopersByOrganization retrieves all developer belonging to an organization
//
func (d *Database) GetDevelopersByOrganization(organizationName string) ([]types.Developer, error) {
	query := "SELECT * FROM developers WHERE organization_name = ? LIMIT 10 ALLOW FILTERING"
	developers := d.runGetDeveloperQuery(query, organizationName)
	if len(developers) > 0 {
		d.dbLookupHitsCounter.WithLabelValues(d.Hostname, "developers").Inc()
		return developers, nil
	}
	d.dbLookupMissesCounter.WithLabelValues(d.Hostname, "developers").Inc()
	return developers, errors.New("Could not retrieve list of developers")
}

// runDeveloperQuery executes CQL query and returns resultset
//
func (d *Database) runGetDeveloperQuery(query, queryParameter string) []types.Developer {
	var developers []types.Developer

	//Set timer to record how long this function run
	timer := prometheus.NewTimer(d.dbLookupHistogram)
	defer timer.ObserveDuration()

	iterable := d.cassandraSession.Query(query, queryParameter).Iter()
	m := make(map[string]interface{})

	// from https://github.com/uber/cherami-server/blob/1de31a4ed1d0a9cd33ff64199f7e91f23e99c11e/cmd/tools/cmq/fix.go
	//
	// for iter.Scan(&uuid, &destinationUUID, &name, &status, &lockTimeoutSeconds, &maxDeliveryCount, &skipOlderMessagesSeconds,
	// 	&deadLetterQueueDestinationUUID, &ownerEmail, &startFrom, &isMultiZone, &activeZone, &zoneConfigs, &delaySeconds, &options) {

	for iterable.MapScan(m) {
		developers = append(developers, types.Developer{
			Apps:             d.unmarshallJSONArrayOfStrings(m["apps"].(string)),
			Attributes:       d.unmarshallJSONArrayOfAttributes(m["attributes"].(string), false),
			DeveloperID:      m["key"].(string),
			CreatedAt:        m["created_at"].(int64),
			CreatedBy:        m["created_by"].(string),
			Email:            m["email"].(string),
			FirstName:        m["first_name"].(string),
			LastName:         m["last_name"].(string),
			LastmodifiedAt:   m["lastmodified_at"].(int64),
			LastmodifiedBy:   m["lastmodified_by"].(string),
			OrganizationName: m["organization_name"].(string),
			// password:          m["password"].(string),
			Salt:     m["salt"].(string),
			Status:   m["status"].(string),
			UserName: m["user_name"].(string),
		})
		m = map[string]interface{}{}
	}
	return developers
}

//CreateDeveloper creates developer
//
func (d *Database) CreateDeveloper(updatedDeveloper types.Developer) error {
	// generate new id, to stay backwards compatible
	// keyformat = "test@@@OE6GkphWHYzkAIgd"
	// we derive is from email address
	updatedDeveloper.DeveloperID = "test@@@123"
	// FIXME
	return d.UpdateDeveloperByID(updatedDeveloper.DeveloperID, updatedDeveloper)
}

//UpdateDeveloperByEmail updates an existing developer
//
func (d *Database) UpdateDeveloperByEmail(developerEmail string, updatedDeveloper types.Developer) error {
	// We first lookup the primary based upon email address
	// updatedDeveloper.DeveloperID could be empty or wrong..
	currentDeveloper, err := d.GetDeveloperByEmail(developerEmail)
	if err != nil {
		return err
	}
	updatedDeveloper.DeveloperID = currentDeveloper.DeveloperID
	return d.UpdateDeveloperByID(updatedDeveloper.DeveloperID, updatedDeveloper)
}

// UpdateDeveloperByID UPSERTs a developer in database
// Upsert is: In case a developer does not exist (primary key not matching) it will create a new row
func (d *Database) UpdateDeveloperByID(developerID string, updatedDeveloper types.Developer) error {
	query := "INSERT INTO developers (key,apps,attributes, " +
		"created_at, created_by, email, " +
		"first_name, last_name, lastmodified_at, " +
		"lastmodified_by, organization_name, status, user_name)" +
		"VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)"

	Apps := d.MarshallArrayOfStringsToJSON(updatedDeveloper.Apps)
	Attributes := d.marshallArrayOfAttributesToJSON(updatedDeveloper.Attributes, false)
	// log.Printf("attributes: %s", updatedDeveloper.Attributes)

	err := d.cassandraSession.Query(query,
		updatedDeveloper.DeveloperID, Apps, Attributes,
		updatedDeveloper.CreatedAt, updatedDeveloper.CreatedBy, updatedDeveloper.Email,
		updatedDeveloper.FirstName, updatedDeveloper.LastName, updatedDeveloper.LastmodifiedAt,
		updatedDeveloper.LastmodifiedBy, updatedDeveloper.OrganizationName, updatedDeveloper.Status,
		updatedDeveloper.UserName).Exec()
	if err == nil {
		return nil
	}
	log.Printf("%+v", err)
	return fmt.Errorf("Could not update developer (%v)", err)
}

//DeleteDeveloperByEmail deletes a developer from database
//
func (d *Database) DeleteDeveloperByEmail(developerEmail string) error {
	developer, err := d.GetDeveloperByEmail(developerEmail)
	if err != nil {
		return err
	}
	query := "DELETE FROM developers WHERE key = ?"
	return d.cassandraSession.Query(query, developer.DeveloperID).Exec()
}

/////////////////////////////////////////////////////////////////////////////////////////

//GetDeveloperAppByName returns details of a DeveloperApplication looked up by Name
//
func (d *Database) GetDeveloperAppByName(organization, developerAppName string) (types.DeveloperApp, error) {
	query := "SELECT * FROM apps WHERE name = ? LIMIT 1"
	developerapps := d.runGetDeveloperAppQuery(query, developerAppName)
	if len(developerapps) > 0 {
		d.dbLookupHitsCounter.WithLabelValues(d.Hostname, "apps").Inc()
		return developerapps[0], nil
	}
	d.dbLookupMissesCounter.WithLabelValues(d.Hostname, "apps").Inc()
	return types.DeveloperApp{}, errors.New("Could not find developer by name")
}

//GetDeveloperAppByID returns details of a DeveloperApplication looked up by ID
//
func (d *Database) GetDeveloperAppByID(organization, developerAppID string) (types.DeveloperApp, error) {
	query := "SELECT * FROM apps WHERE key = ? LIMIT 1"
	developerapps := d.runGetDeveloperAppQuery(query, developerAppID)
	if len(developerapps) > 0 {
		d.dbLookupHitsCounter.WithLabelValues(d.Hostname, "apps").Inc()
		return developerapps[0], nil
	}
	d.dbLookupMissesCounter.WithLabelValues(d.Hostname, "apps").Inc()
	return types.DeveloperApp{}, errors.New("Could not find developer by app id")
}

func (d *Database) runGetDeveloperAppQuery(query, queryParameter string) []types.DeveloperApp {
	var developerapps []types.DeveloperApp

	//Set timer to record how long this function run
	timer := prometheus.NewTimer(d.dbLookupHistogram)
	defer timer.ObserveDuration()

	iterable := d.cassandraSession.Query(query, queryParameter).Iter()
	m := make(map[string]interface{})
	for iterable.MapScan(m) {
		developerapps = append(developerapps, types.DeveloperApp{
			AccessType:  m["access_type"].(string),
			AppFamily:   m["app_family"].(string),
			AppID:       m["app_id"].(string),
			AppType:     m["app_type"].(string),
			Attributes:  d.unmarshallJSONArrayOfAttributes(m["attributes"].(string), false),
			CallbackURL: m["callback_url"].(string),
			CreatedAt:   m["created_at"].(int64),
			CreatedBy:   m["created_by"].(string),
			// DeveloperAppID:   developerAppID,
			DisplayName:      m["display_name"].(string),
			Key:              m["key"].(string),
			LastmodifiedAt:   m["lastmodified_at"].(int64),
			LastmodifiedBy:   m["lastmodified_by"].(string),
			Name:             m["name"].(string),
			OrganizationName: m["organization_name"].(string),
			ParentID:         m["parent_id"].(string),
			ParentStatus:     m["parent_status"].(string),
			Status:           m["status"].(string),
		})
		m = map[string]interface{}{}
	}
	return developerapps
}

///////////////////////////////////////////////////////////////////////////////

//GetAppCredentialByKey returns details of a single apikey
//
func (d *Database) GetAppCredentialByKey(key string) (types.AppCredential, error) {
	var appcredentials []types.AppCredential

	query := "SELECT * FROM app_credentials WHERE key = ? LIMIT 1"
	appcredentials = d.runGetAppCredentialQuery(query, key)
	if len(appcredentials) > 0 {
		d.dbLookupHitsCounter.WithLabelValues(d.Hostname, "app_credentials").Inc()
		return appcredentials[0], nil
	}
	d.dbLookupMissesCounter.WithLabelValues(d.Hostname, "app_credentials").Inc()
	return types.AppCredential{}, errors.New("Could not find apikey")
}

//GetAppCredentialByDeveloperAppID returns an array with apikey details of a developer app
// FIXME contains LIMIT
func (d *Database) GetAppCredentialByDeveloperAppID(organizationAppID string) ([]types.AppCredential, error) {
	var appcredentials []types.AppCredential

	// FIXME hardcoded row limit
	query := "SELECT * FROM app_credentials WHERE organization_app_id = ? LIMIT 1000"
	appcredentials = d.runGetAppCredentialQuery(query, organizationAppID)
	if len(appcredentials) > 0 {
		d.dbLookupHitsCounter.WithLabelValues(d.Hostname, "app_credentials").Inc()
		return appcredentials, nil
	}
	d.dbLookupMissesCounter.WithLabelValues(d.Hostname, "app_credentials").Inc()
	return appcredentials, errors.New("Could not find apikeys of developer app")
}

//runAppCredentialQuery executes CQL query and returns resulset
//
func (d *Database) runGetAppCredentialQuery(query, queryParameter string) []types.AppCredential {
	var appcredentials []types.AppCredential

	timer := prometheus.NewTimer(d.dbLookupHistogram)
	defer timer.ObserveDuration()

	iterable := d.cassandraSession.Query(query, queryParameter).Iter()
	m := make(map[string]interface{})
	for iterable.MapScan(m) {
		appcredential := types.AppCredential{
			ConsumerKey:       m["key"].(string),
			AppStatus:         m["app_status"].(string),
			Attributes:        m["attributes"].(string),
			CompanyStatus:     m["company_status"].(string),
			ConsumerSecret:    m["consumer_secret"].(string),
			CredentialMethod:  m["credential_method"].(string),
			DeveloperStatus:   m["developer_status"].(string),
			ExpiresAt:         m["expires_at"].(int64),
			IssuedAt:          m["issued_at"].(int64),
			OrganizationAppID: m["organization_app_id"].(string),
			OrganizationName:  m["organization_name"].(string),
			Scopes:            m["scopes"].(string),
			Status:            m["status"].(string),
		}
		if m["api_products"].(string) != "" {
			appcredential.APIProducts = make([]types.APIProductStatus, 0)
			json.Unmarshal([]byte(m["api_products"].(string)), &appcredential.APIProducts)
		}
		appcredentials = append(appcredentials, appcredential)
		m = map[string]interface{}{}
	}
	return appcredentials
}

///////////////////////////////////////////////////////////////////////////////

//GetAPIProductByName returns an array with apiproduct details of a developer app
//
func (d *Database) GetAPIProductByName(apiproductname string) (types.APIProduct, error) {
	query := "SELECT * FROM api_products WHERE name = ? LIMIT 1"
	apiproducts := d.runGetAPIProductQuery(query, apiproductname)
	if len(apiproducts) > 0 {
		d.dbLookupHitsCounter.WithLabelValues(d.Hostname, "app_credentials").Inc()
		return apiproducts[0], nil
	}
	d.dbLookupMissesCounter.WithLabelValues(d.Hostname, "app_credentials").Inc()
	return types.APIProduct{}, errors.New("Could not find apikeys of developer app")
}

// runAPIProductQuery executes CQL query and returns resultset
//
func (d *Database) runGetAPIProductQuery(query, queryParameter string) []types.APIProduct {
	var apiproducts []types.APIProduct

	//Set timer to record how long this function run
	timer := prometheus.NewTimer(d.dbLookupHistogram)
	defer timer.ObserveDuration()

	iterable := d.cassandraSession.Query(query, queryParameter).Iter()
	m := make(map[string]interface{})
	for iterable.MapScan(m) {
		apiproduct := types.APIProduct{
			Key:              m["key"].(string),
			ApprovalType:     m["approval_type"].(string),
			CreatedAt:        m["created_at"].(int64),
			CreatedBy:        m["created_by"].(string),
			Description:      m["description"].(string),
			DisplayName:      m["display_name"].(string),
			Environments:     m["environments"].(string),
			LastmodifiedAt:   m["lastmodified_at"].(int64),
			LastmodifiedBy:   m["lastmodified_by"].(string),
			Name:             m["name"].(string),
			OrganizationName: m["organization_name"].(string),
			Scopes:           m["scopes"].(string),
		}
		apiproduct.APIResources = d.unmarshallJSONArrayOfStrings(m["api_resources"].(string))
		apiproduct.Proxies = d.unmarshallJSONArrayOfStrings(m["proxies"].(string))
		apiproduct.Attributes = d.unmarshallJSONArrayOfAttributes(m["attributes"].(string), true)
		apiproducts = append(apiproducts, apiproduct)
		m = map[string]interface{}{}
	}
	return apiproducts
}
