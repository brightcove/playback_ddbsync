package ddbsync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
)

const (
	DBValidTableName = "TestLockTable"
	DBValidName      = "db-name"
	DBValidTTL       = 30 * time.Second
)

type mockDynamo struct {
	UpdateItemOutput *dynamodb.UpdateItemOutput
	UpdateItemError  error
	DeleteItemOutput *dynamodb.DeleteItemOutput
	DeleteItemError  error
}

func (m mockDynamo) UpdateItem(_ context.Context, _ *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	return m.UpdateItemOutput, m.UpdateItemError
}
func (m mockDynamo) DeleteItem(_ context.Context, _ *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	return m.DeleteItemOutput, m.DeleteItemError
}

func newMockedClient(mocked mockDynamo) *Database {
	return &Database{client: mocked, tableName: DBValidTableName}
}

func TestDBPut(t *testing.T) {
	t.Parallel()
	db := newMockedClient(mockDynamo{
		UpdateItemOutput: &dynamodb.UpdateItemOutput{},
	})

	err := db.Acquire(DBValidName, DBValidTTL)
	assert.NoError(t, err)
}

func TestDBPutError(t *testing.T) {
	t.Parallel()
	db := newMockedClient(mockDynamo{
		UpdateItemError: errors.New("UpdateItem Error"),
	})

	err := db.Acquire(DBValidName, DBValidTTL)
	assert.Error(t, err)
}

func TestDBPutErrorLocked(t *testing.T) {
	t.Parallel()
	db := newMockedClient(mockDynamo{
		UpdateItemError: &types.ConditionalCheckFailedException{
			Message: aws.String("condition check failed"),
		},
	})

	err := db.Acquire(DBValidName, DBValidTTL)
	assert.ErrorIs(t, err, ErrLocked)
}

func TestDBDelete(t *testing.T) {
	t.Parallel()
	db := newMockedClient(mockDynamo{
		DeleteItemOutput: &dynamodb.DeleteItemOutput{},
	})

	err := db.Delete(DBValidName)
	assert.NoError(t, err)
}

func TestDBDeleteError(t *testing.T) {
	t.Parallel()
	db := newMockedClient(mockDynamo{
		DeleteItemError: errors.New("dynamodb DeleteItem error"),
	})

	err := db.Delete(DBValidName)
	assert.Error(t, err)
}
