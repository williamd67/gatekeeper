package service

import (
	"fmt"

	"github.com/erikbos/gatekeeper/pkg/db"
	"github.com/erikbos/gatekeeper/pkg/shared"
	"github.com/erikbos/gatekeeper/pkg/types"
)

// ClusterService is
type ClusterService struct {
	db        *db.Database
	changelog *Changelog
}

// NewCluster returns a new cluster instance
func NewCluster(database *db.Database, c *Changelog) *ClusterService {

	return &ClusterService{db: database, changelog: c}
}

// GetAll returns all clusters
func (cs *ClusterService) GetAll() (clusters types.Clusters, err types.Error) {

	return cs.db.Cluster.GetAll()
}

// Get returns details of an cluster
func (cs *ClusterService) Get(clusterName string) (cluster *types.Cluster, err types.Error) {

	return cs.db.Cluster.Get(clusterName)
}

// GetAttributes returns attributes of an cluster
func (cs *ClusterService) GetAttributes(clusterName string) (attributes *types.Attributes, err types.Error) {

	cluster, err := cs.db.Cluster.Get(clusterName)
	if err != nil {
		return nil, err
	}
	return &cluster.Attributes, nil
}

// GetAttribute returns one particular attribute of an cluster
func (cs *ClusterService) GetAttribute(clusterName, attributeName string) (value string, err types.Error) {

	cluster, err := cs.db.Cluster.Get(clusterName)
	if err != nil {
		return "", err
	}
	return cluster.Attributes.Get(attributeName)
}

// Create creates an cluster
func (cs *ClusterService) Create(newCluster types.Cluster, who Requester) (
	types.Cluster, types.Error) {

	if _, err := cs.db.Cluster.Get(newCluster.Name); err == nil {
		return types.NullCluster, types.NewBadRequestError(
			fmt.Errorf("Cluster '%s' already exists", newCluster.Name))
	}
	// Automatically set default fields
	newCluster.CreatedAt = shared.GetCurrentTimeMilliseconds()
	newCluster.CreatedBy = who.User

	if err := cs.updateCluster(&newCluster, who); err != nil {
		return types.NullCluster, err
	}
	cs.changelog.Create(newCluster, who)
	return newCluster, nil
}

// Update updates an existing cluster
func (cs *ClusterService) Update(updatedCluster types.Cluster,
	who Requester) (types.Cluster, types.Error) {

	currentCluster, err := cs.db.Cluster.Get(updatedCluster.Name)
	if err != nil {
		return types.NullCluster, err
	}
	// Copy over fields we do not allow to be updated
	updatedCluster.Name = currentCluster.Name
	updatedCluster.CreatedAt = currentCluster.CreatedAt
	updatedCluster.CreatedBy = currentCluster.CreatedBy

	if err = cs.updateCluster(&updatedCluster, who); err != nil {
		return types.NullCluster, err
	}
	cs.changelog.Update(currentCluster, updatedCluster, who)
	return updatedCluster, nil
}

// UpdateAttributes updates attributes of an cluster
func (cs *ClusterService) UpdateAttributes(clusterName string,
	receivedAttributes types.Attributes, who Requester) types.Error {

	currentCluster, err := cs.db.Cluster.Get(clusterName)
	if err != nil {
		return err
	}
	updatedCluster := currentCluster
	if err = updatedCluster.Attributes.SetMultiple(receivedAttributes); err != nil {
		return err
	}

	if err = cs.updateCluster(updatedCluster, who); err != nil {
		return err
	}
	cs.changelog.Update(currentCluster, updatedCluster, who)
	return nil
}

// UpdateAttribute update an attribute of developer
func (cs *ClusterService) UpdateAttribute(clusterName string,
	attributeValue types.Attribute, who Requester) types.Error {

	currentCluster, err := cs.db.Cluster.Get(clusterName)
	if err != nil {
		return err
	}
	updatedCluster := currentCluster
	updatedCluster.Attributes.Set(attributeValue)

	if err := cs.updateCluster(updatedCluster, who); err != nil {
		return err
	}
	cs.changelog.Update(currentCluster, updatedCluster, who)
	return nil
}

// DeleteAttribute removes an attribute of an cluster and return its former value
func (cs *ClusterService) DeleteAttribute(clusterName, attributeToDelete string,
	who Requester) (string, types.Error) {

	currentCluster, err := cs.db.Cluster.Get(clusterName)
	if err != nil {
		return "", err
	}
	updatedCluster := currentCluster
	oldValue, err := updatedCluster.Attributes.Delete(attributeToDelete)
	if err != nil {
		return "", err
	}

	if err := cs.updateCluster(updatedCluster, who); err != nil {
		return "", err
	}
	cs.changelog.Update(currentCluster, updatedCluster, who)
	return oldValue, nil
}

// updateCluster updates last-modified field(s) and updates cluster in database
func (cs *ClusterService) updateCluster(updatedCluster *types.Cluster, who Requester) types.Error {

	updatedCluster.Attributes.Tidy()
	updatedCluster.LastmodifiedAt = shared.GetCurrentTimeMilliseconds()
	updatedCluster.LastmodifiedBy = who.User
	return cs.db.Cluster.Update(updatedCluster)
}

// Delete deletes an cluster
func (cs *ClusterService) Delete(clusterName string, who Requester) (
	deletedCluster types.Cluster, e types.Error) {

	cluster, err := cs.db.Cluster.Get(clusterName)
	if err != nil {
		return types.NullCluster, err
	}
	if err := cs.db.Cluster.Delete(clusterName); err != nil {
		return types.NullCluster, err
	}
	cs.changelog.Delete(cluster, who)
	return *cluster, nil
}
