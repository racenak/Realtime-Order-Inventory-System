package order_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/adapter/postgres"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/domain"
	"github.com/racenak/Realtime-Order-Inventory-System/tests/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	testDB = helpers.SetupTestDB(&testing.T{})
	m.Run()
}

func setupTest(t *testing.T) {
	helpers.CleanDatabase(t, testDB)
}

func TestOrderRepository_Create(t *testing.T) {
	setupTest(t)
	repo := postgres.NewOrderRepository(testDB)

	order := &domain.Order{
		ID:          uuid.New().String(),
		CustomerID:  uuid.New().String(),
		Status:      domain.StatusPendingPayment,
		Currency:    "USD",
		Subtotal:    100.00,
		TotalAmount: 100.00,
		ShippingAddress: domain.Address{
			Street:  "123 Main St",
			City:    "New York",
			State:   "NY",
			Zip:     "10001",
			Country: "US",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := repo.Create(context.Background(), order)
	require.NoError(t, err)

	fetched, err := repo.GetByID(context.Background(), order.ID)
	require.NoError(t, err)
	assert.Equal(t, order.ID, fetched.ID)
	assert.Equal(t, order.CustomerID, fetched.CustomerID)
	assert.Equal(t, order.Status, fetched.Status)
	assert.Equal(t, order.TotalAmount, fetched.TotalAmount)
}

func TestOrderRepository_GetByID_NotFound(t *testing.T) {
	setupTest(t)
	repo := postgres.NewOrderRepository(testDB)

	_, err := repo.GetByID(context.Background(), uuid.New().String())
	assert.ErrorIs(t, err, domain.ErrOrderNotFound)
}

func TestOrderRepository_List(t *testing.T) {
	setupTest(t)
	repo := postgres.NewOrderRepository(testDB)

	customerID := uuid.New().String()

	for i := 0; i < 3; i++ {
		order := &domain.Order{
			ID:          uuid.New().String(),
			CustomerID:  customerID,
			Status:      domain.StatusPendingPayment,
			Currency:    "USD",
			TotalAmount: 100.00,
			ShippingAddress: domain.Address{
				Street: "123 Main St",
				City:   "New York",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := repo.Create(context.Background(), order)
		require.NoError(t, err)
	}

	orders, total, err := repo.List(context.Background(), customerID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, orders, 3)
}

func TestOrderRepository_UpdateStatus(t *testing.T) {
	setupTest(t)
	repo := postgres.NewOrderRepository(testDB)

	order := &domain.Order{
		ID:          uuid.New().String(),
		CustomerID:  uuid.New().String(),
		Status:      domain.StatusPendingPayment,
		Currency:    "USD",
		TotalAmount: 100.00,
		ShippingAddress: domain.Address{
			Street: "123 Main St",
			City:   "New York",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := repo.Create(context.Background(), order)
	require.NoError(t, err)

	err = repo.UpdateStatus(context.Background(), order.ID, domain.StatusPaid)
	require.NoError(t, err)

	fetched, err := repo.GetByID(context.Background(), order.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPaid, fetched.Status)
}

func TestOrderItemRepository_Create(t *testing.T) {
	setupTest(t)
	orderRepo := postgres.NewOrderRepository(testDB)
	itemRepo := postgres.NewOrderItemRepository(testDB)

	order := &domain.Order{
		ID:          uuid.New().String(),
		CustomerID:  uuid.New().String(),
		Status:      domain.StatusPendingPayment,
		Currency:    "USD",
		TotalAmount: 100.00,
		ShippingAddress: domain.Address{
			Street: "123 Main St",
			City:   "New York",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := orderRepo.Create(context.Background(), order)
	require.NoError(t, err)

	items := []domain.OrderItem{
		{
			ID:          uuid.New().String(),
			OrderID:     order.ID,
			ProductID:   uuid.New().String(),
			SKU:         "SKU-001",
			ProductName: "Test Product",
			Quantity:    2,
			UnitPrice:   50.00,
			TotalPrice:  100.00,
		},
	}

	err = itemRepo.Create(context.Background(), items)
	require.NoError(t, err)

	fetched, err := itemRepo.GetByOrderID(context.Background(), order.ID)
	require.NoError(t, err)
	assert.Len(t, fetched, 1)
	assert.Equal(t, "SKU-001", fetched[0].SKU)
}

func TestOutboxRepository_Create(t *testing.T) {
	setupTest(t)
	repo := postgres.NewOutboxRepository(testDB)

	event := domain.OutboxEvent{
		ID:            uuid.New().String(),
		AggregateType: "order",
		AggregateID:   uuid.New().String(),
		EventType:     "order.created",
		Payload:       `{"order_id":"test"}`,
		Status:        "PENDING",
		CreatedAt:     time.Now().Format(time.RFC3339),
	}

	err := repo.Create(context.Background(), event)
	require.NoError(t, err)

	events, err := repo.GetPending(context.Background(), 10)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "order.created", events[0].EventType)
}

func TestOutboxRepository_MarkPublished(t *testing.T) {
	setupTest(t)
	repo := postgres.NewOutboxRepository(testDB)

	event := domain.OutboxEvent{
		ID:            uuid.New().String(),
		AggregateType: "order",
		AggregateID:   uuid.New().String(),
		EventType:     "order.created",
		Payload:       `{"order_id":"test"}`,
		Status:        "PENDING",
		CreatedAt:     time.Now().Format(time.RFC3339),
	}

	err := repo.Create(context.Background(), event)
	require.NoError(t, err)

	err = repo.MarkPublished(context.Background(), event.ID)
	require.NoError(t, err)

	events, err := repo.GetPending(context.Background(), 10)
	require.NoError(t, err)
	assert.Len(t, events, 0)
}

func TestOrderRepository_CreateWithJSONAddress(t *testing.T) {
	setupTest(t)
	repo := postgres.NewOrderRepository(testDB)

	addr := domain.Address{
		Street:  "456 Oak Ave",
		City:    "Los Angeles",
		State:   "CA",
		Zip:     "90001",
		Country: "US",
	}

	order := &domain.Order{
		ID:              uuid.New().String(),
		CustomerID:      uuid.New().String(),
		Status:          domain.StatusPendingPayment,
		Currency:        "USD",
		TotalAmount:     250.50,
		ShippingAddress: addr,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	err := repo.Create(context.Background(), order)
	require.NoError(t, err)

	fetched, err := repo.GetByID(context.Background(), order.ID)
	require.NoError(t, err)
	assert.Equal(t, addr.Street, fetched.ShippingAddress.Street)
	assert.Equal(t, addr.City, fetched.ShippingAddress.City)
}

func TestOrderRepository_ConcurrentAccess(t *testing.T) {
	setupTest(t)
	repo := postgres.NewOrderRepository(testDB)

	order := &domain.Order{
		ID:          uuid.New().String(),
		CustomerID:  uuid.New().String(),
		Status:      domain.StatusPendingPayment,
		Currency:    "USD",
		TotalAmount: 100.00,
		ShippingAddress: domain.Address{
			Street: "123 Main St",
			City:   "New York",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := repo.Create(context.Background(), order)
	require.NoError(t, err)

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			_ = repo.UpdateStatus(context.Background(), order.ID, domain.StatusPaid)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	fetched, err := repo.GetByID(context.Background(), order.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPaid, fetched.Status)
}

func TestOrderRepository_JSONAddressMarshal(t *testing.T) {
	addr := domain.Address{
		Street:  "789 Pine Rd",
		City:    "Chicago",
		State:   "IL",
		Zip:     "60601",
		Country: "US",
	}

	data, err := json.Marshal(addr)
	require.NoError(t, err)

	var decoded domain.Address
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, addr.Street, decoded.Street)
	assert.Equal(t, addr.City, decoded.City)
}

func TestOutboxRepository_ClaimBatch(t *testing.T) {
	setupTest(t)
	repo := postgres.NewOutboxRepository(testDB)

	// Create 5 pending events
	for i := 0; i < 5; i++ {
		err := repo.Create(context.Background(), domain.OutboxEvent{
			ID:            uuid.New().String(),
			AggregateType: "order",
			AggregateID:   uuid.New().String(),
			EventType:     "order.created",
			Payload:       `{"test": true}`,
			Status:        "PENDING",
			CreatedAt:     time.Now().Format(time.RFC3339),
		})
		require.NoError(t, err)
	}

	// Claim batch of 3
	claimed, err := repo.ClaimBatch(context.Background(), 3)
	require.NoError(t, err)
	assert.Len(t, claimed, 3)

	// Verify claimed events have CLAIMED status
	for _, e := range claimed {
		events, err := repo.GetPending(context.Background(), 100)
		require.NoError(t, err)
		for _, pe := range events {
			assert.NotEqual(t, e.ID, pe.ID, "claimed event should not appear in pending")
		}
	}

	// Claim remaining 2
	remaining, err := repo.ClaimBatch(context.Background(), 10)
	require.NoError(t, err)
	assert.Len(t, remaining, 2)

	// No more pending
	none, err := repo.ClaimBatch(context.Background(), 10)
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestOutboxRepository_MarkFailed(t *testing.T) {
	setupTest(t)
	repo := postgres.NewOutboxRepository(testDB)

	eventID := uuid.New().String()
	err := repo.Create(context.Background(), domain.OutboxEvent{
		ID:            eventID,
		AggregateType: "order",
		AggregateID:   uuid.New().String(),
		EventType:     "order.created",
		Payload:       `{"test": true}`,
		Status:        "PENDING",
		CreatedAt:     time.Now().Format(time.RFC3339),
	})
	require.NoError(t, err)

	err = repo.MarkFailed(context.Background(), eventID)
	require.NoError(t, err)

	// Should not appear in pending anymore
	pending, err := repo.GetPending(context.Background(), 100)
	require.NoError(t, err)
	for _, e := range pending {
		assert.NotEqual(t, eventID, e.ID)
	}
}

func TestOrderRepository_GetByIDempotencyKey(t *testing.T) {
	setupTest(t)
	repo := postgres.NewOrderRepository(testDB)

	order := &domain.Order{
		ID:             uuid.New().String(),
		CustomerID:     uuid.New().String(),
		Status:         domain.StatusPendingPayment,
		Currency:       "USD",
		TotalAmount:    50.00,
		IdempotencyKey: "idem-test-123",
		ShippingAddress: domain.Address{
			Street: "123 Main St",
			City:   "New York",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := repo.Create(context.Background(), order)
	require.NoError(t, err)

	// Find by idempotency key
	found, err := repo.GetByIDempotencyKey(context.Background(), "idem-test-123")
	require.NoError(t, err)
	assert.Equal(t, order.ID, found.ID)

	// Not found
	_, err = repo.GetByIDempotencyKey(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, domain.ErrOrderNotFound)
}
