package database

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"github.com/Elysian-Rebirth/backend-go/internal/config"
)

type StagingStatus string

const (
	StatusPendingQA StagingStatus = "PENDING_QA"
	StatusApproved  StagingStatus = "APPROVED"
)

type StagingDocument struct {
	ID         string        `bson:"_id" json:"id"`
	TenantID   string        `bson:"tenant_id" json:"tenant_id"`
	FileName   string        `bson:"file_name" json:"file_name"`
	RawText    string        `bson:"raw_text" json:"raw_text"`
	Status     StagingStatus `bson:"status" json:"status"`
	ApprovedBy string        `bson:"approved_by,omitempty" json:"approved_by,omitempty"`
	CreatedAt  time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt  time.Time     `bson:"updated_at" json:"updated_at"`
}

type MongoClient struct {
	client     *mongo.Client
	dbName     string
	isMock     bool
	mockStore  map[string]*StagingDocument
	mockMutex  sync.RWMutex
}

// NewMongoClient initializes connection to MongoDB or falls back to in-memory store
func NewMongoClient(cfg *config.Config) (*MongoClient, error) {
	dbName := cfg.MongoDB.DB
	if dbName == "" {
		dbName = "elysian_staging"
	}

	uri := cfg.MongoDB.URI
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Attempt real connection
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	if err == nil {
		err = client.Ping(ctx, nil)
	}

	if err != nil {
		log.Printf("⚠️ MongoDB Staging Warning: Could not connect to real MongoDB (URI: %s). Falling back to resilient in-memory Staging Stubs: %v", uri, err)
		return &MongoClient{
			dbName:    dbName,
			isMock:    true,
			mockStore: make(map[string]*StagingDocument),
		}, nil
	}

	log.Printf("MongoDB Staging Area connected successfully (DB: %s)", dbName)
	return &MongoClient{
		client: client,
		dbName: dbName,
		isMock: false,
	}, nil
}

func (m *MongoClient) collection() *mongo.Collection {
	if m.isMock || m.client == nil {
		return nil
	}
	return m.client.Database(m.dbName).Collection("staging_documents")
}

// IsMock returns true if running in in-memory fallback mode
func (m *MongoClient) IsMock() bool {
	return m.isMock
}

// SaveDocument inserts or updates the document in the staging area
func (m *MongoClient) SaveDocument(ctx context.Context, doc *StagingDocument) error {
	if doc.ID == "" {
		return errors.New("document ID cannot be empty")
	}
	doc.UpdatedAt = time.Now().UTC()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now().UTC()
	}
	if doc.Status == "" {
		doc.Status = StatusPendingQA
	}

	if m.isMock {
		m.mockMutex.Lock()
		defer m.mockMutex.Unlock()
		m.mockStore[doc.ID] = doc
		log.Printf("[Mock Mongo] Saved document %s to in-memory staging", doc.ID)
		return nil
	}

	opts := options.Update().SetUpsert(true)
	filter := bson.M{"_id": doc.ID}
	update := bson.M{"$set": doc}
	_, err := m.collection().UpdateOne(ctx, filter, update, opts)
	return err
}

// GetDocument retrieves a document by ID
func (m *MongoClient) GetDocument(ctx context.Context, id string) (*StagingDocument, error) {
	if m.isMock {
		m.mockMutex.RLock()
		defer m.mockMutex.RUnlock()
		doc, exists := m.mockStore[id]
		if !exists {
			return nil, mongo.ErrNoDocuments
		}
		// Return a copy to avoid external mutation race conditions
		copyDoc := *doc
		return &copyDoc, nil
	}

	var doc StagingDocument
	err := m.collection().FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// UpdateText updates the raw text for a staging document
func (m *MongoClient) UpdateText(ctx context.Context, id string, text string) error {
	if m.isMock {
		m.mockMutex.Lock()
		defer m.mockMutex.Unlock()
		doc, exists := m.mockStore[id]
		if !exists {
			return mongo.ErrNoDocuments
		}
		doc.RawText = text
		doc.UpdatedAt = time.Now().UTC()
		log.Printf("[Mock Mongo] Updated raw text for document %s in-memory", id)
		return nil
	}

	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"raw_text":   text,
			"updated_at": time.Now().UTC(),
		},
	}
	res, err := m.collection().UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// ApproveDocument marks the document status as APPROVED and records the approver
func (m *MongoClient) ApproveDocument(ctx context.Context, id string, approvedBy string) error {
	if m.isMock {
		m.mockMutex.Lock()
		defer m.mockMutex.Unlock()
		doc, exists := m.mockStore[id]
		if !exists {
			return mongo.ErrNoDocuments
		}
		doc.Status = StatusApproved
		doc.ApprovedBy = approvedBy
		doc.UpdatedAt = time.Now().UTC()
		log.Printf("[Mock Mongo] Approved document %s by %s in-memory", id, approvedBy)
		return nil
	}

	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":      StatusApproved,
			"approved_by": approvedBy,
			"updated_at":  time.Now().UTC(),
		},
	}
	res, err := m.collection().UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// Close closes the MongoDB client connection
func (m *MongoClient) Close(ctx context.Context) error {
	if m.isMock || m.client == nil {
		return nil
	}
	return m.client.Disconnect(ctx)
}
