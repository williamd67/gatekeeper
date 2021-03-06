package cache

import (
	"bytes"
	"encoding/gob"

	"go.uber.org/zap"

	"github.com/erikbos/gatekeeper/pkg/types"
)

// fetchEntity fetches an named entity from cache, or from the database
// using the provided fuction.
func (c *Cache) fetchEntity(entityType, itemName string, entity interface{},
	dataRetrieveFunction func() (interface{}, types.Error)) types.Error {

	if c == nil || c.freecache == nil {
		return types.NewDatabaseError(nil)
	}

	// Get cachekey based upon object type & name of item we retrieve
	cacheKey := getCacheKeyAndType(entityType, itemName)
	c.logger.Debug("fetchEntry", zap.String("cachekey", string(cacheKey)))

	// Do we have a cache entry?
	if cachedData, err := c.freecache.Get(cacheKey); err == nil && cachedData != nil {
		// If yes, let's try to decode it
		if err = decode(cachedData, entity); err != nil {
			c.logger.Error("cache decode failed", zap.Error(err))
			return types.NewDatabaseError(err)
		}
		c.metrics.EntityCacheHit(entityType)
		return nil
	}

	// No entry in cache miss
	c.metrics.EntityCacheMiss(entityType)
	// Try to retrieve requested entity from database layer
	data, err := dataRetrieveFunction()
	if err != nil {
		// We could not find entity in db, or error occured
		return err
	}
	encodedData, e := encode(data)
	if e != nil {
		c.logger.Error("cache encoding failed", zap.Error(err))
		return types.NewDatabaseError(e)
	}
	// Store in cache
	if err := c.freecache.Set(cacheKey, encodedData, c.config.TTL); err != nil {
		c.logger.Error("cache store failed", zap.Error(err))
	}
	// We decode the encoded data back into native type(!)
	// We do this do provide the retrieve database back to the calling function
	_ = decode(encodedData, entity)
	return nil
}

// deleteEntry removes an entry from cache
func (c *Cache) deleteEntry(entityType, itemName string) {

	// Get cachekey based upon object type and item's name to delete
	cachekey := getCacheKeyAndType(entityType, itemName)

	_ = c.freecache.Del(cachekey)
}

// decode turns a cached entity back into a native object
func decode(encodedData []byte, data interface{}) error {

	return gob.NewDecoder(bytes.NewBuffer(encodedData)).Decode(data)
}

// decode encodes a native object into a byte stream so it can be stored in cache
func encode(data interface{}) ([]byte, error) {

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// getCacheKeyAndType builds cachekey for an entity
// the cachekey is used to uniquely identify an entity in the cache
func getCacheKeyAndType(entityType, itemName string) (cacheKey []byte) {

	// We use the name of the type as prefix for the cachekey.
	// This is done to prevent cache key collisions for similar
	// named entities of different types.
	return []byte(entityType + "%" + itemName)
}
