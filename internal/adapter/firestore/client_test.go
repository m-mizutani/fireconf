package firestore_test

import (
	"context"
	"errors"
	"testing"

	adminpb "cloud.google.com/go/firestore/apiv1/admin/adminpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/m-mizutani/gt"
)

// mockCreateIndexOperation is a mock implementation of CreateIndexOperation for testing
type mockCreateIndexOperation struct {
	done       bool
	pollError  error
	pollResult *adminpb.Index
	pollCount  int
	waitError  error
	waitResult *adminpb.Index
	doneCalls  int
	shouldFail bool
	failAtPoll int // Fail at specific poll count (0 means never fail via poll)
}

func (m *mockCreateIndexOperation) Done() bool {
	m.doneCalls++
	return m.done
}

func (m *mockCreateIndexOperation) Poll(ctx context.Context, opts ...gax.CallOption) (*adminpb.Index, error) {
	m.pollCount++

	// Simulate operation failure at specific poll count
	if m.failAtPoll > 0 && m.pollCount >= m.failAtPoll {
		m.done = true
		return nil, m.pollError
	}

	// Normal case: operation completes successfully after a few polls
	if m.pollCount >= 3 && !m.shouldFail {
		m.done = true
		return m.pollResult, nil
	}

	// Still in progress
	return nil, nil
}

func (m *mockCreateIndexOperation) Wait(ctx context.Context, opts ...gax.CallOption) (*adminpb.Index, error) {
	return m.waitResult, m.waitError
}

func (m *mockCreateIndexOperation) Metadata() (*adminpb.IndexOperationMetadata, error) {
	return nil, nil
}

func (m *mockCreateIndexOperation) Name() string {
	return "test-operation"
}

// Test mock operation behavior
func TestMockOperationBehavior(t *testing.T) {
	t.Run("Operation completes successfully after polling", func(t *testing.T) {
		mockOp := &mockCreateIndexOperation{
			pollResult: &adminpb.Index{
				Name: "test-index",
			},
		}

		ctx := context.Background()

		// First poll - in progress
		result, err := mockOp.Poll(ctx)
		gt.V(t, result).Nil()
		gt.NoError(t, err)
		gt.Equal(t, mockOp.Done(), false)

		// Second poll - in progress
		result, err = mockOp.Poll(ctx)
		gt.V(t, result).Nil()
		gt.NoError(t, err)
		gt.Equal(t, mockOp.Done(), false)

		// Third poll - completes
		result, err = mockOp.Poll(ctx)
		gt.V(t, result).NotNil()
		gt.NoError(t, err)
		gt.Equal(t, mockOp.Done(), true)
		gt.Equal(t, result.Name, "test-index")
	})

	t.Run("Operation fails during polling", func(t *testing.T) {
		// Test that errors from Poll() are properly handled when operation is done
		mockOp := &mockCreateIndexOperation{
			shouldFail: true,
			failAtPoll: 2,
			pollError:  errors.New("index creation failed: invalid configuration"),
		}

		// Verify the mock behaves as expected
		ctx := context.Background()

		// First poll - in progress
		result, err := mockOp.Poll(ctx)
		gt.V(t, result).Nil()
		gt.NoError(t, err)
		gt.Equal(t, mockOp.Done(), false)

		// Second poll - fails and completes
		result, err = mockOp.Poll(ctx)
		gt.V(t, result).Nil()
		gt.Error(t, err)
		gt.Equal(t, mockOp.Done(), true)
		gt.Equal(t, err.Error(), "index creation failed: invalid configuration")
	})
}
