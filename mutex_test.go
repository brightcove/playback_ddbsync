package ddbsync

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	ValidMutexName      = "mut-test"
	ValidMutexTTL       = 4 * time.Second
	ValidMutexCreated   = 1424385592
	ValidMutexRetryWait = 1 * time.Millisecond
)

var dynamoInternalErr = &types.InternalServerError{
	Message: aws.String("Dynamo Internal Server Error"),
}

type mockDB struct{ mock.Mock }

func (m *mockDB) OnAcquire() *mock.Call {
	return m.On("Acquire", ValidMutexName, ValidMutexTTL)
}
func (m *mockDB) OnDelete() *mock.Call {
	return m.On("Delete", ValidMutexName)
}
func (m *mockDB) Acquire(name string, ttl time.Duration) error {
	return m.Called(name, ttl).Error(0)
}
func (m *mockDB) Delete(name string) error {
	return m.Called(name).Error(0)
}

func newMockedMutex() (*Mutex, *mockDB) {
	db := &mockDB{}
	mutex := NewMutex(ValidMutexName, ValidMutexTTL, db)
	mutex.ReattemptWait = ValidMutexRetryWait
	return mutex, db
}

func TestNew(t *testing.T) {
	t.Parallel()
	underTest, _ := newMockedMutex()
	require.Equal(t, ValidMutexName, underTest.Name)
	require.Equal(t, ValidMutexTTL, underTest.TTL)
}

func TestLock(t *testing.T) {
	t.Parallel()
	underTest, db := newMockedMutex()
	defer db.AssertExpectations(t)

	db.OnAcquire().Return(nil)

	require.NoError(t, underTest.Lock())
}

func TestLockWaitsBeforeRetrying(t *testing.T) {
	t.Parallel()
	underTest, db := newMockedMutex()
	defer db.AssertExpectations(t)
	underTest.ReattemptWait = 300 * time.Millisecond

	db.OnAcquire().Once().Return(ErrLocked)
	db.OnAcquire().Once().Return(dynamoInternalErr)
	db.OnAcquire().Once().Return(errors.New("dynamodb glitch"))
	db.OnAcquire().Once().Return(nil)

	before := time.Now()
	require.NoError(t, underTest.Lock())
	duration := time.Since(before)

	require.True(t, duration > 900*time.Millisecond, "Expected to have waited at least 0.3 secs between each retry, total wait time: %s", duration)
}

func TestLockCutoff(t *testing.T) {
	t.Parallel()
	underTest, db := newMockedMutex()
	defer db.AssertExpectations(t)
	underTest.ReattemptWait = 300 * time.Millisecond
	underTest.Cutoff = 100 * time.Millisecond

	db.OnAcquire().Twice().Return(ErrLocked)

	before := time.Now()
	err := underTest.Lock()
	duration := time.Since(before)

	require.EqualError(t, err, "reached cutoff time")
	require.True(t, duration > 300*time.Millisecond, "Expected to have waited at least 0.3 secs between each retry, total wait time: %s", duration)
}

func TestUnlock(t *testing.T) {
	t.Parallel()
	underTest, db := newMockedMutex()
	defer db.AssertExpectations(t)

	db.OnDelete().Return(nil)

	underTest.Unlock()
}

func TestUnlockGivesUpAfterThreeAttempts(t *testing.T) {
	t.Parallel()
	underTest, db := newMockedMutex()
	defer db.AssertExpectations(t)

	db.OnDelete().Times(3).Return(errors.New("dynamodb is down"))

	underTest.Unlock()
}
