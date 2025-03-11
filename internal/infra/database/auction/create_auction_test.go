package auction_test

import (
	"context"
	"fmt"
	"fullcycle-auction_go/internal/infra/database/auction"
	"os"
	"testing"
	"time"

	"fullcycle-auction_go/internal/entity/auction_entity"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
}

func TestCreateAuction_AutomaticClosure(t *testing.T) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "mongo:latest",
		ExposedPorts: []string{"27017/tcp"},
		WaitingFor:   wait.ForListeningPort("27017/tcp"),
	}
	mongoC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatal(err)
	}

	assert.NoError(t, err, "error starting MongoDB container")

	host, err := mongoC.Host(ctx)
	assert.NoError(t, err, "error retrieving container host")

	mappedPort, err := mongoC.MappedPort(ctx, "27017/tcp")
	assert.NoError(t, err, "error retrieving mapped container port")

	uri := fmt.Sprintf("mongodb://%s:%s", host, mappedPort.Port())

	os.Setenv("AUCTION_INTERVAL", "1s")

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	assert.NoError(t, err, "error connecting to test MongoDB")

	db := client.Database("auction_test_db")

	err = db.Collection("auctions").Drop(ctx)
	assert.NoError(t, err, "error clearing collection before test")

	repo := auction.NewAuctionRepository(db)

	auctionEntity := &auction_entity.Auction{
		Id:          "auction_123",
		ProductName: "Test Product",
		Category:    "General",
		Description: "Test description",
		Condition:   auction_entity.New,
		Status:      auction_entity.Active,
		Timestamp:   time.Now(),
	}

	internalErr := repo.CreateAuction(context.Background(), auctionEntity)
	assert.Nil(t, internalErr, "error creating auction")

	var inserted map[string]interface{}
	err = db.Collection("auctions").FindOne(
		ctx,
		bson.M{"_id": "auction_123"},
	).Decode(&inserted)
	assert.NoError(t, err)
	assert.EqualValues(t, auction_entity.Active, inserted["status"])

	time.Sleep(2 * time.Second)

	var updated map[string]interface{}
	err = db.Collection("auctions").FindOne(
		ctx,
		bson.M{"_id": "auction_123"},
	).Decode(&updated)
	assert.NoError(t, err)
	assert.EqualValues(t, auction_entity.Completed, updated["status"])

	err = mongoC.Terminate(ctx)
	assert.NoError(t, err, "error shutting down Mongo container")

	err = client.Disconnect(ctx)
	assert.NoError(t, err, "error disconnecting from Mongo")
}
