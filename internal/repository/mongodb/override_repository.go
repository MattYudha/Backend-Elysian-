package mongodb

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	db "github.com/Elysian-Rebirth/backend-go/internal/infrastructure/database"
)

type overrideRepository struct {
	mongoClient *db.MongoClient
	mockStore   map[string]*domain.AuditorOverride
	mockMutex   sync.RWMutex
}

func NewOverrideRepository(mongoClient *db.MongoClient) *overrideRepository {
	return &overrideRepository{
		mongoClient: mongoClient,
		mockStore:   make(map[string]*domain.AuditorOverride),
	}
}

func (r *overrideRepository) collection() *mongo.Collection {
	if r.mongoClient.IsMock() {
		return nil
	}
	client := r.mongoClient.GetClient()
	dbName := r.mongoClient.GetDBName()
	if client == nil {
		return nil
	}
	return client.Database(dbName).Collection("auditor_overrides")
}

func (r *overrideRepository) Save(ctx context.Context, override *domain.AuditorOverride) error {
	if override.ID == "" {
		return errors.New("override ID cannot be empty")
	}
	override.Timestamp = time.Now().UTC()

	if r.mongoClient.IsMock() {
		r.mockMutex.Lock()
		defer r.mockMutex.Unlock()
		copyOverride := *override
		r.mockStore[override.TenantID+":"+override.ItemName] = &copyOverride
		return nil
	}

	coll := r.collection()
	if coll == nil {
		return errors.New("mongodb client is not initialized")
	}

	opts := options.Update().SetUpsert(true)
	filter := bson.M{"tenant_id": override.TenantID, "item_name": override.ItemName}
	update := bson.M{"$set": override}
	_, err := coll.UpdateOne(ctx, filter, update, opts)
	return err
}

func (r *overrideRepository) GetByItemName(ctx context.Context, tenantID, itemName string) (*domain.AuditorOverride, error) {
	if r.mongoClient.IsMock() {
		r.mockMutex.RLock()
		defer r.mockMutex.RUnlock()
		key := tenantID + ":" + itemName
		override, exists := r.mockStore[key]
		if !exists {
			return nil, mongo.ErrNoDocuments
		}
		copyOverride := *override
		return &copyOverride, nil
	}

	coll := r.collection()
	if coll == nil {
		return nil, errors.New("mongodb client is not initialized")
	}

	var override domain.AuditorOverride
	filter := bson.M{"tenant_id": tenantID, "item_name": itemName}
	err := coll.FindOne(ctx, filter).Decode(&override)
	if err != nil {
		return nil, err
	}
	return &override, nil
}

func (r *overrideRepository) List(ctx context.Context, tenantID string) ([]*domain.AuditorOverride, error) {
	if r.mongoClient.IsMock() {
		r.mockMutex.RLock()
		defer r.mockMutex.RUnlock()
		var results []*domain.AuditorOverride
		for _, o := range r.mockStore {
			if o.TenantID == tenantID {
				copyOverride := *o
				results = append(results, &copyOverride)
			}
		}
		return results, nil
	}

	coll := r.collection()
	if coll == nil {
		return nil, errors.New("mongodb client is not initialized")
	}

	var results []*domain.AuditorOverride
	filter := bson.M{"tenant_id": tenantID}
	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var o domain.AuditorOverride
		if err := cursor.Decode(&o); err != nil {
			return nil, err
		}
		results = append(results, &o)
	}
	return results, nil
}
