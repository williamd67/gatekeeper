package cassandra

import (
	"fmt"

	"github.com/gocql/gocql"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/erikbos/gatekeeper/pkg/types"
)

const (
	// Prometheus label for metrics of db interactions
	apiProductsMetricLabel = "apiproducts"

	// List of apiproduct columns we use
	apiProductsColumns = `name,
display_name,
description,
attributes,
route_group,
paths,
policies,
created_at,
created_by,
lastmodified_at,
lastmodified_by`
)

// APIProductStore holds our database config
type APIProductStore struct {
	db *Database
}

// NewAPIProductStore creates api product instance
func NewAPIProductStore(database *Database) *APIProductStore {
	return &APIProductStore{
		db: database,
	}
}

// GetAll retrieves all api products
func (s *APIProductStore) GetAll() (types.APIProducts, types.Error) {

	query := "SELECT " + apiProductsColumns + " FROM api_products"

	apiproducts, err := s.runGetAPIProductQuery(query)
	if err != nil {
		s.db.metrics.QueryFailed(apiProductsMetricLabel)
		return types.APIProducts{}, types.NewDatabaseError(err)
	}

	s.db.metrics.QueryHit(apiProductsMetricLabel)
	return apiproducts, nil
}

// Get returns an apiproduct
func (s *APIProductStore) Get(apiproductName string) (*types.APIProduct, types.Error) {

	query := "SELECT " + apiProductsColumns + " FROM api_products WHERE name = ? LIMIT 1"

	apiproducts, err := s.runGetAPIProductQuery(query, apiproductName)
	if err != nil {
		s.db.metrics.QueryFailed(apiProductsMetricLabel)
		return nil, types.NewDatabaseError(err)
	}

	if len(apiproducts) == 0 {
		s.db.metrics.QueryMiss(apiProductsMetricLabel)
		return nil, types.NewItemNotFoundError(
			fmt.Errorf("Cannot find apiproduct '%s'", apiproductName))
	}

	s.db.metrics.QueryHit(apiProductsMetricLabel)
	return &apiproducts[0], nil
}

// runGetAPIProductQuery executes CQL query and returns resultset
func (s *APIProductStore) runGetAPIProductQuery(query string, queryParameters ...interface{}) (types.APIProducts, error) {
	var apiproducts types.APIProducts

	timer := prometheus.NewTimer(s.db.metrics.LookupHistogram)
	defer timer.ObserveDuration()

	var iter *gocql.Iter
	if queryParameters == nil {
		iter = s.db.CassandraSession.Query(query).Iter()
	} else {
		iter = s.db.CassandraSession.Query(query, queryParameters...).Iter()
	}
	if iter.NumRows() == 0 {
		_ = iter.Close()
		return types.APIProducts{}, nil
	}

	m := make(map[string]interface{})
	for iter.MapScan(m) {
		apiproducts = append(apiproducts, types.APIProduct{
			Name:           columnValueString(m, "name"),
			DisplayName:    columnValueString(m, "display_name"),
			Description:    m["description"].(string),
			Attributes:     types.APIProduct{}.Attributes.Unmarshal(columnValueString(m, "attributes")),
			RouteGroup:     m["route_group"].(string),
			Paths:          types.APIProduct{}.Paths.Unmarshal(columnValueString(m, "paths")),
			Policies:       m["policies"].(string),
			CreatedAt:      columnValueInt64(m, "created_at"),
			CreatedBy:      columnValueString(m, "created_by"),
			LastmodifiedAt: columnValueInt64(m, "lastmodified_at"),
			LastmodifiedBy: columnValueString(m, "lastmodified_by"),
		})
		m = map[string]interface{}{}
	}

	if err := iter.Close(); err != nil {
		return types.APIProducts{}, err
	}

	return apiproducts, nil
}

// Update UPSERTs an apiproduct in database
func (s *APIProductStore) Update(p *types.APIProduct) types.Error {

	query := "INSERT INTO api_products (" + apiProductsColumns + ") VALUES(?,?,?,?,?,?,?,?,?,?,?)"
	if err := s.db.CassandraSession.Query(query,
		p.Name,
		p.DisplayName,
		p.Description,
		p.Attributes.Marshal(),
		p.RouteGroup,
		p.Paths.Marshal(),
		p.Policies,
		p.CreatedAt,
		p.CreatedBy,
		p.LastmodifiedAt,
		p.LastmodifiedBy).Exec(); err != nil {

		s.db.metrics.QueryFailed(apiProductsMetricLabel)
		return types.NewDatabaseError(
			fmt.Errorf("Cannot update apiproduct '%s'", p.Name))
	}
	return nil
}

// Delete deletes an apiproduct
func (s *APIProductStore) Delete(apiProduct string) types.Error {

	apiproduct, err := s.Get(apiProduct)
	if err != nil {
		s.db.metrics.QueryFailed(apiProductsMetricLabel)
		return err
	}

	query := "DELETE FROM api_products WHERE name = ?"
	if err := s.db.CassandraSession.Query(query, apiproduct.Name).Exec(); err != nil {
		s.db.metrics.QueryFailed(apiProductsMetricLabel)
		return types.NewDatabaseError(err)
	}
	return nil
}
