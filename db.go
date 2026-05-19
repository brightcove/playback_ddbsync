package ddbsync

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	ErrLocked = errors.New("key is locked")
)

type Database struct {
	client    AWSDynamoer
	tableName string
}

func NewDatabase(tableName string, region string, endpoint string, disableSSL bool) *Database {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		panic(err)
	}
	var opts []func(*dynamodb.Options)
	if endpoint != "" {
		normalized := normalizeEndpoint(endpoint, disableSSL)
		opts = append(opts, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(normalized)
		})
	}
	return &Database{
		client:    dynamodb.NewFromConfig(cfg, opts...),
		tableName: tableName,
	}
}

// normalizeEndpoint ensures the endpoint has a scheme, forcing http:// when
// disableSSL is set (v2 has no DisableSSL flag — scheme is the only knob).
func normalizeEndpoint(endpoint string, disableSSL bool) string {
	hasHTTP := strings.HasPrefix(endpoint, "http://")
	hasHTTPS := strings.HasPrefix(endpoint, "https://")
	if disableSSL {
		if hasHTTPS {
			return "http://" + strings.TrimPrefix(endpoint, "https://")
		}
		if !hasHTTP {
			return "http://" + endpoint
		}
		return endpoint
	}
	if !hasHTTP && !hasHTTPS {
		return "https://" + endpoint
	}
	return endpoint
}

var _ DBer = (*Database)(nil) // Forces compile time checking of the interface

var _ AWSDynamoer = (*dynamodb.Client)(nil) // Forces compile time checking of the interface

type DBer interface {
	Acquire(string, time.Duration) error
	Delete(string) error
}

type AWSDynamoer interface {
	UpdateItem(ctx context.Context, input *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	DeleteItem(ctx context.Context, input *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
}

func (db *Database) Acquire(name string, ttl time.Duration) error {
	now := time.Now()
	_, err := db.client.UpdateItem(context.Background(), &dynamodb.UpdateItemInput{
		TableName: aws.String(db.tableName),
		Key:       key(name),
		ExpressionAttributeNames: map[string]string{
			"#N": "Name",
			"#C": "Created",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now":    dynamoTime(now),
			":cutoff": dynamoTime(now.Add(-ttl)),
		},
		ConditionExpression: aws.String(`attribute_not_exists(#N) OR #C < :cutoff`),
		UpdateExpression:    aws.String(`SET #C = :now`),
	})
	var ccfe *types.ConditionalCheckFailedException
	if errors.As(err, &ccfe) {
		return ErrLocked
	}
	return err
}

func (db *Database) Delete(name string) error {
	_, err := db.client.DeleteItem(context.Background(), &dynamodb.DeleteItemInput{
		TableName: aws.String(db.tableName),
		Key:       key(name),
	})
	return err
}

func dynamoTime(t time.Time) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.FormatInt(t.UnixMilli(), 10)}
}

func key(name string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"Name": &types.AttributeValueMemberS{Value: name},
	}
}
